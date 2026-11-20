package subtitle

import (
	"context"
	"errors"
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
	"tomoru/locales"
)

func TestVisible(t *testing.T) {
	p := prefs.Default("zh-Hans")
	st := live.State{Pending: "コメントも", Draft: "评论也", Lines: []live.Line{
		{ID: 1, Src: "a", Tgt: "A", Done: true},
		{ID: 2, Src: "b", Tgt: "B", Draft: "B draft", Done: false},
		{ID: 3, Src: "c", Err: errors.New("x"), Done: true},
	}}
	p.Shown = 3
	got := Visible(st, 0, p)
	if len(got) != 3 {
		t.Fatalf("%+v", got)
	}
	if got[0].Tgt != "B draft" || !got[0].Draft {
		t.Errorf("a translation streaming in shows the draft: %+v", got[0])
	}
	if got[1].Tgt != locales.TranslationFailed() || !got[1].DimTgt || !got[1].Note {
		t.Errorf("a failed line says why, in the app's language: %+v", got[1])
	}
	if got[0].Note || got[2].Note {
		t.Errorf("translations are not notes: %+v", got)
	}
	if !got[2].Pending || got[2].Key != 4 || got[2].Tgt != "评论也" {
		t.Errorf("the pending sentence: %+v", got[2])
	}

	if got := Visible(live.State{Lines: []live.Line{{ID: 1, Src: "a", Tgt: "A"}, {ID: 2, Src: "b", Tgt: "B", Done: true}}}, 0, p); !got[0].Grows || got[1].Grows {
		t.Errorf("only a translation still streaming in grows: %+v", got)
	}

	// Since: the lines before it went with the last pause.
	if got := Visible(st, 3, p); len(got) != 2 || got[0].Src != "c" {
		t.Errorf("since 3: %+v", got)
	}

	// One at a time: the sentence being spoken keeps the last translation, dimmed.
	p.Shown = 1
	st.Draft = ""
	if got := Visible(st, 0, p); len(got) != 1 || got[0].Src != "コメントも" || !got[0].DimTgt {
		t.Errorf("one at a time: %+v", got)
	}

	// Translation only: what has no translation yet does not show.
	p.Shown, p.Display = 3, prefs.DisplayTgt
	if got := Visible(live.State{Pending: "x", Lines: []live.Line{{ID: 1, Src: "a"}}}, 0, p); len(got) != 0 {
		t.Errorf("translation only: %+v", got)
	}

	// The original only: every line as heard, and what is being said, with no translation, draft
	// or failure, one at a time too.
	p.Display = prefs.DisplaySrc
	st.Draft = "评论也"
	got = Visible(st, 0, p)
	if len(got) != 3 || got[0].Src != "b" || got[2].Src != "コメントも" || !got[2].Pending {
		t.Fatalf("the original only: %+v", got)
	}
	for _, e := range got {
		if e.Tgt != "" || e.Draft || e.DimTgt || e.Swaps {
			t.Errorf("the original only: %+v", e)
		}
	}
	p.Shown = 1
	if got := Visible(st, 0, p); len(got) != 1 || got[0].Src != "コメントも" || got[0].Tgt != "" {
		t.Errorf("the original only, one at a time: %+v", got)
	}
}

// TestOriginalOnly draws the original alone, at its size, and leaves the overlay room for it.
func TestOriginalOnly(t *testing.T) {
	p := prefs.Default("en")
	p.Display = prefs.DisplaySrc
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, []Entry{{Key: 1, Src: "こんばんは", Tgt: "晚上好"}}, p, 1, 600, NoBlock)
	}, 640, 300)
	if _, ok := tt.Find("こんばんは"); !ok || tt.HasText("晚上好") {
		t.Errorf("texts %q", tt.Texts())
	}
	both := p
	both.Display = prefs.DisplayBoth
	if h, all := SlotHeight(p, 1, false), SlotHeight(both, 1, false); h <= 0 || h >= all {
		t.Errorf("slot %v, with the translation %v", h, all)
	}
}

