package batch

import (
	"tomoru/internal/engine"

	"testing"
	"time"
)

func TestMeasures(t *testing.T) {
	if ev := Took(1500 * time.Millisecond); ev.Type != "lag" || ev.T != 1.5 || ev.On {
		t.Errorf("took %+v", ev)
	}
	if ev := Stalled(200 * time.Millisecond); ev.Type != "" {
		t.Errorf("a short send %+v", ev)
	}
	if ev := Stalled(3 * time.Second); ev.Type != engine.EventStall || ev.T != 3 {
		t.Errorf("stalled %+v", ev)
	}
}
