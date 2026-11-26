package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tomoru/internal/resource"
)

func withModel(t *testing.T, data []byte, sum string) {
	t.Helper()
	old := All
	All = []Model{{ID: "m", Files: []File{{"m.gguf", int64(len(data)), sum}}, Size: int64(len(data)), Repo: "r"}}
	t.Cleanup(func() { All = old })
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func server(data []byte, fails int) (*httptest.Server, *int) {
	var mu sync.Mutex
	n := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		fail := n <= fails
		mu.Unlock()
		if fail {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		from := 0
		if rg := r.Header.Get("Range"); rg != "" {
			from, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
		}
		w.Write(data[from:])
	})), &n
}

func wait(t *testing.T, m *Manager, id string, want State) Status {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		st := m.Status(id)
		m.mu.Lock()
		idle := m.jobs[id] == nil
		m.mu.Unlock()
		if st.State == want && idle {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("status %+v, want %v", st, want)
		}
	}
}

func redirect(m *Manager, srv *httptest.Server) {
	m.client = &http.Client{Transport: rewrite{srv.URL}}
}

type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	req = req.Clone(req.Context())
	req.URL, _ = req.URL.Parse(r.to + u.Path)
	req.Host = ""
	return http.DefaultTransport.RoundTrip(req)
}

func TestDownload(t *testing.T) {
	data := []byte(strings.Repeat("tomoru ", 100000))
	withModel(t, data, sha(data))
	dir := t.TempDir()
	srv, _ := server(data, 1)
	defer srv.Close()
	m := NewManager(dir, HuggingFaceFirst, nil)
	redirect(m, srv)

	os.WriteFile(dir+"/m.gguf.part", data[:len(data)/2], 0o644)
	if st := m.Status("m"); st.State != Missing || st.Done != int64(len(data)/2) {
		t.Fatalf("before: %+v", st)
	}
	m.Download("m")
	st := wait(t, m, "m", Ready)
	if b, _ := os.ReadFile(m.Path("m")); string(b) != string(data) || st.Done != st.Total {
		t.Errorf("downloaded %d bytes, status %+v", len(b), st)
	}
	if _, err := os.Stat(dir + "/m.gguf.part"); !os.IsNotExist(err) {
		t.Error("the part is left")
	}

	if err := m.Delete("m"); err != nil || m.Status("m").State != Missing {
		t.Errorf("delete: %v, %+v", err, m.Status("m"))
	}
}

func TestFiles(t *testing.T) {
	a, b := []byte(strings.Repeat("sense ", 50000)), []byte("tokens\n")
	old := All
	All = []Model{{ID: "asr", Dir: "asr-1", Size: int64(len(a) + len(b)), Repo: "r",
		Files: []File{{"model.onnx", int64(len(a)), sha(a)}, {"tokens.txt", int64(len(b)), sha(b)}}}}
	t.Cleanup(func() { All = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "tokens.txt") {
			w.Write(b)
			return
		}
		http.Error(w, "the first is here", http.StatusGone)
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := NewManager(dir, HuggingFaceFirst, nil)
	redirect(m, srv)
	os.MkdirAll(dir+"/asr-1", 0o755)
	os.WriteFile(dir+"/asr-1/model.onnx", a, 0o644)
	if st := m.Status("asr"); st.State != Missing || st.Done != int64(len(a)) {
		t.Fatalf("before: %+v", st)
	}
	m.Download("asr")
	if st := wait(t, m, "asr", Ready); st.Done != st.Total || m.Path("asr") != filepath.Join(dir, "asr-1", "model.onnx") {
		t.Errorf("%+v, path %s", st, m.Path("asr"))
	}
	if got, _ := os.ReadFile(dir + "/asr-1/tokens.txt"); string(got) != string(b) {
		t.Errorf("tokens %q", got)
	}
	if err := m.Delete("asr"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir + "/asr-1"); !os.IsNotExist(err) {
		t.Error("its directory is left")
	}
}

func TestManifest(t *testing.T) {
	for _, m := range All {
		var n int64
		for _, f := range m.Files {
			n += f.Size
		}
		if n != m.Size || len(m.Files) == 0 {
			t.Errorf("%s: size %d, files %d", m.ID, m.Size, n)
		}
	}
}

func TestSlowSource(t *testing.T) {
	data := []byte(strings.Repeat("tomoru ", 200000))
	withModel(t, data, sha(data))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/models/") {
			from := 0
			if rg := r.Header.Get("Range"); rg != "" {
				from, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
				w.WriteHeader(http.StatusPartialContent)
			}
			w.Write(data[from:])
			return
		}
		for i := 0; i < len(data); i += 100 {
			if _, err := w.Write(data[i:min(i+100, len(data))]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), HuggingFaceFirst, nil)
	m.sampleInterval = 300 * time.Millisecond
	redirect(m, srv)
	m.Download("m")
	if st := wait(t, m, "m", Ready); st.Done != int64(len(data)) {
		t.Errorf("%+v", st)
	}
}