// TestAsSaid shows a sentence said in the language translated into in the translation's place,
// alone, as it is being said too; with the original alone, as an original.
func TestAsSaid(t *testing.T) {
	p := prefs.Default("zh-Hans")
	p.Src, p.Shown = "auto", 3
	st := live.State{Pending: "我们BLG", PendingLang: "zh", Lines: []live.Line{
		{ID: 1, Src: "Hello.", Lang: "en", Tgt: "你好。", Done: true},
		{ID: 2, Src: "我们BLG就是最强的。", Lang: "zh", AsSaid: true, Done: true},
	}}
	got := Visible(st, 0, p)
	if len(got) != 3 || got[0].Src != "Hello." || got[0].Lang != "en" {
		t.Fatalf("%+v", got)
	}
	if got[1].Src != "" || got[1].Tgt != "我们BLG就是最强的。" || got[1].DimTgt || got[1].Note {
		t.Errorf("said in the language translated into: %+v", got[1])
	}
	if got[2].Src != "" || got[2].Tgt != "我们BLG" || !got[2].Pending {
		t.Errorf("being said in it: %+v", got[2])
	}
	p.Display = prefs.DisplayTgt
	if got := Visible(st, 0, p); len(got) != 3 {
		t.Errorf("the translation alone: %+v", got)
	}
	p.Display = prefs.DisplaySrc
	if got := Visible(st, 0, p); len(got) != 3 || got[1].Src != "我们BLG就是最强的。" || got[1].Tgt != "" || got[2].Src != "我们BLG" {
		t.Errorf("the original alone: %+v", got)
	}
}

// TestFailure says why a line has no translation, from what the translator's requests did.
func TestFailure(t *testing.T) {
	replies := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, _ := strconv.Atoi(strings.Split(r.URL.Path, "/")[1])
		http.Error(w, "no", code)
	}))
	defer replies.Close()
	status := func(code int) string { return fmt.Sprintf("%s/%d", replies.URL, code) }
	answer := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-answer }))
	defer hang.Close()
	defer close(answer)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	translated := func(url string) error {
		_, err := (&translate.Endpoint{URL: url, Timeout: 50 * time.Millisecond}).Translate(context.Background(), translate.Request{Src: "ja", Tgt: "zh", Text: "はい"}, nil)
		return err
	}
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"behind", live.ErrBehind, locales.TranslationSkipped()},
		{"401", translated(status(http.StatusUnauthorized)), locales.TranslationKeyRefused()},
		{"403", translated(status(http.StatusForbidden)), locales.TranslationKeyRefused()},
		{"402", translated(status(http.StatusPaymentRequired)), locales.TranslationLimited()},
		{"429", translated(status(http.StatusTooManyRequests)), locales.TranslationLimited()},
		{"refused", translated(closed.URL), locales.TranslationNotAnswering()},
		{"timeout", translated(hang.URL), locales.TranslationNotAnswering()},
		{"stalled", translate.ErrStalled, locales.TranslationNotAnswering()},
		{"deadline", fmt.Errorf("decode: %w", context.DeadlineExceeded), locales.TranslationNotAnswering()},
		{"500", translated(status(http.StatusInternalServerError)), locales.TranslationFailed()},
		{"bad URL", translated("ftp://example.com"), locales.TranslationFailed()},
		{"empty", translate.ErrEmpty, locales.TranslationFailed()},
		{"canceled", context.Canceled, locales.TranslationFailed()},
	} {
		if got := Failure(c.err); got != c.want {
			t.Errorf("%s (%v): %q, want %q", c.name, c.err, got, c.want)
		}
	}
}

// TestOrder draws the translation above the source when asked.
func TestOrder(t *testing.T) {
	for _, tgtFirst := range []bool{false, true} {
		p := prefs.Default("en")
		p.TgtFirst = tgtFirst
		tt := ui.NewTester(func(c *ui.Context) {
			Draw(c, []Entry{{Key: 1, Src: "こんばんは", Tgt: "晚上好"}}, p, 1, 600, NoBlock)
		}, 640, 300)
		src, ok1 := tt.Find("こんばんは")
		tgt, ok2 := tt.Find("晚上好")
		if !ok1 || !ok2 || (tgt.Y < src.Y) != tgtFirst {
			t.Errorf("tgtFirst %v: source at %v, translation at %v", tgtFirst, src.Y, tgt.Y)
		}
	}
}

