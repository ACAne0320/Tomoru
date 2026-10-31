package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerCloseCancelsAndWaits(t *testing.T) {
	withModel(t, []byte("model"), sha([]byte("model")))
	started, stopped := make(chan struct{}), make(chan struct{})
	var requests, changes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), false, func() { changes.Add(1) })
	redirect(m, srv)
	m.Download("m")
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start")
	}
	m.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("download request not cancelled")
	}
	m.mu.Lock()
	jobs := len(m.jobs)
	m.mu.Unlock()
	if jobs != 0 {
		t.Fatal("Close returned before the download worker stopped")
	}
	before := changes.Load()
	m.Download("m")
	if changes.Load() != before || requests.Load() != 1 {
		t.Fatal("closed manager accepted new work")
	}
	m.Close()
}

func TestManagerParentCancellation(t *testing.T) {
	withModel(t, []byte("model"), sha([]byte("model")))
	ctx, cancel := context.WithCancel(context.Background())
	m := NewManagerContext(ctx, t.TempDir(), false, nil)
	defer m.Close()
	cancel()
	m.Download("m")
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.jobs) != 0 {
		t.Fatal("cancelled application started a download")
	}
}
