package settings

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

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
	saved, err := prefs.Load(path, "en")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Font != "serif" {
		t.Fatalf("menu settings were not saved: font=%s", saved.Font)
	}
}

func TestSystemFont(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	k := coretest.NewWith(func(d *core.Deps) { d.PrefsPath = path })
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	families := ui.FontFamilies()
	if len(families) < 2 {
		t.Skip("no fonts to list")
	}
	family := families[len(families)/2].Name
	if err := tt.Click(shared.FontName(k.Preferences().Font)); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.FontMore() + "…"); err != nil {
		t.Fatal(err)
	}
	tt.Type(family)
	tt.Frame()
	if err := tt.Click(family); err != nil {
		t.Fatalf("%s not listed: %q", family, tt.Texts())
	}
	if got := k.Preferences().Font; string(got) != family {
		t.Fatalf("font %q, want %q", got, family)
	}
	if saved, _ := prefs.Load(path, "en"); string(saved.Font) != family {
		t.Fatalf("saved font %q", saved.Font)
	}
	if tt.HasText(locales.FontSearch()) {
		t.Error("the list stays open after choosing")
	}
	if _, ok := tt.Find(family); !ok {
		t.Errorf("the picker doesn't show %s: %q", family, tt.Texts())
	}

	k.UpdatePrefs(func(p *prefs.Prefs) { p.Font = "Not A Font Tomoru Knows" })
	tt.Frame()
	if !tt.HasText(shared.FontName(prefs.Fonts[0])) || tt.HasText("Not A Font Tomoru Knows") {
		t.Errorf("a font not installed shows: %q", tt.Texts())
	}

	if err := tt.Click(shared.FontName(prefs.Fonts[0])); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.FontMore() + "…"); err != nil {
		t.Fatal(err)
	}
	tt.Type("zzzz no such font")
	tt.Frame()
	if !tt.HasText(locales.FontNoMatch()) {
		t.Errorf("no match isn't said: %q", tt.Texts())
	}
}

func TestFontSampleLang(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	for _, c := range []struct {
		display  prefs.Display
		tgtFirst bool
		src      bool
	}{{prefs.DisplayBoth, false, true}, {prefs.DisplayBoth, true, false}, {prefs.DisplayTgt, false, false}, {prefs.DisplaySrc, true, true}} {
		k.UpdatePrefs(func(p *prefs.Prefs) { p.Src, p.Tgt, p.Display, p.TgtFirst = "ja", "en", c.display, c.tgtFirst })
		s.openFonts()
		tt.Frame()
		line := subtitle.Sample(k.Preferences(), 8)
		want, not := line.Tgt, line.Src
		if c.src {
			want, not = not, want
		}
		if n, m := count(tt.Texts(), want), count(tt.Texts(), not); n < 3 || m > 1 {
			t.Errorf("display %d, translation first %v: %q %d times, %q %d", c.display, c.tgtFirst, want, n, not, m)
		}
		s.fonts.open = false
	}
}

func count(texts []string, s string) int {
	n := 0
	for _, t := range texts {
		if t == s {
			n++
		}
	}
	return n
}

func TestHideAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	k := coretest.NewWith(func(d *core.Deps) { d.PrefsPath = path })
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.HideWaitMore()); err != nil {
		t.Fatal(err)
	}
	row, _ := tt.Find(locales.HideAfterLabel())
	more, _ := tt.Find(locales.HideWaitMore())
	tt.ClickAt(more.X+more.W-12, row.Y+row.H/2)
	tt.Frame()
	if tt.HasText(locales.HideWait()) {
		t.Errorf("the seconds show with hiding off: %q", tt.Texts())
	}
	saved, err := prefs.Load(path, "en")
	if err != nil {
		t.Fatal(err)
	}
	if saved.HideAfterSeconds != 6 || !saved.KeepShown || k.HideAfter() != 0 {
		t.Fatalf("saved %v s, kept %v; hides after %v", saved.HideAfterSeconds, saved.KeepShown, k.HideAfter())
	}
}

func TestInactiveSceneLanguageMenuPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenes.json")
	k := coretest.NewWith(func(d *core.Deps) { d.ScenesPath = path })
	id := k.AddScene("Review")
	original := k.Preferences().Src
	s := New(k, &coretest.Nav{})
	s.Tab, s.sceneEdit.scene = "scenes", id
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

func TestUILanguageBackToSystem(t *testing.T) {
	k := coretest.New()
	k.UpdatePrefs(func(p *prefs.Prefs) { p.UILang = "ja" })
	s := New(k, &coretest.Nav{})
	s.Tab = "general"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(i18n.Autonym("ja")); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.UiLanguageSystem()); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	if got := k.Preferences().UILang; got != "" {
		t.Fatalf("UI language %q, want the system's", got)
	}
}
