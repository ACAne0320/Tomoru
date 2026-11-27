package overlay

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
)

const (
	fitMargin = 28
	fitStep   = 40
	fitLinger = 400 * time.Millisecond
)

type fit struct {
	frame       mygo.Rectangle
	fitted      bool
	hidden      bool
	asked       placement
	strayTo     mygo.Rectangle
	lastTop     float32
	reach       span
	chromeUntil time.Time
	drawnUntil  time.Time
}

type placement struct {
	frame  mygo.Rectangle
	hidden bool
}

func (o *Overlay) whole() mygo.Rectangle {
	return mygo.Rectangle{X: o.area.X, Y: o.area.Y, Width: o.area.Width, Height: o.area.Height + below}
}

func (o *Overlay) canvas(ww float32) (width, x float32, y int) {
	if !o.fitted {
		return ww, 0, 0
	}
	return float32(o.area.Width), float32(o.frame.X - o.area.X), o.frame.Y - o.area.Y
}

func (o *Overlay) top(e ui.Element, guess float32) float32 {
	if b := e.Bounds(); b.H > 0 {
		return o.lastTop + b.Y
	}
	return guess
}

func (o *Overlay) moveAhead(p prefs.Prefs) {
	if !fitsWindow || o.win == nil || !o.fitted || o.hidden || !o.reach.set || hasChildWindows(o.win) {
		return
	}
	width, bottom := float32(o.area.Width), float32(o.blockBottom())
	want := o.around(ui.Rect{X: float32(p.AreaX-p.AreaW/2) * width, Y: bottom + o.reach.top, W: float32(p.AreaW) * width, H: o.reach.bottom - o.reach.top})
	if want == o.frame || want.Width != o.frame.Width || want.Height != o.frame.Height {
		return
	}
	o.frame, o.asked = want, placement{frame: want}
	o.win.SetBounds(want)
}

func (o *Overlay) place(c *ui.Context, drawn bool, drawing ui.Rect) {
	if !fitsWindow || o.win == nil {
		return
	}
	now := c.Now()
	if drawn {
		o.drawnUntil = now.Add(fitLinger)
	}
	want := placement{frame: o.frame}
	if drawing.H > 0 && !hasChildWindows(o.win) {
		want.frame = o.around(drawing)
	}
	if wait := o.drawnUntil.Sub(now); wait > 0 {
		c.After(wait)
	} else {
		want = placement{frame: o.frame, hidden: true}
	}
	if want == o.asked && !o.strayed() {
		return
	}
	o.asked = want
	o.win.Update(func() { o.moveFrame(want) })
}

func (o *Overlay) strayed() bool {
	b := o.win.Bounds()
	if !o.moved() || b == o.strayTo {
		return false
	}
	o.strayTo = b
	return true
}

func (o *Overlay) moved() bool {
	b := o.win.Bounds()
	return max(abs(b.X-o.frame.X), abs(b.Y-o.frame.Y), abs(b.Width-o.frame.Width), abs(b.Height-o.frame.Height)) > 1
}

func abs(v int) int { return max(v, -v) }

func (o *Overlay) around(r ui.Rect) mygo.Rectangle {
	whole := o.whole()
	x0 := max(whole.X, whole.X+int(r.X)-fitMargin)
	x1 := min(whole.X+whole.Width, whole.X+int(r.X+r.W)+fitMargin)
	y1 := min(whole.Y+whole.Height, int(r.Y+r.H)+fitMargin)
	y0 := max(whole.Y, y1-steps(y1-int(r.Y)+fitMargin))
	return mygo.Rectangle{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

func (o *Overlay) moveFrame(p placement) {
	if o.asked != p {
		return
	}
	if p.hidden && !o.hidden {
		o.win.SetOpacity(0)
	}
	if p.frame != o.frame || o.moved() {
		o.frame, o.fitted = p.frame, p.frame != o.whole()
		anchorBottomLeft(o.win)
		o.win.SetBounds(p.frame)
	}
	if !p.hidden && o.hidden {
		o.win.SetOpacity(1)
	}
	o.hidden = p.hidden
}

type span struct {
	top, bottom float32
	set         bool
}

func (s *span) add(top, bottom float32) {
	if s.set {
		top, bottom = min(s.top, top), max(s.bottom, bottom)
	}
	s.top, s.bottom, s.set = top, bottom, true
}

func steps(h int) int { return (h + fitStep - 1) / fitStep * fitStep }
