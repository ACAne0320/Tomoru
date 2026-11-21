package kit

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

func Levels(c *ui.Context, on bool, bars int, color ui.Color) ui.Element {
	still := c.Preferences().ReduceMotion
	box := ui.Box(c).Shrink(0)
	a := float32(.12)
	if on {
		a = 1
	}
	amp := box.AnimateTo("amp", a, ui.Animation{Duration: 300 * time.Millisecond})
	return box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W / float32(bars)
		t := float64(p.Now().UnixMilli()) / 1000
		if still {
			t = 0
		}
		for i := range bars {
			v := .5 + .5*math.Sin(t*5.1+float64(i)*.9)*math.Sin(t*2.3+float64(i)*.37)
			h := max(2, r.H*.35, r.H*(.18+.82*float32(v))*amp)
			p.Fill(ui.Rect{X: r.X + float32(i)*w + w*.2, Y: r.Y + (r.H-h)/2, W: w * .6, H: h}, color, w*.3)
		}
		if !still && amp > .01 {
			p.After(33 * time.Millisecond)
		}
	})
}
