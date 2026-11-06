package kit

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// Kind is the weight of a button among the others on the screen.
type Kind int

const (
	// Primary is the screen's one main action, in the lantern's amber: Start translating, Use this.
	Primary Kind = iota
	// Strong switches a state, in indigo: Stop translating; a chosen chip is its face too.
	Strong
	// Plain is any other, raised a step above the card it is on: Download, Adjust position.
	Plain
	// Soft is a gentle invitation, on the lantern's tint: Learn names from the records.
	Soft
	// Quiet has no face until hovered: an action beside a row of a list.
	Quiet
	// Danger is a deletion, in red, once asked: Delete in a dialog that asks.
	Danger
)

// destructive is a kind's bit that turns its face red as the pointer comes over it.
const destructive Kind = 1 << 4

// Destructive is kind, its face turning red as the pointer comes over it, its ink white, as the
// close buttons of title bars do: what stops, cancels what runs, deletes, closes or quits. A
// dialog's Cancel, which backs out, is not.
func Destructive(kind Kind) Kind { return kind | destructive }

// Close closes a window the app draws the controls of, or quits: Quiet, but red once hovered.
const Close = Quiet | destructive

// ControlSize is a control's height: Small in rows and cards, Medium in a page's header, Large for the
// home page's main action.
type ControlSize int

const (
	Small ControlSize = iota
	Medium
	Large
)

// height is a control's height in points.
func (s ControlSize) height() float32 {
	switch s {
	case Large:
		return 48
	case Medium:
		return 38
	}
	return 32
}

// hover is how fast a control's face follows the pointer.
var hover = ui.Animation{Duration: 120 * time.Millisecond}

// face is the colors of a button of kind: at rest, hovered, pressed, and its ink.
func face(p *Palette, kind Kind) (rest, hovered, pressed, ink ui.Color) {
	switch kind {
	case Primary:
		return p.Lantern, p.LanternHover, p.LanternPressed, p.LanternInk
	case Strong:
		return p.Strong, p.Strong.Mix(p.StrongInk, .12), p.Strong.Mix(p.StrongInk, .24), p.StrongInk
	case Soft:
		return p.LanternTint, p.LanternTint.Mix(p.Lantern, .18), p.LanternTint.Mix(p.Lantern, .3), p.LanternText
	case Quiet:
		return ui.Transparent, p.Raised.Mix(p.LineStrong, .5), p.LineStrong, p.InkWarm
	case Danger:
		return p.Danger, p.Danger.Mix(p.Paper, .12), p.Danger.Mix(p.Ink, .2), p.Paper
	}
	return p.Raised, p.Raised.Mix(p.LineStrong, .5), p.LineStrong, p.Ink
}

// press styles b, a ButtonBase, with the face of kind as the pointer is over it or presses it.
func press(c *ui.Context, b ui.Element, kind Kind) ui.Element {
	p := Colors(c)
	base := kind &^ destructive
	rest, hovered, pressed, ink := face(p, base)
	edge := p.RaisedLine
	red := kind&destructive != 0 && (b.Hovered() || b.Pressed())
	if kind&destructive != 0 && rest.A == 0 {
		rest = p.CloseHover.Alpha(0) // no face at rest: the red fades in, with no grey on the way
	}
	col := rest
	switch {
	case red && b.Pressed():
		col, ink, edge = p.CloseHover.Mix(p.Shade, .18), p.Starlight, p.CloseHover.Mix(p.Shade, .18)
	case red:
		col, ink, edge = p.CloseHover, p.Starlight, p.CloseHover
	case b.Pressed():
		col = pressed
	case b.Hovered():
		col = hovered
	}
	if kind&destructive != 0 {
		ink = b.AnimateColor("ink", ink, hover)
		edge = b.AnimateColor("edge", edge, hover)
	}
	b = b.Background(b.AnimateColor("face", col, hover)).TextColor(ink).Cursor(ui.CursorPointer)
	if base == Plain {
		b = p.RaisedShadow.Cast(b.Border(1, edge))
	}
	return b
}

