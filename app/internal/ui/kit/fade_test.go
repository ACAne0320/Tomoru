package kit

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFadeScroll(t *testing.T) {
	var s ui.ScrollState
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(Colors(c).Paper).Children(func() {
			FadeScroll(c, &s, func() {
				ui.Box(c).Height(400).Background(Colors(c).Ink)
			})
		})
	}, 100, 100)
	bottom := func() (r uint8) {
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		img := tt.Image()
		return img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()-1).R
	}
	if s.MaxY <= 0 {
		t.Fatalf("does not scroll: %+v", s)
	}
	if r := bottom(); r < 200 {
		t.Errorf("more below, the edge is %d, not faded into the paper", r)
	}
	s.Y = math.MaxFloat32
	if r := bottom(); r > 100 {
		t.Errorf("at the end, the edge is %d, still faded", r)
	}
}
