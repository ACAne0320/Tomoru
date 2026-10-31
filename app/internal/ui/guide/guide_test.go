package guide

import (
	"context"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// TestSteps goes through the guide: the languages, the permission (granted; Windows asks none), the
// engines, trying (skipped: tests have no sessions), the shortcut, selecting to translate
// (skipped), and done.
func TestSteps(t *testing.T) {
	k := coretest.New()
	if prefs.SystemASR() == "" {
		k.Prefs.ASR, k.Prefs.ASRAPIs = core.QwenCloud, []prefs.ASRAPI{{ID: core.QwenCloud, Key: true}} // the engines step has it chosen
	}
	nav := &coretest.Nav{}
	g := New(k, nav)
	tt := ui.NewTester(g.View, Width, Height)
	for _, step := range []struct {
		step       int
		want, next string
	}{
		{stepWelcome, locales.GuideWelcomeBody(), locales.Continue()},
		{stepPermission, locales.SystemAudioOnlyDescription(), locales.Continue()},
		{stepRecognition, locales.EngineQwenAsr(), locales.Continue()},
		{stepTranslation, shared.CloudName, locales.Continue()},
		{stepTry, shared.OnWindows(locales.TryHearing(), locales.TryHearingWin()), locales.Skip()},
		{stepShortcuts, locales.GuideShortcutsBody(), locales.Continue()},
		{stepPicture, locales.GuideSelectAsk(), locales.Skip()},
		{stepReady, shared.OnWindows(locales.GuideReadyMenuBar(), locales.GuideReadyTray()), locales.Done()},
	} {
		if skipped(step.step) {
			continue
		}
		i := step.step
		if !tt.HasText(step.want) {
			t.Fatalf("step %d: no %q in %q", i, step.want, tt.Texts())
		}
		if k.Prefs.GuideStep != i {
			t.Errorf("step %d kept as %d", i, k.Prefs.GuideStep)
		}
		if err := tt.Click(step.next); err != nil {
			t.Fatal(err)
		}
	}
	if !k.Prefs.Onboarded || !slices.Equal(nav.Went, []string{"finish"}) {
		t.Errorf("onboarded %v, went %q", k.Prefs.Onboarded, nav.Went)
	}
}

// TestTry checks off hearing, recognizing and translating as subtitles come, and then offers to
// go on; going back stops translating.
func TestTry(t *testing.T) {
	k := coretest.New()
	g := New(k, &coretest.Nav{})
	g.Step, g.next = stepTry, stepTry
	tt := ui.NewTester(g.View, Width, Height)
	if !k.Started || g.try.works() || tt.HasText(locales.Continue()) {
		t.Fatalf("started %v, works %v", k.Started, g.try.works())
	}
	// A session, as tests have none: listening, then a sentence heard and translated.
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: "listening", Voice: true}
	tt.Frame()
	if !g.try.heard || g.try.recognized {
		t.Errorf("heard %v, recognized %v", g.try.heard, g.try.recognized)
	}
	k.State.Lines = []live.Line{{ID: 1, Src: "皆さん、こんばんは。", Tgt: "大家晚上好。", Done: true}}
	tt.Frame()
	// It works at once; it shows as the stage's lights get to recognition, then to translation.
	if !g.try.works() || tt.HasText(locales.TrySuccess()) {
		t.Fatalf("works %v, shown before the lights got there; %q", g.try.works(), tt.Texts())
	}
	tt.Advance(2 * travel)
	tt.Frame()
	if !tt.HasText(locales.TrySuccess()) || !tt.HasText("大家晚上好。") {
		t.Fatalf("%q", tt.Texts())
	}
	if err := tt.Click(locales.Back()); err != nil {
		t.Fatal(err)
	}
	if g.Step != stepTranslation || k.Running {
		t.Errorf("step %d, running %v", g.Step, k.Running)
	}
}

