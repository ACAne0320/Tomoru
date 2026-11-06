package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// CustomSwatch is the last of a row of color schemes: colors of one's own. A pill, unlike the
// schemes' discs, with a ring of all hues, as macOS's color wells have, and its name, label.
// Chosen, it is ringed in the lantern's amber as they are.
func CustomSwatch(c *ui.Context, label string, on bool) ui.Element {
	p := Colors(c)
	const h = 40
	b := ui.ButtonBase(c).Selected(on).Height(h).Padding(0, Space(4), 0, Space(3)).Gap(Space(2)).Radius(Pill(h)).
		Label(label).Cursor(ui.CursorPointer)
	face := p.Card
	if b.Hovered() || b.Pressed() {
		face = p.Fill
	}
	b = b.Background(b.AnimateColor("face", face, hover))
	if on {
		b = b.Border(2, p.Lantern)
	} else {
		b = b.Border(1, p.LineStrong)
	}
	return b.Children(func() {
		hueRing(c, 22)
		ui.Text(c, label).FontSize(TextControl).FontWeight(WeightBold).TextColor(p.Ink).SingleLine()
	})
}

// hueRing is a ring d across going round all hues.
func hueRing(c *ui.Context, d float32) ui.Element {
	return ui.Box(c).Size(d, d).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		const n, width = 36, 3.5
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-width/2
		at := func(i int) (float32, float32) {
			a := 2*math.Pi*float64(i)/n - math.Pi/2
			return cx + rad*float32(math.Cos(a)), cy + rad*float32(math.Sin(a))
		}
		for i := range n {
			x0, y0 := at(i)
			x1, y1 := at(i + 1)
			// A little past the next, so that the pieces join.
			seg := (&ui.Path{}).MoveTo(x0, y0).LineTo(x1+(x1-x0)*.15, y1+(y1-y0)*.15)
			pt.StrokePath(seg, width, ui.Oklch(.74, .16, 360*float32(i)/n))
		}
	})
}
