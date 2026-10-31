package models

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServerWaitCancelsRequests(t *testing.T) {
	for _, stage := range []string{"/health", "/v1/models"} {
		t.Run(stage, func(t *testing.T) {
			started := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != stage {
					w.WriteHeader(http.StatusOK)
					return
				}
				close(started)
				if stage == "/v1/models" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer srv.Close()
			p := &process{url: srv.URL, model: "test.gguf", done: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- p.wait(ctx) }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request never started")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled request: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not interrupt the request")
			}
		})
	}
}

func TestServerExitInterruptsHealth(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	exited := make(chan struct{})
	p := &process{url: srv.URL, done: exited}
	done := make(chan error, 1)
	go func() { done <- p.wait(context.Background()) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request never started")
	}
	close(exited)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("exited process reported ready")
		}
	case <-time.After(time.Second):
		t.Fatal("process exit did not interrupt the request")
	}
}

func TestServerRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	_, err := serverRequest(context.Background(), &http.Client{Timeout: 25 * time.Millisecond}, srv.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unresponsive health request: %v", err)
	}
}

// TestSleeper is the child of the tests that need one that runs until stopped: this test binary,
// which sleeps, on every platform.
func TestSleeper(t *testing.T) {
	if os.Getenv("TOMORU_SLEEPER") == "" {
		t.Skip("the child of TestServerStopReapsChild")
	}
	time.Sleep(time.Minute)
}

func TestServerStopReapsChild(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestSleeper$")
	cmd.Env = append(os.Environ(), "TOMORU_SLEEPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	s := &Server{cur: &process{cmd: cmd, done: done}}
	t.Cleanup(s.Stop)
	s.Stop()
	if s.cur != nil {
		t.Fatal("stopped server remains current")
	}
	select {
	case <-done:
	default:
		t.Fatal("Stop returned before reaping the child")
	}
	s.Stop() // stopping twice is safe
}

func TestEnsureCancelledDoesNotStart(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	script := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	s := NewServer(script, nil)
	defer s.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Ensure(ctx, "model"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("cancelled request started a process")
	}
}

// Queued Ensure calls must recheck cancellation after waiting for a stopping
// process to release the server lock. Starting even briefly would revive a child
// after the application has already stopped its server.
func TestEnsureCancelledWhileWaitingForLock(t *testing.T) {
	s := NewServer("does-not-exist", nil)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &checkedContext{Context: parent, checked: make(chan struct{})}
	s.mu.Lock()
	done := make(chan error, 1)
	go func() { _, err := s.Ensure(ctx, "model"); done <- err }()
	<-ctx.checked // Ensure observed a live context before it waited for the lock.
	cancel()
	s.mu.Unlock()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Ensure attempted to start: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled Ensure stayed blocked")
	}
}

// checkedContext signals after reading the first Err result, before returning it.
type checkedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *checkedContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

// TestWarm: warming asks for a translation, with a picture once the vision projector is loaded.
func TestWarm(t *testing.T) {
	for _, vision := range []string{"", "mmproj.gguf"} {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
				t.Errorf("%s %s", r.Method, r.URL.Path)
			}
			json.NewDecoder(r.Body).Decode(&got)
			w.Write([]byte(`{"choices":[]}`))
		}))
		p := &process{url: srv.URL, vision: vision, done: make(chan struct{})}
		if err := p.warm(nil); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		content := got["messages"].([]any)[0].(map[string]any)["content"]
		parts, picture := content.([]any)
		if picture != (vision != "") {
			t.Errorf("vision %q: content %v", vision, content)
		}
		if picture && !strings.HasPrefix(parts[0].(map[string]any)["image_url"].(map[string]any)["url"].(string), "data:image/png;base64,") {
			t.Errorf("picture %v", parts[0])
		}
	}
}

// TestWarmExit: a server that exits while warming stops it.
func TestWarmExit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body) // read, the request's going is seen
		<-r.Context().Done()
	}))
	defer srv.Close()
	exited := make(chan struct{})
	p := &process{url: srv.URL, done: exited}
	done := make(chan error, 1)
	go func() { done <- p.warm(nil) }()
	time.Sleep(50 * time.Millisecond)
	close(exited)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("exited server warmed")
		}
	case <-time.After(time.Second):
		t.Fatal("exit did not stop warming")
	}
}
