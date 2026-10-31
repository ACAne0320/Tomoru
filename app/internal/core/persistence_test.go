package core

import (
	"os"
	"path/filepath"
	"testing"

	"tomoru/internal/prefs"
)

func TestPreferenceActionPersistsWithoutWindow(t *testing.T) {
	c, _ := newCore(t)
	c.deps.PrefsPath = filepath.Join(t.TempDir(), "settings.json")
	c.SetTgt("ja")
	p, err := prefs.Load(c.deps.PrefsPath, "zh-Hans")
	if err != nil || p.Tgt != "ja" {
		t.Fatalf("saved target %q: %v", p.Tgt, err)
	}
	// Calling a save is storage only. It must not modify the live glossary.
	g := c.glossary.Load()
	if err := c.SavePrefs(); err != nil || c.glossary.Load() != g {
		t.Fatalf("save refreshed business state: %v", err)
	}
}

func TestPreferenceFailedSaveRemainsDirty(t *testing.T) {
	c, _ := newCore(t)
	dir := filepath.Join(t.TempDir(), "not-created")
	c.deps.PrefsPath = filepath.Join(dir, "settings.json")
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Shown = 7 })
	if c.SaveErr == nil || c.saved.Shown == 7 {
		t.Fatal("failed write acknowledged")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := c.SavePrefs(); err != nil {
		t.Fatal(err)
	}
	p, err := prefs.Load(c.deps.PrefsPath, "zh-Hans")
	if err != nil || p.Shown != 7 || c.SaveErr != nil {
		t.Fatalf("retry: %+v %v", p, err)
	}
}

func TestPreferenceDraftIsDetachedAndMergesOnlyEdits(t *testing.T) {
	c, _ := newCore(t)
	c.SaveEndpoint(prefs.Endpoint{ID: "a", Name: "original"}, "")
	before := c.Preferences()
	after := before.Clone()
	after.Shown = 4
	// An action can run while a view holds its draft.
	c.SetTgt("ja")
	c.ApplyPreferences(before, after)
	if c.Preferences().Tgt != "ja" || c.Preferences().Shown != 4 {
		t.Fatal("draft overwrote a newer action")
	}
	after.Endpoints[0].Name = "mutated"
	if c.Preferences().Endpoints[0].Name != "original" {
		t.Fatal("draft aliases live preferences")
	}
}
