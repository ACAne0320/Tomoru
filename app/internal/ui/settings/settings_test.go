package settings

import (
	"fmt"
	"slices"
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

// TestCustomColor chooses the original text's color in the picker of its custom well.
func TestCustomColor(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.SchemeCustom()); err != nil {
		t.Fatal(err)
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
}

// TestScheme chooses a color scheme, which sets the three colors.
func TestScheme(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "subtitles"
	k.Prefs.Edge = 1
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.SchemeSakura()); err != nil {
		t.Fatal(err)
	}
	if p := k.Prefs; p.SrcColor != "#ffffff" || p.TgtColor != "#ffb3cf" || p.EdgeColor != "#4a1030" {
		t.Errorf("colors %s %s %s", p.SrcColor, p.TgtColor, p.EdgeColor)
	}
}

// TestShortcut records new global shortcuts: starting and stopping's, then the picture's, which
// cannot be the same.
func TestShortcut(t *testing.T) {
	k := coretest.New()
	nav := &coretest.Nav{}
	s := New(k, nav)
	s.Tab = "general"
	tt := ui.NewTester(s.View, Width, 900)
	record := func() {
		t.Helper()
		// The field's first keycap: the field's name is its label's too.
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

	// The picture's field, by its last keycap: S of ⌥⇧S.
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

// TestAddEndpoint adds a translation API from the end of the cloud's list of the engines page, uses it,
// and deletes it.
func TestAddEndpoint(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	wants := []string{"Index-Translate 35B", locales.EngineCloudThanks()}
	if prefs.SystemASR() != "" {
		wants = append(wants, locales.EngineApple())
	}
	for _, want := range wants {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("Index-Translate 2B") {
		t.Errorf("a local model shows while the cloud is chosen: %q", tt.Texts())
	}
	// The star at the end of Index-Translate's row goes to its repository.
	if err := tt.Click(locales.StarOnGithub()); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != core.CloudRepo {
		t.Errorf("opened %q", got)
	}
	if err := tt.Click(locales.AddApi()); err != nil {
		t.Fatal(err)
	}
	// The name, then the address and the model, a Tab away each.
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
	if len(k.Prefs.Endpoints) != 1 || k.Prefs.Endpoints[0].URL != "http://localhost:11434" || k.Prefs.Endpoints[0].Model != "qwen3" {
		t.Fatalf("endpoints %+v", k.Prefs.Endpoints)
	}
	if !tt.HasText("qwen3 · localhost:11434") {
		t.Errorf("not listed: %q", tt.Texts())
	}
	if err := tt.Click("Mine"); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != "custom:"+k.Prefs.Endpoints[0].ID {
		t.Errorf("engine %q, texts %q", k.Prefs.MT, tt.Texts())
	}

	// Deleting it doesn't ask: it says so, with a button that puts it back.
	if err := tt.Click(locales.Edit()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(k.Prefs.Endpoints) != 0 || !tt.HasText(locales.EndpointDeleted("Mine")) {
		t.Fatalf("endpoints %+v, texts %q", k.Prefs.Endpoints, tt.Texts())
	}
	if err := tt.Click(locales.Undo()); err != nil {
		t.Fatal(err)
	}
	if len(k.Prefs.Endpoints) != 1 || k.Prefs.MT != "custom:"+k.Prefs.Endpoints[0].ID {
		t.Errorf("after undo: endpoints %+v, engine %q", k.Prefs.Endpoints, k.Prefs.MT)
	}
}

// TestLocalModel chooses a model on this computer to translate with: the cloud translates until it
// is downloaded, and using the cloud again drops it.
func TestLocalModel(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	s.sides[core.MT] = false // as translation's "On device" does: recognition's is first
	tt.Frame()
	for _, want := range []string{"Index-Translate 2B", "Index-Translate 9B", locales.DownloadModel(), shared.ModelNote(models.All[1], models.Status{}, "")} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if k.Chosen(core.MT) != "cloud" {
		t.Errorf("looking chose %q", k.Chosen(core.MT))
	}
	if err := tt.Click("Index-Translate 2B"); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.MT != "cloud" || k.Chosen(core.MT) != "local-2b" || !tt.HasText(locales.EngineWaiting(shared.CloudName)) {
		t.Fatalf("in use %q, chosen %q, texts %q", k.Prefs.MT, k.Chosen(core.MT), tt.Texts())
	}
	s.sides[core.MT] = true
	tt.Frame()
	if err := tt.Click("Index-Translate 35B"); err != nil {
		t.Fatal(err)
	}
	if k.Chosen(core.MT) != "cloud" || tt.HasText(locales.EngineWaiting(shared.CloudName)) {
		t.Errorf("chosen %q, texts %q", k.Chosen(core.MT), tt.Texts())
	}
}

// TestDeleteModel asks by a model's button before deleting it, which can't be undone; deleting the
// one in use asks in a dialog, which says what takes over.
func TestDeleteModel(t *testing.T) {
	m := &coretest.Models{St: map[string]models.Status{"local-2b": {State: models.Ready}}}
	k := coretest.NewWith(func(d *core.Deps) { d.Models = m })
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	s.sides[core.MT] = false
	tt.Frame()
	e, _ := k.Engine(core.MT, "local-2b")
	question, size := locales.AskDeleteModel(shared.EngineName(e)), shared.Size(e.Model.Size)

	// By its button: Escape keeps it; Delete, a Tab from Cancel, deletes it.
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

	// In use, the cloud takes over, as the dialog says.
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

// TestDiscardChanges closes a dialog with Escape at once while nothing changed; with changes, it
// asks first: Escape again goes back to them, Discard throws them away.
func TestDiscardChanges(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	open := func() {
		t.Helper()
		if err := tt.Click(locales.AddApi()); err != nil {
			t.Fatal(err)
		}
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

// TestRecognitionCloud uses the cloud's recognition, which needs setting up: until it is,
// recognition stays on this computer; set up, it is used.
func TestRecognitionCloud(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.TagCloud()); err != nil { // recognition's
		t.Fatal(err)
	}
	if !tt.HasText(locales.EngineQwenAsrDescription()) || !tt.HasText(locales.SetUp()) || k.Chosen(core.ASR) != prefs.DefaultASR() {
		t.Fatalf("chosen %q, texts %q", k.Chosen(core.ASR), tt.Texts())
	}
	if err := tt.Click(locales.EngineQwenAsr()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != prefs.DefaultASR() || k.Chosen(core.ASR) != core.QwenCloud || !tt.HasText(locales.AsrCloudIntro()) {
		t.Fatalf("in use %q, chosen %q, texts %q", k.Prefs.ASR, k.Chosen(core.ASR), tt.Texts())
	}
	if err := tt.Click(locales.EndpointKey()); err != nil {
		t.Fatal(err)
	}
	tt.Type("sk-1")
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	if a := k.ASRAPI(core.QwenCloud); k.Prefs.ASR != core.QwenCloud || !a.Key || a.Model != "qwen-audio-3.1-asr-flash-streaming" {
		t.Errorf("after saving: in use %q, %+v", k.Prefs.ASR, a)
	}
	if !tt.HasText("qwen-audio-3.1-asr-flash-streaming · dashscope.aliyuncs.com") || tt.HasText(locales.EngineWaitingSetup(locales.EngineApple())) {
		t.Errorf("texts %q", tt.Texts())
	}

	// OpenAI's beside it, set up as it is chosen, at a compatible service's address.
	if !tt.HasText(locales.EngineOpenAiDescription()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("OpenAI"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AsrOpenAiIntro()) || k.Prefs.ASR != core.QwenCloud {
		t.Fatalf("in use %q, texts %q", k.Prefs.ASR, tt.Texts())
	}
	for field, text := range map[string]string{locales.EndpointKey(): "sk-2", locales.AsrCloudHost(): "https://api.groq.com/openai/v1"} {
		if err := tt.Click(field); err != nil {
			t.Fatal(err)
		}
		tt.Type(text)
	}
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != core.OpenAI || !k.ASRAPI(core.OpenAI).Key || !tt.HasText("gpt-live-transcribe · api.groq.com") {
		t.Errorf("in use %q, %+v, texts %q", k.Prefs.ASR, k.Prefs.ASRAPIs, tt.Texts())
	}

	// On device: the system's, and the model to download, which the system's stands in for.
	if prefs.SystemASR() == "" {
		return // no system's on this one
	}
	s.sides[core.ASR] = false
	tt.Frame()
	if err := tt.Click(locales.EngineApple()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != "apple" {
		t.Errorf("in use %q", k.Prefs.ASR)
	}
	if err := tt.Click("SenseVoice Small"); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.ASR != "apple" || k.Chosen(core.ASR) != "sensevoice-small" || !tt.HasText(locales.EngineWaiting(locales.EngineApple())) {
		t.Errorf("in use %q, chosen %q, texts %q", k.Prefs.ASR, k.Chosen(core.ASR), tt.Texts())
	}
}

// TestEndpointTest shows whether a test worked and in how many milliseconds: an answer without a
// translation did not work, and a model that only thought says so.
func TestEndpointTest(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "engines"
	tt := ui.NewTester(s.View, Width, 1400)
	s.editEndpoint(prefs.Endpoint{Name: "DeepSeek", URL: "https://api.deepseek.com", Model: "m"})
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
	} {
		k.Tests[s.edit.test] = c.r
		tt.Frame()
		for _, want := range c.want {
			if !tt.HasText(want) {
				t.Errorf("%+v: no %q in %q", c.r, want, tt.Texts())
			}
		}
	}
}

// TestScenes adds a scene, a name to its glossary, edits how it was misheard in place, uses the
// scene, and deletes the name and the scene.
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

	// A name: as written, its translation, how it was heard wrong; Return adds it.
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
	// One more way it was misheard, typed in its cell, under its column's head: the separator stays
	// as typed.
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

	// The name's button deletes it, the scene's menu the scene.
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

// TestHistory shows the switch that keeps the subtitles and the sessions kept, the latest shown;
// filters them by scene, shows a session's lines with the names corrected, and deletes it.
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
	for _, want := range []string{locales.KeepHistory(), "FF7 · Safari", "Old (" + locales.DeletedScene() + ")", shared.LangName("en") + " → " + shared.LangName("zh") + " · " + locales.HistoryLines(5)} {
		if !hasTextContaining(tt, want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if k.History.Shown != "/h/gone.jsonl" {
		t.Errorf("shown %q, not the latest", k.History.Shown)
	}
	// The FF7 scene's only.
	if err := tt.Click("FF7"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if hasTextContaining(tt, "Old") || k.History.Shown != "/h/ff7.jsonl" {
		t.Errorf("shown %q, texts %q", k.History.Shown, tt.Texts())
	}
	// A line skipped says so, as old files and new ones keep it; another failure as it came.
	k.History.Lines = []history.Entry{{At: start, Src: "まずはクラウドの武器", Heard: "まずは黒ウドの武器", Tgt: "先看克劳德的武器"},
		{At: start, Src: "スキップ", Err: "behind"}, {At: start, Src: "失敗", Err: "translate: 500 Internal Server Error: down"}}
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
	// Deleting it doesn't ask: it says so, with a button that puts it back.
	if err := tt.Click(locales.DeleteRecord()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !hasTextContaining(tt, locales.DeletedItem("")) || !tt.HasText(locales.Undo()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

// TestHistoryEmpty says what brings sessions to an empty history: turning the switch on, then
// playing something; filtered to a scene with none while others have some, only that.
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
	if err := tt.Click(locales.TurnOnHistory()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !k.Prefs.History || !tt.HasText(locales.NoHistory()) || tt.HasText(locales.TurnOnHistory()) {
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

	// The scene's glossary: learning from the records it has, else adding a name.
	k.History.Records[0].Head.Scene = id
	s.Tab, s.sceneEdit.scene, s.sceneEdit.id = "scenes", id, true
	tt.Frame()
	if !tt.HasText(locales.NoTermsLearn()) || !tt.HasText(locales.LearnFromRecords()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

// TestCorrections marks the names the glossary corrected in a line: as its names tell them, else
// the stretch the line differs in.
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

// hasTextContaining tells whether a text shown has s in it.
func hasTextContaining(tt *ui.Tester, s string) bool {
	return slices.ContainsFunc(tt.Texts(), func(t string) bool { return strings.Contains(t, s) })
}

// TestLearn learns names from a scene's sessions, from the scene's page: it asks for an API first, then lists what was
// found, each taken at first; one turned down, the other goes into the scene.
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

	// No chat model's API yet: add one, and the dialog comes back with it.
	if !tt.HasText(locales.LearnNoEndpoint()) {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click(locales.AddEndpoint()); err != nil {
		t.Fatal(err)
	}
	tt.Frame() // the dialog's first field has the keyboard
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

	// What the model found, as the core has it once done.
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

// TestOrder puts the translation on top, and has no order with the translation only.
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
	if tt.Frame(); tt.HasText(locales.OrderLabel()) {
		t.Error("an order with the translation only")
	}
}

// TestRerunGuide runs the guide again as on a first launch: it is kept where it is left, to come
// back to after a quit.
func TestRerunGuide(t *testing.T) {
	k := coretest.New()
	k.Prefs.Onboarded, k.Prefs.GuideStep = true, 3
	nav := &coretest.Nav{}
	s := New(k, nav)
	s.Tab = "about"
	tt := ui.NewTester(s.View, Width, Height)
	if err := tt.Click(locales.RerunGuide()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.Onboarded || k.Prefs.GuideStep != 0 || !slices.Contains(nav.Went, "guide") {
		t.Errorf("onboarded %v, step %d, went %q", k.Prefs.Onboarded, k.Prefs.GuideStep, nav.Went)
	}
}

// TestProxy shows the system's proxy, which is used at first, an address that is none, and no
// address once the proxy is off; each choice goes to the app's transport.
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
