package settings

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/history"
	"tomoru/internal/learn"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

func TestPages(t *testing.T) {
	for tab, want := range map[string]string{
		"general":   locales.AutoStart(),
		"subtitles": locales.ColorScheme(),
		"engines":   locales.DraftLabel(),
		"about":     locales.AboutLicense(),
	} {
		s := New(coretest.New(), &coretest.Nav{})
		s.Tab = tab
		tt := ui.NewTester(s.View, Width, 1200)
		if !tt.HasText(want) {
			t.Errorf("%s: no %q in %q", tab, want, tt.Texts())
		}
	}
}

func TestResetStyle(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	k.Prefs.TgtSize = 40
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.ResetStyle()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.TgtSize != 28 {
		t.Errorf("translation size %v", k.Prefs.TgtSize)
	}
}

func TestCustomColor(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1600)
	if tt.HasText(locales.SrcColor()) {
		t.Error("custom colors with a scheme chosen")
	}
	if err := tt.Click(locales.SchemeCustom()); err != nil {
		t.Fatal(err)
	}
	if tt.Frame(); !tt.HasText(locales.SrcColor()) || !tt.HasText(locales.CustomColor()) {
		t.Fatalf("no custom colors in %q", tt.Texts())
	}
	if err := tt.Click(locales.CustomColor()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Red"); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.SrcColor != "#ef4444" || k.Prefs.TgtColor != subtitle.Schemes[0].Tgt {
		t.Errorf("colors %s %s", k.Prefs.SrcColor, k.Prefs.TgtColor)
	}
	if err := tt.Click(locales.SchemeSakura()); err != nil {
		t.Fatal(err)
	}
	if tt.Frame(); tt.HasText(locales.SrcColor()) {
		t.Error("custom colors after a scheme is chosen")
	}
}

func TestScheme(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	k.Prefs.Edge = prefs.EdgeOutline
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.SchemeSakura()); err != nil {
		t.Fatal(err)
	}
	if p := k.Prefs; p.SrcColor != "#ffffff" || p.TgtColor != "#ffb3cf" || p.EdgeColor != "#4a1030" {
		t.Errorf("colors %s %s %s", p.SrcColor, p.TgtColor, p.EdgeColor)
	}
}

func TestStyleTellsOverlay(t *testing.T) {
	k := coretest.New()
	changes := 0
	k.OnChange(func() { changes++ })
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	for range 10 {
		tt.Advance(time.Second)
	}
	if changes != 0 {
		t.Errorf("%d changes while the preview rolled", changes)
	}
	if err := tt.Click(locales.SchemeSakura()); err != nil {
		t.Fatal(err)
	}
	if changes == 0 || k.Preferences().TgtColor != "#ffb3cf" {
		t.Errorf("%d changes, translation color %s", changes, k.Preferences().TgtColor)
	}
}