// TestShortcuts changes the shortcut that starts and stops translating on its step.
func TestShortcuts(t *testing.T) {
	k := coretest.New()
	k.Prefs.GuideStep = stepShortcuts
	nav := &coretest.Nav{}
	g := New(k, nav)
	tt := ui.NewTester(g.View, Width, Height)
	// The field's last keycap, T of ⌥⇧T: the step's text names the shortcut too.
	r, ok := tt.Find("T")
	if !ok {
		t.Fatalf("no keycaps: %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyS)
	if k.Prefs.Shortcut != "Ctrl+Alt+S" || nav.Shortcut != "Ctrl+Alt+S" || !tt.HasText(kit.ShortcutKeys("Ctrl+Alt+S")[0]) || !tt.HasText("S") {
		t.Errorf("%s %s, %q", k.Prefs.Shortcut, nav.Shortcut, tt.Texts())
	}

	// Pressed on its step, it is held, only seen working; going on lets it go.
	if !k.Held[core.StartStop] || tt.HasText(locales.ShortcutWorks()) {
		t.Fatalf("held %v; %q", k.Held, tt.Texts())
	}
	if !k.Press(core.StartStop) {
		t.Error("not held: it would start or stop translating")
	}
	tt.Frame()
	if !tt.HasText(locales.ShortcutWorks()) {
		t.Errorf("not seen working: %q", tt.Texts())
	}
	if err := tt.Click(locales.Continue()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Held[core.StartStop] {
		t.Errorf("still held on step %d", g.Step)
	}
}

// TestSelect offers selecting text on the screen, for the user to want or skip: wanted, it asks for
// Screen Recording, then sends to System Settings; on, it has the shortcut chosen and the sample
// selected, showing its translation.
func TestSelect(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Select = func(context.Context) ([]byte, image.Rectangle, error) { return nil, image.Rectangle{}, nil }
	})
	k.Prefs.GuideStep = stepPicture
	k.Screen = "unknown"
	g := New(k, &coretest.Nav{})
	tt := ui.NewTester(g.View, Width, Height)
	if !tt.HasText(locales.GuideSelectAsk()) || !tt.HasText(locales.ScreenSelectWhy()) || !tt.HasText(locales.Skip()) {
		t.Fatalf("not asked: %q", tt.Texts())
	}
	if err := tt.Click(locales.ScreenSelectTurnOn()); err != nil {
		t.Fatal(err)
	}
	if !k.ScreenAsking || !tt.HasText(locales.ScreenSelectAsking()) {
		t.Errorf("asking %v: %q", k.ScreenAsking, tt.Texts())
	}
	k.ScreenAsking, k.Screen = false, "denied"
	tt.Frame()
	if !tt.HasText(locales.GuideSelectSettings()) || !tt.HasText(locales.OpenSystemSettings()) {
		t.Errorf("to System Settings: %q", tt.Texts())
	}
	k.Screen = "granted" // turned on in System Settings
	tt.Frame()
	keys := kit.ShortcutText("Alt+Shift+S")
	if !tt.HasText(locales.GuideSelectPress(keys)) || tt.HasText(locales.GuideSelectFrame()) || !tt.HasText(locales.Continue()) || !k.Held[core.PictureShortcut] {
		t.Fatalf("on: held %v; %q", k.Held, tt.Texts())
	}
	// Pressed first, the shortcut is seen working, and the next check shows: pressed then, it
	// selects the sample.
	k.Press(core.PictureShortcut)
	tt.Frame()
	if !tt.HasText(locales.ShortcutWorks()) || !tt.HasText(locales.GuideSelectFrame()) || !tt.HasText(locales.GuideSelectHere()) || k.Held[core.PictureShortcut] {
		t.Fatalf("works: held %v; %q", k.Held, tt.Texts())
	}
	k.Image = core.ImageView{PNG: assets.SamplePicture("ja"), Text: "艾露娜：克劳德……"}
	tt.Frame()
	if !tt.HasText("艾露娜：克劳德……") || tt.HasText(locales.GuideSelectHere()) {
		t.Errorf("translated: %q", tt.Texts())
	}
	// Closing the card that showed it clears the picture: the check stays done, its translation
	// under it.
	k.Image = core.ImageView{}
	tt.Frame()
	if !tt.HasText("艾露娜：克劳德……") || tt.HasText(locales.GuideSelectHere()) || !shared.Windows && tt.HasText(locales.GuideSelectAllow()) {
		t.Errorf("card closed: %q", tt.Texts())
	}
}

// TestAnsweredFocus brings the app back to the front once the system's prompt is answered, which
// gives the front to another app.
func TestAnsweredFocus(t *testing.T) {
	if !core.AsksAccess() {
		t.Skip("the system asks no permission")
	}
	k := coretest.New()
	k.Prefs.GuideStep = stepPermission
	nav := &coretest.Nav{}
	g := New(k, nav)
	k.Asking, k.Access = true, "unknown"
	tt := ui.NewTester(g.View, Width, Height)
	if slices.Contains(nav.Went, "focus") {
		t.Fatalf("focused while asking: %q", nav.Went)
	}
	k.Asking, k.Access = false, "granted"
	tt.Frame()
	tt.Frame()
	if n := len(slices.DeleteFunc(slices.Clone(nav.Went), func(s string) bool { return s != "focus" })); n != 1 {
		t.Errorf("focused %d times: %q", n, nav.Went)
	}
}

// TestTryCloudFailed says the cloud may be down, and what else to use, when its lines fail.
func TestTryCloudFailed(t *testing.T) {
	k := coretest.New()
	g := New(k, &coretest.Nav{})
	g.Step, g.next = stepTry, stepTry
	tt := ui.NewTester(g.View, Width, Height)
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: "listening", Voice: true,
		Lines: []live.Line{{ID: 1, Src: "Hello there.", Done: true, Err: "stream error: INTERNAL_ERROR"}}}
	tt.Frame()
	if g.try.translated || !tt.HasText(locales.TranslationFailedCloud()) {
		t.Errorf("translated %v; %q", g.try.translated, tt.Texts())
	}
}

