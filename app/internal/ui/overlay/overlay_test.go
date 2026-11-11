package overlay

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core/coretest"
	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

func TestSubtitles(t *testing.T) {
	k := coretest.New()
	o := &Overlay{core: k, nav: &coretest.Nav{}}
	k.Running, k.Updated = true, time.Now()
	k.State = live.State{Phase: "listening", Pending: "コメントも", Draft: "评论也", Lines: []live.Line{
		{ID: 1, Src: "みなさん、こんばんは。", Tgt: "大家晚上好。", Done: true},
		{ID: 2, Src: "今日は頑張ります。", Tgt: "今天", Draft: "今天我会努力", Done: false},
	}}
	tt := ui.NewTester(o.View, 1400, 900)
	for _, want := range []string{"今日は頑張ります。", "今天我会努力", "コメントも", "评论也"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("みなさん、こんばんは。") {
		t.Errorf("shows more than two: %q", tt.Texts())
	}
	// With no new subtitles for a while, they go, unless they are kept.
	k.Updated = time.Now().Add(-time.Minute)
	tt.Frame()
	if tt.HasText("コメントも") {
		t.Errorf("still shows %q", tt.Texts())
	}
	k.Prefs.KeepShown = true
	tt.Frame()
	if !tt.HasText("コメントも") {
		t.Errorf("kept: %q", tt.Texts())
	}
}

func TestNotice(t *testing.T) {
	k := coretest.New()
	o := &Overlay{core: k, nav: &coretest.Nav{}}
	k.NoteAt = time.Now()
	k.Prefs.ASR = "apple" // recognition that needs no download, whatever the system
	keys := kit.ShortcutText("Alt+Shift+T")
	tt := ui.NewTester(o.View, 1400, 900)
	if !tt.HasText(locales.NoticeReady(keys)) {
		t.Errorf("at launch: %q", tt.Texts())
	}
	k.Start()
	tt.Frame()
	if !tt.HasText(locales.NoticeStatus(locales.StatusPreparing(), shared.Langs("ja", "zh"))) {
		t.Errorf("starting: %q", tt.Texts())
	}
	// With the original only, there is no language translated into to name.
	k.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplaySrc })
	tt.Frame()
	if !tt.HasText(locales.NoticeStatus(locales.StatusPreparing(), shared.LangName("ja"))) {
		t.Errorf("starting with the original only: %q", tt.Texts())
	}
	k.Stop()
	k.Err = &engine.Error{Code: "no-permission"}
	tt.Frame()
	if !tt.HasText(locales.NoticeRetry(locales.StatusNoPermission(), keys)) {
		t.Errorf("after an error: %q", tt.Texts())
	}
	// The recognizer's network slow: said, for the user to check it.
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: "listening", Slow: true}
	tt.Frame()
	if !tt.HasText(locales.NoticeSlowNetwork()) {
		t.Errorf("slow: %q", tt.Texts())
	}
}

// TestResize drags a side handle as a hand does, the pointer leaving the narrow handle at once:
// the subtitle area follows until the button goes up.
func TestResize(t *testing.T) {
	k := coretest.New()
	o := &Overlay{core: k, nav: &coretest.Nav{}, area: mygo.Rectangle{Width: 1000, Height: 520}}
	k.Running, k.Updated = true, time.Now()
	k.State = live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "Hello there.", Tgt: "你好。", Done: true}}}
	tt := ui.NewTester(o.View, 1000, 600)
	o.hover = true
	tt.Frame()
	w0 := k.Prefs.AreaW
	b := o.block
	x, y := b.X+b.W-7, b.Y+b.H-20 // on the right handle
	tt.Press(x, y)
	tt.Move(x+40, y)
	tt.Frame()
	if want := w0 + .08; math.Abs(k.Prefs.AreaW-want) > .005 {
		t.Errorf("40 off the handle: width %.3f, not %.3f", k.Prefs.AreaW, want)
	}
	tt.Release(x+40, y)
	tt.Move(x+80, y)
	tt.Frame()
	if want := w0 + .08; math.Abs(k.Prefs.AreaW-want) > .005 {
		t.Errorf("let go: width %.3f, not %.3f", k.Prefs.AreaW, want)
	}
}

// TestSceneMenu switches the scene from the toolbar, which brings the scene's languages, changes
// the scene's language from it, and goes to the scenes' page.
func TestSceneMenu(t *testing.T) {
	k := coretest.New()
	nav := &coretest.Nav{}
	o := &Overlay{core: k, nav: nav, area: mygo.Rectangle{Width: 1000, Height: 520}}
	id := k.AddScene("Vtube")
	k.SetScene(id) // a scene keeps the languages chosen in it
	k.SetSrc("en")
	k.SetScene("")
	k.Running, k.Updated = true, time.Now()
	tt := ui.NewTester(o.View, 1000, 600)
	o.hover = true
	tt.Frame()
	tt.Advance(time.Second) // the toolbar fades in
	open := func() {
		t.Helper()
		if err := tt.Click(locales.Scene() + ": " + shared.SceneName(*k.Scene())); err != nil {
			t.Fatalf("%v: %q", err, tt.Texts())
		}
	}
	open()
	vtube := "Vtube · " + shared.LangName("en") + " → " + shared.LangName("zh")
	if err := tt.ChooseMenuItem(vtube); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	if k.Prefs.Scene != id || k.Prefs.Src != "en" {
		t.Errorf("scene %q, from %q", k.Prefs.Scene, k.Prefs.Src)
	}
	open()
	if err := tt.ChooseMenuItem(locales.SpokenLanguage(), shared.LangName("ja")); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	if sc, _ := k.SceneByID(id); k.Prefs.Src != "ja" || sc.Src != "ja" {
		t.Errorf("spoken %q, the scene keeps %q", k.Prefs.Src, sc.Src)
	}
	// The menu reaches below the block: the pointer going down to its last item leaves the
	// block, which ends the hover before the item is chosen.
	open()
	o.hover = false
	tt.Frame()
	tt.Advance(time.Second) // the toolbar fades out
	if err := tt.ChooseMenuItem(locales.ManageScenes() + "…"); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	if len(nav.Went) == 0 || nav.Went[len(nav.Went)-1] != "settings scenes" {
		t.Errorf("went %q", nav.Went)
	}
	// A new scene is made, in use at once, and shown on the scenes' page to be named.
	tt.Frame()
	before := len(k.SceneList())
	o.hover = true
	tt.Frame()
	tt.Advance(time.Second)
	if err := tt.Click(locales.Scene() + ": " + shared.SceneName(*k.Scene())); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.NewScene() + "…"); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	if len(k.SceneList()) != before+1 || k.Prefs.Scene != k.SceneList()[before].ID || nav.Went[len(nav.Went)-1] != "settings scenes" {
		t.Errorf("scenes %d, in use %q, went %q", len(k.SceneList()), k.Prefs.Scene, nav.Went)
	}
}
