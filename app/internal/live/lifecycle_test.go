package live

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/engine"
	"tomoru/internal/translate"
)

type translateFunc func(context.Context, translate.Request, func(string)) (string, error)

func (f translateFunc) Translate(ctx context.Context, r translate.Request, delta func(string)) (string, error) {
	return f(ctx, r, delta)
}

type lifecycleLog struct {
	mu     sync.Mutex
	events []LineEvent
	state  State
}

func (l *lifecycleLog) onLine(e LineEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *lifecycleLog) onState(st State) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = st
}

func (l *lifecycleLog) completed() int {
	n := 0
	for _, e := range l.events {
		if e.Kind == LineCompleted {
			n++
		}
	}
	return n
}

// commitSentence feeds a distinct final utterance without coupling tests to Session's queue.
func commitSentence(s *Session, id int) {
	start, end := float64(id*2), float64(id*2+1)
	s.handle(engine.Event{Type: "result", Final: true, Tokens: []engine.Token{{
		Text: fmt.Sprintf("これは第%d文です。", id), Start: &start, End: &end,
	}}})
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker")
	}
}

// Every sentence survives display eviction, and a skipped translation gets its own final
// event immediately, without blocking recognition or waiting for earlier translations.
func TestLineEventsBacklogAndOverflow(t *testing.T) {
	for _, count := range []int{12, 66} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var first sync.Once
			tr := translateFunc(func(ctx context.Context, r translate.Request, delta func(string)) (string, error) {
				first.Do(func() { close(started) })
				select {
				case <-release:
				case <-ctx.Done():
					return "", ctx.Err()
				}
				delta("partial")
				return "translated " + r.Text, nil
			})
			var got lifecycleLog
			s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, OnLine: got.onLine}, got.onState)
			t.Cleanup(func() { s.Stop(); s.Wait() })
			commitSentence(s, 1)
			waitSignal(t, started)
			for id := 2; id <= count; id++ {
				commitSentence(s, id)
			}
			waitFor(t, &got.mu, func() bool { return got.state.Stats.Lines == count })
			got.mu.Lock()
			if len(got.state.Lines) != MaxLines || got.state.Lines[0].ID != count-MaxLines+1 {
				t.Errorf("display snapshot did not evict older lines: %+v", got.state.Lines)
			}
			if count == 66 {
				last := got.events[len(got.events)-1]
				if got.completed() != 1 || last.Kind != LineCompleted || last.Line.ID != 66 || last.Line.Err != "behind" {
					t.Errorf("overflow final event missing: %+v", got.events)
				}
			}
			got.mu.Unlock()
			close(release)
			waitFor(t, &got.mu, func() bool { return got.completed() == count })
			s.Stop()
			s.Wait()
			checkLifecycle(t, got.events, count)
			for _, e := range got.events {
				if e.Kind == LineCompleted && e.Line.Err == "" && e.Line.Tgt != "translated "+e.Line.Src {
					t.Errorf("lost translation for evicted sentence: %+v", e)
				}
			}
		})
	}
}

func checkLifecycle(t *testing.T, events []LineEvent, count int) {
	t.Helper()
	committed, completed := map[int]bool{}, map[int]bool{}
	for _, e := range events {
		id := e.Line.ID
		if id < 1 || id > count {
			t.Errorf("unexpected ID: %+v", e)
		}
		switch e.Kind {
		case LineCommitted:
			if committed[id] || e.Line.Done {
				t.Errorf("duplicate or already final commit: %+v", e)
			}
			committed[id] = true
		case LineProgress:
			if !committed[id] || completed[id] || e.Line.Done {
				t.Errorf("progress outside line lifetime: %+v", e)
			}
		case LineCompleted:
			if !committed[id] || completed[id] || !e.Line.Done {
				t.Errorf("missing commit or duplicate completion: %+v", e)
			}
			completed[id] = true
		default:
			t.Errorf("unknown event: %+v", e)
		}
	}
	if len(committed) != count || len(completed) != count {
		t.Errorf("committed %d, completed %d; want %d", len(committed), len(completed), count)
	}
}

