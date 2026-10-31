package live

import (
	"sync"
	"testing"
	"time"

	"tomoru/internal/engine"
)

type statsClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *statsClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *statsClock) set(at time.Time) {
	c.mu.Lock()
	c.at = at
	c.mu.Unlock()
}

func TestStatsTimestampSurvivesQueuedReports(t *testing.T) {
	before := time.Date(2026, 10, 21, 23, 59, 58, 0, time.FixedZone("CST", 8*60*60))
	clock := &statsClock{at: before}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var got []State
	s := newSession(Config{Now: clock.now}, func(st State) {
		if len(got) == 0 {
			close(entered)
			<-release
		}
		got = append(got, st)
	})
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		s.Stop()
		s.Wait()
	})
	s.handle(engine.Event{Type: "state", State: "listening"})
	waitSignal(t, entered)
	clock.set(before.Add(time.Second))
	s.update(func(st *State) { st.Stats.Lines++ })
	clock.set(before.Add(3 * time.Second))
	s.update(func(st *State) { st.Stats.Chars += 7 })
	clock.set(before.Add(4 * time.Second))
	s.Stop()
	// All reports are delivered on a later date, including the terminal report.
	clock.set(before.Add(48 * time.Hour))
	once.Do(func() { close(release) })
	s.Wait()
	if len(got) != 4 {
		t.Fatalf("received %d reports; want 4", len(got))
	}
	for i, offset := range []time.Duration{0, time.Second, 3 * time.Second, 4 * time.Second} {
		if want := before.Add(offset); !got[i].StatsAt.Equal(want) {
			t.Errorf("report %d timestamp = %v; want %v", i, got[i].StatsAt, want)
		}
	}
	if got[1].Stats.Lines != 1 || got[1].Stats.Chars != 0 || got[2].Stats.Chars != 7 || got[3].Phase != "stopped" {
		t.Fatalf("queued snapshots changed: %+v", got)
	}
}

func TestSpeechEndPreservesCrossMidnightAudioInterval(t *testing.T) {
	start := time.Date(2026, 10, 21, 23, 59, 58, 0, time.FixedZone("CST", 8*60*60))
	for _, withClock := range []bool{true, false} {
		name := "audio_clock"
		if !withClock {
			name = "event_time_fallback"
		}
		t.Run(name, func(t *testing.T) {
			clock := &statsClock{at: start}
			var got []State
			s := newSession(Config{Now: clock.now}, func(st State) { got = append(got, st) })
			t.Cleanup(func() { s.Stop(); s.Wait() })
			if withClock {
				s.handle(engine.Event{Type: "clock", T: 100})
			}
			s.handle(engine.Event{Type: "voice", On: true, T: 100})
			end := start.Add(4 * time.Second)
			reported := end
			if withClock {
				// Detection reports late; audio still ended at 00:00:02.
				reported = end.Add(5 * time.Second)
			}
			clock.set(reported)
			s.handle(engine.Event{Type: "voice", On: false, T: 104})
			clock.set(reported.Add(time.Second))
			// Duplicate voice-off notifications must not move the interval's endpoint.
			s.handle(engine.Event{Type: "voice", On: false, T: 105})
			s.Stop()
			s.Wait()
			var previous float64
			increments := 0
			for _, st := range got {
				delta := st.Stats.Speech - previous
				previous = st.Stats.Speech
				if delta > 0 {
					increments++
					if delta != 4 || !st.SpeechEnd.Equal(end) || !st.StatsAt.Equal(reported) {
						t.Errorf("speech report: delta=%v end=%v at=%v", delta, st.SpeechEnd, st.StatsAt)
					}
					if inferred := st.SpeechEnd.Add(-time.Duration(delta * float64(time.Second))); !inferred.Equal(start) {
						t.Errorf("speech started %v; want %v", inferred, start)
					}
				}
				if st.Stats.Speech > 0 && !st.SpeechEnd.Equal(end) {
					t.Errorf("later report changed speech endpoint: %v", st.SpeechEnd)
				}
			}
			if increments != 1 {
				t.Errorf("got %d positive speech increments; want 1", increments)
			}
		})
	}
}

func TestStatsTimestampDefaultsToWallClock(t *testing.T) {
	before := time.Now()
	var last State
	s := newSession(Config{}, func(st State) { last = st })
	s.Stop()
	s.Wait()
	if last.StatsAt.Before(before) || last.StatsAt.After(time.Now()) {
		t.Errorf("default stats timestamp = %v", last.StatsAt)
	}
}
