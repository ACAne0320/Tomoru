package asr

import (
	"sync"
	"time"

	"tomoru/internal/engine"
)

// A cloud recognizer's network is slow when three of its last five measures (batch.Took: a
// result, or a round trip, after its audio was sent) took slowAfter or more, or sending audio
// stalled as long (batch.Stalled); quick again once its last three took under quickUnder. The
// session then says so where the subtitles show, for the user to check the network or the proxy:
// no recognizer makes live subtitles of a slow network. Each recognizer measures what it can
// (docs/recognition.md, step 6); the decision is here, alike for all, and for any added.
//
// On a good connection, OpenAI's live transcription gave a turn's final text 0.5 s after its
// commit, its transcription API decoded 2–5 s of audio in 1–1.8 s, and Qwen's pings came back in
// 0.37 s at the median, 2.6 s at worst; over a proxy that lost connections, final text came 10 s
// late and more, and sending audio stalled 5 s.
const (
	slowAfter  = 4 * time.Second
	quickUnder = 2 * time.Second
	weighed    = 5
)

// lag weighs a recognizer's measures.
type lag struct {
	mu     sync.Mutex
	delays []time.Duration
	slow   bool
}

// note weighs a measure, an event "lag": it tells whether the network is slow now, and whether
// that changed.
func (l *lag) note(ev engine.Event) (slow, changed bool) {
	d := time.Duration(ev.T * float64(time.Second))
	l.mu.Lock()
	defer l.mu.Unlock()
	was := l.slow
	if ev.On { // sending stalled
		l.slow = l.slow || d >= slowAfter
		return l.slow, l.slow != was
	}
	l.delays = append(l.delays, d)
	if len(l.delays) > weighed {
		l.delays = l.delays[1:]
	}
	late, quick := 0, 0
	for i, d := range l.delays {
		if d >= slowAfter {
			late++
		}
		if i >= len(l.delays)-3 && d < quickUnder {
			quick++
		}
	}
	switch {
	case late >= 3:
		l.slow = true
	case quick == 3:
		l.slow = false
	}
	return l.slow, l.slow != was
}

// watched has a cloud recognizer's measures of its network said as the session takes it: an
// event "slow", on or off, as it changes; the measures themselves go no further.
func watched(build func(Env) (Factory, error)) func(Env) (Factory, error) {
	return func(env Env) (Factory, error) {
		f, err := build(env)
		if f == nil || err != nil {
			return f, err
		}
		return func(lang string, on func(engine.Event)) (Recognizer, error) {
			l := &lag{}
			return f(lang, func(ev engine.Event) {
				if ev.Type != "lag" {
					on(ev)
					return
				}
				if slow, changed := l.note(ev); changed {
					on(engine.Event{Type: "slow", On: slow})
				}
			})
		}, nil
	}
}
