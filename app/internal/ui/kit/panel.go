package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

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

func ListCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Radius(RadiusField).Background(p.Card).Border(1, p.Line).Clip()
}

func MenuIcon(c *ui.Context, icon *ui.SVG) ui.Element {
	return ui.Icon(c, icon).FontSize(menuIcon).TextColor(Colors(c).LanternText)
}

func MenuLevels(c *ui.Context, on bool) ui.Element {
	return Levels(c, on, 4, Colors(c).LanternText).Size(menuIcon, menuIcon)
}

const menuIcon = IconSizeMenu

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
