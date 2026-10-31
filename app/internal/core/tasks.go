package core

import (
	"context"
	"sync"
	"time"
)

// tasks owns application workers. Cancelling also invalidates callbacks already
// queued for the main thread; workers never need that thread to finish.
type tasks struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	wg       sync.WaitGroup
	postMain func(func())
}

func newTasks(parent context.Context, post func(func())) *tasks {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &tasks{ctx: ctx, cancel: cancel, postMain: post}
}

func (t *tasks) Go(fn func()) {
	t.mu.Lock()
	if t.closed || t.ctx.Err() != nil {
		t.mu.Unlock()
		return
	}
	t.wg.Add(1)
	t.mu.Unlock()
	go func() { defer t.wg.Done(); fn() }()
}

func (t *tasks) Post(fn func()) {
	if t.ctx.Err() != nil {
		return
	}
	t.postMain(func() {
		if t.ctx.Err() == nil {
			fn()
		}
	})
}

func (t *tasks) close(timeout time.Duration) bool {
	t.mu.Lock()
	t.closed = true
	t.cancel()
	t.mu.Unlock()
	done := make(chan struct{})
	go func() { t.wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
