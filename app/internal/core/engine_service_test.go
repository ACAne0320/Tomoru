package core

import (
	"context"
	"testing"
	"time"

	"tomoru/internal/prefs"
)

func engineServiceHarness(t *testing.T) (*EngineService, chan func()) {
	t.Helper()
	posted := make(chan func(), 8)
	work := newTasks(context.Background(), func(fn func()) { posted <- fn })
	s := newEngineService(work, nil, nil, nil, nil, nil)
	t.Cleanup(func() {
		s.stopTests()
		if !work.close(time.Second) {
			t.Error("endpoint test worker did not end")
		}
	})
	return s, posted
}

func takeEngineResult(t *testing.T, posted <-chan func()) func() {
	t.Helper()
	select {
	case fn := <-posted:
		return fn
	case <-time.After(time.Second):
		t.Fatal("endpoint test did not report")
		return nil
	}
}

func TestForgetEndpointTestCancelsAndRejectsLateResult(t *testing.T) {
	s, posted := engineServiceHarness(t)
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	s.startTest("dialog", time.Minute, func(ctx context.Context) TestResult {
		started <- ctx
		<-release // A misbehaving provider still returns a successful result after cancellation.
		return TestResult{Text: "late"}
	}, nil)
	ctx := <-started
	s.ForgetTest("dialog")
	select {
	case <-ctx.Done():
	default:
		t.Fatal("forget did not cancel the active request")
	}
	close(release)
	takeEngineResult(t, posted)()
	if _, ok := s.Tests["dialog"]; ok {
		t.Fatal("forgotten result reappeared")
	}

	// Forgetting after the worker already posted is equally final.
	s.startTest("dialog", time.Minute, func(context.Context) TestResult { return TestResult{Text: "queued"} }, nil)
	queued := takeEngineResult(t, posted)
	s.ForgetTest("dialog")
	queued()
	if _, ok := s.Tests["dialog"]; ok {
		t.Fatal("queued result reappeared after forget")
	}
}

func TestEndpointTestReplacementKeepsNewestResult(t *testing.T) {
	s, posted := engineServiceHarness(t)
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	s.startTest("same", time.Minute, func(ctx context.Context) TestResult { started <- ctx; <-release; return TestResult{Text: "old"} }, nil)
	oldCtx := <-started
	s.startTest("same", time.Minute, func(context.Context) TestResult { return TestResult{Text: "new"} }, nil)
	select {
	case <-oldCtx.Done():
	default:
		t.Fatal("replacement left old request active")
	}
	takeEngineResult(t, posted)()
	if got := s.Tests["same"]; got.Running || got.Text != "new" || got.Err != nil {
		t.Fatalf("new result lost: %+v", got)
	}
	close(release)
	takeEngineResult(t, posted)()
	if s.Tests["same"].Text != "new" {
		t.Fatal("old request overwrote replacement")
	}
}

func TestEngineShutdownInvalidatesQueuedTests(t *testing.T) {
	s, posted := engineServiceHarness(t)
	s.startTest("closing", time.Minute, func(context.Context) TestResult { return TestResult{Text: "late"} }, nil)
	queued := takeEngineResult(t, posted)
	s.stopTests()
	if !s.work.close(time.Second) {
		t.Fatal("shutdown did not wait for worker")
	}
	queued()
	if got := s.Tests["closing"]; got.Running || got.Text != "" || got.Err != context.Canceled {
		t.Fatalf("shutdown result changed: %+v", got)
	}
	s.startTest("after", time.Minute, func(context.Context) TestResult { t.Error("started after shutdown"); return TestResult{} }, nil)
	if _, ok := s.Tests["after"]; ok {
		t.Fatal("test created after shutdown")
	}
}

func TestEngineServiceOwnsDeferredSelection(t *testing.T) {
	s, _ := engineServiceHarness(t)
	s.models = &fakeModels{ready: map[string]bool{}}
	var local Engine
	for _, e := range engineCatalog(MT, nil) {
		if e.ID == "local-2b" {
			local = e
		}
	}
	if s.choose(local, nil) || s.chosen(MT, "cloud") != "local-2b" {
		t.Fatal("missing model was not deferred")
	}
	if got := s.readyPending(nil, nil); len(got) != 0 {
		t.Fatal("missing model became usable")
	}
	s.Download("local-2b")
	if got := s.readyPending(nil, nil); len(got) != 1 || got[0].ID != "local-2b" {
		t.Fatalf("download readiness lost: %+v", got)
	}
	if s.chosen(MT, "cloud") != "cloud" {
		t.Fatal("consumed pending choice remained")
	}
	s.pending[MT] = prefs.CustomPrefix + "removed"
	s.readyPending(nil, nil)
	if len(s.pending) != 0 {
		t.Fatal("deleted provider remained pending")
	}
}
