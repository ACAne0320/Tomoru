package core

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"tomoru/internal/learn"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

type askFunc func(context.Context, string, string, int) (string, error)

func (f askFunc) Chat(ctx context.Context, system, user string, max int) (string, error) {
	return f(ctx, system, user, max)
}

type retranslateFunc func(context.Context, translate.Request, func(string)) (string, error)

func (f retranslateFunc) Translate(ctx context.Context, r translate.Request, delta func(string)) (string, error) {
	return f(ctx, r, delta)
}

func newLearningHarness(t *testing.T) (*LearningService, *fake) {
	t.Helper()
	f := &fake{}
	work := newTasks(context.Background(), func(fn func()) {
		f.mu.Lock()
		f.posted = append(f.posted, fn)
		f.mu.Unlock()
	})
	s := &LearningService{work: work}
	t.Cleanup(func() {
		s.CancelLearning()
		s.CancelRetranslating()
		if !work.close(time.Second) {
			t.Error("learning worker did not stop")
		}
	})
	return s, f
}

func waitLearningSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("learning worker timed out")
	}
}

func joinLearningWorkers(t *testing.T, s *LearningService) {
	t.Helper()
	done := make(chan struct{})
	go func() { s.work.wg.Wait(); close(done) }()
	waitLearningSignal(t, done)
}

func drainLearningPosts(f *fake) {
	f.mu.Lock()
	posted := f.posted
	f.posted = nil
	f.mu.Unlock()
	for _, fn := range posted {
		fn()
	}
}

func TestLearningServiceSnapshotsAndDeferredResults(t *testing.T) {
	s, f := newLearningHarness(t)
	path := keepSession(t, t.TempDir(), "scene", time.Now(), "在刘北赛道上")
	started, release := make(chan struct{}), make(chan struct{})
	var prompt string
	r := learningRequest{
		Paths: []string{path}, Scene: "original",
		Context: learn.Context{Name: "original scene", Terms: []scene.Term{{Src: "existing name", Heard: []string{"heard original"}}}, Rejected: []string{"rejected original"}},
		Ready: func(context.Context) (learn.Asker, error) {
			close(started)
			<-release
			return askFunc(func(_ context.Context, _, user string, _ int) (string, error) {
				prompt = user
				return `{"terms":[{"heard":"刘北","src":"纽北","tgt":"Nürburgring","sure":true}]}`, nil
			}), nil
		},
	}
	s.startLearning(r)
	waitLearningSignal(t, started)
	r.Paths[0] = "missing"
	r.Context.Terms[0].Src = "mutated term"
	r.Context.Rejected[0] = "mutated rejection"
	s.Learning.Paths[0] = "also missing"
	close(release)
	joinLearningWorkers(t, s)
	// The worker's deferred context cleanup has already run; successful queued UI results
	// must still be accepted because this operation was not explicitly canceled.
	if s.learning.ctx.Err() == nil {
		t.Fatal("worker did not release its context")
	}
	drainLearningPosts(f)
	if !s.Learning.Finished || s.Learning.Running || s.Learning.Done != 1 || len(s.Learning.Suggestions) != 1 {
		t.Fatalf("queued successful result lost: %+v", s.Learning)
	}
	if strings.Contains(prompt, "mutated") || !strings.Contains(prompt, "existing name") || !strings.Contains(prompt, "rejected original") {
		t.Errorf("request read mutable settings: %q", prompt)
	}
}

func TestLearningReplacementRejectsLateModelResult(t *testing.T) {
	s, f := newLearningHarness(t)
	path := keepSession(t, t.TempDir(), "scene", time.Now(), "在刘北赛道上")
	started, release := make(chan struct{}), make(chan struct{})
	s.startLearning(learningRequest{Paths: []string{path}, Scene: "old", Ready: func(context.Context) (learn.Asker, error) {
		return askFunc(func(ctx context.Context, _, _ string, _ int) (string, error) {
			close(started)
			<-release // emulate a provider returning a successful result after cancellation
			return `{"terms":[{"heard":"刘北","src":"纽北","tgt":"old","sure":true}]}`, nil
		}), nil
	}})
	waitLearningSignal(t, started)
	s.startLearning(learningRequest{Paths: []string{path}, Scene: "new", Ready: func(context.Context) (learn.Asker, error) {
		return askFunc(func(context.Context, string, string, int) (string, error) {
			return `{"terms":[{"heard":"刘北","src":"纽北","tgt":"new","sure":true}]}`, nil
		}), nil
	}})
	close(release)
	joinLearningWorkers(t, s)
	drainLearningPosts(f)
	if s.Learning.Scene != "new" || !s.Learning.Finished || s.Learning.Done != 1 || len(s.Learning.Suggestions) != 1 || s.Learning.Suggestions[0].Tgt != "new" {
		t.Fatalf("replaced run changed the new learning state: %+v", s.Learning)
	}
}

