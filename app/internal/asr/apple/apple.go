// Package apple is what the app tells Apple's recognition (SpeechTranscriber), which runs in the
// macOS engine: when to make its result final. Left to itself, it ends an utterance long after the
// speaker paused, and Chinese clauses later still; asked in a pause, it ends there, with what was
// said before it (docs/recognition.md). Asking mid-speech makes it end the utterance there,
// inserting punctuation and dropping or garbling words, so it is asked only inside pauses.
package apple

import (
	"strings"
	"unicode"

	"tomoru/internal/engine"
	"tomoru/internal/lang"
)

const (
	// still is how long the hypothesis stays unchanged that is a pause: it changes only about once
	// a second while speech goes on.
	still = 1.3
	// clausePause is the speaker's silence, by voice activity, that ends a Chinese clause: the
	// recognizer ends Chinese clauses late, and finalizing at a pause kept their words, where in
	// English it dropped some ("Today we are …"), in Japanese moved a greeting's halves apart, and
	// in Korean garbled words at the cuts (the benchmark, 2026-10-20, bench/README.md).
	clausePause = 0.2
)

// Hint tells, for speech in lang, as each of the engine's events comes, when to make the result
// final: through when ok.
func Hint(lang string) func(engine.Event) (through float64, ok bool) {
	h := &hint{clauses: chinese(lang), lastFinalize: -1}
	return h.event
}

type hint struct {
	clauses      bool // finalize at the speaker's pauses: Chinese
	clock        float64
	vol          string // the hypothesis since the last final result
	volEnd       float64
	volChangedAt float64
	lastFinalize float64

	voiceKnown, voiceOn bool
	offAt               float64 // when the voice last stopped
	paused              float64 // the last pause finalized at
	heard               bool    // speech recognized since that pause
}

func (h *hint) event(ev engine.Event) (float64, bool) {
	switch ev.Type {
	case "clock":
		h.clock = max(h.clock, ev.T)
	case "voice":
		h.voiceKnown, h.voiceOn = true, ev.On
		if !ev.On {
			h.offAt = ev.T
		}
	case "result":
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
	// The speaker paused, in Chinese: finalize inside the pause, just after it began.
	if h.clauses && h.voiceKnown && !h.voiceOn && h.clock-h.offAt >= clausePause && h.paused < h.offAt && h.heard {
		h.paused, h.heard = h.offAt, false
		if t := h.offAt + 0.1; t > h.lastFinalize+0.05 {
			h.lastFinalize = t
			return t, true
		}
	}
	// The hypothesis stopped changing: a pause, at its end.
	if h.vol != "" && h.clock-h.volChangedAt >= still && h.volEnd > h.lastFinalize+0.05 {
		h.lastFinalize = h.volEnd
		return h.volEnd, true
	}
	return 0, false
}

// chinese tells whether speech in tag is Chinese, Cantonese too.
func chinese(tag string) bool {
	l, _, _ := strings.Cut(strings.ToLower(tag), "-")
	return lang.Chinese(l)
}

func hasSpeech(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0
}