func TestStalledSource(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers bool
		bytes   int
		slow    int64
	}{
		{"headers", false, 0, 1000},
		{"body", true, 0, 1000},
		{"partial", true, 5, 1000},
		{"after-fast-start", true, 5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("a small complete model")
			withModel(t, data, sha(data))
			const have = 3
			stopped := make(chan struct{})
			mirrorRange := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/models/") {
					if tc.headers {
						w.WriteHeader(http.StatusPartialContent)
						w.Write(data[have : have+tc.bytes])
						w.(http.Flusher).Flush()
					}
					<-r.Context().Done()
					close(stopped)
					return
				}
				mirrorRange <- r.Header.Get("Range")
				w.WriteHeader(http.StatusPartialContent)
				w.Write(data[have+tc.bytes:])
			}))
			defer srv.Close()
			m := NewManager(t.TempDir(), HuggingFaceFirst, nil)
			defer m.Close()
			m.sampleInterval, m.minMirrorSpeed = 100*time.Millisecond, tc.slow
			redirect(m, srv)
			if err := os.WriteFile(m.Path("m")+".part", data[:have], 0o644); err != nil {
				t.Fatal(err)
			}
			m.Download("m")
			wait(t, m, "m", Ready)
			if got, want := <-mirrorRange, fmt.Sprintf("bytes=%d-", have+tc.bytes); got != want {
				t.Errorf("mirror Range = %q, want %q", got, want)
			}
			if got, err := os.ReadFile(m.Path("m")); err != nil || string(got) != string(data) {
				t.Errorf("downloaded %q, %v", got, err)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("abandoned request was not cancelled")
			}
		})
	}
}

func TestLastSourceKeepsSlowProgress(t *testing.T) {
	data := []byte("a slowly delivered model")
	withModel(t, data, sha(data))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/models/") {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		for _, b := range data {
			if _, err := w.Write([]byte{b}); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), HuggingFaceFirst, nil)
	defer m.Close()
	m.sampleInterval = 100 * time.Millisecond
	redirect(m, srv)
	m.Download("m")
	wait(t, m, "m", Ready)
}

func TestMirrorWithAnotherFile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		have   int
		length bool
	}{
		{"whole", 0, true},
		{"whole-unsized", 0, false},
		{"rest", 1000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(strings.Repeat("tomoru ", 100000))
			other := append([]byte("a newer upload "), data...)
			withModel(t, data, sha(data))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := data
				if strings.HasPrefix(r.URL.Path, "/models/") {
					body = other
				}
				from := 0
				if rg := r.Header.Get("Range"); rg != "" {
					from, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
					if from >= len(body) {
						http.Error(w, "", http.StatusRequestedRangeNotSatisfiable)
						return
					}
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(body)-1, len(body)))
				}
				if tc.length {
					w.Header().Set("Content-Length", strconv.Itoa(len(body)-from))
				}
				if from > 0 {
					w.WriteHeader(http.StatusPartialContent)
				}
				w.Write(body[from:])
			}))
			defer srv.Close()
			m := NewManager(t.TempDir(), ModelScopeFirst, nil)
			defer m.Close()
			redirect(m, srv)
			if tc.have > 0 {
				os.WriteFile(m.Path("m")+".part", data[:tc.have], 0o644)
			}
			m.Download("m")
			wait(t, m, "m", Ready)
			if got, _ := os.ReadFile(m.Path("m")); string(got) != string(data) {
				t.Errorf("downloaded %d bytes, want %d", len(got), len(data))
			}
		})
	}
}

