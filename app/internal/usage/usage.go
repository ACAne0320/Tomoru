// Package usage is how much translating did, day by day and by the engines that did it, and how
// long its steps took: numbers only, never what was said. The app keeps it in a small file, and the
// settings' statistics page shows it.
package usage

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Counts are how much translating did, and how long it took.
type Counts struct {
	Run    float64 `json:"run"`    // elapsed session/listening seconds, including silence
	Lines  int     `json:"lines"`  // sentences committed
	Speech float64 `json:"speech"` // seconds someone spoke, by the audio clock, as voice detection tells
	Chars  int     `json:"chars"`  // letters of the translations done, spaces left out
	// Audio is the seconds of audio sent to a cloud recognizer, which bills them, silence too.
	Audio float64 `json:"audio,omitempty"`
	// Failed are the sentences whose translation failed.
	Failed int `json:"failed,omitempty"`
	// InTokens and OutTokens are the tokens translating took, drafts too, as the services counted
	// them: those that say.
	InTokens  int `json:"inTokens,omitempty"`
	OutTokens int `json:"outTokens,omitempty"`
	// Images are the pictures translated, a translation got for each.
	Images  int     `json:"images,omitempty"`
	Latency Latency `json:"latency"`
}

// Latency is how long each step took, from what the user hears to the translation they read.
type Latency struct {
	// FirstText is from the speaker's voice starting to the first text recognized of it.
	FirstText Hist `json:"firstText"`
	// Final is from a word's end to the recognizer's final result for it.
	Final Hist `json:"final"`
	// FirstTranslation is from a sentence showing to the first words of its translation, waiting
	// for the sentences before it included; Translation to its translation done.
	FirstTranslation Hist `json:"firstTranslation"`
	Translation      Hist `json:"translation"`
	// Queue is from a sentence showing to its translation starting: waiting for the sentences
	// before it, which FirstTranslation includes.
	Queue Hist `json:"queue"`
	// EndToEnd is from the end of a sentence's speech to the first words of its translation: what
	// the user waits. A sentence cut from volatile text learns its end from the final result after
	// it; one whose final result never comes is not counted.
	EndToEnd Hist `json:"endToEnd"`
}

// Add adds o to c.
func (c *Counts) Add(o Counts) {
	c.Run += o.Run
	c.Lines += o.Lines
	c.Speech += o.Speech
	c.Chars += o.Chars
	c.Audio += o.Audio
	c.Failed += o.Failed
	c.InTokens += o.InTokens
	c.OutTokens += o.OutTokens
	c.Images += o.Images
	l, m := &c.Latency, o.Latency
	l.FirstText.Merge(m.FirstText)
	l.Final.Merge(m.Final)
	l.FirstTranslation.Merge(m.FirstTranslation)
	l.Translation.Merge(m.Translation)
	l.Queue.Merge(m.Queue)
	l.EndToEnd.Merge(m.EndToEnd)
}

// Since is the cumulative change from previous to c within the same session. Every field is
// subtracted directly, including histogram buckets; callers own session boundaries and resets.
func (c Counts) Since(previous Counts) Counts {
	return Counts{
		Run:       c.Run - previous.Run,
		Lines:     c.Lines - previous.Lines,
		Speech:    c.Speech - previous.Speech,
		Chars:     c.Chars - previous.Chars,
		Audio:     c.Audio - previous.Audio,
		Failed:    c.Failed - previous.Failed,
		InTokens:  c.InTokens - previous.InTokens,
		OutTokens: c.OutTokens - previous.OutTokens,
		Images:    c.Images - previous.Images,
		Latency: Latency{
			FirstText:        histSince(c.Latency.FirstText, previous.Latency.FirstText),
			Final:            histSince(c.Latency.Final, previous.Latency.Final),
			FirstTranslation: histSince(c.Latency.FirstTranslation, previous.Latency.FirstTranslation),
			Translation:      histSince(c.Latency.Translation, previous.Latency.Translation),
			Queue:            histSince(c.Latency.Queue, previous.Latency.Queue),
			EndToEnd:         histSince(c.Latency.EndToEnd, previous.Latency.EndToEnd),
		},
	}
}