// Off disables b, a Button or an IconButton, when off: on the track's fill, its label faint, it
// does nothing until what it needs is there.
func Off(c *ui.Context, b ui.Element, off bool) ui.Element {
	if !off {
		return b
	}
	p := Colors(c)
	return b.Disabled(true).Background(p.Fill).TextColor(p.Faint).Border(1, p.Fill).Shadow(0, 0, 0, 0, ui.Transparent).Cursor(ui.CursorDefault)
}

// PictureClose is the close button over a picture, showing icon: a see-through disc of shade with
// a white mark, which hides little of what is under it, and turns the red of Close under the
// pointer. It fades in while shown, the pointer over the picture, and out as it leaves.
func PictureClose(c *ui.Context, icon *ui.SVG, label string, shown bool) ui.Element {
	p := Colors(c)
	const h = 28
	b := ui.ButtonBase(c).Size(h, h).Radius(Pill(h)).Label(label).Tooltip(label).Shrink(0).Cursor(ui.CursorPointer)
	col := p.Shade.Alpha(.38)
	switch {
	case b.Pressed():
		col = p.CloseHover.Mix(p.Shade, .18)
	case b.Hovered():
		col = p.CloseHover
	}
	var o float32
	if shown {
		o = 1
	}
	b = b.Background(b.AnimateColor("face", col, hover)).TextColor(p.Starlight.Alpha(.92)).Opacity(b.AnimateTo("shown", o, hover))
	return b.Children(func() { ui.Icon(c, icon).FontSize(IconSizeSmall) })
}

// Button is a pill with an icon, if icon is not nil, and a label: the app's buttons, by kind.
func Button(c *ui.Context, kind Kind, size ControlSize, icon *ui.SVG, label string) ui.Element {
	h := size.height()
	text, glyph, gap := float32(TextControl), float32(IconSizeSmall), Space(2)
	if size == Large {
		text, glyph, gap = TextSection, IconSizeTile, Space(3)
	}
	b := ui.ButtonBase(c).Height(h).Padding(0, Pill(h)*.8).Gap(gap).Radius(Pill(h)).FontSize(text).FontWeight(WeightBold).Shrink(0)
	b = press(c, b, kind)
	if kind == Primary && size == Large {
		b = b.Shadow(0, 8, 24, 0, Colors(c).Lantern.Alpha(.45))
	}
	return b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(glyph)
		}
		if label != "" {
			ui.Text(c, label).SingleLine()
		}
	})
}

// IconButton is a round button showing icon alone, named label for assistive technology and in
// its tooltip.
func IconButton(c *ui.Context, kind Kind, size ControlSize, icon *ui.SVG, label string) ui.Element {
	h := size.height() - 2
	b := ui.ButtonBase(c).Size(h, h).Radius(Pill(h)).Label(label).Tooltip(label).Shrink(0)
	glyph := float32(IconSizeButton)
	if size == Small {
		glyph = IconSizeSmall
	}
	return press(c, b, kind).Children(func() { ui.Icon(c, icon).FontSize(glyph) })
}

// DeleteButton is the one way a row or a page deletes: a trash can, Quiet until the pointer
// comes over it and turns it red.
func DeleteButton(c *ui.Context, size ControlSize, label string) ui.Element {
	return IconButton(c, Destructive(Quiet), size, assets.Trash, label)
}

// QuietInset is how far in a Small IconButton's glyph is from its edges, as a Quiet one shows no
// face: what ends beside it, as a download's progress over a cancel, ends that far in to line up.
const QuietInset = 8

// CancelButton stops what runs, as a download: a cross, Quiet until it turns red.
func CancelButton(c *ui.Context, label string) ui.Element {
	return IconButton(c, Destructive(Quiet), Small, assets.Close, label)
}
