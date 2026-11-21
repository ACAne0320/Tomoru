package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

func TextField(c *ui.Context, v *string) ui.Element {
	p := Colors(c)
	in := ui.TextInputBase(c, v).Height(Space(10)).Padding(0, Space(3)).Radius(RadiusField).FontSize(TextBody).TextColor(p.Ink)
	return fieldFace(c, in)
}

func TextBox(c *ui.Context, v *string) ui.Element {
	p := Colors(c)
	in := ui.TextAreaBase(c, v).Padding(Space(3)).Radius(RadiusField).FontSize(TextBody).TextColor(p.Ink)
	return fieldFace(c, in)
}

func fieldFace(c *ui.Context, in ui.Element) ui.Element {
	p := Colors(c)
	edge, w := p.Line, float32(1)
	if in.Focused() {
		edge, w = p.Lantern, 1.5
	}
	return in.Background(p.Paper).Border(w, edge)
}

func CellInput(c *ui.Context, v *string) ui.Element {
	p := Colors(c)
	in := ui.TextInputBase(c, v).Height(Space(8)).Padding(0, Space(2)).Radius(RadiusInner).FontSize(TextBody).TextColor(p.Ink).MinWidth(0)
	face, edge := p.Paper.Alpha(0), p.Lantern.Alpha(0)
	switch {
	case in.Focused():
		face, edge = p.Paper, p.Lantern
	case in.Hovered():
		face = p.Paper
	}
	return in.Background(in.AnimateColor("face", face, hover)).Border(1.5, edge)
}

func InUse(c *ui.Context) ui.Element {
	p := Colors(c)
	const d = 18
	return ui.Box(c).Size(d, d).Radius(Pill(d)).Background(p.Ok).Center().Shrink(0).Children(func() {
		ui.Icon(c, assets.Check).FontSize(TextMicro).TextColor(p.Starlight)
	})
}

func InUseLabel(c *ui.Context, label string) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Shrink(0).Children(func() {
		InUse(c)
		ui.Text(c, label).FontSize(TextControl).FontWeight(WeightBold).TextColor(Colors(c).OkText).SingleLine()
	})
}
