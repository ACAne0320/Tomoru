package guide

import (
	"context"
	"errors"
	"image"
	"slices"
	"strings"
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

func TestSteps(t *testing.T) {
	k := coretest.New()
	if prefs.SystemASR() == "" {
		k.Prefs.ASR = "sensevoice-small"
	}
	nav := &coretest.Nav{}
	g := New(k, nav)
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	for _, step := range []struct {
		step       int
		want, next string
	}{
		{stepWelcome, locales.GuideWelcomeBody(), locales.Continue()},
		{stepPermission, locales.SystemAudioOnlyDescription(), locales.Continue()},
		{stepRecognition, "Qwen3-ASR", locales.Continue()},
		{stepTranslation, shared.CloudName, locales.Continue()},
		{stepTry, shared.MacOr(locales.TryHearing(), locales.TryHearingWin()), locales.Skip()},
		{stepShortcuts, locales.GuideShortcutsBody(), locales.Continue()},
		{stepPicture, locales.GuideSelectAsk(), locales.Skip()},
		{stepReady, shared.MacOr(locales.GuideReadyMenuBar(), locales.GuideReadyTray()), locales.Done()},
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

func TestTry(t *testing.T) {
	k := coretest.New()
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTry, stepTry
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	if !k.Started || g.try.works() || tt.HasText(locales.Continue()) {
		t.Fatalf("started %v, works %v", k.Started, g.try.works())
	}
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: live.PhaseListening, Voice: true}
	tt.Frame()
	if !g.try.heard || g.try.recognized {
		t.Errorf("heard %v, recognized %v", g.try.heard, g.try.recognized)
	}
	k.State.Lines = []live.Line{{ID: 1, Src: "皆さん、こんばんは。", Tgt: "大家晚上好。", Outcome: live.Translated}}
	tt.Frame()
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

func TestShortcuts(t *testing.T) {
	k := coretest.New()
	k.Prefs.GuideStep = stepShortcuts
	nav := &coretest.Nav{}
	g := New(k, nav)
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
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

	if !k.Holds(core.StartStop) || tt.HasText(locales.ShortcutWorks()) {
		t.Fatalf("held %v; %q", k.Holds(core.StartStop), tt.Texts())
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
	if k.Holds(core.StartStop) {
		t.Errorf("still held on step %d", g.Step)
	}
}

func TestSelect(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Select = func(context.Context) ([]byte, image.Rectangle, error) {
			return nil, image.Rectangle{}, context.Canceled
		}
	})
	k.Prefs.GuideStep = stepPicture
	k.Screen = "unknown"
	g := New(k, &coretest.Nav{})
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
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
	k.Screen = "granted"
	tt.Frame()
	keys := kit.ShortcutText("Alt+Shift+S")
	if !tt.HasText(locales.GuideSelectPress(keys)) || tt.HasText(locales.GuideSelectFrame()) || !tt.HasText(locales.Continue()) || !k.Holds(core.PictureShortcut) {
		t.Fatalf("on: held %v; %q", k.Holds(core.PictureShortcut), tt.Texts())
	}
	k.Press(core.PictureShortcut)
	tt.Frame()
	if !tt.HasText(locales.ShortcutWorks()) || !tt.HasText(locales.GuideSelectFrame()) || !tt.HasText(locales.GuideSelectHere()) || k.Holds(core.PictureShortcut) {
		t.Fatalf("works: held %v; %q", k.Holds(core.PictureShortcut), tt.Texts())
	}
	k.Image = core.ImageView{PNG: assets.SamplePicture("ja"), Text: "艾露娜：克劳德……"}
	tt.Frame()
	if !tt.HasText("艾露娜：克劳德……") || tt.HasText(locales.GuideSelectHere()) {
		t.Errorf("translated: %q", tt.Texts())
	}
	k.Image = core.ImageView{}
	tt.Frame()
	if !tt.HasText("艾露娜：克劳德……") || tt.HasText(locales.GuideSelectHere()) || !shared.Windows && tt.HasText(locales.GuideSelectAllow()) {
		t.Errorf("card closed: %q", tt.Texts())
	}
}

func TestAnsweredFocus(t *testing.T) {
	if !core.AsksAccess() {
		t.Skip("the system asks no permission")
	}
	k := coretest.New()
	k.Prefs.GuideStep = stepPermission
	nav := &coretest.Nav{}
	g := New(k, nav)
	k.Asking, k.Access = true, "unknown"
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
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

func TestTryCloudFailed(t *testing.T) {
	k := coretest.New()
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTry, stepTry
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: live.PhaseListening, Voice: true,
		Lines: []live.Line{{ID: 1, Src: "Hello there.", Outcome: live.Failed, Err: errors.New("stream error: INTERNAL_ERROR")}}}
	tt.Frame()
	if g.try.translated || !tt.HasText(locales.TranslationFailedCloud()) {
		t.Errorf("translated %v; %q", g.try.translated, tt.Texts())
	}
}

func TestLocalModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{}}
	k := coretest.NewWith(func(d *core.Deps) {
		d.Models = m
		d.Memory = func() (uint64, bool) { return 8 << 30, true }
	})
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTranslation, stepTranslation
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	local := models.All[0]
	if err := tt.Click(locales.DownloadModel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Downloads, []string{local.ID}) || k.Chosen(core.MT) != local.ID || k.InUse(core.MT) != "cloud" {
		t.Fatalf("downloads %q, chosen %s, in use %s", m.Downloads, k.Chosen(core.MT), k.InUse(core.MT))
	}
	m.St[local.ID] = models.Status{State: models.Downloading, Done: 100 << 20, Total: local.Size}
	tt.Frame()
	if !tt.HasText(locales.Downloading(shared.Size(100<<20), shared.Size(local.Size))) {
		t.Errorf("no progress: %q", tt.Texts())
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Cancels, []string{local.ID}) || k.Chosen(core.MT) != "cloud" {
		t.Fatalf("cancels %q, chosen %s", m.Cancels, k.Chosen(core.MT))
	}

	m.St[local.ID] = models.Status{State: models.Missing, Done: 100 << 20}
	tt.Frame()
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

func TestEnginesNeedRecognition(t *testing.T) {
	k := coretest.New()
	k.Prefs.ASR = ""
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepRecognition, stepRecognition
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	if tt.HasText(locales.EngineQwenAsr()) || tt.HasText(locales.TagCloud()) {
		t.Errorf("the cloud's recognition offered: %q", tt.Texts())
	}
	tt.Click(locales.Continue())
	tt.Frame()
	if g.Step != stepRecognition {
		t.Fatalf("went on to step %d without recognition", g.Step)
	}
	if err := tt.Click("Qwen3-ASR"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(locales.Continue()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.Step != stepTranslation || k.Chosen(core.ASR) != "qwen3-asr-1.7b" {
		t.Errorf("step %d with %q chosen", g.Step, k.Chosen(core.ASR))
	}
}

func TestTryWaitsForModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{}}
	k := coretest.NewWith(func(d *core.Deps) { d.Models = m })
	k.Prefs.ASR = ""
	sv, _ := models.ByID("sensevoice-small")
	m.St[sv.ID] = models.Status{State: models.Downloading, Done: 100 << 20, Total: sv.Size}
	k.Choose(core.ASR, sv.ID)
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTry, stepTry
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
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

func TestFamilyRows(t *testing.T) {
	mem := uint64(16 << 30)
	k := coretest.NewWith(func(d *core.Deps) {
		d.Models = &coretest.Models{St: map[string]models.Status{}}
		d.GPU = func() (uint64, bool) { return 12 << 30, true }
		d.Memory = func() (uint64, bool) { return mem, true }
	})
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTranslation, stepTranslation
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	large, _ := models.ByID("local-9b")
	hy, _ := models.ByID("hy-mt3-7b")
	for _, want := range []string{"Index-Translate", "Hy-MT", "9B · " + shared.ModelNote(large, models.Status{}, k.Runs(large)), "Hy-MT3 7B · " + shared.ModelNote(hy, models.Status{}, k.Runs(hy))} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("Hy-MT2") {
		t.Errorf("Hy-MT2 not downloaded, shown: %q", tt.Texts())
	}
	if n := strings.Count(strings.Join(tt.Texts(), "\n"), locales.TagRecommended()); n != 1 {
		t.Errorf("%d recommended: %q", n, tt.Texts())
	}
	mem = 8 << 30
	tt.Frame()
	small, _ := models.ByID("local-2b")
	if !tt.HasText("2B · " + shared.ModelNote(small, models.Status{}, k.Runs(small))) {
		t.Errorf("8 GB: %q", tt.Texts())
	}
}

func TestHyMTRow(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Models = &coretest.Models{St: map[string]models.Status{"hy-mt2-1.8b": {State: models.Ready}, "hy-mt2-7b": {State: models.Ready}}}
		d.GPU = func() (uint64, bool) { return 12 << 30, true }
		d.Memory = func() (uint64, bool) { return 16 << 30, true }
	})
	g := New(k, &coretest.Nav{})
	g.Step, g.pendingStep = stepTranslation, stepTranslation
	tt := ui.NewTester(g.View, ContentWidth, ContentHeight)
	three, _ := models.ByID("hy-mt3-7b")
	if _, ok := tt.Find("Hy-MT"); !ok || !tt.HasText("Hy-MT3 7B · "+shared.ModelNote(three, models.Status{}, k.Runs(three))) || tt.HasText("Hy-MT2") {
		t.Errorf("Hy-MT2 downloaded: %q", tt.Texts())
	}
	if !k.Use(core.MT, "hy-mt2-7b") {
		t.Fatal("Hy-MT2 7B not used")
	}
	tt.Frame()
	two, _ := models.ByID("hy-mt2-7b")
	if !tt.HasText("Hy-MT2 7B · "+shared.ModelNote(two, models.Status{State: models.Ready}, k.Runs(two))) || tt.HasText("Hy-MT3 7B") || k.Prefs.MT != "hy-mt2-7b" {
		t.Errorf("Hy-MT2 7B chosen: in use %q, %q", k.Prefs.MT, tt.Texts())
	}
	for _, s := range tt.Texts() {
		if strings.Contains(s, "Hy-MT Hy-MT") || strings.Contains(s, "· ·") || strings.Contains(s, "·  ·") {
			t.Errorf("%q", s)
		}
	}
}