func TestRetranslationCancellationPreventsWriteAndRefresh(t *testing.T) {
	s, f := newLearningHarness(t)
	path := keepSession(t, t.TempDir(), "scene", time.Now(), "在刘北赛道上")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	refreshes := 0
	s.rewritten = func() { refreshes++ }
	s.startRetranslating(retranslationRequest{Paths: []string{path}, Terms: []scene.Term{{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}}}, Ready: func(context.Context) (translate.Translator, error) {
		return retranslateFunc(func(ctx context.Context, _ translate.Request, _ func(string)) (string, error) {
			close(started)
			<-ctx.Done()
			return "late successful result", nil
		}), nil
	}})
	waitLearningSignal(t, started)
	s.CancelRetranslating()
	joinLearningWorkers(t, s)
	drainLearningPosts(f)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("canceled retranslation rewrote history")
	}
	if refreshes != 0 || s.Retranslating.Done != 0 || s.Retranslating.Running || !s.Retranslating.Finished {
		t.Errorf("canceled result refreshed or changed progress: %+v; refreshes=%d", s.Retranslating, refreshes)
	}
}

func TestRetranslationSnapshotsAndQueuedRefreshGeneration(t *testing.T) {
	s, f := newLearningHarness(t)
	path := keepSession(t, t.TempDir(), "scene", time.Now(), "在刘北赛道上")
	started, release := make(chan struct{}), make(chan struct{})
	refreshes := 0
	s.rewritten = func() { refreshes++ }
	r := retranslationRequest{Paths: []string{path}, Terms: []scene.Term{{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}}}, Ready: func(context.Context) (translate.Translator, error) {
		close(started)
		<-release
		return marked{}, nil
	}}
	s.startRetranslating(r)
	waitLearningSignal(t, started)
	r.Paths[0] = "missing"
	r.Terms[0].Heard[0] = "something else"
	r.Terms[0].Tgt = "mutated"
	close(release)
	joinLearningWorkers(t, s)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte("Nürburgring")) || bytes.Contains(after, []byte("mutated")) {
		t.Errorf("glossary snapshot not preserved: %s", after)
	}
	// The write completed, but its UI notifications are still queued. Starting a new operation
	// must discard the old final progress and refresh, without undoing its committed file.
	s.startRetranslating(retranslationRequest{})
	drainLearningPosts(f)
	if refreshes != 0 || s.Retranslating.Total != 0 || !s.Retranslating.Finished {
		t.Errorf("old refresh escaped: %+v; refreshes=%d", s.Retranslating, refreshes)
	}
}

func TestLearningAndRetranslationShutdown(t *testing.T) {
	s, f := newLearningHarness(t)
	path := keepSession(t, t.TempDir(), "scene", time.Now(), "在刘北赛道上")
	learningStarted, retranslationStarted := make(chan struct{}), make(chan struct{})
	s.startLearning(learningRequest{Paths: []string{path}, Ready: func(context.Context) (learn.Asker, error) {
		return askFunc(func(ctx context.Context, _ string, _ string, _ int) (string, error) {
			close(learningStarted)
			<-ctx.Done()
			return "", ctx.Err()
		}), nil
	}})
	s.startRetranslating(retranslationRequest{Paths: []string{path}, Terms: []scene.Term{{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}}}, Ready: func(context.Context) (translate.Translator, error) {
		return retranslateFunc(func(ctx context.Context, _ translate.Request, _ func(string)) (string, error) {
			close(retranslationStarted)
			<-ctx.Done()
			return "", ctx.Err()
		}), nil
	}})
	waitLearningSignal(t, learningStarted)
	waitLearningSignal(t, retranslationStarted)
	// The same actions used by Core.Shutdown cancel each feature before joining root workers.
	s.CancelLearning()
	s.CancelRetranslating()
	if !s.work.close(time.Second) {
		t.Fatal("shutdown failed to join model operations")
	}
	drainLearningPosts(f)
	if s.Learning.Running || s.Retranslating.Running || len(s.Learning.Suggestions) != 0 || s.Retranslating.Done != 0 {
		t.Errorf("shutdown accepted queued results: %+v / %+v", s.Learning, s.Retranslating)
	}
}
