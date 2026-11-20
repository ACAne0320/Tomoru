package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestStatusMarkRests draws no frame while nothing moves: the eyes once they have looked around,
// and with Reduce Motion every state, a voice heard included.
func TestStatusMarkRests(t *testing.T) {
	loud := func(time.Time) float32 { return .8 }
	s, since := MarkReady, time.Now()
	view := func(c *ui.Context) { StatusMark(c, s, since, loud, ui.RGB(255, 255, 255)) }
	tt := ui.NewTester(view, 200, 100)
	if !tt.FramePending() {
		t.Error("the eyes do not open")
	}
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("the eyes still move after looking around")
	}

	tt = ui.NewTesterWith(view, 200, 100, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	for _, s = range []MarkState{MarkReady, MarkWaiting, MarkLive, MarkPaused} {
		since = tt.Now()
		tt.Frame()
		tt.Advance(time.Second)
		if tt.FramePending() {
			t.Errorf("state %d moves with Reduce Motion", s)
		}
	}
}
