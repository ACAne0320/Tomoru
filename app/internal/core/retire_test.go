package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tomoru/internal/models"
	"tomoru/internal/prefs"
)

func retiredCore(t *testing.T, path string, mt string, m *fakeModels) *Core {
	t.Helper()
	p := prefs.Default("zh-Hans")
	p.ASR, p.MT = "apple", mt
	if err := prefs.Save(path, p); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return New(p, Deps{PrefsPath: path, Models: m, Server: &fakeServer{}, Secrets: fakeSecrets{}, Post: func(fn func()) { fn() },
		EnginePath: func() (string, error) { return "/engine", nil }})
}

func modelFiles(t *testing.T, dir string, m models.Model) (stale []string, mine string) {
	t.Helper()
	name := m.MainFile().Name
	stale = []string{filepath.Join(dir, name), filepath.Join(dir, name+".part")}
	mine = filepath.Join(dir, "Hy-MT2-notes.txt")
	for _, f := range append(stale, mine) {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return stale, mine
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRetiredModels(t *testing.T) {
	for _, old := range models.Retired {
		t.Run(old.ID, func(t *testing.T) {
			path, dir := filepath.Join(t.TempDir(), "settings.json"), t.TempDir()
			stale, mine := modelFiles(t, dir, old)
			m := &fakeModels{ready: map[string]bool{}, dir: dir}
			c := retiredCore(t, path, old.ID, m)
			if c.Prefs.MT != old.Successor {
				t.Fatalf("in use %q, want %q", c.Prefs.MT, old.Successor)
			}
			if saved, err := prefs.Load(path, "zh-Hans"); err != nil || saved.MT != old.Successor {
				t.Errorf("saved %q, %v", saved.MT, err)
			}
			for _, f := range stale {
				if exists(f) {
					t.Errorf("%s kept", filepath.Base(f))
				}
			}
			if !exists(mine) {
				t.Error("another file removed")
			}
			c.Start()
			if !errors.Is(c.Err, ErrModelMissing) || c.Running {
				t.Errorf("started: running %v, err %v", c.Running, c.Err)
			}
			if len(m.ready) != 0 {
				t.Errorf("downloaded unasked: %v", m.ready)
			}
			if again := retiredCore(t, path, old.Successor, m); again.Prefs.MT != old.Successor || !exists(mine) {
				t.Errorf("started again: in use %q", again.Prefs.MT)
			}
		})
	}
}

func TestRetiredModelsKeptUntilSaved(t *testing.T) {
	old := models.Retired[0]
	dir := t.TempDir()
	stale, _ := modelFiles(t, dir, old)
	c := retiredCore(t, filepath.Join(dir, "gone", "settings.json"), old.ID, &fakeModels{ready: map[string]bool{}, dir: dir})
	if c.Prefs.MT != old.Successor {
		t.Errorf("in use %q, want %q", c.Prefs.MT, old.Successor)
	}
	for _, f := range stale {
		if !exists(f) {
			t.Errorf("%s removed before the settings were saved", filepath.Base(f))
		}
	}
}

func TestRetiredModelsKeptWhileMoving(t *testing.T) {
	old := models.Retired[0]
	dir := t.TempDir()
	stale, _ := modelFiles(t, dir, old)
	retiredCore(t, filepath.Join(t.TempDir(), "settings.json"), "cloud", &fakeModels{ready: map[string]bool{}, dir: dir, moving: models.Move{Active: true}})
	for _, f := range stale {
		if !exists(f) {
			t.Errorf("%s removed while moving", filepath.Base(f))
		}
	}
}
