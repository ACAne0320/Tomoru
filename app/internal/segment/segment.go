package segment

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

type Token struct {
	Text       string
	Start, End float64
}

type Segment struct {
	Text       string
	Timed      bool
	Start, End float64
	Reason     string
	Seq        int
}

type Ended struct {
	Seq int
	End float64
}

type Action struct {
	Commit *Segment
}

type Config struct {
	VolatileSentences bool
	FinalsEnd         bool
	TailWait          float64
	MaxWidth          int
}

type Segmenter struct {
	cfg            Config
	stable         []Token
	committedUntil float64
	vol            string
	volEnd         float64
	volCommitted   string
	volSegs        []volSeg
	ended          []Ended
	seq            int
	lastSpeechEnd  float64
	clock          float64

	voiceKnown bool
	voiceOn    bool
	offAt      float64
}

type volSeg struct{ seq, end int }

func (s *Segmenter) Ended() []Ended {
	e := s.ended
	s.ended = nil
	return e
}

func New(cfg Config) *Segmenter {
	if cfg.TailWait == 0 {
		cfg.TailWait = 0.6
	}
	if cfg.MaxWidth == 0 {
		cfg.MaxWidth = 90
	}
	return &Segmenter{cfg: cfg}
}

func (s *Segmenter) Final(incoming []Token) []Action {
	var toks []Token
	for _, t := range incoming {
		if t.Start >= s.committedUntil-0.005 && t.Text != "" {
			toks = append(toks, t)
		}
	}
	if s.volCommitted != "" && len(toks) > 0 {
		finalText := joinTokens(toks)
		s.timeVolSegs(toks, finalText)
		if !hasSpeech(finalText) {
			s.committedUntil = toks[len(toks)-1].End
			s.volCommitted, s.volSegs, toks = "", nil, nil
		} else if len(norm(finalText)) < len(norm(s.volCommitted)) {
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

func (s *Segmenter) Volatile(text string, start, end float64) []Action {
	s.clock = max(s.clock, end)
	s.vol = text
	s.volEnd = end
	if hasSpeech(text) {
		s.lastSpeechEnd = max(s.lastSpeechEnd, end)
	}
	return s.evaluate()
}

func (s *Segmenter) Voice(on bool, t float64) []Action {
	s.voiceKnown, s.voiceOn = true, on
	if !on {
		s.offAt = t
	}
	return s.evaluate()
}

func (s *Segmenter) Tick(clock float64) []Action {
	s.clock = max(s.clock, clock)
	return s.evaluate()
}

func (s *Segmenter) Pending() string { return clean(s.stableText() + s.volPending()) }

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

	for seg := s.commitFirstSentence(); seg != nil; seg = s.commitFirstSentence() {
		commit(seg)
	}

	if hasSpeech(s.stableText()) && (s.cfg.FinalsEnd || !hasSpeech(s.volPending()) && s.quiet()) {
		commit(s.commitStable("pause"))
	}

	for Width(clean(s.stableText()+s.volPending())) > s.cfg.MaxWidth {
		seg := s.commitWithin()
		if seg == nil {
			break
		}
		commit(seg)
	}
	return out
}

func (s *Segmenter) quiet() bool {
	if s.voiceKnown {
		return !s.voiceOn && s.clock-s.offAt >= s.cfg.TailWait
	}
	return s.clock-s.lastSpeechEnd >= s.cfg.TailWait
}

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
		if seg := s.commitWithin(); seg != nil {
			return seg
		}
	}
	return s.commit(full, len(st), b, "sentence")
}

func (s *Segmenter) commitStable(reason string) *Segment {
	st := []rune(s.stableText())
	return s.commit(st, len(st), len(st), reason)
}

func (s *Segmenter) commitWithin() *Segment {
	st := []rune(s.stableText())
	full := append(st, []rune(s.volPending())...)
	fits := 0
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

func (s *Segmenter) commit(full []rune, stableLen, cut int, reason string) *Segment {
	seg := &Segment{Reason: reason}
	if cut <= stableLen {
		k := tokenCount(s.stable, cut)
		if k == 0 {
			return nil
		}
		seg.Timed, seg.Start, seg.End = true, s.stable[0].Start, s.stable[k-1].End
		s.committedUntil = seg.End
		s.stable = s.stable[k:]
	} else {
		if len(s.stable) > 0 {
			seg.Start = s.stable[0].Start
			s.committedUntil = s.stable[len(s.stable)-1].End
		}
		s.stable = nil
		s.volCommitted += string(full[stableLen:cut])
	}
	seg.Text = clean(string(full[:cut]))
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

func Letters(s string) int { return len(norm(s)) }

func isSpeech(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func hasSpeech(s string) bool { return strings.IndexFunc(s, isSpeech) >= 0 }

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

func alignEnd(a, h string) int {
	x := norm(a)
	hr := []rune(h)
	type at struct {
		r   rune
		end int
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
	prev := make([]int, len(hs)+1)
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
	best := 0
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

func clean(s string) string {
	s = spaceBeforeCJK.ReplaceAllString(s, "$1")
	s = spaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func isClauseMark(r rune) bool { return strings.ContainsRune("，、,；;：:", r) }
