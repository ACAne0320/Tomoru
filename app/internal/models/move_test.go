package models

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withTwo makes All a model of a file, a, and one in a directory of its own, b, both downloaded
// into dir.
func withTwo(t *testing.T, dir string) {
	t.Helper()
	a, b := []byte(strings.Repeat("a", 3000)), []byte(strings.Repeat("b", 5000))
	old := All
	All = []Model{
		{ID: "a", Files: []File{{"a.gguf", int64(len(a)), sha(a)}}, Size: int64(len(a))},
		{ID: "b", Dir: "b-1", Files: []File{{"b.onnx", int64(len(b)), sha(b)}}, Size: int64(len(b))},
	}
	t.Cleanup(func() { All = old })
	os.MkdirAll(filepath.Join(dir, "b-1"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.gguf"), a, 0o644)
	os.WriteFile(filepath.Join(dir, "b-1", "b.onnx"), b, 0o644)
}

func move(t *testing.T, m *Manager, to string) error {
	t.Helper()
	res := make(chan error, 1)
	m.Move(to, func(err error) { res <- err })
	select {
	case err := <-res:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the move did not end")
		return nil
	}
}

// noRename has the manager rename as across disks: never.
func noRename(m *Manager) {
	m.rename = func(string, string) error { return errors.New("another disk") }
}

func TestMove(t *testing.T) {
	for _, across := range []bool{false, true} {
		t.Run(map[bool]string{false: "rename", true: "copy"}[across], func(t *testing.T) {
			from, to := t.TempDir(), filepath.Join(t.TempDir(), "elsewhere")
			withTwo(t, from)
			m := NewManager(from, false, nil)
			defer m.Close()
			if across {
				noRename(m)
			}
			if err := move(t, m, to); err != nil {
				t.Fatal(err)
			}
			if m.Dir() != to || m.Path("b") != filepath.Join(to, "b-1", "b.onnx") {
				t.Fatalf("dir %s, b at %s", m.Dir(), m.Path("b"))
			}
			for _, id := range []string{"a", "b"} {
				if st := m.Status(id); st.State != Ready {
					t.Errorf("%s: %+v", id, st)
				}
			}
			if _, err := os.Stat(from); !os.IsNotExist(err) {
				t.Error("the old directory is left:", err)
			}
			if mv := m.Moving(); mv.Active || mv.Err != nil || mv.Done != mv.Total || mv.Total != 8000 {
				t.Errorf("moving: %+v", mv)
			}
			left, _ := filepath.Glob(filepath.Join(to, "*", "*.moving"))
			if more, _ := filepath.Glob(filepath.Join(to, "*.moving")); len(left)+len(more) > 0 {
				t.Error("copies left:", left, more)
			}
		})
	}
}

// TestMoveBack moves to a directory that has the models already: they are kept, not copied again.
func TestMoveBack(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	withTwo(t, from)
	withTwo(t, to)
	m := NewManager(from, false, nil)
	defer m.Close()
	noRename(m)
	before, _ := os.Stat(filepath.Join(to, "a.gguf"))
	if err := move(t, m, to); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(filepath.Join(to, "a.gguf"))
	if !os.SameFile(before, after) || m.Status("a").State != Ready {
		t.Error("the model there was written again")
	}
	if _, err := os.Stat(filepath.Join(from, "a.gguf")); !os.IsNotExist(err) {
		t.Error("the model moved is left where it was")
	}
}

// TestMoveFails leaves everything as it was when a file can't be put in the new directory: the
// files renamed and the files copied before it.
func TestMoveFails(t *testing.T) {
	for _, across := range []bool{false, true} {
		t.Run(map[bool]string{false: "renamed", true: "copied"}[across], func(t *testing.T) {
			from, to := t.TempDir(), t.TempDir()
			withTwo(t, from)
			os.MkdirAll(filepath.Join(to, "b-1", "b.onnx"), 0o755) // in the way
			m := NewManager(from, false, nil)
			defer m.Close()
			if across {
				noRename(m)
			} else {
				m.rename = func(a, b string) error {
					if strings.HasSuffix(a, "b.onnx") {
						return errors.New("another disk")
					}
					return os.Rename(a, b)
				}
			}
			if err := move(t, m, to); err == nil {
				t.Fatal("moved")
			}
			if m.Dir() != from || m.Moving().Err == nil || m.Moving().Active {
				t.Errorf("dir %s, moving %+v", m.Dir(), m.Moving())
			}
			for _, id := range []string{"a", "b"} {
				if st := m.Status(id); st.State != Ready {
					t.Errorf("%s: %+v", id, st)
				}
			}
			if _, err := os.Stat(filepath.Join(to, "a.gguf")); !os.IsNotExist(err) {
				t.Error("a is left in the new directory")
			}
			if left, _ := filepath.Glob(filepath.Join(to, "b-1", "*.moving")); len(left) > 0 {
				t.Error("copies left:", left)
			}
		})
	}
}

// TestMovePart moves what came in of a model, which a download goes on from there.
func TestMovePart(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	withModel(t, []byte("model"), sha([]byte("model")))
	os.WriteFile(filepath.Join(from, "m.gguf.part"), []byte("mo"), 0o644)
	m := NewManager(from, false, nil)
	defer m.Close()
	if err := move(t, m, to); err != nil {
		t.Fatal(err)
	}
	if st := m.Status("m"); st.State != Missing || st.Done != 2 {
		t.Errorf("status %+v", st)
	}
	if b, _ := os.ReadFile(filepath.Join(to, "m.gguf.part")); string(b) != "mo" {
		t.Errorf("part %q", b)
	}
}

// TestMoveHolds starts no download, deletes nothing and starts no other move while the models move,
// and says what was there.
func TestMoveHolds(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	withTwo(t, from)
	m := NewManager(from, false, nil)
	defer m.Close()
	srv, _ := server(nil, 100)
	defer srv.Close()
	redirect(m, srv)
	m.Delete("b") // missing, to download
	inside, release := make(chan struct{}), make(chan struct{})
	m.rename = func(a, b string) error {
		select {
		case inside <- struct{}{}:
		default:
		}
		<-release
		return os.Rename(a, b)
	}
	res := make(chan error, 1)
	m.Move(to, func(err error) { res <- err })
	<-inside
	m.Download("b")
	if err := m.Delete("a"); err != nil {
		t.Fatal(err)
	}
	busy := make(chan error, 1)
	m.Move(t.TempDir(), func(err error) { busy <- err })
	m.mu.Lock()
	jobs := len(m.jobs)
	m.mu.Unlock()
	if jobs != 0 || m.Status("a").State != Ready || !m.Moving().Active || !errors.Is(<-busy, errBusy) {
		t.Errorf("while moving: %d downloads, a %+v, %+v", jobs, m.Status("a"), m.Moving())
	}
	close(release)
	if err := <-res; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "a.gguf")); err != nil {
		t.Error("a was not moved:", err)
	}
}

