// Package segment turns a recognizer's output into subtitle segments as speech goes on, where
// the recognizer ends its sentences (docs/recognition.md): sentences are the recognizer's to tell,
// by its punctuation and its end of utterance, not ours by pauses or length.
//
// The recognizer gives two kinds of input:
//   - Final tokens: fixed, each timed. Bookkeeping is in audio time (committedUntil). A sentence
//     they finish is committed. What follows the last sentence end is committed with them where a
//     final result ends an utterance (Config.FinalsEnd: a service's own end of utterance); else it
//     waits for the next final result, which joins it, or for the speaker to stop.
//   - Volatile text: the live hypothesis since the last final one, revised as more is heard, and
//     untimed. Where the recognizer's volatile sentence ends hold (Config.VolatileSentences), a
//     sentence it ends is committed at once, without waiting for a final result. Where the
//     committed text sits inside a revised hypothesis is found by fuzzy alignment (edit distance
//     over letters and digits), never by string offsets.
//
// One rule is ours, a safety: a sentence past two lines is cut, within them, at its last clause
// mark, or else between its last two words.
//
// A Segmenter is not safe for concurrent use.
package segment

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

// Token is a word or character of a final result, timed in seconds of audio.
type Token struct {
	Text       string
	Start, End float64
}

// Segment is a committed piece of speech: a sentence, or what was said before a pause, or a
// clause cut from a long stretch.
type Segment struct {
	Text string
	// Timed tells whether Start and End are known: a segment committed from volatile text has no
	// times yet.
	Timed      bool
	Start, End float64
	Reason     string // "sentence", "pause" (the speaker stopped mid-sentence) or "length"
	Seq        int    // its number, from 1, in the order committed
}

// Ended is when the speech of a segment committed untimed ended, as its final result tells, by
// the segment's Seq.
type Ended struct {
	Seq int
	End float64
}

// Action is what the caller does after an input: show a segment.
type Action struct {
	Commit *Segment
}

// Config tunes a Segmenter. The zero value of a field takes its default.
type Config struct {
	// VolatileSentences commits sentences the volatile text ends: for recognizers whose volatile
	// sentence ends hold (Apple's; Qwen's cloud, steadied by qwencloud).
	VolatileSentences bool
	// FinalsEnd tells the recognizer's final results end utterances, as a service ends one where it
	// hears the speaker stop: what they hold is committed at once. Else (stretches of speech a
	// voice detector cut, which may pause mid-sentence) their unfinished tail waits for the next.
	FinalsEnd bool
	// TailWait is the silence, in seconds, after which the unfinished tail of final results is
	// committed: by voice activity, else since the last speech heard; 0.6. 1.0 had SenseVoice's
	// lines final 1 s later and translated 0.4 s later at p90, and long lines 4 in 100, joining
	// sentences across pauses (the benchmark's quick set, 2026-10-31).
	TailWait float64
	// MaxWidth is the width of two lines of subtitles, a CJK character as wide as two letters, past
	// which a sentence is cut; 90, as the benchmark's Long % counts it.
	MaxWidth int
}

// Segmenter is the state machine. Feed it with Final, Volatile and Tick, in the order they
// happen, and act on what they return.
type Segmenter struct {
	cfg            Config
	stable         []Token // final, not yet committed
	committedUntil float64
	vol            string // the live hypothesis since the last final result
	volEnd         float64
	volCommitted   string // the part of the hypothesis already committed, not final yet
	// volSegs are the segments that end in volCommitted, untimed, and where they end in it: its
	// runes up to there.
	volSegs       []volSeg
	ended         []Ended // what Ended gives next
	seq           int     // the last segment's Seq
	lastSpeechEnd float64 // the latest audio time known to hold speech, final or volatile
	clock         float64

	// Voice activity, once the recognizer's engine reports it.
	voiceKnown bool
	voiceOn    bool
	offAt      float64 // when the voice last stopped
}

type volSeg struct{ seq, end int }

// Ended gives the segments committed untimed whose final results came since it was last called,
// and when their speech ended.
func (s *Segmenter) Ended() []Ended {
	e := s.ended
	s.ended = nil
	return e
}

