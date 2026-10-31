package asr

import (
	"testing"
	"time"

	"tomoru/internal/asr/batch"
	"tomoru/internal/engine"
)

// TestLag says the network is slow once three of the last five measures came late, or sending
// stalled, and quick again once the last three came soon.
func TestLag(t *testing.T) {
	var said []bool
	l := &lag{}
	note := func(ev engine.Event) {
		if slow, changed := l.note(ev); changed {
			said = append(said, slow)
		}
	}
	for _, s := range []float64{5, .5, 5, 4} { // seconds: one late is not slow
		note(batch.Took(time.Duration(s * float64(time.Second))))
	}
	if len(said) != 1 || !said[0] {
		t.Fatalf("three late of four: %v", said)
	}
	note(batch.Took(time.Second))
	note(batch.Took(time.Second))
	if len(said) != 1 {
		t.Fatalf("two quick: %v", said)
	}
	note(batch.Took(time.Second))
	if len(said) != 2 || said[1] {
		t.Fatalf("three quick: %v", said)
	}
	note(batch.Stalled(2 * time.Second))
	note(batch.Stalled(5 * time.Second))
	if len(said) != 3 || !said[2] {
		t.Fatalf("stalled: %v", said)
	}
}

// fakeRecognizer sends what it is given to send.
type fakeRecognizer struct{ done chan struct{} }

func (fakeRecognizer) Audio(engine.Event) error { return nil }
func (f fakeRecognizer) Stop()                  {}
func (f fakeRecognizer) Done() <-chan struct{}  { return f.done }
func (fakeRecognizer) Err() error               { return nil }

// TestWatched turns a cloud recognizer's measures into slow and quick, and passes the rest; every
// cloud recognizer is watched.
func TestWatched(t *testing.T) {
	var send func(engine.Event)
	f, _ := watched(func(Env) (Factory, error) {
		return func(lang string, on func(engine.Event)) (Recognizer, error) {
			send = on
			return fakeRecognizer{}, nil
		}, nil
	})(Env{})
	var got []engine.Event
	f("ja", func(ev engine.Event) { got = append(got, ev) })
	send(engine.Event{Type: "clock", T: 1})
	for range 3 {
		send(batch.Took(5 * time.Second))
	}
	if len(got) != 2 || got[0].Type != "clock" || got[1].Type != "slow" || !got[1].On {
		t.Fatalf("%+v", got)
	}
	for _, s := range specs("darwin") {
		if s.Cloud {
			if _, err := s.New(Env{Service: Service{Key: "k"}}); err != nil {
				t.Errorf("%s: %v", s.ID, err)
			}
		}
	}
}
