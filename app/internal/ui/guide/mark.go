package guide

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/kit"
)

// mark is the state of something the guide waits for, size DIPs wide: a ring while it waits, a
// spinner while it works, and once done a green disc popping up with a check drawn in; or a
// warning.
func mark(c *ui.Context, size float32, done, busy, failed bool) ui.Element {
	pal := kit.Colors(c)
	box := ui.Box(c).Size(size, size).Center()
	on := float32(0)
	if done {
		on = 1
	}
	disc := box.AnimateTo("disc", on, ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond, Bounce: .4}})
	tick := box.AnimateTo("tick", on, ui.Animation{Duration: 320 * time.Millisecond, Ease: ui.EaseOut, Delay: 120 * time.Millisecond})
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2
		switch {
		case failed:
			p.Fill(r, pal.Lantern, rad)
			p.Line(cx, cy-rad*.45, cx, cy+rad*.1, rad*.2, pal.Starlight)
			p.Fill(ui.Rect{X: cx - rad*.11, Y: cy + rad*.3, W: rad * .22, H: rad * .22}, pal.Starlight, rad*.11)
			return
		case disc < 1 && !busy:
			p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, pal.LineStrong, rad-1, 1.5)
		}
		if disc > 0 {
			d := rad * disc
			p.Fill(ui.Rect{X: cx - d, Y: cy - d, W: 2 * d, H: 2 * d}, pal.Ok, d)
		}
		if tick > 0 {
			check := (&ui.Path{}).MoveTo(cx-rad*.42, cy+rad*.02).LineTo(cx-rad*.1, cy+rad*.34).LineTo(cx+rad*.45, cy-rad*.32)
			p.StrokePath(check.Trim(0, tick), max(1.6, rad*.2), pal.Starlight)
		}
	})
	if busy && !done && !failed {
		box.Children(func() { ui.Spinner(c).FontSize(size * .7) })
	}
	return box
}
