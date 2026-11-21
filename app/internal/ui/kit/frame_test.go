package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAppFrameNavTop starts the sidebar's pages as far down under the title bar of each platform as
// in full screen, which has none: below the traffic lights, beside the buttons of the others.
func TestAppFrameNavTop(t *testing.T) {
	selected := "a"
	pages := []NavEntry{{ID: "a", Label: "Home"}, {ID: "b", Label: "About"}}
	tt := ui.NewTester(func(c *ui.Context) {
		AppFrame(c, &selected, pages[:1], pages[1:], func() { ui.Text(c, "Page") })
	}, 800, 500)
	top := func(bar ui.TitleBar) float32 {
		tt.SetTitleBar(bar)
		tt.Frame()
		r, ok := tt.Find("Home")
		if !ok {
			t.Fatalf("%+v: no first page", bar)
		}
		return r.Y
	}
	want := top(ui.TitleBar{})
	for name, bar := range map[string]ui.TitleBar{
		"macOS":         {Height: 52, Left: 80, Bottom: 34},
		"Windows":       {Height: TitleBarHeight, Right: 138, Bottom: TitleBarHeight},
		"Linux, start":  {Height: TitleBarHeight, Left: 100, Bottom: TitleBarHeight},
		"a tall header": {Height: 46, Right: 120, Bottom: 46},
	} {
		got := top(bar)
		if name != "a tall header" && got != want {
			t.Errorf("%s: the first page at %g, in full screen at %g", name, got, want)
		}
		if got < bar.Bottom {
			t.Errorf("%s: the first page at %g, under the buttons to %g", name, got, bar.Bottom)
		}
	}
}
