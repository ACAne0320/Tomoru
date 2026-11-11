package subtitle

import (
	"context"
	"errors"
	"fmt"
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
	if got[1].Tgt != locales.TranslationFailed() || !got[1].DimTgt {
		t.Errorf("a failed line: %+v", got[1])
	}
	if !got[2].Pending || got[2].Key != 4 || got[2].Tgt != "评论也" {
		t.Errorf("the pending sentence: %+v", got[2])
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