// TestMoveStopsDownloads stops a download before moving, and moves what came in of it.
func TestMoveStopsDownloads(t *testing.T) {
	data := []byte(strings.Repeat("tomoru ", 1000))
	withModel(t, data, sha(data))
	from, to := t.TempDir(), t.TempDir()
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data[:100])
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	m := NewManager(from, false, nil)
	defer m.Close()
	redirect(m, srv)
	m.Download("m")
	<-started
	for m.Status("m").Done < 100 {
		time.Sleep(5 * time.Millisecond)
	}
	if err := move(t, m, to); err != nil {
		t.Fatal(err)
	}
	if st := m.Status("m"); st.State != Missing || st.Done != 100 {
		t.Errorf("status %+v", st)
	}
	if _, err := os.Stat(filepath.Join(from, "m.gguf.part")); !os.IsNotExist(err) {
		t.Error("the part is left where it was")
	}
}

// TestMoveSameDir moves to the directory the models are in, by another name: nothing moves, and
// nothing is removed as moved.
func TestMoveSameDir(t *testing.T) {
	from := t.TempDir()
	withTwo(t, from)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(from, alias); err != nil {
		t.Skip("no symlinks:", err)
	}
	m := NewManager(from, false, nil)
	defer m.Close()
	if err := move(t, m, alias); err != nil {
		t.Fatal(err)
	}
	if m.Dir() != from {
		t.Errorf("dir %s, want %s", m.Dir(), from)
	}
	for _, id := range []string{"a", "b"} {
		if st := m.Status(id); st.State != Ready {
			t.Errorf("%s: %+v", id, st)
		}
	}
}

func TestDestination(t *testing.T) {
	current := t.TempDir()
	other := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	os.Symlink(current, alias)
	for _, c := range []struct{ chosen, want string }{
		{other, filepath.Join(other, FolderName)},
		{filepath.Join(other, FolderName), filepath.Join(other, FolderName)},
		{filepath.Join(other, "tomoru models"), filepath.Join(other, "tomoru models")},
		{current, current},
		{alias, alias},
	} {
		if got := Destination(c.chosen, current); got != c.want {
			t.Errorf("Destination(%s) = %s, want %s", c.chosen, got, c.want)
		}
	}
}
