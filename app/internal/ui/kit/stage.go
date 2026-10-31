package kit

import "github.com/egoist/mygo/ui"

// stars are the stage's, where they shine: fractions of its width and height, and a size.
var stars = []struct{ x, y, r float32 }{
	{.09, .13, 1.5}, {.23, .27, 1}, {.41, .1, 1}, {.62, .2, 1.5}, {.84, .34, 1}, {.14, .46, 1}, {.93, .08, 1}, {.72, .52, 1},
}

// Stage is the night sky that subtitles are previewed on, its stars, and the lantern's warmth
// rising from below. Give it children: the subtitles, at its foot.
func Stage(c *ui.Context) ui.Element {
	p := Colors(c)
	box := ui.Column(c).Radius(RadiusCard + 2).Background(p.Stage).Clip().Justify(ui.End).AlignItems(ui.Center)
	return box.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Clip(r, RadiusCard+2, func() {
			for _, s := range stars {
				d := 2 * s.r
				pt.Fill(ui.Rect{X: r.X + s.x*r.W - s.r, Y: r.Y + s.y*r.H - s.r, W: d, H: d}, ui.RGBA(255, 246, 214, .7), s.r)
			}
			// The warmth: a shadow cast up by a band below the stage, wide and soft.
			band := ui.Rect{X: r.X + r.W*.2, Y: r.Y + r.H + 10, W: r.W * .6, H: 40}
			pt.Shadow(band, 20, 0, -30, 120, 20, p.Lantern.Alpha(.32))
		})
	})
}