// New returns a Segmenter.
func New(cfg Config) *Segmenter {
	if cfg.TailWait == 0 {
		cfg.TailWait = 0.6
	}
	if cfg.MaxWidth == 0 {
		cfg.MaxWidth = 90
	}
	return &Segmenter{cfg: cfg}
}

// Final takes the tokens of a final result.
func (s *Segmenter) Final(incoming []Token) []Action {
	var toks []Token
	for _, t := range incoming {
		if t.Start >= s.committedUntil-0.005 && t.Text != "" {
			toks = append(toks, t)
		}
	}
	if s.volCommitted != "" && len(toks) > 0 {
		// Some of this final result was committed from volatile text already: drop that part.
		finalText := joinTokens(toks)
		s.timeVolSegs(toks, finalText)
		if len(norm(finalText)) < len(norm(s.volCommitted)) {
			// The final result covers only part of what was committed.
			vc := []rune(s.volCommitted)
			d := alignEnd(finalText, s.volCommitted)
			s.volCommitted = string(vc[d:])
			for i := range s.volSegs {
				s.volSegs[i].end = max(0, s.volSegs[i].end-d)
			}
			s.committedUntil = toks[len(toks)-1].End
			toks = nil
		} else {
			cut := alignEnd(s.volCommitted, finalText)
			k := tokenCount(toks, cut)
			if k > 0 {
				s.committedUntil = toks[k-1].End
			}
			toks = toks[k:]
			s.volCommitted, s.volSegs = "", nil
		}
	}
	s.stable = append(s.stable, toks...)
	s.vol = ""
	if len(incoming) > 0 {
		s.volEnd = max(s.volEnd, incoming[len(incoming)-1].End)
	}
	for i := len(incoming) - 1; i >= 0; i-- {
		if hasSpeech(incoming[i].Text) {
			s.lastSpeechEnd = max(s.lastSpeechEnd, incoming[i].End)
			break
		}
	}
	s.clock = max(s.clock, s.volEnd)
	return s.evaluate()
}

// timeVolSegs times the segments committed untimed that the final tokens toks, of text finalText,
// cover: their speech ended with the token their text ends in.
func (s *Segmenter) timeVolSegs(toks []Token, finalText string) {
	vc := []rune(s.volCommitted)
	covered := len(norm(finalText))
	keep := s.volSegs[:0]
	for _, v := range s.volSegs {
		upTo := string(vc[:min(v.end, len(vc))])
		if len(norm(upTo)) > covered {
			keep = append(keep, v)
			continue
		}
		if k := tokenCount(toks, alignEnd(upTo, finalText)); k > 0 {
			s.ended = append(s.ended, Ended{Seq: v.seq, End: toks[k-1].End})
		}
	}
	s.volSegs = keep
}

// Volatile takes the live hypothesis for the audio from start to end, which replaces the last.
func (s *Segmenter) Volatile(text string, start, end float64) []Action {
	s.clock = max(s.clock, end)
	s.vol = text
	s.volEnd = end
	if hasSpeech(text) {
		s.lastSpeechEnd = max(s.lastSpeechEnd, end)
	}
	return s.evaluate()
}

// Voice takes voice activity: the speaker's voice started (on) or stopped at t, seconds of audio.
func (s *Segmenter) Voice(on bool, t float64) []Action {
	s.voiceKnown, s.voiceOn = true, on
	if !on {
		s.offAt = t
	}
	return s.evaluate()
}

// Tick moves the audio clock, which commits the tail the speaker stopped after.
func (s *Segmenter) Tick(clock float64) []Action {
	s.clock = max(s.clock, clock)
	return s.evaluate()
}

// Pending is the text heard and not committed yet, to show as it is spoken.
func (s *Segmenter) Pending() string { return clean(s.stableText() + s.volPending()) }

// CommittedUntil is the audio time that committed segments reach, by final tokens.
func (s *Segmenter) CommittedUntil() float64 { return s.committedUntil }

func (s *Segmenter) stableText() string { return joinTokens(s.stable) }