func histSince(current, previous Hist) Hist {
	current.N -= previous.N
	current.Sum -= previous.Sum
	for i := range current.B {
		current.B[i] -= previous.B[i]
	}
	return current
}

// Bounds are the upper bounds of a Hist's buckets, in milliseconds, but the last bucket's, which
// holds the rest.
var Bounds = [...]float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1200, 1400, 1600, 1800, 2000, 2500, 3000, 4000, 5000, 7000, 10000}

// Hist counts durations by bucket: enough to tell their median and their slow ones without
// keeping each. It is a value: a copy shares nothing.
type Hist struct {
	N   int                  `json:"n"`
	Sum float64              `json:"sum"` // milliseconds
	B   [len(Bounds) + 1]int `json:"b"`
}

// Add counts d.
func (h *Hist) Add(d time.Duration) {
	ms := max(0, float64(d)/float64(time.Millisecond))
	h.N++
	h.Sum += ms
	h.B[sort.SearchFloat64s(Bounds[:], ms)]++
}

// Merge adds o's durations to h's.
func (h *Hist) Merge(o Hist) {
	h.N += o.N
	h.Sum += o.Sum
	for i := range h.B {
		h.B[i] += o.B[i]
	}
}

// Quantile is the duration q of those counted are within (0.5, the median), as the buckets tell
// it, in a straight line through a bucket; ok is false if none were counted. Past the last bound
// it is the last bound.
func (h Hist) Quantile(q float64) (d time.Duration, ok bool) {
	if h.N == 0 {
		return 0, false
	}
	want, seen := q*float64(h.N), 0.0
	for i, n := range h.B {
		if n == 0 || seen+float64(n) < want {
			seen += float64(n)
			continue
		}
		if i == len(Bounds) {
			return ms(Bounds[i-1]), true
		}
		lo := 0.0
		if i > 0 {
			lo = Bounds[i-1]
		}
		return ms(lo + (Bounds[i]-lo)*(want-seen)/float64(n)), true
	}
	return ms(Bounds[len(Bounds)-1]), true
}

func ms(v float64) time.Duration { return time.Duration(v * float64(time.Millisecond)) }

// Engines are the engines translating used: their IDs, as the settings name them (prefs.Prefs's
// ASR and MT), and, for those the user sets up, what they were then.
type Engines struct {
	ASR      string `json:"asr"`
	ASRModel string `json:"asrModel,omitempty"` // the cloud recognizer's model
	MT       string `json:"mt"`
	MTName   string `json:"mtName,omitempty"`  // an endpoint's name
	MTModel  string `json:"mtModel,omitempty"` // an endpoint's model
	// Untranslated is subtitles that showed the original alone: no translator, MT "", and their
	// lines not translated.
	Untranslated bool `json:"untranslated,omitempty"`
}

// Use is what translating did with some engines.
type Use struct {
	Engines
	Counts
}

// Day is what translating did on a day, by the engines it used.
type Day struct {
	Day  string `json:"day"` // the local date, 2006-01-02
	Uses []Use  `json:"uses"`
}

// Total is the day's counts, whatever the engines.
func (d Day) Total() Counts { return d.TotalOf(func(Use) bool { return true }) }

// TotalOf is the day's counts of the uses has keeps.
func (d Day) TotalOf(has func(Use) bool) Counts {
	var c Counts
	for _, u := range d.Uses {
		if has(u) {
			c.Add(u.Counts)
		}
	}
	return c
}

// Ledger is the days translating did something, the earliest first.
type Ledger struct {
	Days []Day `json:"days"`
}

// DayOf is the local date of t, as a ledger keeps it.
func DayOf(t time.Time) string { return t.Format(time.DateOnly) }

// Add adds c, done with e, to day.
func (l *Ledger) Add(day string, e Engines, c Counts) {
	i, found := slices.BinarySearchFunc(l.Days, day, func(d Day, day string) int { return strings.Compare(d.Day, day) })
	if !found {
		l.Days = slices.Insert(l.Days, i, Day{Day: day})
	}
	d := &l.Days[i]
	for j := range d.Uses {
		if d.Uses[j].Engines == e {
			d.Uses[j].Counts.Add(c)
			return
		}
	}
	d.Uses = append(d.Uses, Use{Engines: e, Counts: c})
}

