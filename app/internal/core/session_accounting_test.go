package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func TestFailedStartupRejectsItsQueuedStopCallback(t *testing.T) {
	c, f := newCore(t)
	want := errors.New("capture could not start")
	c.SessionController.startSession = func(_ live.Config, report func(live.State)) (Session, error) {
		report(live.State{Phase: "stopped"})
		return nil, want
	}
	c.Start()
	f.flush(t, 1)
	if c.Err != want || c.Running {
		t.Fatalf("startup failure lost: %v, running=%v", c.Err, c.Running)
	}
	f.flush(t, 1)
	if c.Err != want || c.Running || IsExited(c.Err) {
		t.Fatalf("queued stop replaced startup failure: %v", c.Err)
	}
}

func TestStopAccountsForRuntimeReportsStillQueuedForUI(t *testing.T) {
	c, f := newCore(t)
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded = true })
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "listening", Stats: live.Stats{Lines: 5, Chars: 30}})
	// Do not flush the app queue. Stop invalidates those UI updates but must keep their totals.
	c.Stop()
	if got := c.TodayStats(); got.Lines != 5 || got.Chars != 30 {
		t.Fatalf("queued usage lost: %+v", got)
	}
	f.flush(t, 1)
	if got := c.TodayStats(); got.Lines != 5 || got.Chars != 30 {
		t.Fatalf("retired report changed totals: %+v", got)
	}
}

type reportingStopSession struct{ stop, wait func() }

func (s reportingStopSession) Stop() { s.stop() }
func (s reportingStopSession) Wait() { s.wait() }

func TestStopAccountsForFinalReportBeforeWaitReturns(t *testing.T) {
	c, f := newCore(t)
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded = true })
	c.SessionController.startSession = func(_ live.Config, report func(live.State)) (Session, error) {
		return reportingStopSession{
			stop: func() { report(live.State{Phase: "stopped", Stats: live.Stats{Lines: 3}}) },
			wait: func() { report(live.State{Phase: "stopped", Stats: live.Stats{Lines: 3, Chars: 15}}) },
		}, nil
	}
	c.Start()
	f.flush(t, 1)
	c.Shutdown()
	if got := c.TodayStats(); got.Lines != 3 || got.Chars != 15 {
		t.Fatalf("final usage lost during shutdown: %+v", got)
	}
}

func TestSessionPreparationRunsWithCancellableContext(t *testing.T) {
	c, f := newCore(t)
	started, ended := make(chan struct{}), make(chan struct{})
	c.SessionController.start(sessionRequest{
		Prepare: func(ctx context.Context, cfg *live.Config) error {
			close(started)
			<-ctx.Done()
			close(ended)
			return ctx.Err()
		},
		Ready: func(context.Context) (translate.Translator, error) {
			t.Error("translator prepared after cancellation")
			return translate.Cloud(), nil
		},
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("preparation blocked application thread")
	}
	c.Stop()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("preparation was not canceled")
	}
	c.work.wg.Wait()
	f.flush(t, 1)
	if len(f.sessions) != 0 || c.Running {
		t.Fatal("canceled preparation started a runtime")
	}
}

func TestSessionPreparationUpdatesOnlyItsConfigSnapshot(t *testing.T) {
	c, f := newCore(t)
	config := live.Config{Src: "en", Tgt: "zh"}
	c.SessionController.start(sessionRequest{
		Config:  config,
		Prepare: func(_ context.Context, cfg *live.Config) error { cfg.Src = "ja"; return nil },
		Ready:   func(context.Context) (translate.Translator, error) { return translate.Cloud(), nil },
	})
	f.flush(t, 1)
	if config.Src != "en" || len(f.sessions) != 1 || f.sessions[0].cfg.Src != "ja" {
		t.Fatal("prepared config was not isolated or applied")
	}
	c.Stop()
}
