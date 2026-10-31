package shell

import (
	"context"
	"sync"

	"github.com/egoist/mygo"
)

// Post schedules callbacks in FIFO order without waiting for the main thread. The
// caller can therefore finish a worker while shutdown waits on the main thread.
// Cancelling ctx rejects new work and invalidates already queued callbacks.
func Post(ctx context.Context) func(func()) {
	post, _ := postQueue(ctx, mygo.RunOnMain)
	return post
}

func postQueue(ctx context.Context, dispatch func(func())) (func(func()), <-chan struct{}) {
	var mu sync.Mutex
	var queue []func()
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { mu.Lock(); queue = nil; mu.Unlock() }()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wake:
			}
			for ctx.Err() == nil {
				mu.Lock()
				batch := queue
				queue = nil
				mu.Unlock()
				if len(batch) == 0 {
					break
				}
				for _, fn := range batch {
					if ctx.Err() != nil {
						return
					}
					dispatch(func() {
						if ctx.Err() == nil {
							fn()
						}
					})
				}
			}
		}
	}()
	return func(fn func()) {
		if fn == nil {
			return
		}
		mu.Lock()
		if ctx.Err() != nil {
			mu.Unlock()
			return
		}
		queue = append(queue, fn)
		mu.Unlock()
		select {
		case wake <- struct{}{}:
		default:
		}
	}, done
}