// Stopping seals the partial output immediately, even if a provider takes time to return after
// cancellation. Wait still joins that provider, and its late deltas/final answer are discarded.
func TestStopPreservesPartialAndWaitsForWorker(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	tr := translateFunc(func(ctx context.Context, _ translate.Request, delta func(string)) (string, error) {
		delta("partial")
		close(started)
		<-ctx.Done()
		<-release
		delta("late delta")
		return "late completion", nil
	})
	var got lifecycleLog
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, OnLine: got.onLine}, got.onState)
	commitSentence(s, 1)
	waitSignal(t, started)
	for id := 2; id <= 12; id++ {
		commitSentence(s, id)
	}
	s.Stop()
	waitFor(t, &got.mu, func() bool { return got.completed() == 12 })
	waited := make(chan struct{})
	go func() { s.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned before the translator finished")
	default:
	}
	close(release)
	waitSignal(t, waited)
	// Stale recognizer events cannot create more committed lines after Stop.
	commitSentence(s, 13)
	checkLifecycle(t, got.events, 12)
	for _, e := range got.events {
		if e.Kind != LineCompleted {
			continue
		}
		if e.Line.Err != context.Canceled.Error() {
			t.Errorf("completion: %+v", e)
		}
		if e.Line.ID == 1 && e.Line.Tgt != "partial" {
			t.Errorf("partial lost: %+v", e)
		}
		if e.Line.ID > 1 && e.Line.Tgt != "" {
			t.Errorf("queued line translated after stop: %+v", e)
		}
	}
	if got.state.Phase != "stopped" {
		t.Errorf("phase = %q", got.state.Phase)
	}
}

func TestParentCancellationJoinsDraftAndRejectsLateResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	tr := translateFunc(func(ctx context.Context, r translate.Request, _ func(string)) (string, error) {
		if !r.Draft {
			t.Error("unexpected committed translation")
		}
		close(started)
		<-ctx.Done()
		<-release
		return "late draft", nil
	})
	var got lifecycleLog
	s := newSession(Config{VolatileSentences: true, Context: ctx, Src: "ja", Tgt: "zh", Translator: tr, Draft: true}, got.onState)
	s.handle(engine.Event{Type: "result", Start: 0, End: 1, Tokens: []engine.Token{{Text: "今日は新しいゲーム"}}})
	waitSignal(t, started)
	cancel()
	waitFor(t, &got.mu, func() bool { return got.state.Phase == "stopped" })
	waited := make(chan struct{})
	go func() { s.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned before the draft finished")
	default:
	}
	close(release)
	waitSignal(t, waited)
	if got.state.Draft != "" || got.state.Err != context.Canceled {
		t.Errorf("late draft or wrong cancellation: %+v", got.state)
	}
}

func TestCallbackCanStopWithoutDeadlock(t *testing.T) {
	stopped := make(chan struct{})
	var s *Session
	var events []LineEvent
	s = newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: upper{}, OnLine: func(e LineEvent) {
		events = append(events, e)
		if e.Kind == LineCommitted {
			s.Stop()
			close(stopped)
		}
	}}, nil)
	commitSentence(s, 1)
	waitSignal(t, stopped)
	s.Wait()
	checkLifecycle(t, events, 1)
}

// A committed sentence retains the already shown draft when its full translation is canceled.
func TestCanceledLineRetainsDraft(t *testing.T) {
	var got lifecycleLog
	tr := translateFunc(func(ctx context.Context, r translate.Request, _ func(string)) (string, error) {
		if r.Draft {
			return "available draft", nil
		}
		<-ctx.Done()
		return "", ctx.Err()
	})
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, Draft: true, OnLine: got.onLine}, got.onState)
	s.handle(engine.Event{Type: "result", Start: 0, End: 1, Tokens: []engine.Token{{Text: "今日は新しいゲーム"}}})
	waitFor(t, &got.mu, func() bool { return got.state.Draft == "available draft" })
	s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日は新しいゲームをやります。"}}})
	s.Stop()
	s.Wait()
	checkLifecycle(t, got.events, 1)
	last := got.events[len(got.events)-1]
	if last.Line.Draft != "available draft" || last.Line.Tgt != "" || last.Line.Err != context.Canceled.Error() {
		t.Errorf("canceled draft not retained: %+v", last)
	}
}

func TestCommittedSentenceDiscardsStaleDraft(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	tr := translateFunc(func(ctx context.Context, r translate.Request, delta func(string)) (string, error) {
		if !r.Draft {
			return "final translation", nil
		}
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return "stale draft", nil
	})
	var got lifecycleLog
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, Draft: true, OnLine: got.onLine}, got.onState)
	s.handle(engine.Event{Type: "result", Start: 0, End: 1, Tokens: []engine.Token{{Text: "今日は新しいゲーム"}}})
	waitSignal(t, started)
	s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日は新しいゲームをやります。"}}})
	close(release)
	waitFor(t, &got.mu, func() bool { return got.completed() == 1 })
	waitFor(t, &s.mu, func() bool { return !s.draftBusy })
	s.Stop()
	s.Wait()
	if got.state.Draft != "" || got.state.Lines[0].Draft != "" {
		t.Errorf("stale draft displayed: %+v", got.state)
	}
	checkLifecycle(t, got.events, 1)
}

