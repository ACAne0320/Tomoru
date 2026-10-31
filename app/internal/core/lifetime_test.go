package core

import (
	"context"
	"testing"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func TestShutdownCancelsSystemWorkAndDiscardsQueuedCallbacks(t *testing.T) {
	c, f := newCore(t)
	started, stopped := make(chan struct{}), make(chan struct{})
	c.deps.RunEngine = func(ctx context.Context, _ string, _ ...string) ([]engine.Event, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return []engine.Event{{Type: "access", State: "granted"}}, nil
	}
	c.CheckAccess()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker not started")
	}
	c.work.Post(func() { c.Access = "late mutation" })
	c.Shutdown()
	select {
	case <-stopped:
	default:
		t.Fatal("shutdown did not join worker")
	}
	if c.Context().Err() == nil {
		t.Fatal("root still active")
	}
	f.flush(t, 1)
	if c.Access != "" {
		t.Fatalf("late callback applied: %q", c.Access)
	}
	c.Start()
	if c.Running {
		t.Fatal("started after shutdown")
	}
}

func TestSessionReplacementRejectsLatePreparation(t *testing.T) {
	c, f := newCore(t)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	c.SessionController.start(sessionRequest{Ready: func(context.Context) (translate.Translator, error) {
		close(started)
		<-release
		close(done)
		return translate.Cloud(), nil
	}})
	<-started
	c.Stop()
	c.Start()
	f.until(t, func() bool { return len(f.sessions) == 1 })
	close(release)
	<-done
	// Worker cancellation can race queue delivery; joining ensures its late post is present.
	c.work.wg.Wait()
	f.mu.Lock()
	queued := f.posted
	f.posted = nil
	f.mu.Unlock()
	for _, fn := range queued {
		fn()
	}
	if len(f.sessions) != 1 {
		t.Fatalf("stale preparation started %d sessions", len(f.sessions))
	}
	c.Stop()
}

func TestNaturalStopDrainsFinalEvents(t *testing.T) {
	c, f, s := recordingCore(t)
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: 1, Src: "last"}})
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 1, Src: "last", Tgt: "final", Done: true}})
	s.report(live.State{Phase: "stopped"})
	f.until(t, func() bool { return !c.Running })
	if es := recordedEntries(t, c); len(es) != 1 || es[0].Tgt != "final" {
		t.Fatalf("lost final line: %+v", es)
	}
}

type blockingCredentials struct {
	started chan struct{}
	release chan struct{}
}

func (s *blockingCredentials) Get(string) ([]byte, error) {
	close(s.started)
	<-s.release
	return []byte("test-key"), nil
}
func (*blockingCredentials) Set(string, []byte) error { return nil }
func (*blockingCredentials) Delete(string) error      { return nil }

func TestCloudCredentialPreparationDoesNotBlockApplicationThread(t *testing.T) {
	c, f := newCore(t)
	secrets := &blockingCredentials{started: make(chan struct{}), release: make(chan struct{})}
	c.EngineService.secrets = secrets
	c.UpdatePrefs(func(p *prefs.Prefs) { p.ASR = QwenCloud; p.ASRAPIs = []prefs.ASRAPI{{ID: QwenCloud, Key: true}} })
	returned := make(chan struct{})
	// This goroutine stands in for the application thread. No test accesses Core
	// until its return is synchronized; credential IO must run on another worker.
	go func() { c.Start(); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		close(secrets.release)
		t.Fatal("Start blocked on credentials")
	}
	select {
	case <-secrets.started:
	case <-time.After(time.Second):
		close(secrets.release)
		t.Fatal("credentials were not prepared")
	}
	close(secrets.release)
	f.until(t, func() bool { return c.session != nil })
	if f.sessions[0].cfg.Recognize == nil {
		t.Fatal("prepared cloud recognizer missing")
	}
	c.Shutdown()
}
