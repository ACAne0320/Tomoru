package subtitle

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/locales"
)

func TestVisible(t *testing.T) {
	p := prefs.Default("zh-Hans")
	st := live.State{Pending: "コメントも", Draft: "评论也", Lines: []live.Line{
		{ID: 1, Src: "a", Tgt: "A", Done: true},
		{ID: 2, Src: "b", Tgt: "B", Draft: "B draft", Done: false},
		{ID: 3, Src: "c", Err: "x", Done: true},
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
	p.Shown, p.Display = 3, 1
	if got := Visible(live.State{Pending: "x", Lines: []live.Line{{ID: 1, Src: "a"}}}, 0, p); len(got) != 0 {
		t.Errorf("translation only: %+v", got)
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