type delayedChild struct {
	requested chan struct{}
	done      chan struct{}
	once      sync.Once
}

func (p *delayedChild) Finalize(float64) error { return nil }
func (p *delayedChild) Stop()                  { p.once.Do(func() { close(p.requested) }) }
func (p *delayedChild) Done() <-chan struct{}  { return p.done }
func (p *delayedChild) Err() error             { return nil }

// A child that finishes starting after parent cancellation must still be stopped and joined.
func TestCancellationDuringProcessStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newStartingSession(Config{Context: ctx, Translator: upper{}}, nil)
	cancel()
	child := &delayedChild{requested: make(chan struct{}), done: make(chan struct{})}
	s.installProcesses(child, child)
	close(s.started)
	waitSignal(t, child.requested)
	waitSignal(t, s.workersDone)
	waited := make(chan struct{})
	go func() { s.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned before the child exited")
	default:
	}
	close(child.done)
	waitSignal(t, waited)
}

// Reporting may lag behind capture, so consent/scene metadata belongs to the commitment itself,
// never to the later callback. Every progress/completion must retain that original snapshot.
func TestLineMetadataCapturedBeforeDelayedDispatch(t *testing.T) {
	type metadata struct {
		Scene   string
		Enabled bool
	}
	var source atomic.Pointer[metadata]
	oldPolicy := &metadata{Scene: "old scene", Enabled: true}
	newPolicy := &metadata{Scene: "new scene", Enabled: false}
	source.Store(oldPolicy)
	var captures atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var events []LineEvent
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: upper{}, LineMetadata: func() any {
		captures.Add(1)
		return source.Load()
	}, OnLine: func(ev LineEvent) { events = append(events, ev) }}, func(st State) {
		if st.Phase == "listening" && st.Stats.Lines == 0 {
			close(started)
			<-release
		}
	})
	s.handle(engine.Event{Type: "state", State: "listening"})
	waitSignal(t, started)
	commitSentence(s, 1)
	source.Store(newPolicy)
	commitSentence(s, 2)
	waitFor(t, &s.mu, func() bool { return len(s.active) == 0 })
	s.Stop()
	close(release)
	s.Wait()
	checkLifecycle(t, events, 2)
	if captures.Load() != 2 {
		t.Fatalf("metadata capture count = %d; want once per commit", captures.Load())
	}
	for _, ev := range events {
		want := oldPolicy
		if ev.Line.ID == 2 {
			want = newPolicy
		}
		if ev.Metadata != want {
			t.Errorf("metadata changed between commitment and dispatch: %+v; want %+v", ev, want)
		}
	}
}

type startupRecognizer struct{ *delayedChild }

func (*startupRecognizer) Audio(engine.Event) error { return nil }

func TestFailedStartDrainsSynchronousRecognizerEvents(t *testing.T) {
	callbackStarted, release := make(chan struct{}), make(chan struct{})
	returned := make(chan error, 1)
	var events []LineEvent
	child := &delayedChild{requested: make(chan struct{}), done: make(chan struct{})}
	go func() { <-child.requested; close(child.done) }()
	cfg := Config{Engine: filepath.Join(t.TempDir(), "missing-engine"), Src: "ja", Tgt: "zh", Translator: upper{}, Recognize: func(_ string, onEvent func(engine.Event)) (asr.Recognizer, error) {
		start, end := 0.0, 1.0
		onEvent(engine.Event{Type: "result", Final: true, Tokens: []engine.Token{{Text: "これは最初の文です。", Start: &start, End: &end}}})
		return &startupRecognizer{child}, nil
	}, OnLine: func(ev LineEvent) {
		events = append(events, ev)
		if ev.Kind == LineCommitted {
			close(callbackStarted)
			<-release
		}
	}}
	go func() {
		s, err := Start(cfg, nil)
		if s != nil {
			s.Stop()
			s.Wait()
		}
		returned <- err
	}()
	waitSignal(t, callbackStarted)
	select {
	case <-returned:
		t.Fatal("failed Start returned while its line callback was still running")
	default:
	}
	close(release)
	select {
	case err := <-returned:
		if err == nil {
			t.Fatal("missing engine did not fail startup")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("startup cleanup did not finish")
	}
	// Reading without an extra callback lock also makes the race detector verify that no
	// dispatcher can continue mutating this stream after the failed constructor returns.
	checkLifecycle(t, events, 1)
	select {
	case <-child.done:
	default:
		t.Fatal("failed Start left its recognizer alive")
	}
}
