package panel

import (
	"errors"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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
	tt := ui.NewTester(New(k, nav).View, Width, 400)
	tt.Frame()
	for _, want := range []string{locales.HomeIdle(), locales.SourceAll(), shared.LangName("ja"), locales.Start()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if h := fittedHeight(k, nav); h < 200 || h > 340 {
		t.Errorf("a panel %d tall", h)
	}
	for _, label := range []string{locales.TranslateScreen(), locales.MoveSubtitles(), locales.LockSubtitles(), locales.OpenTomoru()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(nav.Went, []string{"image", "adjust", "lock", "settings"}) {
		t.Errorf("went %q", nav.Went)
	}
	k.Prefs.Locked = true
	tt.Frame()
	if err := tt.Click(locales.UnlockSubtitles()); err != nil || nav.Went[len(nav.Went)-1] != "lock" {
		t.Errorf("unlock: %v, went %q", err, nav.Went)
	}
	if err := tt.Click(locales.Start()); err != nil || !k.Started {
		t.Errorf("start: %v, started %v", err, k.Started)
	}
}

func fittedHeight(k *core.Core, nav *coretest.Nav) int {
	p := New(k, nav)
	var h float32
	tt := ui.NewTester(func(c *ui.Context) { h = p.frame(c).Bounds().H }, Width, 1000)
	tt.Frame()
	return int(math.Ceil(float64(h)))
}

func TestErrorTaller(t *testing.T) {
	nav := &coretest.Nav{}
	k := coretest.New()
	idle := fittedHeight(k, nav)
	k.Err = errors.New("the engine stopped")
	h := fittedHeight(k, nav)
	if h <= idle {
		t.Errorf("with an error %d tall, idle %d", h, idle)
	}
	tt := ui.NewTester(New(k, nav).View, Width, h)
	tt.Frame()
	gallery(t, tt, "panel-error")
}

func TestPanelError(t *testing.T) {
	nav := &coretest.Nav{}
	k := coretest.New()
	k.Err = core.ErrMTCloudKey
	tt := ui.NewTester(New(k, nav).View, Width, fittedHeight(k, nav))
	if !tt.HasText(locales.ErrMtCloudKey()) {
		t.Errorf("no error: %q", tt.Texts())
	}
	if err := tt.Click(locales.OpenEngines()); err != nil || !slices.Equal(nav.Went, []string{"settings engines"}) {
		t.Errorf("open engines: %v, went %q", err, nav.Went)
	}
	tt.Frame()
	gallery(t, tt, "panel-error-engines")
	if err := tt.Click(locales.OpenTomoru()); err != nil {
		t.Error(err)
	}
	k.Err = errors.New("network")
	tt.Frame()
	if err := tt.Click(locales.Retry()); err != nil || !k.Started {
		t.Errorf("retry: %v, started %v", err, k.Started)
	}
}

func TestPanelCount(t *testing.T) {
	k := coretest.New()
	k.Err, k.Running = nil, true
	for _, n := range []int{1, 1234} {
		k.State = live.State{Phase: live.PhaseListening, Stats: live.Stats{Lines: n}}
		tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, shotHeight)
		tt.Frame()
		if !tt.HasText(locales.HistoryLines(n)) {
			t.Errorf("%d: no %q in %q", n, locales.HistoryLines(n), tt.Texts())
		}
	}
}

func TestPanelRunning(t *testing.T) {
	k := coretest.New()
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: live.PhaseListening, Stats: live.Stats{Lines: 412}}
	tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, shotHeight)
	tt.SetScale(2)
	tt.Advance(1e9)
	tt.Frame()
	for _, want := range []string{locales.HomeTranslating(), locales.PauseShort()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	gallery(t, tt, "panel")
	if err := tt.Click(locales.PauseShort()); err != nil {
		t.Fatal(err)
	}
	gallery(t, tt, "panel-idle")
}

func TestPanelClock(t *testing.T) {
	var tt *ui.Tester
	k := coretest.NewWith(func(d *core.Deps) {
		d.Now = func() time.Time {
			if tt == nil {
				return time.Now()
			}
			return tt.Now()
		}
	})
	tt = ui.NewTester(New(k, &coretest.Nav{}).View, Width, shotHeight)
	k.Start()
	if !k.Running {
		t.Fatalf("not running: %v", k.Err)
	}
	k.State = live.State{Phase: live.PhaseListening}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Advance(700 * time.Millisecond)
	if !tt.HasText("0:01") {
		t.Errorf("a second in: %q", tt.Texts())
	}
	tt.Advance(time.Second)
	if !tt.HasText("0:02") {
		t.Errorf("two seconds in: %q", tt.Texts())
	}
}

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

func TestPanelScene(t *testing.T) {
	k := coretest.New()
	id := k.AddScene("FF7")
	tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, shotHeight)
	if err := tt.Click(locales.Scene()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("FF7 · " + shared.Langs("ja", "zh")); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	if k.Prefs.Scene != id {
		t.Errorf("in use %q", k.Prefs.Scene)
	}
	if err := tt.Click(locales.OpenTomoru()); err != nil {
		t.Error(err)
	}
}

func TestPanelSourceKeepsItsApp(t *testing.T) {
	k := coretest.New()
	k.Apps = []core.App{{Name: "Google Chrome", ID: "chrome.exe", Playing: true}}
	tt := ui.NewTester(New(k, &coretest.Nav{}).View, Width, shotHeight)
	if err := tt.Click(locales.Source()); err != nil {
		t.Fatal(err)
	}
	k.Apps = []core.App{{Name: "bilibili", ID: "bilibili.exe", Playing: true}, {Name: "Google Chrome", ID: "chrome.exe", Playing: true}}
	tt.Frame()
	if err := tt.ChooseMenuItem("Google Chrome"); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	if k.Prefs.Source != "chrome.exe" {
		t.Errorf("source %q", k.Prefs.Source)
	}
}

const shotHeight = 297