func (s *Segmenter) volPending() string {
	if s.volCommitted == "" {
		return s.vol
	}
	return string([]rune(s.vol)[alignEnd(s.volCommitted, s.vol):])
}

func (s *Segmenter) evaluate() []Action {
	var out []Action
	commit := func(seg *Segment) {
		if seg != nil {
			out = append(out, Action{Commit: seg})
		}
	}

	// 1. Complete sentences, one segment each, where the recognizer ends them: in final results,
	// then in the hypothesis where its sentence ends hold. One too long for two lines is cut
	// within them, as in 3.
	for seg := s.commitFirstSentence(); seg != nil; seg = s.commitFirstSentence() {
		commit(seg)
	}

	// 2. The recognizer ended the utterance, or the speaker stopped mid-sentence: the tail of
	// final results is all they said.
	if hasSpeech(s.stableText()) && (s.cfg.FinalsEnd || !hasSpeech(s.volPending()) && s.quiet()) {
		commit(s.commitStable("pause"))
	}

	// 3. Too long without a sentence end: cut within two lines, as often as it takes.
	for Width(clean(s.stableText()+s.volPending())) > s.cfg.MaxWidth {
		seg := s.commitWithin()
		if seg == nil {
			break
		}
		commit(seg)
	}
	return out
}

// quiet tells whether the speaker has been silent for TailWait: by voice activity, else since the
// last speech the recognizer heard.
func (s *Segmenter) quiet() bool {
	if s.voiceKnown {
		return !s.voiceOn && s.clock-s.offAt >= s.cfg.TailWait
	}
	return s.clock-s.lastSpeechEnd >= s.cfg.TailWait
}

// commitFirstSentence commits the first complete sentence of the stable text and the hypothesis,
// if there is one. A sentence of punctuation alone is dropped, and returns nil too.
func (s *Segmenter) commitFirstSentence() *Segment {
	st := []rune(s.stableText())
	full := st
	if s.cfg.VolatileSentences {
		full = append(st, []rune(s.volPending())...)
	}
	b := firstSentenceEnd(full)
	if b < 0 {
		return nil
	}
	if Width(clean(string(full[:b]))) > s.cfg.MaxWidth {
		// Too long for two lines: cut within them, as text with no sentence end is (rule 3).
		if seg := s.commitWithin(); seg != nil {
			return seg
		}
	}
	return s.commit(full, len(st), b, "sentence")
}

// commitStable commits the final tokens not committed yet.
func (s *Segmenter) commitStable(reason string) *Segment {
	st := []rune(s.stableText())
	return s.commit(st, len(st), len(st), reason)
}

// commitWithin commits what fits in two lines of the pending text, cut at its last clause mark
// there, if that leaves a third of the lines or more; else between its last two words there; else
// at its last clause mark there at all. Nil when it has none of these: Chinese or Japanese with no
// comma.
func (s *Segmenter) commitWithin() *Segment {
	st := []rune(s.stableText())
	full := append(st, []rune(s.volPending())...)
	fits := 0 // how many runes fit in two lines
	for fits < len(full) && Width(clean(string(full[:fits+1]))) <= s.cfg.MaxWidth {
		fits++
	}
	clause, word := -1, -1
	for i := 1; i < fits; i++ {
		if isClauseMark(full[i-1]) && len(norm(string(full[:i]))) >= 6 {
			clause = i
		}
		if unicode.IsSpace(full[i]) && isSpeech(full[i-1]) && len(norm(string(full[:i]))) >= 6 {
			word = i
		}
	}
	switch {
	case clause > 0 && Width(clean(string(full[:clause])))*3 >= s.cfg.MaxWidth:
		return s.commit(full, len(st), clause, "length")
	case word > 0:
		return s.commit(full, len(st), word, "length")
	case clause > 0:
		return s.commit(full, len(st), clause, "length")
	}
	return nil
}

// Width is how wide text shows: a wide or fullwidth character, as CJK's, as wide as two letters.
func Width(text string) int {
	w := 0
	for _, r := range text {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			w += 2
		default:
			w++
		}
	}
	return w
}

