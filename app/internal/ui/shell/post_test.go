package shell

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestPostDoesNotBlockBehindMainThread(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var got []int
	post, done := postQueue(ctx, func(fn func()) { once.Do(func() { close(entered); <-release }); fn() })
	post(func() { got = append(got, 0) })
	<-entered
	enqueued := make(chan struct{})
	go func() {
		for i := 1; i < 2000; i++ {
			post(func() { got = append(got, i) })
		}
		close(enqueued)
	}()
	select {
	case <-enqueued:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("posting blocks while main thread is busy")
	}
	finished := make(chan struct{})
	post(func() { close(finished) })
	close(release)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("callbacks not drained")
	}
	want := make([]int, 2000)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(got, want) {
		t.Fatalf("FIFO lost callbacks: got %d", len(got))
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue did not stop")
	}
}

func TestPostDiscardsCallbacksAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	ran := false
	post, done := postQueue(ctx, func(fn func()) { close(entered); <-release; fn() })
	post(func() { ran = true })
	<-entered
	post(func() { ran = true })
	cancel()
	post(func() { ran = true })
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled queue did not stop")
	}
	if ran {
		t.Fatal("callback ran after cancellation")
	}
}
