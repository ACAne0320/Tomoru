package prefs

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReportsReadFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, "en"); err == nil {
		t.Fatal("directory read failure hidden")
	}
	if _, err := Load(filepath.Join(dir, "missing"), "en"); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}

func TestFailedSavePreservesPreviousSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	p := Default("en")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(path)
	p.Shown = math.NaN()
	if err := Save(path, p); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(old, got) {
		t.Fatal("previous settings were truncated")
	}
}
