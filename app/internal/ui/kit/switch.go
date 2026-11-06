package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Switch turns *on on or off, named label for assistive technology: the lantern's amber when on.
func Switch(c *ui.Context, on *bool, label string) ui.Element {
	p := Colors(c)
	const w, h, in = 44, 26, 3
	sw := ui.SwitchBase(c, on).Size(w, h).Radius(Pill(h)).Label(label).Cursor(ui.CursorPointer)
	pos := sw.AnimateTo("knob", b2f(*on), ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: .15}})
	off := p.SwitchOff
	sw = sw.Background(off.Mix(p.Lantern, max(0, min(1, pos))))
	rtl := c.Direction() == ui.RightToLeft
	return sw.Draw(func(pt *ui.Painter, r ui.Rect) {
		d := r.H - 2*in
		at := pos
		if rtl {
			at = 1 - pos
		}
		knob := ui.Rect{X: r.X + in + at*(r.W-r.H), Y: r.Y + in, W: d, H: d}
		pt.Shadow(knob, d/2, 0, 1, 3, 0, ui.RGBA(0, 0, 0, .25))
		pt.Fill(knob, ui.RGB(255, 255, 255), d/2)
	})
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// SwitchPill is a Switch with its name before it, on a plain button's raised face, as tall as a
// page's header's buttons: a setting of the whole page, beside its title. Changed reports a change.
func SwitchPill(c *ui.Context, on *bool, label string) ui.Element {
	p := Colors(c)
	h := Medium.height()
	var sw ui.Element
	p.RaisedShadow.Cast(ui.Row(c).Height(h).Gap(Space(3)).Padding(0, Space(2), 0, Space(4)).AlignItems(ui.Center).
		Radius(Pill(h)).Background(p.Raised).Border(1, p.RaisedLine)).Children(func() {
		ui.Text(c, label).FontSize(TextControl).FontWeight(WeightBold).TextColor(p.Ink).SingleLine()
		sw = Switch(c, on, label)
	})
	return sw
}