func TestOlderFiles(t *testing.T) {
	data, before := []byte("the new upload"), []byte("the old one")
	old := All
	All = []Model{{ID: "m", Files: []File{{"m.gguf", int64(len(data)), sha(data)}}, Older: []File{{"m.gguf", int64(len(before)), sha(before)}}, Size: int64(len(data)), Repo: "r"}}
	t.Cleanup(func() { All = old })
	asked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { asked = true; w.Write(data) }))
	defer srv.Close()
	m := NewManager(t.TempDir(), HuggingFaceFirst, nil)
	defer m.Close()
	redirect(m, srv)
	os.WriteFile(m.Path("m"), before, 0o644)
	if st := m.Status("m"); st.State != Ready || st.Done != st.Total {
		t.Fatalf("old upload: %+v", st)
	}
	m.Download("m")
	if got, _ := os.ReadFile(m.Path("m")); asked || string(got) != string(before) {
		t.Errorf("downloaded again: %v, %q", asked, got)
	}
	os.WriteFile(m.Path("m"), []byte("neither"), 0o644)
	if st := m.Status("m"); st.State != Missing {
		t.Errorf("another file: %+v", st)
	}
}

func TestOlderNamesFiles(t *testing.T) {
	for _, m := range All {
		for _, o := range m.Older {
			if !slices.ContainsFunc(m.Files, func(f File) bool { return f.Name == o.Name && f.Size != o.Size }) {
				t.Errorf("%s: older %s is none of its files", m.ID, o.Name)
			}
		}
	}
}

func TestPinnedRevisions(t *testing.T) {
	commit := func(rev string) bool {
		_, err := hex.DecodeString(rev)
		return len(rev) == 40 && err == nil
	}
	for _, m := range All {
		if !commit(m.Rev) || !commit(m.MSRev) {
			t.Errorf("%s: Hugging Face %q, ModelScope %q", m.ID, m.Rev, m.MSRev)
		}
	}
}

func TestChecksum(t *testing.T) {
	data := []byte("not the model")
	withModel(t, data, sha([]byte("the model")))
	srv, _ := server(data, 0)
	defer srv.Close()
	m := NewManager(t.TempDir(), HuggingFaceFirst, nil)
	redirect(m, srv)
	m.Download("m")
	if st := wait(t, m, "m", Missing); st.Err != ErrChecksum || st.Done != 0 {
		t.Errorf("%+v", st)
	}
}

