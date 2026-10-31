package apple

import (
	"math"
	"testing"

	"tomoru/internal/engine"
)

func vol(text string, start, end float64) engine.Event {
	return engine.Event{Type: "result", Start: start, End: end, Tokens: []engine.Token{{Text: text}}}
}

func clock(t float64) engine.Event { return engine.Event{Type: "clock", T: t} }

// TestStill finalizes once the hypothesis has stopped changing, at its end, and only once.
func TestStill(t *testing.T) {
	h := Hint("ja")
	for _, ev := range []engine.Event{vol("先看看", 10, 11), vol("先看看角色创建界面吧", 10, 12), clock(12.5), clock(13.2)} {
		if _, ok := h(ev); ok {
			t.Fatalf("finalized while speech may go on, at %+v", ev)
		}
	}
	if through, ok := h(clock(13.3)); !ok || through != 12 {
		t.Fatalf("finalized %v %v", through, ok)
	}
	if _, ok := h(clock(14)); ok {
		t.Fatal("finalized again")
	}
	// A final result clears the hypothesis: nothing waits.
	h(vol("好的", 14, 14.5))
	h(engine.Event{Type: "result", Final: true, End: 14.5, Tokens: []engine.Token{{Text: "好的"}}})
	if _, ok := h(clock(20)); ok {
		t.Fatal("finalized a final result")
	}
}

// TestClausePause finalizes Chinese inside the speaker's pause, once it has lasted 0.2 s, once a
// pause; and not other languages.
func TestClausePause(t *testing.T) {
	h := Hint("zh-CN")
	h(engine.Event{Type: "voice", On: true, T: 0})
	h(vol("大家好欢迎", 0, 1.0))
	if _, ok := h(engine.Event{Type: "voice", On: false, T: 1.1}); ok {
		t.Fatal("finalized at once")
	}
	through, ok := h(clock(1.35))
	if !ok || math.Abs(through-1.2) > 1e-9 {
		t.Fatalf("finalized %v %v", through, ok)
	}
	if _, ok := h(clock(1.4)); ok {
		t.Fatal("finalized the same pause again")
	}
	// A pause with nothing heard since the last is no clause.
	h(engine.Event{Type: "voice", On: true, T: 1.5})
	h(engine.Event{Type: "voice", On: false, T: 1.6})
	if _, ok := h(clock(1.9)); ok {
		t.Fatal("finalized a pause with nothing said")
	}

	j := Hint("ja")
	j(engine.Event{Type: "voice", On: true, T: 0})
	j(vol("みなさん", 0, 1))
	j(engine.Event{Type: "voice", On: false, T: 1.1})
	if _, ok := j(clock(1.5)); ok {
		t.Fatal("ja finalized at a pause")
	}
}
