package asr

import (
	"sync"
	"time"

	"tomoru/internal/engine"
)

const (
	slowAfter  = 4 * time.Second
	quickUnder = 2 * time.Second
	weighed    = 5
)

type lag struct {
	mu     sync.Mutex
	delays []time.Duration
	slow   bool
}

func (l *lag) note(ev engine.Event) (slow, changed bool) {
	d := time.Duration(ev.T * float64(time.Second))
	l.mu.Lock()
	defer l.mu.Unlock()
	was := l.slow
	if ev.On {
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

func watched(build func(Env) (Factory, error)) func(Env) (Factory, error) {
	return func(env Env) (Factory, error) {
		f, err := build(env)
		if f == nil || err != nil {
			return f, err
		}
		return func(lang string, on func(engine.Event)) (Recognizer, error) {
			l := &lag{}
			return f(lang, func(ev engine.Event) {
				if ev.Type != engine.EventLag {
					on(ev)
					return
				}
				if slow, changed := l.note(ev); changed {
					on(engine.Event{Type: engine.EventSlow, On: slow})
				}
			})
		}, nil
	}
}
