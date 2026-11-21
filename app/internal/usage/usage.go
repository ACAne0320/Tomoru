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

type Counts struct {
	Run       float64 `json:"run"`
	Lines     int     `json:"lines"`
	Speech    float64 `json:"speech"`
	Chars     int     `json:"chars"`
	Audio     float64 `json:"audio,omitempty"`
	Failed    int     `json:"failed,omitempty"`
	InTokens  int     `json:"inTokens,omitempty"`
	OutTokens int     `json:"outTokens,omitempty"`
	Images    int     `json:"images,omitempty"`
	Latency   Latency `json:"latency"`
}

type Latency struct {
	FirstText        Hist `json:"firstText"`
	Final            Hist `json:"final"`
	FirstTranslation Hist `json:"firstTranslation"`
	Translation      Hist `json:"translation"`
	Queue            Hist `json:"queue"`
	EndToEnd         Hist `json:"endToEnd"`
}

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

var Bounds = [...]float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1200, 1400, 1600, 1800, 2000, 2500, 3000, 4000, 5000, 7000, 10000}

type Hist struct {
	N   int                  `json:"n"`
	Sum float64              `json:"sum"`
	B   [len(Bounds) + 1]int `json:"b"`
}

func (h *Hist) Add(d time.Duration) {
	ms := max(0, float64(d)/float64(time.Millisecond))
	h.N++
	h.Sum += ms
	h.B[sort.SearchFloat64s(Bounds[:], ms)]++
}

func (h *Hist) Merge(o Hist) {
	h.N += o.N
	h.Sum += o.Sum
	for i := range h.B {
		h.B[i] += o.B[i]
	}
}

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

type Engines struct {
	ASR          string `json:"asr"`
	ASRModel     string `json:"asrModel,omitempty"`
	MT           string `json:"mt"`
	MTName       string `json:"mtName,omitempty"`
	MTModel      string `json:"mtModel,omitempty"`
	Untranslated bool   `json:"untranslated,omitempty"`
}

type Use struct {
	Engines
	Counts
}

type Day struct {
	Day  string `json:"day"`
	Uses []Use  `json:"uses"`
}

func (d Day) Total() Counts { return d.TotalOf(func(Use) bool { return true }) }

func (d Day) TotalOf(has func(Use) bool) Counts {
	var c Counts
	for _, u := range d.Uses {
		if has(u) {
			c.Add(u.Counts)
		}
	}
	return c
}

type Ledger struct {
	Days []Day `json:"days"`
}

func DayOf(t time.Time) string { return t.Format(time.DateOnly) }

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

func (l *Ledger) AddRun(start, end time.Time, engines Engines) {
	l.addInterval(start, end, engines, end.Sub(start).Seconds(), func(seconds float64) Counts {
		return Counts{Run: seconds}
	})
}

func (l *Ledger) AddSpeech(end time.Time, seconds float64, engines Engines) {
	if seconds <= 0 {
		return
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration == 0 {
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

func (l Ledger) Day(day string) Counts {
	for _, d := range l.Days {
		if d.Day == day {
			return d.Total()
		}
	}
	return Counts{}
}

func (l Ledger) Clone() Ledger {
	days := slices.Clone(l.Days)
	for i := range days {
		days[i].Uses = slices.Clone(days[i].Uses)
	}
	return Ledger{Days: days}
}

func Load(path string) (Ledger, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Ledger{}, nil
	}
	if err != nil {
		return Ledger{}, err
	}
	var f struct {
		Days         []Day  `json:"days"`
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
