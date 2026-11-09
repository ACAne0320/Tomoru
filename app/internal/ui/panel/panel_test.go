package panel

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/live"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

func TestPanel(t *testing.T) {
	nav := &coretest.Nav{}
	k := coretest.New()
	tt := ui.NewTester(New(k, nav).View, Width, Height+50) // taller than it is: it fits itself
	tt.Frame()
	for _, want := range []string{locales.HomeIdle(), locales.SourceAll(), shared.LangName("ja"), locales.Start()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if nav.PanelHeight == 0 || nav.PanelHeight > Height+40 {
		t.Errorf("fits a panel %d tall", nav.PanelHeight)
	}
	// The foot's icons, by the names they show on hovering them.
	for _, label := range []string{locales.TranslateScreen(), locales.MoveSubtitles(), locales.LockSubtitles(), locales.OpenTomoru()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(nav.Went, []string{"image", "adjust", "lock", "settings"}) {
		t.Errorf("went %q", nav.Went)
	}
	// Locked, the lock is named for unlocking them.
	k.Prefs.Locked = true
	tt.Frame()
	if err := tt.Click(locales.UnlockSubtitles()); err != nil || nav.Went[len(nav.Went)-1] != "lock" {
		t.Errorf("unlock: %v, went %q", err, nav.Went)
	}
	if err := tt.Click(locales.Start()); err != nil || !k.Started {
		t.Errorf("start: %v, started %v", err, k.Started)
	}
}

// TestMeasure measures the panel as tall as it fits itself, with an error over it and its action,
// which make it taller than Height.
func TestMeasure(t *testing.T) {
	nav := &coretest.Nav{}
	k := coretest.New()
	k.Err = errors.New("the engine stopped")
	tt := ui.NewTester(New(k, nav).View, Width, 2*Height)
	tt.Frame()
	h := Measure(k, nav)
	if h != nav.PanelHeight || h == Height {
		t.Errorf("measures %d, fits %d", h, nav.PanelHeight)
	}
	tt.SetSize(Width, h)
	gallery(t, tt, "panel-error")
}

// TestPanelError says why translating stopped, with the action that resolves it under it: an
// engine to set up opens Speech & translation; anything else tries again.
func TestPanelError(t *testing.T) {
	nav := &coretest.Nav{}
	k := coretest.New()
	k.Err = core.ErrMTCloudKey
	tt := ui.NewTester(New(k, nav).View, Width, 2*Height)
	if !tt.HasText(locales.ErrMtCloudKey()) {
		t.Errorf("no error: %q", tt.Texts())
	}
	if err := tt.Click(locales.OpenEngines()); err != nil || !slices.Equal(nav.Went, []string{"settings engines"}) {
		t.Errorf("open engines: %v, went %q", err, nav.Went)
	}
	tt.SetSize(Width, Measure(k, nav))
	tt.Frame()
	gallery(t, tt, "panel-error-engines")
	// All of it fits: the icons at its foot show.
	if err := tt.Click(locales.OpenTomoru()); err != nil {
		t.Error(err)
	}
	k.Err = errors.New("network")
	tt.Frame()
	if err := tt.Click(locales.Retry()); err != nil || !k.Started {
		t.Errorf("retry: %v, started %v", err, k.Started)
	}
}

// TestPanelRunning shows the run's numbers while translating, and stops it.
func TestPanelRunning(t *testing.T) {
	k := coretest.New()
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: "listening", Stats: live.Stats{Lines: 412}}
	tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, Height)
	tt.SetScale(2)
	tt.Advance(1e9)
	tt.Frame()
	for _, want := range []string{locales.HomeTranslating(), locales.Stop()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	gallery(t, tt, "panel")
	if err := tt.Click(locales.Stop()); err != nil {
		t.Fatal(err)
	}
	gallery(t, tt, "panel-idle") // the sticker asleep
}

// gallery saves what tt shows, light and dark, as name-light.png and name-dark.png in the
// directory TOMORU_GALLERY names, if any.
func gallery(t *testing.T, tt *ui.Tester, name string) {
	dir := coretest.GalleryDir(t)
	if dir == "" {
		return
	}
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Advance(1e9)
		tt.Frame()
		file := name + "-light.png"
		if dark {
			file = name + "-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// TestPanelScene chooses the scene in use.
func TestPanelScene(t *testing.T) {
	k := coretest.New()
	id := k.AddScene("FF7")
	tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, Height)
	if err := tt.Click(locales.Scene()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("FF7 · " + shared.Langs("ja", "zh")); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	if k.Prefs.Scene != id {
		t.Errorf("in use %q", k.Prefs.Scene)
	}
	// All of it fits: the icons at its foot show.
	if err := tt.Click(locales.OpenTomoru()); err != nil {
		t.Error(err)
	}
}