// AddRun divides the elapsed running time across local calendar days in start's location.
// Ending exactly at midnight adds nothing to the following day; empty/reversed intervals do
// nothing. Calendar midnights keep daylight-saving days at their actual elapsed length.
func (l *Ledger) AddRun(start, end time.Time, engines Engines) {
	l.addInterval(start, end, engines, end.Sub(start).Seconds(), func(seconds float64) Counts {
		return Counts{Run: seconds}
	})
}

// AddSpeech attributes seconds of speech to [end-seconds, end], split at local midnight in end's
// location. It keeps the original floating-point total, including sub-nanosecond rounding from
// translating audio seconds to time.Duration.
func (l *Ledger) AddSpeech(end time.Time, seconds float64, engines Engines) {
	if seconds <= 0 {
		return
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration == 0 {
		// A positive interval smaller than time.Time's resolution still belongs to the date
		// immediately before its exclusive endpoint, including an endpoint at midnight.
		l.Add(DayOf(end.Add(-time.Nanosecond)), engines, Counts{Speech: seconds})
		return
	}
	l.addInterval(end.Add(-duration), end, engines, seconds, func(seconds float64) Counts {
		return Counts{Speech: seconds}
	})
}

func (l *Ledger) addInterval(start, end time.Time, engines Engines, remaining float64, counts func(float64) Counts) {
	if !end.After(start) {
		return
	}
	for start.Before(end) {
		next := nextLocalDay(start)
		if end.Before(next) {
			next = end
		}
		seconds := next.Sub(start).Seconds()
		if next.Equal(end) {
			seconds = remaining
		}
		l.Add(DayOf(start), engines, counts(seconds))
		remaining -= seconds
		start = next
	}
}

// nextLocalDay finds the first instant after start with a different local date. Most days
// start at 00:00, but some zones advance or repeat the clock at midnight itself; time.Date
// alone can then select the previous date or the second occurrence of midnight.
func nextLocalDay(start time.Time) time.Time {
	year, month, day := start.Date()
	sameDate := func(t time.Time) bool {
		y, m, d := t.Date()
		return y == year && m == month && d == day
	}
	next := time.Date(year, month, day+1, 0, 0, 0, 0, start.Location())
	if next.After(start) && !sameDate(next) && sameDate(next.Add(-time.Nanosecond)) {
		return next
	}
	// Locate the new date even when the timezone skips its nominal midnight (or a whole
	// calendar date), then narrow to the exact boundary without assuming a 24-hour day.
	for !next.After(start) || sameDate(next) {
		next = next.Add(time.Hour)
	}
	previous := start
	for next.Sub(previous) > time.Nanosecond {
		middle := previous.Add(next.Sub(previous) / 2)
		if sameDate(middle) {
			previous = middle
		} else {
			next = middle
		}
	}
	return next
}

// Day is the counts of day, whatever the engines.
func (l Ledger) Day(day string) Counts {
	for _, d := range l.Days {
		if d.Day == day {
			return d.Total()
		}
	}
	return Counts{}
}

// Clone is a copy of l that shares nothing with it.
func (l Ledger) Clone() Ledger {
	days := slices.Clone(l.Days)
	for i := range days {
		days[i].Uses = slices.Clone(days[i].Uses)
	}
	return Ledger{Days: days}
}

// Load reads the ledger kept at path; none there is an empty one. A file of the numbers of a day
// only, as the app kept before the ledger, is that day's, its engines unknown.
func Load(path string) (Ledger, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Ledger{}, nil
	}
	if err != nil {
		return Ledger{}, err
	}
	var f struct {
		Days []Day `json:"days"`
		// The day's numbers, as kept before.
		Day          string `json:"Day"`
		Run, Speech  float64
		Lines, Chars int
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return Ledger{}, err
	}
	l := Ledger{Days: f.Days}
	if len(l.Days) == 0 && f.Day != "" {
		l.Add(f.Day, Engines{}, Counts{Run: f.Run, Lines: f.Lines, Speech: f.Speech, Chars: f.Chars})
	}
	slices.SortFunc(l.Days, func(a, b Day) int { return strings.Compare(a.Day, b.Day) })
	return l, nil
}

// Save keeps l at path, in a file written whole before it replaces the one there.
func (l Ledger) Save(path string) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
