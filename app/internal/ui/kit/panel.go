package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// PanelFrame is the menu bar panel of docs/design-system.md: the night's band at its top, which
// head builds, the paper under it, which body builds, and the foot, under a rule. The flyout shows
// nothing else: it returns the panel, as tall as what it holds, to fit the flyout to it.
func PanelFrame(c *ui.Context, head, body, foot func()) ui.Element {
	Paper(c)
	p := Colors(c)
	c.Root().Background(ui.Transparent)
	return ui.Column(c).Shrink(0).Radius(RadiusFrame).Background(p.Paper).Clip().Children(func() {
		ui.Row(c).Background(p.Night).Padding(Space(4)).Gap(Space(3)).AlignItems(ui.Center).Children(head)
		ui.Column(c).Padding(Space(4)).Gap(Space(3)).Children(body)
		ui.Row(c).Padding(Space(2), Space(4)).Gap(Space(2)).AlignItems(ui.Center).BorderWidth(1, 0, 0).BorderColor(p.Line).Children(foot)
	})
}

// Sticker is the mascot's sticker, h tall, taking the room of a square of side room in a row: it
// spills into the row's padding above, below and to the left, and what is left of its width into
// the gap after it. Awake, she and the penguin open their eyes; the two pictures cross-fade.
func Sticker(c *ui.Context, h, room float32, awake bool) ui.Element {
	w := h * assets.StickerAspect
	spill := (h - room) / 2
	b := ui.Box(c).Size(w, h).Margin(-spill, room-w+spill, -spill, -spill).Shrink(0)
	var open float32
	if awake {
		open = 1
	}
	open = b.AnimateTo("awake", open, Fade)
	return b.Children(func() {
		ui.Image(c, assets.StickerAsleep).Absolute().Left(0).Top(0).Size(w, h)
		ui.Image(c, assets.Sticker).Absolute().Left(0).Top(0).Size(w, h).Opacity(open)
	})
}

// PickerField is a wide button showing value, which opens a native menu that build fills: a
// language of the panel. Name it with Label.
func PickerField(c *ui.Context, value string, build func(m *ui.Menu)) ui.Element {
	p := Colors(c)
	const h = 40
	b := ui.ButtonBase(c).Height(h).Padding(0, Space(3)).Gap(Space(2)).Radius(RadiusField).Justify(ui.SpaceBetween).
		FontSize(TextBody).FontWeight(WeightBold).MinWidth(0).Border(1, p.Line).Cursor(ui.CursorPointer)
	col := p.Card
	if b.Hovered() || b.Pressed() {
		col = p.Fill
	}
	return b.Background(b.AnimateColor("face", col, hover)).TextColor(p.Ink).Menu(build).Children(func() {
		ui.Text(c, value).MaxLines(1).MinWidth(0)
		ui.Icon(c, assets.Chevron).FontSize(TextControl).TextColor(p.InkWarm)
	})
}

// ListCard is a white card of rows, edge to edge: MenuRows.
func ListCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Radius(RadiusField).Background(p.Card).Border(1, p.Line).Clip()
}

// MenuIcon is a MenuRow's icon, of the lantern's amber.
func MenuIcon(c *ui.Context, icon *ui.SVG) ui.Element {
	return ui.Icon(c, icon).FontSize(menuIcon).TextColor(Colors(c).LanternText)
}

// MenuLevels is a MenuRow's Levels in its icon's place, as what a source hears: rising while a
// voice is heard.
func MenuLevels(c *ui.Context, on bool) ui.Element {
	return Levels(c, on, 4, Colors(c).LanternText).Size(menuIcon, menuIcon)
}

// menuIcon is the size of a MenuRow's icon.
const menuIcon = IconSizeMenu

// MenuRow is a row of a ListCard that opens a native menu build fills: what lead builds first, its
// icon (MenuIcon), then its name, the value chosen, and what after builds beside it, as a count.
// Rows after the first, i above 0, have a rule above them.
func MenuRow(c *ui.Context, i int, lead func(), label, value string, after func(), build func(m *ui.Menu)) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Height(Space(11)).Padding(0, Space(3)).Gap(Space(3)).Justify(ui.Start).Label(label).Cursor(ui.CursorPointer)
	if i > 0 {
		b = b.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
	}
	col := p.Fill.Alpha(0)
	if b.Hovered() || b.Pressed() {
		col = p.Fill
	}
	return b.Background(b.AnimateColor("face", col, hover)).Menu(build).Children(func() {
		lead()
		ui.Text(c, label).FontSize(TextControl).TextColor(p.InkMuted).MinWidth(Space(9)).SingleLine()
		ui.Row(c).Grow(1).MinWidth(0).Gap(Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, value).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink).MaxLines(1).MinWidth(0)
			if after != nil {
				after()
			}
		})
		ui.Icon(c, assets.Chevron).FontSize(TextControl).TextColor(p.InkWarm)
	})
}
