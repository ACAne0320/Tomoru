package batch

import (
	"time"

	"tomoru/internal/engine"
)

const minStall = time.Second

func Took(d time.Duration) engine.Event { return engine.Event{Type: engine.EventLag, T: d.Seconds()} }

func Stalled(d time.Duration) engine.Event {
	if d < minStall {
		return engine.Event{}
	}
	return engine.Event{Type: engine.EventLag, T: d.Seconds(), On: true}
}
