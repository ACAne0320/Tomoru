package apple

import (
	"strings"
	"unicode"

	"tomoru/internal/engine"
	"tomoru/internal/lang"
)

const (
	still       = 1.3
	clausePause = 0.2
)

func Finalizer(lang string) func(engine.Event) (through float64, ok bool) {
	h := &hint{clauses: chinese(lang), lastFinalize: -1}
	return h.event
}

type hint struct {
	clauses      bool
	clock        float64
	vol          string
	volEnd       float64
	volChangedAt float64
	lastFinalize float64

	voiceKnown, voiceOn bool
	offAt               float64
	paused              float64
	heard               bool
}

func (h *hint) event(ev engine.Event) (float64, bool) {
	switch ev.Type {
	case engine.EventClock:
		h.clock = max(h.clock, ev.T)
	case engine.EventVoice:
		h.voiceKnown, h.voiceOn = true, ev.On
		if !ev.On {
			h.offAt = ev.T
		}
	case engine.EventResult:
		text := ev.Text()
		if hasSpeech(text) {
			h.heard = true
		}
		if ev.Final {
			h.vol = ""
			break
		}
		h.clock = max(h.clock, ev.End)
		if text != h.vol {
			h.volChangedAt = h.clock
		}
		h.vol, h.volEnd = text, ev.End
	default:
		return 0, false
	}
	if h.clauses && h.voiceKnown && !h.voiceOn && h.clock-h.offAt >= clausePause && h.paused < h.offAt && h.heard {
		h.paused, h.heard = h.offAt, false
		if t := h.offAt + 0.1; t > h.lastFinalize+0.05 {
			h.lastFinalize = t
			return t, true
		}
	}
	if h.vol != "" && h.clock-h.volChangedAt >= still && h.volEnd > h.lastFinalize+0.05 {
		h.lastFinalize = h.volEnd
		return h.volEnd, true
	}
	return 0, false
}

func chinese(tag string) bool {
	l, _, _ := strings.Cut(strings.ToLower(tag), "-")
	return lang.Chinese(l)
}

func hasSpeech(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0
}