// TestLocalModel chooses the local model before it is downloaded, and uses it once it is; cancels
// and resumes its download, which says why it failed; goes on while it downloads.
func TestLocalModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{}}
	k := coretest.NewWith(func(d *core.Deps) { d.Models = m })
	g := New(k, &coretest.Nav{})
	g.Step, g.next = stepTranslation, stepTranslation
	tt := ui.NewTester(g.View, Width, Height)
	local := models.All[0]
	if err := tt.Click(locales.DownloadModel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Downloads, []string{local.ID}) || k.Chosen(core.MT) != local.ID || k.InUse(core.MT) != "cloud" {
		t.Fatalf("downloads %q, chosen %s, in use %s", m.Downloads, k.Chosen(core.MT), k.InUse(core.MT))
	}
	m.St[local.ID] = models.Status{State: models.Downloading, Done: 100 << 20, Total: local.Size}
	tt.Frame()
	// Its progress under its name, in one line, and nothing else said: the step says the cloud
	// translates meanwhile.
	if !tt.HasText(locales.Downloading(shared.Size(100<<20), shared.Size(local.Size))) || tt.HasText(locales.EngineWaiting(shared.CloudName)) {
		t.Errorf("no progress, or a waiting note: %q", tt.Texts())
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Cancels, []string{local.ID}) || k.Chosen(core.MT) != "cloud" {
		t.Fatalf("cancels %q, chosen %s", m.Cancels, k.Chosen(core.MT))
	}

	m.St[local.ID] = models.Status{State: models.Missing, Done: 100 << 20}
	tt.Frame()
	// Stopped, how much is in stays under its name, so that the button stays short in the narrow
	// column.
	if in := locales.Downloading(shared.Size(100<<20), shared.Size(local.Size)); !tt.HasText(in) {
		t.Errorf("no %q: %q", in, tt.Texts())
	}
	if err := tt.Click(locales.DownloadModel()); err != nil {
		t.Fatal(err)
	}
	m.St[local.ID] = models.Status{State: models.Missing, Err: errors.New("no network")}
	tt.Frame()
	if !tt.HasText(locales.DownloadFailed("no network")) || !tt.HasText(locales.Retry()) {
		t.Errorf("no failure: %q", tt.Texts())
	}

	// Downloading, it holds up nothing: the guide goes on, the cloud translating.
	m.St[local.ID] = models.Status{State: models.Downloading, Done: 100 << 20, Total: local.Size}
	tt.Frame()
	if !tt.HasText(locales.GuideMtLater()) || !tt.HasText(locales.TagRecommended()) {
		t.Errorf("the cloud first: %q", tt.Texts())
	}
	if err := tt.Click(locales.Continue()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.Step != stepTry || k.InUse(core.MT) != "cloud" {
		t.Fatalf("step %d, in use %s while downloading", g.Step, k.InUse(core.MT))
	}

	m.St[local.ID] = models.Status{State: models.Ready}
	k.Changed()
	tt.Frame()
	if k.InUse(core.MT) != local.ID {
		t.Errorf("in use %s once downloaded", k.InUse(core.MT))
	}
}