// commit commits full[:cut], where full is the stable text (its first stableLen runes) and then
// the pending hypothesis.
func (s *Segmenter) commit(full []rune, stableLen, cut int, reason string) *Segment {
	seg := &Segment{Reason: reason}
	if cut <= stableLen {
		// All within final tokens: commit by token, with their times.
		k := tokenCount(s.stable, cut)
		if k == 0 {
			return nil
		}
		seg.Timed, seg.Start, seg.End = true, s.stable[0].Start, s.stable[k-1].End
		s.committedUntil = seg.End
		s.stable = s.stable[k:]
	} else {
		// All the final tokens, and the start of the hypothesis.
		if len(s.stable) > 0 {
			seg.Start = s.stable[0].Start
			s.committedUntil = s.stable[len(s.stable)-1].End
		}
		s.stable = nil
		s.volCommitted += string(full[stableLen:cut])
	}
	seg.Text = clean(string(full[:cut]))
	// What is left of punctuation alone, as a lone "。", is no subtitle.
	if !hasSpeech(seg.Text) {
		return nil
	}
	s.seq++
	seg.Seq = s.seq
	if !seg.Timed {
		s.volSegs = append(s.volSegs, volSeg{seq: seg.Seq, end: len([]rune(s.volCommitted))})
	}
	return seg
}

// Letters counts the letters and digits of s: how much speech it holds, punctuation and spaces
// aside.
func Letters(s string) int { return len(norm(s)) }

// Helpers.

func isSpeech(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func hasSpeech(s string) bool { return strings.IndexFunc(s, isSpeech) >= 0 }

// norm is s as alignment compares it: letters and digits, lowercased.
func norm(s string) []rune {
	var out []rune
	for _, r := range s {
		if isSpeech(r) {
			out = append(out, unicode.ToLower(r))
		}
	}
	return out
}

func joinTokens(toks []Token) string {
	var b strings.Builder
	for _, t := range toks {
		b.WriteString(t.Text)
	}
	return b.String()
}

// alignEnd returns where a, text committed already, ends inside the hypothesis h, as an index of
// h's runes. It aligns a against a prefix of h by edit distance over their letters and digits,
// and skips the punctuation and spaces that follow the match.
func alignEnd(a, h string) int {
	x := norm(a)
	hr := []rune(h)
	type at struct {
		r   rune
		end int // the index just after it in hr
	}
	var hs []at
	for i, r := range hr {
		if isSpeech(r) {
			hs = append(hs, at{unicode.ToLower(r), i + 1})
		}
	}
	if len(x) == 0 || len(hs) == 0 {
		return 0
	}
	prev := make([]int, len(hs)+1) // aligning nothing against hs[:j] costs j
	for j := range prev {
		prev[j] = j
	}
	cur := make([]int, len(hs)+1)
	for _, xr := range x {
		cur[0] = prev[0] + 1
		for j := 1; j <= len(hs); j++ {
			sub := prev[j-1]
			if hs[j-1].r != xr {
				sub++
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, sub)
		}
		prev, cur = cur, prev
	}
	best := 0 // the lowest cost, then the shortest prefix
	for j := range prev {
		if prev[j] < prev[best] {
			best = j
		}
	}
	end := 0
	if best > 0 {
		end = hs[best-1].end
	}
	for end < len(hr) && !isSpeech(hr[end]) {
		end++
	}
	return end
}

// tokenCount returns how many of the first tokens lie before cut, a rune index of their text.
func tokenCount(toks []Token, cut int) int {
	k, pos := 0, 0
	for _, t := range toks {
		pos += len([]rune(t.Text))
		if pos > cut {
			break
		}
		k++
	}
	return k
}

var (
	spaceBeforeCJK = regexp.MustCompile(`[\s\p{Zs}]+([，。！？、；：」』）])`)
	spaces         = regexp.MustCompile(`[\s\p{Zs}]+`)
)

// clean evens out the recognizer's spacing: SpeechAnalyzer writes " ，" in Chinese and Japanese,
// and starts English words with a space.
func clean(s string) string {
	s = spaceBeforeCJK.ReplaceAllString(s, "$1")
	s = spaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func isClauseMark(r rune) bool { return strings.ContainsRune("，、,；;：:", r) }
