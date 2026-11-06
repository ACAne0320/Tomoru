package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestDecode sends the stretch as a WAV, with the model, the language and the text before it, and
// reads the answer.
func TestDecode(t *testing.T) {
	var got []map[string]string
	var auth, path string
	var audio []byte
	answers := []string{`{"text":"その後、大事はないか。"}`, `{"text":" Hello."}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		fields := map[string]string{}
		for k, v := range r.MultipartForm.Value {
			fields[k] = v[0]
		}
		got = append(got, fields)
		f, h, err := r.FormFile("file")
		if err != nil || !strings.HasSuffix(h.Filename, ".wav") {
			t.Errorf("file %v %v", h, err)
		} else {
			audio, _ = io.ReadAll(f)
		}
		w.Write([]byte(answers[len(got)-1]))
	}))
	defer srv.Close()
	d := New(Config{URL: srv.URL + "/v1/", Key: "sk-1"}, "zh-Hant")
	defer d.Stop()
	ps, err := d.Decode(context.Background(), make([]byte, 3200))
	if err != nil || len(ps) != 11 || ps[0].Text != "そ" || ps[10].Text != "。" {
		t.Fatalf("%v %v", ps, err)
	}
	if path != "/v1/audio/transcriptions" || auth != "Bearer sk-1" || len(audio) != 44+3200 || string(audio[:4]) != "RIFF" {
		t.Fatalf("path %q, auth %q, %d bytes", path, auth, len(audio))
	}
	f := got[0]
	if f["model"] != Models[0] || f["language"] != "zh" || f["response_format"] != "json" || f["prompt"] != "" {
		t.Fatalf("fields %v", f)
	}
	if _, err := d.Decode(context.Background(), make([]byte, 3200)); err != nil {
		t.Fatal(err)
	}
	if got[1]["prompt"] != "その後、大事はないか。" {
		t.Errorf("the next decode's prompt %q", got[1]["prompt"])
	}
}

func TestISO639(t *testing.T) {
	for in, want := range map[string]string{"ja": "ja", "zh": "zh", "zh-Hant": "zh", "yue": "", "fil": "", "pt": "pt"} {
		if got := iso639(in); got != want {
			t.Errorf("iso639(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTail(t *testing.T) {
	if got := tail("短い。", 10); got != "短い。" {
		t.Error(got)
	}
	if got := tail("one two three four five six", 12); got != "five six" {
		t.Errorf("%q", got)
	}
}

// TestErrors tells a refused key, says the service's message, and tries a decode the server
// failed once more.
func TestErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer bad":
			http.Error(w, `{"error":{"message":"Incorrect API key provided"}}`, http.StatusUnauthorized)
		case "Bearer model":
			http.Error(w, `{"error":{"message":"The model does not exist"}}`, http.StatusBadRequest)
		default:
			if calls.Add(1) == 1 {
				http.Error(w, "busy", http.StatusBadGateway)
				return
			}
			w.Write([]byte(`{"text":"ok"}`))
		}
	}))
	defer srv.Close()
	if _, err := New(Config{URL: srv.URL, Key: "bad"}, "en").Decode(context.Background(), nil); !errors.Is(err, ErrAuth) {
		t.Errorf("a refused key: %v", err)
	}
	var e *Error
	if _, err := New(Config{URL: srv.URL, Key: "model"}, "en").Decode(context.Background(), nil); !errors.As(err, &e) || e.Message != "The model does not exist" {
		t.Errorf("a bad model: %v", err)
	}
	if ps, err := New(Config{URL: srv.URL, Key: "ok"}, "en").Decode(context.Background(), nil); err != nil || len(ps) != 1 || calls.Load() != 2 {
		t.Errorf("tried again: %v %v, %d calls", ps, err, calls.Load())
	}
	if took, err := Test(context.Background(), Config{URL: srv.URL, Key: "ok"}); err != nil || took < 0 { // 0 where the clock is coarse, as on Windows
		t.Errorf("test: %v %v", took, err)
	}
}

// TestStop cancels a decode in flight.
func TestStop(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	d := New(Config{URL: srv.URL}, "en")
	errc := make(chan error, 1)
	go func() { _, err := d.Decode(context.Background(), nil); errc <- err }()
	time.Sleep(50 * time.Millisecond)
	d.Stop()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("a stopped decode succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not cancel the decode")
	}
}

// TestKeywords gives gpt-transcribe the scene's names as keywords, and another model in its prompt,
// before the text before.
func TestKeywords(t *testing.T) {
	var got []map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(1 << 20)
		got = append(got, r.MultipartForm.Value)
		w.Write([]byte(`{"text":"一心様より。"}`))
	}))
	defer srv.Close()
	names := []string{"一心", "葦<名>", "竜\n胤"}
	New(Config{URL: srv.URL, Model: "gpt-transcribe", Keywords: names}, "ja").Decode(context.Background(), nil)
	d := New(Config{URL: srv.URL, Model: "whisper-large-v3", Keywords: names}, "ja")
	d.Decode(context.Background(), nil)
	d.Decode(context.Background(), nil)
	if k := got[0]["keywords[]"]; strings.Join(k, "|") != "一心|葦名|竜 胤" || got[0]["prompt"] != nil {
		t.Errorf("gpt-transcribe: %v", got[0])
	}
	if p := got[1]["prompt"]; len(p) != 1 || p[0] != "一心, 葦名, 竜 胤." || got[1]["keywords[]"] != nil {
		t.Errorf("another model: %v", got[1])
	}
	if p := got[2]["prompt"]; len(p) != 1 || p[0] != "一心, 葦名, 竜 胤. 一心様より。" {
		t.Errorf("then: %v", got[2]["prompt"])
	}
}
