package kit

import (
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMixPalette(t *testing.T) {
	if got := mixPalette(&Light, &Dark, 0, 0); got != Light {
		t.Error("t=0 differs from Light")
	}
	if got := mixPalette(&Light, &Dark, 1, 1); got != Dark {
		t.Error("t=1 differs from Dark")
	}
	mid := mixPalette(&Light, &Dark, .5, .5)
	lv, dv, mv := reflect.ValueOf(Light), reflect.ValueOf(Dark), reflect.ValueOf(mid)
	typ := lv.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		lf, df, mf := lv.Field(i), dv.Field(i), mv.Field(i)
		if reflect.DeepEqual(lf.Interface(), df.Interface()) {
			if !reflect.DeepEqual(mf.Interface(), lf.Interface()) {
				t.Errorf("%s: unchanged token moved at t=0.5", name)
			}
			continue
		}
		if reflect.DeepEqual(mf.Interface(), lf.Interface()) || reflect.DeepEqual(mf.Interface(), df.Interface()) {
			t.Errorf("%s: not interpolated at t=0.5 (got %v, light %v, dark %v)", name, mf.Interface(), lf.Interface(), df.Interface())
		}
	}
}

func TestThemeTransition(t *testing.T) {
	var paper, ink, bg ui.Color
	view := func(c *ui.Context) {
		Paper(c)
		p := Colors(c)
		paper, ink, bg = p.Paper, p.Ink, c.Theme().Background
		ui.Box(c).Fill().Background(paper)
	}
	tt := ui.NewTester(view, 120, 80)
	if paper != Light.Paper || ink != Light.Ink || bg != Light.Paper {
		t.Fatalf("start: paper=%v ink=%v bg=%v", paper, ink, bg)
	}

	tt.SetDark(true)
	tt.Advance(themeInkFade.Delay / 2)
	tt.Frame()
	if paper == Light.Paper || paper == Dark.Paper {
		t.Fatalf("early surface did not lead: %v", paper)
	}
	if ink != Light.Ink {
		t.Fatalf("ink moved before delay: got %v, want %v", ink, Light.Ink)
	}

	tt.Advance(themeSurfaceFade.Duration / 2)
	tt.Frame()
	if paper == Light.Paper || paper == Dark.Paper || ink == Light.Ink || ink == Dark.Ink {
		t.Fatalf("midpoint did not blend both layers: paper=%v ink=%v", paper, ink)
	}
	if bg != paper {
		t.Fatalf("Paper theme bg=%v != palette=%v", bg, paper)
	}

	total := themeInkFade.Delay + themeInkFade.Duration
	tt.Advance(total)
	tt.Frame()
	if paper != Dark.Paper || ink != Dark.Ink || bg != Dark.Paper {
		t.Fatalf("settled dark: paper=%v ink=%v bg=%v", paper, ink, bg)
	}
	if tt.FramePending() {
		t.Fatal("still requesting frames after theme settled")
	}

	tt.SetDark(false)
	tt.Advance(total / 2)
	tt.Frame()
	if paper == Light.Paper || paper == Dark.Paper {
		t.Fatalf("reverse midpoint did not blend: %v", paper)
	}
	tt.SetDark(true)
	tt.Advance(16 * time.Millisecond)
	tt.Frame()
	if paper == Light.Paper || paper == Dark.Paper {
		t.Fatalf("interrupted reversal jumped: %v", paper)
	}
	tt.Advance(total)
	tt.Frame()
	if paper != Dark.Paper || ink != Dark.Ink {
		t.Fatalf("re-settled dark: paper=%v ink=%v", paper, ink)
	}
}

func TestThemeTransitionReduceMotion(t *testing.T) {
	var paper ui.Color
	view := func(c *ui.Context) {
		Paper(c)
		paper = Colors(c).Paper
		ui.Box(c).Fill().Background(paper)
	}
	tt := ui.NewTesterWith(view, 120, 80, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.SetDark(true)
	tt.Frame()
	if paper != Dark.Paper {
		t.Fatalf("reduce motion: got %v, want %v", paper, Dark.Paper)
	}
	if tt.FramePending() {
		t.Fatal("reduce motion still animating")
	}
}

func TestAnimateColorSyncsWithTheme(t *testing.T) {
	var face ui.Color
	view := func(c *ui.Context) {
		Paper(c)
		b := ui.Box(c).Fill()
		face = AnimateColor(c, b, "face", Fade, func(p *Palette) ui.Color { return p.SegOn })
		b.Background(face)
	}
	tt := ui.NewTester(view, 120, 80)
	if face != Light.SegOn {
		t.Fatalf("start: %v", face)
	}
	tt.SetDark(true)
	tt.Advance(themeSurfaceFade.Duration / 2)
	tt.Frame()
	if face == Light.SegOn || face == Dark.SegOn {
		t.Fatalf("midpoint did not blend: %v", face)
	}
	tt.Advance(themeSurfaceFade.Duration/2 + time.Millisecond)
	tt.Frame()
	if face != Dark.SegOn {
		t.Fatalf("lagged behind theme end: got %v, want %v", face, Dark.SegOn)
	}
	tt.Advance(themeInkFade.Delay + themeInkFade.Duration)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("still animating after theme finished")
	}
}

func TestEmptyThemeTransition(t *testing.T) {
	view := func(c *ui.Context) {
		Paper(c)
		ui.Box(c).Fill().Background(Colors(c).Paper).Children(func() {
			Empty(c, ArtScenes, "empty", nil)
		})
	}
	tt := ui.NewTester(view, 240, 220)
	tt.SetDark(true)
	tt.Advance(themeSurfaceFade.Duration / 2)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("expected mid-transition frame pending")
	}
	tt.Advance(themeInkFade.Delay + themeInkFade.Duration)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("expected empty view settled")
	}
}
