package imagecard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/translate"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// TestCard asks for Screen Recording when there is no picture, then selects one; shows the
// picture's translation as it comes, which Copy copies, into the language chosen on it; why it
// failed, with a way to try again; and, long, scrolls in the card, following its end, the foot kept
// at the bottom.
func TestCard(t *testing.T) {
	k := coretest.NewWith(func(d *core.Deps) {
		d.ImageEndpoint = &translate.Endpoint{URL: "http://127.0.0.1:1", Model: "m", Style: translate.Index} // what it brings is dropped
		d.Select = func(context.Context) ([]byte, image.Rectangle, error) { return nil, image.Rectangle{}, nil }
	})
	closed, picked := false, 0
	d := New(k, func() { closed = true }, func() { picked++ })
	tt := ui.NewTester(d.View, Width, Height)

	k.Screen = "unknown"
	tt.Frame()
	if !tt.HasText(locales.ScreenSelectWhy()) || tt.HasText(locales.TranslateInto()) {
		t.Errorf("no picture: %q", tt.Texts())
	}
	if err := tt.Click(locales.ScreenSelectTurnOn()); err != nil || !k.ScreenAsking {
		t.Errorf("turn on: %v, asking %v", err, k.ScreenAsking)
	}
	k.ScreenAsking, k.Screen = false, "denied"
	tt.Frame()
	if !tt.HasText(locales.GuideSelectSettings()) || !tt.HasText(locales.OpenSystemSettings()) {
		t.Errorf("to System Settings: %q", tt.Texts())
	}
	k.Screen = "granted" // turned on in System Settings
	tt.Frame()
	if err := tt.Click(locales.TranslateScreen()); err != nil || picked != 1 {
		t.Errorf("select: %v, picked %d", err, picked)
	}

	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 600, 120)))
	k.Image = core.ImageView{PNG: b.Bytes(), Running: true}
	tt.Frame()
	if !tt.HasText(locales.ImageTranslating()) || !tt.HasText(locales.ImageNote()) {
		t.Errorf("translating: %q", tt.Texts())
	}
	// Come all at once, it comes in evenly: a part first, then the rest, though done meanwhile.
	k.Image.Text = "艾露娜：克劳德。"
	tt.Frame()
	tt.Advance(16 * time.Millisecond)
	tt.Frame()
	if texts := strings.Join(tt.Texts(), ""); tt.HasText("艾露娜：克劳德。") || !strings.Contains(texts, "艾") {
		t.Errorf("streaming: %q", tt.Texts())
	}
	k.Image.Running = false
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("艾露娜：克劳德。") {
		t.Errorf("translated: %q", tt.Texts())
	}
	if err := tt.Click(locales.Copy()); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "艾露娜：克劳德。" || !tt.HasText(locales.Copied()) {
		t.Errorf("copied %q, %q", tt.Clipboard(), tt.Texts())
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if !tt.HasText(locales.Copy()) {
		t.Errorf("still copied: %q", tt.Texts())
	}

	// Into another language: the picture again, the subtitles' kept.
	if !tt.HasText(shared.LangName("zh")) {
		t.Errorf("into %q: %q", k.ImageTgt(), tt.Texts())
	}
	if err := tt.Click(locales.TranslateInto()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(shared.LangName("en")); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	tt.Frame()
	if k.ImageTgt() != "en" || k.Prefs.Tgt != "zh" || !k.Image.Running || !tt.HasText(shared.LangName("en")) {
		t.Errorf("into English: %q, subtitles %q, %+v", k.ImageTgt(), k.Prefs.Tgt, k.Image)
	}

	// Longer than the card, as it comes: it scrolls to its end, under the picture, over the foot.
	var long []string
	for i := range 40 {
		long = append(long, fmt.Sprintf("第%d句", i+1))
	}
	k.Image.Text, k.Image.Running = strings.Join(long, "\n\n"), true
	for range 60 {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}
	last, ok := tt.Find("第40句")
	copyButton, _ := tt.Find(locales.Copy())
	if !ok || last.Y+last.H > copyButton.Y || copyButton.Y+copyButton.H > Height {
		t.Errorf("the end %v, Copy %v, in a card %d tall", last, copyButton, Height)
	}
	k.Image.Running = false

	k.Image = core.ImageView{PNG: b.Bytes(), Err: errors.New("503 Service Unavailable")}
	tt.Frame()
	if !tt.HasText(locales.ImageFailed()) || !tt.HasText("503 Service Unavailable") || !tt.HasText(locales.Retry()) {
		t.Errorf("failed: %q", tt.Texts())
	}

	if err := tt.Click(locales.Close()); err != nil || !closed {
		t.Errorf("close: %v, closed %v", err, closed)
	}
}

// TestLines shows a translation as Markdown, its lines apart but in code: a dialogue a speaker a
// line, a list, emphasis and code; and as it streams in, each piece as much of it as came.
func TestLines(t *testing.T) {
	text := strings.Join([]string{"艾露娜：走吧。", "克劳德：嗯。", "", "## 差异", "- 只有 **Markdown** 版", "", "```", "a", "b", "```", "完"}, "\n")
	for i := range len(text) {
		if !strings.HasPrefix(lines(text), lines(text[:i])) && utf8.RuneStart(text[i]) {
			t.Fatalf("%q is not a piece of %q", lines(text[:i]), lines(text))
		}
	}
	o := markdown.Options{Extensions: parser.GFM}
	tt := ui.NewTester(func(c *ui.Context) { markdown.View(c, lines(text), &o) }, Width, Height)
	tt.Frame()
	got := strings.Join(tt.Texts(), "|")
	for _, want := range []string{"艾露娜：走吧。\n克劳德：嗯。", "差异", "Markdown", "a\nb", "完"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if strings.Contains(got, "**") || strings.Contains(got, "##") || strings.Contains(got, "a  ") {
		t.Errorf("as written: %q", tt.Texts())
	}
}

// TestFit sizes a card for a short strip of dialogue so that it holds the picture, lines of the
// translation and the foot, and no more; a tall picture's, its thumbnail at most thumbHeight, a
// little more. Waiting, the card says so; its thumbnail clicked, the picture shows whole.
func TestFit(t *testing.T) {
	strip := FitHeight(Width, 1200, 120)
	if strip >= Height || strip < MinHeight {
		t.Errorf("a strip: %d tall", strip)
	}
	if tall := FitHeight(Width, 800, 1200); tall <= strip || tall > Height {
		t.Errorf("a page: %d tall, a strip %d", tall, strip)
	}

	k := coretest.NewWith(func(d *core.Deps) {
		d.ImageEndpoint = &translate.Endpoint{URL: "http://127.0.0.1:1", Model: "m", Style: translate.Index}
	})
	d := New(k, func() {}, func() {})
	peeked := 0
	d.Peek = func() { peeked++ }
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1200, 120)))
	k.Image = core.ImageView{PNG: b.Bytes(), Running: true}
	tt := ui.NewTester(d.View, Width, strip)
	tt.Frame()
	waiting, _ := tt.Find(locales.ImageTranslating())
	copyButton, ok := tt.Find(locales.Copy())
	if !ok || copyButton.Y+copyButton.H > float32(strip) || copyButton.Y-(waiting.Y+waiting.H) < fitLines*lineHeight {
		t.Errorf("waiting at %v, Copy %v, in a card %d tall", waiting, copyButton, strip)
	}

	if err := tt.Click(locales.ImageZoomIn()); err != nil || peeked != 1 {
		t.Errorf("whole: %v, peeked %d", err, peeked)
	}
}