func TestShortcut(t *testing.T) {
	k := coretest.New()
	nav := &coretest.Nav{}
	s := New(k, nav)
	s.Tab = "general"
	tt := ui.NewTester(s.View, Width, 900)
	record := func() {
		t.Helper()
		r, ok := tt.Find(kit.ShortcutKeys(k.Prefs.Shortcut)[0])
		if !ok {
			t.Fatalf("no keycaps: %q", tt.Texts())
		}
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		if !tt.HasText(locales.ShortcutRecording()) {
			t.Fatalf("not recording: %q", tt.Texts())
		}
	}
	record()
	tt.Key(0, ui.KeyK)
	if !tt.HasText(locales.ShortcutNeedsModifier()) && !tt.HasText(locales.ShortcutNeedsModifierWin()) || k.Prefs.Shortcut != prefs.DefaultShortcut {
		t.Errorf("K alone: %s, %q", k.Prefs.Shortcut, tt.Texts())
	}
	tt.Key(ui.Super|ui.Shift, ui.KeyK)
	if k.Prefs.Shortcut != "Shift+Cmd+K" || nav.Shortcut != "Shift+Cmd+K" || !tt.HasText(kit.ShortcutKeys("Cmd+K")[0]) || tt.HasText(locales.ShortcutRecording()) {
		t.Errorf("⇧⌘K: %s %s, %q", k.Prefs.Shortcut, nav.Shortcut, tt.Texts())
	}
	record()
	tt.Key(ui.Super, ui.KeySpace)
	if k.Prefs.Shortcut != "Shift+Cmd+K" || !tt.HasText(locales.ShortcutTaken(kit.ShortcutText("Cmd+Space"))) {
		t.Errorf("taken: %s, %q", k.Prefs.Shortcut, tt.Texts())
	}
	record()
	tt.Key(0, ui.KeyEscape)
	if tt.HasText(locales.ShortcutRecording()) || k.Prefs.Shortcut != "Shift+Cmd+K" {
		t.Errorf("escape: %s, %q", k.Prefs.Shortcut, tt.Texts())
	}

	r, ok := tt.Find("S")
	if !ok || !tt.HasText(locales.ImageShortcut()) {
		t.Fatalf("no picture's field: %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Key(ui.Super|ui.Shift, ui.KeyK)
	if k.Prefs.ImageShortcut != prefs.DefaultImageShortcut || !tt.HasText(locales.ShortcutDuplicate(kit.ShortcutText("Shift+Cmd+K"))) {
		t.Errorf("the other's keys: %s, %q", k.Prefs.ImageShortcut, tt.Texts())
	}
	r, _ = tt.Find("S")
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !tt.HasText(locales.ShortcutRecording()) {
		t.Fatalf("not recording again: %v %q", r, tt.Texts())
	}
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyP)
	if k.Prefs.ImageShortcut != "Ctrl+Alt+P" || nav.Image != "Ctrl+Alt+P" || k.Prefs.Shortcut != "Shift+Cmd+K" {
		t.Errorf("⌃⌥P: picture's %s (%s), start's %s", k.Prefs.ImageShortcut, nav.Image, k.Prefs.Shortcut)
	}
}

func enginesPage(t *testing.T, m *coretest.Models) (*core.Core, *Settings, *ui.Tester) {
	t.Helper()
	k := coretest.NewWith(func(d *core.Deps) {
		if m != nil {
			d.Models = m
		}
	})
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	return k, s, ui.NewTester(s.View, Width, 1800)
}

func TestAddEndpoint(t *testing.T) {
	k, _, tt := enginesPage(t, nil)
	wants := []string{locales.EngineIndexCloud(), "Index-Translate", "Hy-MT2", locales.TagLocal(), locales.TagCloud(), locales.EngineCloudThanks()}
	if prefs.SystemASR() != "" {
		wants = append(wants, locales.EngineApple())
	}
	for _, want := range wants {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	for _, gone := range []string{locales.CloudSendsText(), "Index-Translate 2B"} {
		if tt.HasText(gone) {
			t.Errorf("%q shows: %q", gone, tt.Texts())
		}
	}
	if err := tt.Click("GitHub"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != core.CloudRepo {
		t.Errorf("opened %q", got)
	}
	if err := tt.Click(locales.AddApi()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.EndpointIntro()) {
		t.Fatalf("no form: %q", tt.Texts())
	}
	if err := tt.Click(locales.EndpointName()); err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"Mine", "http://localhost:11434/v1", "qwen3"} {
		if i > 0 {
			tt.Key(0, ui.KeyTab)
		}
		tt.Type(text)
	}
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(k.Prefs.Endpoints) != 1 || k.Prefs.Endpoints[0].URL != "http://localhost:11434" || k.Prefs.Endpoints[0].Model != "qwen3" {
		t.Fatalf("endpoints %+v", k.Prefs.Endpoints)
	}
	if k.Prefs.MT != "cloud" || !tt.HasText("qwen3 · localhost:11434") || tt.HasText(locales.EndpointIntro()) {
		t.Errorf("saved: in use %q, texts %q", k.Prefs.MT, tt.Texts())
	}
	if err := tt.Click(locales.UseEngine("Mine")); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != "custom:"+k.Prefs.Endpoints[0].ID {
		t.Errorf("engine %q, texts %q", k.Prefs.MT, tt.Texts())
	}

	tt.Frame()
	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(k.Prefs.Endpoints) != 0 || !tt.HasText(locales.EndpointDeletedInUse("Mine")) {
		t.Fatalf("endpoints %+v, texts %q", k.Prefs.Endpoints, tt.Texts())
	}
	if err := tt.Click(locales.Undo()); err != nil {
		t.Fatal(err)
	}
	if len(k.Prefs.Endpoints) != 1 || k.Prefs.MT != "custom:"+k.Prefs.Endpoints[0].ID {
		t.Errorf("after undo: endpoints %+v, engine %q", k.Prefs.Endpoints, k.Prefs.MT)
	}
}

func TestDownloadThenUse(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{}}
	k, _, tt := enginesPage(t, m)
	if err := tt.Click("Index-Translate"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{locales.SizesNotDownloaded(2), locales.ModelSizes(), "2B", "9B", shared.SizeNote(models.All[0]) + " · " + shared.ModelNote(models.All[0], models.Status{}, k.Runs(models.All[0]))} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click(locales.UseEngine("Index-Translate")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("2B"); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != "cloud" || k.Chosen(core.MT) != "cloud" {
		t.Fatalf("not downloaded, chose: in use %q, chosen %q", k.Prefs.MT, k.Chosen(core.MT))
	}
	if err := tt.Click(locales.DownloadModel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Downloads, []string{"local-2b"}) || k.Prefs.MT != "cloud" || k.Chosen(core.MT) != "cloud" {
		t.Fatalf("downloads %q, in use %q, chosen %q", m.Downloads, k.Prefs.MT, k.Chosen(core.MT))
	}
	m.St["local-2b"] = models.Status{State: models.Downloading, Done: 400 << 20, Total: models.All[0].Size}
	tt.Frame()
	if !tt.HasText(locales.Downloading(shared.Size(400<<20), shared.Size(models.All[0].Size))) {
		t.Fatalf("no progress: %q", tt.Texts())
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Cancels, []string{"local-2b"}) {
		t.Errorf("cancels %q", m.Cancels)
	}
	m.St["local-2b"] = models.Status{State: models.Ready}
	k.Changed()
	tt.Frame()
	if k.Prefs.MT != "cloud" || !tt.HasText(locales.ModelDownloaded("Index-Translate 2B")) || !tt.HasText(locales.Downloaded()) {
		t.Fatalf("downloaded: in use %q, texts %q", k.Prefs.MT, tt.Texts())
	}
	if err := tt.Click(locales.Use()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.MT != "local-2b" {
		t.Errorf("used: in use %q, texts %q", k.Prefs.MT, tt.Texts())
	}
	if err := tt.Click(locales.UseEngine(locales.EngineIndexCloud())); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.UseEngine("Index-Translate")); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != "local-2b" {
		t.Errorf("the family's radio: in use %q", k.Prefs.MT)
	}
}

func TestTooLarge(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Models = &coretest.Models{St: map[string]models.Status{}}
		d.Memory = func() (uint64, bool) { return 8 << 30, true }
	})
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1800)
	if err := tt.Click("Index-Translate"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.TooLarge(16, 8)) {
		t.Errorf("not said: %q", tt.Texts())
	}
	n := 0
	for _, text := range tt.Texts() {
		if text == locales.DownloadModel() {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d downloads offered, want the 2B's: %q", n, tt.Texts())
	}
}

func TestDeleteModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{"local-2b": {State: models.Ready}}}
	k, _, tt := enginesPage(t, m)
	if err := tt.Click("Index-Translate"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, _ := k.Engine(core.MT, "local-2b")
	question, size := locales.AskDeleteModel(shared.EngineName(e)), shared.Size(e.Model.Size)

	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(question) || !tt.HasText(locales.AskDeleteModelDetail(size)) {
		t.Fatalf("not asked: texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText(question) || len(m.St) != 1 {
		t.Fatalf("after Escape: models %v, texts %q", m.St, tt.Texts())
	}
	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(m.St) != 0 || !tt.HasText(locales.ModelDeleted(shared.EngineName(e))) {
		t.Fatalf("models %v, texts %q", m.St, tt.Texts())
	}

	m.St["local-2b"] = models.Status{State: models.Ready}
	k.SetEngine("local-2b")
	tt.Frame()
	if k.Prefs.MT != "local-2b" {
		t.Fatalf("in use %q", k.Prefs.MT)
	}
	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(question) || !tt.HasText(locales.AskDeleteModelInUse(shared.CloudName, size)) {
		t.Fatalf("not said: texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(m.St) != 0 || k.Prefs.MT != "cloud" {
		t.Errorf("models %v, in use %q", m.St, k.Prefs.MT)
	}
}

func TestDiscardChanges(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "scenes"
	tt := ui.NewTester(s.View, Width, 1400)
	open := func() {
		t.Helper()
		s.editAPI(&s.edit, prefs.Endpoint{})
		tt.Frame()
	}
	open()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText(locales.EndpointIntro()) {
		t.Fatalf("open without changes: texts %q", tt.Texts())
	}
	open()
	if err := tt.Click(locales.EndpointName()); err != nil {
		t.Fatal(err)
	}
	tt.Type("Mine")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !tt.HasText(locales.EndpointIntro()) || !tt.HasText(locales.AskDiscard()) {
		t.Fatalf("not asked: texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !tt.HasText(locales.EndpointIntro()) || tt.HasText(locales.AskDiscard()) || !tt.HasText(locales.Save()) {
		t.Fatalf("not back to editing: texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.Click(locales.Discard()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(locales.EndpointIntro()) || len(k.Prefs.Endpoints) != 0 {
		t.Errorf("endpoints %+v, texts %q", k.Prefs.Endpoints, tt.Texts())
	}
}

func TestRecognitionCloud(t *testing.T) {
	k, _, tt := enginesPage(t, nil)
	if err := tt.Click(locales.EngineQwenAsr()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.EngineQwenAsrDescription()) || !tt.HasText(locales.AsrCloudIntro()) || !tt.HasText(locales.NeedsKey()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.UseEngine(locales.EngineQwenAsr())); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != prefs.SystemASR() {
		t.Fatalf("not set up, used: %q", k.Prefs.ASR)
	}
	if err := tt.Click(locales.EndpointKey()); err != nil {
		t.Fatal(err)
	}
	tt.Type("sk-1")
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a := k.API(core.QwenCloud); k.Prefs.ASR != prefs.SystemASR() || !a.Key || a.Model != "qwen-audio-3.1-asr-flash-streaming" {
		t.Errorf("after saving: in use %q, %+v", k.Prefs.ASR, a)
	}
	line := "qwen-audio-3.1-asr-flash-streaming · dashscope.aliyuncs.com · " + locales.KeySaved()
	if !tt.HasText(line) || tt.HasText(locales.AsrCloudIntro()) {
		t.Errorf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.UseEngine(locales.EngineQwenAsr())); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != core.QwenCloud {
		t.Errorf("set up, not used: %q", k.Prefs.ASR)
	}
	tt.Frame()
	if err := tt.Click(locales.Edit()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.AsrCloudIntro()) {
		t.Fatalf("no form: %q", tt.Texts())
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(locales.AsrCloudIntro()) {
		t.Errorf("the form stays: %q", tt.Texts())
	}

	if err := tt.Click("OpenAI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(locales.AsrCloudHost()) {
		t.Errorf("the address shows: %q", tt.Texts())
	}
	if err := tt.Click(locales.ModelAndAddress()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for field, text := range map[string]string{locales.EndpointKey(): "sk-2", locales.AsrCloudHost(): "https://api.groq.com/openai/v1"} {
		if err := tt.Click(field); err != nil {
			t.Fatal(err)
		}
		tt.Type(text)
	}
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.ASR != core.QwenCloud || !k.API(core.OpenAI).Key || !tt.HasText("gpt-live-transcribe · api.groq.com · "+locales.KeySaved()) {
		t.Errorf("in use %q, %+v, texts %q", k.Prefs.ASR, k.Prefs.ASRAPIs, tt.Texts())
	}
}

func TestCloudSizes(t *testing.T) {
	k, _, tt := enginesPage(t, nil)
	if err := tt.Click(locales.EngineIndexCloud()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"2B", "9B", "35B"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("9B"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.MT != "cloud-9b" {
		t.Errorf("in use %q", k.Prefs.MT)
	}
}

func TestTranslationCloud(t *testing.T) {
	k, _, tt := enginesPage(t, nil)
	name := locales.EngineHyMtCloud()
	if err := tt.Click(name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.EngineHyMtCloudDescription()) || !tt.HasText(locales.MtHyMtIntro()) || !tt.HasText(locales.NeedsKey()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.UseEngine(name)); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT == core.HyMTCloud {
		t.Fatal("not set up, used")
	}
	if err := tt.Click(locales.ModelAndAddress()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Pro"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.EndpointKey()); err != nil {
		t.Fatal(err)
	}
	tt.Type("sk-th")
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a := k.API(core.HyMTCloud); k.Prefs.MT == core.HyMTCloud || !a.Key || a.Model != "hy-mt2-pro" || len(k.Prefs.ASRAPIs) != 0 {
		t.Errorf("after saving: in use %q, %+v", k.Prefs.MT, k.Prefs.MTAPIs)
	}
	if !tt.HasText("hy-mt2-pro · tokenhub.tencentmaas.com · " + locales.KeySaved()) {
		t.Errorf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.Use()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != core.HyMTCloud {
		t.Errorf("set up, not used: %q", k.Prefs.MT)
	}
}

func TestDeafRecognizer(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{"sensevoice-small": {State: models.Ready}}}
	k, _, tt := enginesPage(t, m)
	k.Prefs.Src, k.Prefs.Tgt = "fr", "zh"
	if prefs.SystemASR() == "" {
		k.Prefs.ASR = core.OpenAI
	}
	tt.Frame()
	if err := tt.Click("SenseVoice"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fr, en := shared.LangName("fr"), shared.LangName("en")
	if !tt.HasText(locales.DoesntHear(fr)) || !tt.HasText(locales.DoesntHearSwitch(fr, en)) || !tt.HasText(locales.SupportedLanguages()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

func TestEndpointTest(t *testing.T) {
	_, s, tt := enginesPage(t, nil)
	k := s.core
	s.addingAPI = true
	s.editAPI(&s.api, prefs.Endpoint{Name: "DeepSeek", URL: "https://api.deepseek.com", Model: "m"})
	tt.Frame()
	if !tt.HasText(locales.EndpointIntro()) {
		t.Errorf("says not which APIs work: %q", tt.Texts())
	}
	for _, c := range []struct {
		r    core.TestResult
		want []string
	}{
		{core.TestResult{Text: "大家晚上好。", Took: 812 * time.Millisecond}, []string{locales.TestSucceeded("812"), "大家晚上好。"}},
		{core.TestResult{Took: 1300 * time.Millisecond}, []string{locales.TestFailed("1300"), locales.TestNoTranslation()}},
		{core.TestResult{Err: translate.ErrOnlyThinking, Took: 1300 * time.Millisecond}, []string{locales.TestFailed("1300"), locales.TestOnlyThinking()}},
		{core.TestResult{Text: "大家晚上好。", Vision: prefs.VisionYes}, []string{"大家晚上好。", locales.ReadsPictures()}},
		{core.TestResult{Text: "大家晚上好。", Vision: prefs.VisionNo}, []string{"大家晚上好。", locales.ReadsNoPictures()}},
	} {
		k.Tests[s.api.test] = c.r
		tt.Frame()
		for _, want := range c.want {
			if !tt.HasText(want) {
				t.Errorf("%+v: no %q in %q", c.r, want, tt.Texts())
			}
		}
	}
}

func TestEndpointFormat(t *testing.T) {
	_, s, tt := enginesPage(t, nil)
	s.addingAPI = true
	s.editAPI(&s.api, prefs.Endpoint{Name: "Mine", URL: "http://127.0.0.1:8080", Model: "m"})
	tt.Frame()
	if err := tt.Click(locales.EndpointFormatChat()); err != nil {
		t.Fatal(err)
	}
	if got, want := tt.Menu(), []string{locales.EndpointFormatChat(), "Index-Translate", "Hy-MT2"}; !slices.Equal(got, want) {
		t.Fatalf("menu %q, want %q", got, want)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("Hy-MT2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.api.e.Format != "hy-mt" {
		t.Errorf("format %q", s.api.e.Format)
	}
}

func TestImageEngine(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Models = &coretest.Models{St: map[string]models.Status{"local-2b": {State: models.Ready}}}
	})
	k.SaveEndpoint(prefs.Endpoint{Name: "Text only", URL: "https://api.deepseek.com", Model: "m"}, "")
	k.Prefs.Endpoints[0].Vision = prefs.VisionNo
	k.SaveEndpoint(prefs.Endpoint{Name: "Qwen-VL", URL: "https://dashscope.aliyuncs.com", Model: "qwen-vl"}, "")
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1800)
	for _, want := range []string{locales.ImageTitle(), locales.ImageEngine(), locales.ImageShortcutRow()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	row, ok := tt.Find(locales.ImageEngine())
	if !ok {
		t.Fatalf("no picture's engine: %q", tt.Texts())
	}
	tt.ClickAt(Width-kit.Space(8)-kit.Space(5)-kit.Space(10), row.Y+row.H/2)
	if got, want := tt.Menu(), []string{locales.TagCloud(), "Index-Translate 2B", "Index-Translate 9B", "Index-Translate 35B", "Qwen-VL", "-", locales.TagLocal(), "Index-Translate 2B"}; !slices.Equal(got, want) {
		t.Fatalf("menu %q, want %q", got, want)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("Index-Translate 9B"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.ImageMT != "cloud-9b" {
		t.Errorf("pictures with %q, texts %q", k.Prefs.ImageMT, tt.Texts())
	}
}

func TestScenes(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "scenes"
	tt := ui.NewTester(s.View, Width, 1200)
	for _, want := range []string{locales.TabScenes(), locales.SceneGeneral(), locales.SceneInUse(), locales.NoScenes(), locales.NoTermsYet()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click(locales.NewScene()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(k.Scenes) != 2 || !tt.HasText(locales.UseScene()) || !tt.HasText(locales.SceneAbout()) || tt.HasText(locales.NoScenes()) {
		t.Fatalf("%d scenes, texts %q", len(k.Scenes), tt.Texts())
	}
	id := k.Scenes[1].ID

	if err := tt.Click(locales.NewTerm()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for i, text := range []string{"クラウド", "克劳德", "クラウト、グラウド"} {
		if i > 0 {
			tt.Key(0, ui.KeyTab)
		}
		tt.Type(text)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if terms := k.Scenes[1].Terms; len(terms) != 1 || terms[0].Tgt != "克劳德" || !slices.Equal(terms[0].Heard, []string{"クラウト", "グラウド"}) {
		t.Fatalf("terms %+v", terms)
	}
	r, ok := tt.Find(locales.TermHeard())
	if !ok {
		t.Fatal("no column of the ways misheard")
	}
	tt.ClickAt(r.X+20, r.Y+r.H+30)
	tt.Frame()
	if !tt.Focused(locales.TermHeard()) {
		t.Fatal("the cell has no keyboard")
	}
	tt.Key(0, ui.KeyEnd)
	tt.Type(locales.TermHeardSep())
	tt.Frame()
	tt.Type("黒ウド")
	tt.Frame()
	if heard := k.Scenes[1].Terms[0].Heard; !slices.Equal(heard, []string{"クラウト", "グラウド", "黒ウド"}) {
		t.Errorf("heard %q", heard)
	}

	if err := tt.Click(locales.UseScene()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.Scene != id || tt.HasText(locales.UseScene()) {
		t.Errorf("in use %q, texts %q", k.Prefs.Scene, tt.Texts())
	}

	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(k.Scenes[1].Terms) != 0 || !tt.HasText(locales.DeletedItem("クラウド")) {
		t.Fatalf("terms %+v, texts %q", k.Scenes[1].Terms, tt.Texts())
	}
	name := shared.SceneName(k.Scenes[1])
	if err := tt.Click(locales.MoreActions()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.DeleteScene()); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	tt.Frame()
	if len(k.Scenes) != 1 || k.Prefs.Scene != "" || !tt.HasText(locales.SceneGeneral()) || !tt.HasText(locales.DeletedItem(name)) {
		t.Errorf("%d scenes, in use %q, texts %q", len(k.Scenes), k.Prefs.Scene, tt.Texts())
	}
}

func TestHistory(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) { d.HistoryDir = t.TempDir() })
	s := New(k, &coretest.Nav{})
	id := k.AddScene("FF7")
	k.AddTerm(id, scene.Term{Src: "クラウド", Tgt: "克劳德", Heard: []string{"黒ウド"}})
	start := time.Date(2026, 10, 19, 15, 30, 0, 0, time.Local)
	k.History = core.HistoryView{Loaded: true, Records: []history.Record{
		{Path: "/h/gone.jsonl", Head: history.Head{Scene: "gone", Name: "Old", Src: "en", Tgt: "zh", Start: start.Add(time.Hour)}, Lines: 5},
		{Path: "/h/ff7.jsonl", Head: history.Head{Scene: id, Src: "ja", Tgt: "zh", Start: start, App: "Safari"}, Lines: 2},
	}}
	s.Tab = "history"
	tt := ui.NewTester(s.View, Width, 1000)
	for _, want := range []string{locales.KeepHistory(), "FF7", shared.LangName("ja") + " → " + shared.LangName("zh") + " · " + locales.HistoryLines(2) + " · Safari", "Old (" + locales.DeletedScene() + ")", shared.LangName("en") + " → " + shared.LangName("zh") + " · " + locales.HistoryLines(5)} {
		if !hasTextContaining(tt, want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if k.History.Shown != "/h/gone.jsonl" {
		t.Errorf("shown %q, not the latest", k.History.Shown)
	}
	if err := tt.Click("FF7"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if hasTextContaining(tt, "Old") || k.History.Shown != "/h/ff7.jsonl" {
		t.Errorf("shown %q, texts %q", k.History.Shown, tt.Texts())
	}
	k.History.Lines = []history.Entry{{KeptAt: start, Src: "まずはクラウドの武器", BeforeGlossary: "まずは黒ウドの武器", Tgt: "先看克劳德的武器"},
		{KeptAt: start, Src: "スキップ", Err: "behind"}, {KeptAt: start, Src: "失敗", Err: "translate: 500 Internal Server Error: down"}}
	tt.Frame()
	for _, want := range []string{"先看克劳德的武器", locales.TermFixed("黒ウド"), shared.OnWindows(locales.ShowInFinder(), locales.ShowInExplorer()),
		locales.TranslationSkipped(), "translate: 500 Internal Server Error: down"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if slices.Contains(tt.Texts(), "behind") {
		t.Errorf("a skipped line shows its stored reason: %q", tt.Texts())
	}
	if err := tt.Click(locales.DeleteRecord()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !hasTextContaining(tt, locales.DeletedItem("")) || !tt.HasText(locales.Undo()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

func TestHistoryEmpty(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) { d.HistoryDir = t.TempDir() })
	s := New(k, &coretest.Nav{})
	s.Tab = "history"
	tt := ui.NewTester(s.View, Width, 1000)
	k.History = core.HistoryView{Loaded: true}
	tt.Frame()
	if !tt.HasText(locales.HistoryOff()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	r, _ := tt.Find(locales.KeepHistory())
	tt.ClickAt(Width-kit.Space(2)-kit.Space(8)-kit.Space(5)-22, r.Y+r.H/2)
	tt.Frame()
	if !k.Prefs.History || !tt.HasText(locales.NoHistory()) {
		t.Fatalf("kept %v, texts %q", k.Prefs.History, tt.Texts())
	}
	id := k.AddScene("FF7")
	k.History = core.HistoryView{Loaded: true, Records: []history.Record{
		{Path: "/h/general.jsonl", Head: history.Head{Src: "ja", Tgt: "zh", Start: time.Date(2026, 10, 19, 15, 30, 0, 0, time.Local)}, Lines: 2},
	}}
	tt.Frame()
	if err := tt.Click("FF7"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.HistorySceneEmpty()) || tt.HasText(locales.NoHistory()) {
		t.Errorf("texts %q", tt.Texts())
	}

	k.History.Records[0].Head.Scene = id
	s.Tab, s.sceneEdit.scene, s.sceneEdit.id = "scenes", id, true
	tt.Frame()
	if !tt.HasText(locales.NoTermsLearn()) || !tt.HasText(locales.LearnFromRecords()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

func TestCorrections(t *testing.T) {
	terms := []scene.Term{{Src: "クラウド", Tgt: "克劳德", Heard: []string{"黒ウド"}}, {Src: "マテリア", Heard: []string{"マテリヤ"}}}
	for _, c := range []struct {
		src, heard string
		marks      []mark
		was        []string
	}{
		{"まずはクラウドのマテリア", "まずは黒ウドのマテリヤ", []mark{{"まずは", false}, {"クラウド", true}, {"の", false}, {"マテリア", true}}, []string{"黒ウド", "マテリヤ"}},
		{"SU7 来了", "舒七 来了", []mark{{"SU7", true}, {" 来了", false}}, []string{"舒七"}},
	} {
		marks, was := corrections(c.src, c.heard, terms)
		if !slices.Equal(marks, c.marks) || !slices.Equal(was, c.was) {
			t.Errorf("corrections(%q, %q) = %v %q", c.src, c.heard, marks, was)
		}
	}
}

func hasTextContaining(tt *ui.Tester, s string) bool {
	return slices.ContainsFunc(tt.Texts(), func(t string) bool { return strings.Contains(t, s) })
}

func TestLearn(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) { d.HistoryDir = t.TempDir() })
	id := k.AddScene("雷军演讲")
	k.History = core.HistoryView{Loaded: true, Records: []history.Record{
		{Path: "/h/a.jsonl", Head: history.Head{Scene: id, Src: "zh", Tgt: "en", Start: time.Now()}, Lines: 40},
	}}
	s := New(k, &coretest.Nav{})
	s.Tab = "scenes"
	s.sceneEdit = sceneEdit{id: true, scene: id}
	tt := ui.NewTester(s.View, Width, 1000)
	if err := tt.Click(locales.LearnFromRecords()); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	tt.Frame()

	if !tt.HasText(locales.LearnNoEndpoint()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.AddEndpoint()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for i, text := range []string{"Mine", "https://api.example.com", "chat"} {
		if i > 0 {
			tt.Key(0, ui.KeyTab)
		}
		tt.Type(text)
	}
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.LearnSends("Mine")) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.LearnStart()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !k.Learning.Running || k.Prefs.Learn != k.Prefs.Endpoints[0].ID || !hasTextContaining(tt, locales.LearnProgress(0, 0)) {
		t.Fatalf("learning %+v, texts %q", k.Learning, tt.Texts())
	}

	k.Learning = core.Learning{Finished: true, Scene: id, Paths: []string{"/h/a.jsonl"}, Suggestions: []learn.Suggestion{
		{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北", "刘备"}, Count: 4, Example: "在刘北赛道上驰骋"},
		{Src: "卢伟冰", Tgt: "Lu Weibing", Heard: []string{"撸时辰"}, Count: 1, Example: "撸时辰说"},
	}}
	tt.Frame()
	for _, want := range []string{"「在刘北赛道上驰骋」", locales.TermHeard() + ": 刘北, 刘备 · " + locales.HistoryLines(4)} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("卢伟冰 → Lu Weibing"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AddTerm()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	sc, _ := k.SceneByID(id)
	if len(sc.Terms) != 1 || sc.Terms[0].Src != "纽北" || !slices.Equal(sc.Rejected, []string{"撸时辰"}) {
		t.Fatalf("scene %+v", sc)
	}
	if !hasTextContaining(tt, locales.LearnTook(1, "雷军演讲")) || !tt.HasText(locales.Retranslate()) {
		t.Errorf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.Done()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(locales.Retranslate()) || k.Learning.Finished {
		t.Errorf("still open: %q", tt.Texts())
	}
}

func TestLearnGeneral(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "scenes"
	tt := ui.NewTester(s.View, Width, 1000)
	s.learnFrom([]string{"/h/a.jsonl"}, "")
	k.Learning = core.Learning{Finished: true, Paths: []string{"/h/a.jsonl"}, Suggestions: []learn.Suggestion{
		{Src: "Faker", Tgt: "Faker", Heard: []string{"Baker"}, Count: 3, Example: "Baker is back"},
	}}
	tt.Frame()
	if !tt.HasText(locales.NewScene()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.AddTerm()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	scenes := k.SceneList()
	if len(scenes) != 2 || len(scenes[0].Terms) != 0 || len(scenes[1].Terms) != 1 {
		t.Fatalf("scenes %+v", scenes)
	}
}

func TestGeneralGlossary(t *testing.T) {
	k := coretest.New()
	k.AddTerm("", scene.Term{Src: "Faker", Tgt: "Faker"})
	s := New(k, &coretest.Nav{})
	s.Tab = "scenes"
	s.sceneEdit = sceneEdit{id: true, scene: ""}
	tt := ui.NewTester(s.View, Width, 1000)
	if !tt.HasText(locales.TermSrc()) || tt.HasText(locales.TermHeard()) {
		t.Errorf("texts %q", tt.Texts())
	}
	id := k.AddScene("LoL")
	k.AddTerm(id, scene.Term{Src: "Faker", Tgt: "Faker"})
	s.sceneEdit = sceneEdit{id: true, scene: id}
	tt.Frame()
	if !tt.HasText(locales.TermHeard()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

func TestOrder(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.OrderTgtFirst()); err != nil {
		t.Fatal(err)
	}
	if !k.Prefs.TgtFirst {
		t.Error("the source stays on top")
	}
	if err := tt.Click(locales.DisplayTgt()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.OrderLabel()) || !tt.HasText(locales.SrcSize()) {
		t.Error("the order or the original's size gone with the translation only")
	}
	if err := tt.Click(locales.OrderSrcFirst()); err != nil {
		t.Fatal(err)
	}
	if !k.Prefs.TgtFirst {
		t.Error("the order changed with the translation only")
	}
}

func TestOriginalOnly(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.DisplaySrc()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if k.Prefs.Display != prefs.DisplaySrc || k.Prefs.Translates() {
		t.Errorf("display %d, texts %q", k.Prefs.Display, tt.Texts())
	}
	if err := tt.Click(locales.OrderTgtFirst()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.TgtFirst {
		t.Error("the order changed with the original only")
	}
	s.Tab = "home"
	tt.Frame()
	if !tt.HasText(locales.HomeSubtitlesOff()) || tt.HasText(locales.HomeIdle()) {
		t.Errorf("home's title: %q", tt.Texts())
	}
	if err := tt.Click(locales.StartSubtitles()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !k.Running || !tt.HasText(locales.HomeSubtitling()) || !tt.HasText(locales.PauseSubtitles()) {
		t.Errorf("running %v: %q", k.Running, tt.Texts())
	}
	k.Stop()
	k.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplayBoth })
	tt.Frame()
	if !tt.HasText(locales.HomeIdle()) || !tt.HasText(locales.StartTranslating()) {
		t.Errorf("translating again: %q", tt.Texts())
	}
}

func TestSizeOff(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	click := func() {
		r, ok := tt.Find(strconv.Itoa(int(k.Prefs.SrcSize)))
		if !ok {
			t.Fatalf("no original size in %q", tt.Texts())
		}
		tt.ClickAt(r.X-kit.Space(30), r.Y+r.H/2)
		tt.Frame()
	}
	before := k.Prefs.SrcSize
	click()
	if k.Prefs.SrcSize == before {
		t.Fatal("a click on the slider changes nothing")
	}
	k.Prefs.SrcSize, k.Prefs.Display = before, 1
	tt.Frame()
	click()
	if k.Prefs.SrcSize != before {
		t.Errorf("the original's size changed to %v with the translation only", k.Prefs.SrcSize)
	}
}

func TestRerunGuide(t *testing.T) {
	k := coretest.New()
	k.Prefs.Onboarded, k.Prefs.GuideStep = true, 3
	nav := &coretest.Nav{}
	s := New(k, nav)
	s.Tab = "about"
	tt := ui.NewTester(s.View, Width, Height)
	if err := tt.Click(locales.AboutRerun()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.Onboarded || k.Prefs.GuideStep != 0 || !slices.Contains(nav.Went, "guide") {
		t.Errorf("onboarded %v, step %d, went %q", k.Prefs.Onboarded, k.Prefs.GuideStep, nav.Went)
	}
}

func TestProxy(t *testing.T) {
	var got []string
	k := coretest.NewWith(func(d *core.Deps) {
		d.Proxy = func(off bool, addr string) { got = append(got, fmt.Sprint(off, " ", addr)) }
	})
	k.SystemProxy = "127.0.0.1:7897"
	s := New(k, &coretest.Nav{})
	s.Tab = "general"
	tt := ui.NewTester(s.View, Width, 1400)
	tt.Frame()
	if !tt.HasText(locales.ProxySystem("127.0.0.1:7897")) || k.Prefs.NoProxy || len(got) == 0 || got[0] != "false " {
		t.Fatalf("at first: %v, %q", got, tt.Texts())
	}
	k.UpdatePrefs(func(p *prefs.Prefs) { p.ProxyURL = "127.0" })
	tt.Frame()
	if !tt.HasText(locales.ProxyInvalid()) {
		t.Errorf("no address: %q", tt.Texts())
	}
	k.UpdatePrefs(func(p *prefs.Prefs) { p.ProxyURL = " 127.0.0.1:7890 " })
	tt.Frame()
	if k.Prefs.ProxyURL != "127.0.0.1:7890" || got[len(got)-1] != "false 127.0.0.1:7890" || tt.HasText(locales.ProxyInvalid()) {
		t.Errorf("an address: %q, %v", k.Prefs.ProxyURL, got)
	}
	k.UpdatePrefs(func(p *prefs.Prefs) { p.NoProxy = true })
	tt.Frame()
	if got[len(got)-1] != "true 127.0.0.1:7890" || tt.HasText(locales.ProxyAddress()) {
		t.Errorf("off: %v, %q", got, tt.Texts())
	}
}

func TestModelsFolder(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{"local-2b": {State: models.Ready}}}
	k := coretest.NewWith(func(d *core.Deps) { d.Models = m })
	s := New(k, &coretest.Nav{})
	s.Tab, s.shown[core.MT] = "engines", "Index-Translate"
	tt := ui.NewTester(s.View, Width, 1800)
	tt.Frame()
	if !tt.HasText(locales.ModelsFolder()) || !tt.HasText("/models") {
		t.Fatalf("no folder: texts %q", tt.Texts())
	}

	m.MoveStatus = models.Move{Active: true, Done: 1 << 30, Total: 4 << 30}
	tt.Frame()
	if !tt.HasText(locales.ModelsMoving(shared.Size(1<<30), shared.Size(4<<30))) || tt.HasText(locales.ModelsFolderChange()) {
		t.Fatalf("moving: texts %q", tt.Texts())
	}
	if err := tt.Click(locales.Delete()); err == nil {
		t.Error("a model can be deleted while moving")
	}

	m.MoveStatus = models.Move{Done: 4 << 30, Total: 4 << 30}
	tt.Frame()
	if !tt.HasText(locales.ModelsMoved()) || !tt.HasText(locales.ModelsFolderChange()) {
		t.Fatalf("moved: texts %q", tt.Texts())
	}

	m.MoveStatus = models.Move{Err: models.ErrNoSpace{Need: 2 << 30}}
	tt.Frame()
	if !tt.HasText(locales.NoSpace(shared.Size(2 << 30))) {
		t.Errorf("failed: texts %q", tt.Texts())
	}
}

func TestFeedback(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.Version, d.OS, d.Memory = "0.1.4", "macOS 26.6.2", func() (uint64, bool) { return 24 << 30, true }
	})
	s := New(k, &coretest.Nav{})
	s.Tab = "about"
	tt := ui.NewTester(s.View, Width, Height)
	if err := tt.Click(locales.FeedbackOpen()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.FeedbackIntroProblem()) {
		t.Fatalf("no dialog: %q", tt.Texts())
	}
	for _, want := range []string{"Tomoru 0.1.4", "macOS 26.6.2", locales.FeedbackPrivate()} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if texts := strings.Join(tt.Texts(), "\n"); strings.Contains(texts, "GB memory") {
		t.Errorf("the summary shows the memory: %q", texts)
	}
	tt.Type("Subtitles stopped")
	if err := tt.Click(locales.FeedbackCopy()); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); !strings.Contains(got, "Subtitles stopped") || !strings.Contains(got, "24 GB memory") || !strings.Contains(got, "## Steps") || !tt.HasText(locales.Copied()) {
		t.Errorf("copied %q, %q", got, tt.Texts())
	}
	if err := tt.Click(locales.FeedbackOnGitHub()); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || !strings.HasPrefix(got[0], core.Repo+"/issues/new?") || !strings.Contains(got[0], "what=Subtitles+stopped") || !strings.Contains(got[0], "bug_report.yml") {
		t.Errorf("opened %q", got)
	}
	if err := tt.Click(locales.FeedbackIdea()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if !tt.HasText(locales.FeedbackIntroIdea()) || tt.HasText(locales.FeedbackIntroProblem()) {
		t.Errorf("idea's intro: %q", tt.Texts())
	}
	if err := tt.Click(locales.FeedbackCopy()); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); !strings.Contains(got, "## What I'd like\n\nSubtitles stopped") || strings.Contains(got, "Logs: in") {
		t.Errorf("idea copied %q", got)
	}
	if err := tt.Click(locales.FeedbackOnGitHub()); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 2 || !strings.Contains(got[1], "template=feature_request.yml") || !strings.Contains(got[1], "problem=Subtitles+stopped") {
		t.Errorf("opened %q", got)
	}
}
