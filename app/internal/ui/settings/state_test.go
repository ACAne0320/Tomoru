package settings

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// Native menus invoke their actions after the frame that created them. Persisting
// the frame's draft cannot capture those actions; they must call application commands.
func TestNativePreferenceMenusPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	k := coretest.NewWith(func(d *core.Deps) { d.PrefsPath = path })
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(shared.FontName(k.Preferences().Font)); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(shared.FontName("serif")); err != nil {
		t.Fatal(err)
	}
	if got := k.Preferences().Font; got != "serif" {
		t.Fatalf("font action: %s", got)
	}
	if err := tt.Click(locales.HideAfterValue(int(k.Preferences().HideAfter))); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.HideAfterValue(7)); err != nil {
		t.Fatal(err)
	}
	saved, err := prefs.Load(path, "en")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Font != "serif" || saved.HideAfter != 7 {
		t.Fatalf("menu settings were not saved: font=%s timeout=%v", saved.Font, saved.HideAfter)
	}
}

func TestInactiveSceneLanguageMenuPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenes.json")
	k := coretest.NewWith(func(d *core.Deps) { d.ScenesPath = path })
	id := k.AddScene("Review")
	original := k.Preferences().Src
	s := New(k, &coretest.Nav{})
	s.Tab, s.sceneEdit.scene, s.sceneEdit.id = "scenes", id, true
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.SpokenLanguage()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(shared.LangName("ko")); err != nil {
		t.Fatal(err)
	}
	saved, err := scene.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sc := range saved {
		if sc.ID == id {
			found = sc.Src == "ko"
		}
	}
	if !found {
		t.Fatalf("scene language action not saved: %+v", saved)
	}
	if k.Preferences().Src != original {
		t.Fatal("editing an inactive scene changed the live language")
	}
}
