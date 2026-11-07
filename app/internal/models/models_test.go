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
	"testing"
	"time"

	"tomoru/internal/resource"
)

// withModel makes All one small model served by srv, for the test.
func withModel(t *testing.T, data []byte, sum string) {
	t.Helper()
	old := All
	All = []Model{{ID: "m", Files: []File{{"m.gguf", int64(len(data)), sum}}, Size: int64(len(data)), Repo: "r"}}
	t.Cleanup(func() { All = old })
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// server serves data at any path, honoring Range, and fails the first fails requests.
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

// redirect makes the manager's client fetch every URL from srv.
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
	srv, _ := server(data, 1) // the first source fails: the second serves it
	defer srv.Close()
	m := NewManager(dir, false, nil)
	redirect(m, srv)

	// Half of it came in before: the download goes on from there.
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

// TestFiles downloads a model of two files into its directory, going on with the second when the
// first is there.
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
	m := NewManager(dir, false, nil)
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

// TestManifest: a model's size is its files'.
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

// TestSlowSource gives up on a source that trickles, and goes on from the next.
func TestSlowSource(t *testing.T) {
	data := []byte(strings.Repeat("tomoru ", 200000))
	withModel(t, data, sha(data))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/models/") { // ModelScope: fast, from where asked
			from := 0
			if rg := r.Header.Get("Range"); rg != "" {
				from, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
				w.WriteHeader(http.StatusPartialContent)
			}
			w.Write(data[from:])
			return
		}
		for i := 0; i < len(data); i += 100 { // Hugging Face: a trickle
			if _, err := w.Write(data[i:min(i+100, len(data))]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()
	m := NewManager(t.TempDir(), false, nil)
	m.window = 300 * time.Millisecond
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
			m := NewManager(t.TempDir(), false, nil)
			defer m.Close()
			m.window, m.slow = 100*time.Millisecond, tc.slow
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

// The final mirror may be slow for the entire download. Progress, however small,
// keeps it alive beyond the source-selection window; there is no whole-file deadline.
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
	m := NewManager(t.TempDir(), false, nil)
	defer m.Close()
	m.window = 100 * time.Millisecond
	redirect(m, srv)
	m.Download("m")
	wait(t, m, "m", Ready)
}

func TestChecksum(t *testing.T) {
	data := []byte("not the model")
	withModel(t, data, sha([]byte("the model")))
	srv, _ := server(data, 0)
	defer srv.Close()
	m := NewManager(t.TempDir(), false, nil)
	redirect(m, srv)
	m.Download("m")
	if st := wait(t, m, "m", Missing); st.Err != ErrChecksum || st.Done != 0 {
		t.Errorf("%+v", st)
	}
}

// TestServer runs the bundled llama-server on the 2B model, when both are on this computer.
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

// TestRealDownload downloads the 2B model for a few seconds from the real sources, stops, and goes
// on from where it stopped, when TOMORU_DOWNLOAD is "hf" or "china" (ModelScope first).
func TestRealDownload(t *testing.T) {
	src := os.Getenv("TOMORU_DOWNLOAD")
	if src == "" {
		t.Skip(`set TOMORU_DOWNLOAD=hf or china to download from the real sources`)
	}
	m := NewManager(t.TempDir(), src == "china", nil)
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

// TestVisions: a translation model that reads pictures comes with its vision projector, and each
// recognition model llama-server runs with its audio projector, one of its files, which VisionOf
// finds once the model is downloaded; tomoru-asr's recognition models have none.
func TestVisions(t *testing.T) {
	for _, m := range All {
		if m.ASR && (m.Vision != "") != m.Served() || m.Vision != "" && !slices.ContainsFunc(m.Files, func(f File) bool { return f.Name == m.Vision }) {
			t.Errorf("%s: vision %q", m.ID, m.Vision)
		}
	}
	dir := t.TempDir()
	m := NewManager(dir, false, nil)
	mod, _ := ByID("local-2b")
	for i, f := range mod.Files {
		if m.VisionOf(m.Path("local-2b")) != "" {
			t.Fatalf("%d files: vision found", i)
		}
		w, err := os.Create(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		w.Truncate(f.Size)
		w.Close()
	}
	if got := m.VisionOf(m.Path("local-2b")); got != filepath.Join(dir, mod.Vision) {
		t.Errorf("downloaded: %q", got)
	}
	if got := m.VisionOf(m.Path("local-9b")); got != "" {
		t.Errorf("another model's: %q", got)
	}
}

// TestServerVision has the bundled llama-server serve the 2B model without its vision projector,
// then, for a picture, again with it, on the same port, where a session's translator sends its sentences; when
// both are on this computer.
func TestServerVision(t *testing.T) {
	bin, err := resource.Bin("llama-server", filepath.Join("..", "..", "resources"))
	if err != nil {
		t.Skip(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Skip(err)
	}
	m := NewManager(dir, false, nil)
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
	s.Vision = m.VisionOf
	if slices.Contains(s.cur.cmd.Args, "--mmproj") {
		t.Errorf("translating only: args %q", s.cur.cmd.Args)
	}
	again, err := s.EnsureVision(context.Background(), model)
	if err != nil || again != url {
		t.Fatalf("with vision: %s, %v; was %s", again, err, url)
	}
	p := s.cur
	if p.vision != m.VisionOf(model) || !slices.Contains(p.cmd.Args, "--mmproj") {
		t.Errorf("args %q", p.cmd.Args)
	}
	if again, err := s.Ensure(context.Background(), model); err != nil || again != url || s.cur != p {
		t.Errorf("translating again: %s, %v; restarted %v", again, err, s.cur != p)
	}
}