// TestOpacity draws the text and the box as opaque as chosen, and a sentence still being spoken
// in the original's color, a little faint. The tester's page is white.
func TestOpacity(t *testing.T) {
	draw := func(text, box float64) (ink, beside uint8) {
		p := prefs.Default("en")
		p.Display, p.Style, p.Edge, p.SrcColor = prefs.DisplaySrc, 0, 0, "#ff0000"
		p.TextOpacity, p.BoxOpacity = text, box
		tt := ui.NewTester(func(c *ui.Context) {
			Draw(c, []Entry{{Key: 1, Src: "こんばんは", Pending: true}}, p, 1, 600, NoBlock)
		}, 640, 300)
		r, ok := tt.Find("こんばんは")
		if !ok {
			t.Fatalf("texts %q", tt.Texts())
		}
		img := tt.Image()
		var g uint8
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if px := img.RGBAAt(x, y); px.R > ink {
					ink, g = px.R, px.G
				}
			}
		}
		if box == 100 && g > ink/4 {
			t.Errorf("text %v: not in the original's color: red %d, green %d", text, ink, g)
		}
		return ink, img.RGBAAt(int(r.X)-6, int(r.Y+r.H/2)).G
	}
	full, black := draw(100, 100)
	half, _ := draw(50, 100)
	_, white := draw(100, 0)
	if full < 190 || full > 220 {
		t.Errorf("pending text at %d, want about .8 of 255", full)
	}
	if d := int(half) - int(full)/2; d < -12 || d > 12 {
		t.Errorf("half opacity at %d, full at %d", half, full)
	}
	if black > 5 || white < 250 {
		t.Errorf("beside the text, an opaque box %d, none %d", black, white)
	}
}

// TestBalance wraps a sentence said in lines of about the same length, as wide as the overlay is
// by default, a band as wide as they are; one being said, or a translation streaming in, wraps as
// it comes.
func TestBalance(t *testing.T) {
	const zh = "既然你已经决定独自去北方的雪原找她，那就带上这把剑吧，它一定会保护你的。"
	width := float32(.62*1512 - 24)
	draw := func(style int, e Entry) (lines [2]int, r ui.Rect, img *image.RGBA) {
		p := prefs.Default("zh-Hans")
		p.Style = style
		e.Key, e.Src = 1, "Take this sword."
		tt := ui.NewTester(func(c *ui.Context) { Draw(c, []Entry{e}, p, 1, width, NoBlock) }, int(width)+24, 400)
		r, ok := tt.Find(zh)
		if !ok {
			t.Fatalf("texts %q", tt.Texts())
		}
		img = tt.Image()
		// How wide the ink of each line is: the translation's ice blue, on the tester's white page or
		// a gray band.
		for i := range lines {
			left, right := int(r.X+r.W), 0
			for y := int(r.Y + r.H*float32(i)/2); y < int(r.Y+r.H*float32(i+1)/2); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					if px := img.RGBAAt(x, y); int(px.B)-int(px.R) > 60 {
						left, right = min(left, x), max(right, x)
					}
				}
			}
			lines[i] = max(right-left, 0)
		}
		return lines, r, img
	}
	similar := func(l [2]int) bool { return min(l[0], l[1])*10 >= max(l[0], l[1])*8 }

	for _, style := range []int{1, 0} {
		l, r, img := draw(style, Entry{Tgt: zh})
		if !similar(l) {
			t.Errorf("style %d: lines %v wide, want about the same", style, l)
		}
		if beside := img.RGBAAt(int(r.X+r.W)+30, int(r.Y+r.H/2)); style == 0 && beside.R < 250 {
			t.Errorf("the band reaches past its lines: %v beside them", beside)
		}
	}
	for _, e := range []Entry{{Tgt: zh, Pending: true, Draft: true}, {Tgt: zh, Grows: true}} {
		if l, _, _ := draw(1, e); similar(l) || l[0] < l[1] {
			t.Errorf("%+v: lines %v wide, want the first full", e, l)
		}
	}
}