// TestEnginesNeedRecognition goes on from the recognition step only with recognition chosen: none,
// or the cloud's without its key, is no recognition.
func TestEnginesNeedRecognition(t *testing.T) {
	k := coretest.New()
	k.Prefs.ASR = ""
	g := New(k, &coretest.Nav{})
	g.Step, g.next = stepRecognition, stepRecognition
	tt := ui.NewTester(g.View, Width, Height)
	tt.Click(locales.Continue())
	tt.Frame()
	k.Choose(core.ASR, core.QwenCloud)
	tt.Frame()
	tt.Click(locales.Continue())
	tt.Frame()
	if g.Step != stepRecognition {
		t.Fatalf("went on to step %d without recognition", g.Step)
	}
	k.Prefs.ASR, k.Prefs.ASRAPIs = core.QwenCloud, []prefs.ASRAPI{{ID: core.QwenCloud, Key: true}}
	tt.Frame()
	if err := tt.Click(locales.Continue()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.Step != stepTranslation {
		t.Errorf("step %d with the cloud's recognition", g.Step)
	}
}

// TestTryWaitsForModel has the try step wait for the recognition model chosen, its download's
// progress shown, and translate once it is in use.
func TestTryWaitsForModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{}}
	k := coretest.NewWith(func(d *core.Deps) { d.Models = m })
	k.Prefs.ASR = ""
	sv, _ := models.ByID("sensevoice-small")
	m.St[sv.ID] = models.Status{State: models.Downloading, Done: 100 << 20, Total: sv.Size}
	k.Choose(core.ASR, sv.ID)
	g := New(k, &coretest.Nav{})
	g.Step, g.next = stepTry, stepTry
	tt := ui.NewTester(g.View, Width, Height)
	if k.Started || k.Err != nil {
		t.Fatalf("started %v (%v) before the model is downloaded", k.Started, k.Err)
	}
	if !tt.HasText(locales.TryModelDownloading(shared.Size(100<<20), shared.Size(sv.Size))) || !tt.HasText(locales.TryModelThen()) {
		t.Errorf("no progress: %q", tt.Texts())
	}

	m.St[sv.ID] = models.Status{State: models.Missing, Err: errors.New("no network")}
	tt.Frame()
	if err := tt.Click(locales.Retry()); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	if !slices.Equal(m.Downloads, []string{sv.ID}) {
		t.Errorf("downloads %q", m.Downloads)
	}

	m.St[sv.ID] = models.Status{State: models.Ready}
	k.Changed()
	tt.Frame()
	if k.InUse(core.ASR) != sv.ID || !k.Started {
		t.Errorf("in use %q, started %v once downloaded", k.InUse(core.ASR), k.Started)
	}
}
