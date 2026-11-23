package kit

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

type Kind int

const (
	Primary Kind = iota
	Strong
	Plain
	Soft
	Quiet
	Danger
)

const destructive Kind = 1 << 4

func Destructive(kind Kind) Kind { return kind | destructive }

const Close = Quiet | destructive

type ControlSize int

const (
	Small ControlSize = iota
	Medium
	Large
)

func (s ControlSize) height() float32 {
	switch s {
	case Large:
		return 48
	case Medium:
		return 38
	}
	return 32
}

func (s ControlSize) iconSide() float32 { return s.height() - 2 }

var hover = ui.Animation{Duration: 120 * time.Millisecond}

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

func pressColors(p *Palette, kind Kind, hovered, pressed bool) (col, ink, edge ui.Color) {
	base := kind &^ destructive
	rest, hov, prs, ink := face(p, base)
	edge = p.RaisedLine
	red := kind&destructive != 0 && (hovered || pressed)
	if kind&destructive != 0 && rest.A == 0 {
		rest = p.CloseHover.Alpha(0)
	}
	switch {
	case red && pressed:
		return p.CloseHover.Mix(p.Shade, .18), p.Starlight, p.CloseHover.Mix(p.Shade, .18)
	case red:
		return p.CloseHover, p.Starlight, p.CloseHover
	case pressed:
		return prs, ink, edge
	case hovered:
		return hov, ink, edge
	}
	return rest, ink, edge
}

func press(c *ui.Context, b ui.Element, kind Kind) ui.Element {
	p := Colors(c)
	base := kind &^ destructive
	hovered, pressed := b.Hovered(), b.Pressed()
	_, ink, edge := pressColors(p, kind, hovered, pressed)
	if kind&destructive != 0 {
		ink = AnimateColor(c, b, "ink", hover, func(p *Palette) ui.Color {
			_, ink, _ := pressColors(p, kind, hovered, pressed)
			return ink
		})
		edge = AnimateColor(c, b, "edge", hover, func(p *Palette) ui.Color {
			_, _, edge := pressColors(p, kind, hovered, pressed)
			return edge
		})
	}
	col := AnimateColor(c, b, "face", hover, func(p *Palette) ui.Color {
		col, _, _ := pressColors(p, kind, hovered, pressed)
		return col
	})
	b = b.Background(col).TextColor(ink).Cursor(ui.CursorPointer)
	if base == Plain {
		b = p.RaisedShadow.Cast(b.Border(1, edge))
	}
	return b
}

func Off(c *ui.Context, b ui.Element, off bool) ui.Element {
	if !off {
		return b
	}
	p := Colors(c)
	return b.Disabled(true).Background(p.Fill).TextColor(p.Faint).Border(1, p.Fill).Shadow(0, 0, 0, 0, ui.Transparent).Cursor(ui.CursorDefault)
}

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

func IconButton(c *ui.Context, kind Kind, size ControlSize, icon *ui.SVG, label string) ui.Element {
	return TurnedIconButton(c, kind, size, icon, label, 0)
}

func TurnedIconButton(c *ui.Context, kind Kind, size ControlSize, icon *ui.SVG, label string, degrees float32) ui.Element {
	h := size.iconSide()
	b := ui.ButtonBase(c).Size(h, h).Radius(Pill(h)).Label(label).Tooltip(label).Shrink(0)
	glyph := float32(IconSizeButton)
	if size == Small {
		glyph = IconSizeSmall
	}
	return press(c, b, kind).Children(func() {
		g := ui.Icon(c, icon).FontSize(glyph)
		if degrees != 0 {
			g.Rotate(g.AnimateTo("turn", degrees, Spring))
		}
	})
}

func DeleteButton(c *ui.Context, size ControlSize, label string) ui.Element {
	return IconButton(c, Destructive(Quiet), size, assets.Trash, label)
}

var QuietInset = (Small.iconSide() - IconSizeSmall) / 2

func CancelButton(c *ui.Context, label string) ui.Element {
	return IconButton(c, Destructive(Quiet), Small, assets.Close, label)
}

func Disclosure(c *ui.Context, open *bool, label string) bool {
	p := Colors(c)
	b := ui.ButtonBase(c).Gap(Space(1)).AlignSelf(ui.Start).Cursor(ui.CursorPointer).Expanded(*open).Label(label)
	ink := p.InkMuted
	if b.Hovered() {
		ink = p.Ink
	}
	turn := float32(-90)
	if *open {
		turn = 0
	}
	b.Children(func() {
		ui.Icon(c, assets.Chevron).FontSize(TextControl).TextColor(ink).Rotate(turn)
		ui.Text(c, label).FontSize(TextBody).FontWeight(WeightBold).TextColor(ink)
	})
	if b.Clicked() {
		*open = !*open
	}
	return *open
}