func TestServer(t *testing.T) {
	bin, err := resource.Bin("llama-server", filepath.Join("..", "..", "resources"))
	if err != nil {
		t.Skip(err)
	}
	home, _ := os.UserHomeDir()
	model := home + "/models/llama.cpp/Index-Translate-2B.Q4_K_M.gguf"
	if _, err := os.Stat(model); err != nil {
		t.Skip(err)
	}
	s := NewServer(bin, nil)
	defer s.Stop()
	url, err := s.Ensure(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Ensure(context.Background(), model)
	if err != nil || again != url {
		t.Errorf("started again: %s, %v", again, err)
	}
	s.Stop()
	if _, err := http.Get(url + "/health"); err == nil {
		time.Sleep(200 * time.Millisecond)
		if _, err := http.Get(url + "/health"); err == nil {
			t.Error("still answers after Stop")
		}
	}
}

func TestRealDownload(t *testing.T) {
	src := os.Getenv("TOMORU_DOWNLOAD")
	if src == "" {
		t.Skip(`set TOMORU_DOWNLOAD=hf or china to download from the real sources`)
	}
	mirrors := HuggingFaceFirst
	if src == "china" {
		mirrors = ModelScopeFirst
	}
	m := NewManager(t.TempDir(), mirrors, nil)
	part := func() int64 { return m.Status("local-2b").Done }
	m.Download("local-2b")
	time.Sleep(4 * time.Second)
	m.Cancel("local-2b")
	st := wait(t, m, "local-2b", Missing)
	first := st.Done
	t.Logf("first: %d bytes, err %v", first, st.Err)
	if first == 0 || st.Err != nil {
		t.Fatalf("nothing came: %+v", st)
	}
	m.Download("local-2b")
	time.Sleep(3 * time.Second)
	m.Cancel("local-2b")
	wait(t, m, "local-2b", Missing)
	t.Logf("then: %d bytes (%.1f MB/s)", part(), float64(part()-first)/3/1e6)
	if part() <= first {
		t.Errorf("did not go on: %d after %d", part(), first)
	}
}

func TestVisions(t *testing.T) {
	for _, m := range All {
		if m.ASR && (m.Projector != "") != m.Served() || m.Projector != "" && !slices.ContainsFunc(m.Files, func(f File) bool { return f.Name == m.Projector }) {
			t.Errorf("%s: vision %q", m.ID, m.Projector)
		}
	}
	dir := t.TempDir()
	m := NewManager(dir, HuggingFaceFirst, nil)
	mod, _ := ByID("local-2b")
	for i, f := range mod.Files {
		if m.ProjectorOf(m.Path("local-2b")) != "" {
			t.Fatalf("%d files: vision found", i)
		}
		w, err := os.Create(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		w.Truncate(f.Size)
		w.Close()
	}
	if got := m.ProjectorOf(m.Path("local-2b")); got != filepath.Join(dir, mod.Projector) {
		t.Errorf("downloaded: %q", got)
	}
	if got := m.ProjectorOf(m.Path("local-9b")); got != "" {
		t.Errorf("another model's: %q", got)
	}
}

func TestServerVision(t *testing.T) {
	bin, err := resource.Bin("llama-server", filepath.Join("..", "..", "resources"))
	if err != nil {
		t.Skip(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Skip(err)
	}
	m := NewManager(dir, HuggingFaceFirst, nil)
	if m.Status("local-2b").State != Ready {
		t.Skip("the 2B model is not downloaded")
	}
	s := NewServer(bin, nil)
	defer s.Stop()
	model := m.Path("local-2b")
	url, err := s.Ensure(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	s.projectorOf = m.ProjectorOf
	if slices.Contains(s.cur.cmd.Args, "--mmproj") {
		t.Errorf("translating only: args %q", s.cur.cmd.Args)
	}
	again, err := s.EnsureProjector(context.Background(), model)
	if err != nil || again != url {
		t.Fatalf("with vision: %s, %v; was %s", again, err, url)
	}
	p := s.cur
	if p.vision != m.ProjectorOf(model) || !slices.Contains(p.cmd.Args, "--mmproj") {
		t.Errorf("args %q", p.cmd.Args)
	}
	if again, err := s.Ensure(context.Background(), model); err != nil || again != url || s.cur != p {
		t.Errorf("translating again: %s, %v; restarted %v", again, err, s.cur != p)
	}
}

func TestLegacyModels(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer srv.Close()
	dir := t.TempDir()
	m := NewManager(dir, HuggingFaceFirst, nil)
	defer m.Close()
	redirect(m, srv)
	small, _ := ByID("hy-mt2-1.8b")
	f := small.MainFile()
	if err := os.WriteFile(filepath.Join(dir, f.Name), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(dir, f.Name), f.Size); err != nil {
		t.Fatal(err)
	}
	if st := m.Status("hy-mt2-1.8b"); st.State != Ready {
		t.Fatalf("downloaded Hy-MT2 1.8B: %+v", st)
	}
	for _, id := range []string{"hy-mt2-7b", "hy-mt2-1.8b"} {
		m.Download(id)
	}
	if st := m.Status("hy-mt2-7b"); st.State != Missing || requests.Load() != 0 {
		t.Fatalf("Hy-MT2 7B downloaded: %+v, %d requests", st, requests.Load())
	}
	if err := m.Delete("hy-mt2-1.8b"); err != nil {
		t.Fatal(err)
	}
	m.Download("hy-mt2-1.8b")
	if st := m.Status("hy-mt2-1.8b"); st.State != Missing || requests.Load() != 0 {
		t.Fatalf("Hy-MT2 1.8B after delete: %+v, %d requests", st, requests.Load())
	}
}
