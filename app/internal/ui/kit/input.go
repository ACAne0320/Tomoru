package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// TextField is a single-line field on the paper, editing *v: the paper's face with a hairline, the
// lantern's ring while it has the keyboard. Name it with Label.
func TextField(c *ui.Context, v *string) ui.Element {
	p := Colors(c)
	in := ui.TextInputBase(c, v).Height(Space(10)).Padding(0, Space(3)).Radius(RadiusField).FontSize(TextBody).TextColor(p.Ink)
	return fieldFace(c, in)
}

// TextBox is TextField's of several lines: a scene's description.
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

// CellInput is a cell of a table, editing *v: text until the pointer is over it, the paper's tint
// then, and the lantern's ring while it has the keyboard. Name it with Label.
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

// ListTile is an item of a list beside what it shows, as the scenes and the records: what lead
// builds at its start, its title and meta under it on a line each, and what trailing builds at its
// end, lead and trailing centred on the two lines. The item shown is on the band, the others on
// sand. Clicked reports a click.
func ListTile(c *ui.Context, on bool, title, meta string, lead, trailing func()) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Padding(Space(3), Space(4)).Gap(Space(3)).Radius(RadiusTile).Justify(ui.Start).AlignItems(ui.Center).
		Role(ui.RoleTab).Selected(on).Label(title).Cursor(ui.CursorPointer)
	face := p.Sand
	switch {
	case on:
		face = p.Band
	case b.Hovered() || b.Pressed():
		face = p.Sand.Mix(p.Band, .5)
	}
	return b.Background(b.AnimateColor("face", face, hover)).Children(func() {
		if lead != nil {
			lead()
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
			ui.Text(c, title).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink).MaxLines(1)
			if meta != "" {
				ui.Text(c, meta).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(1)
			}
		})
		if trailing != nil {
			trailing()
		}
	})
}

// InUse is the mark of what is in use, a scene, a service: a green disc with a check.
func InUse(c *ui.Context) ui.Element {
	p := Colors(c)
	const d = 18
	return ui.Box(c).Size(d, d).Radius(Pill(d)).Background(p.Ok).Center().Shrink(0).Children(func() {
		ui.Icon(c, assets.Check).FontSize(TextMicro).TextColor(p.Starlight)
	})
}

// InUseLabel is InUse with what it says beside it, in green: "In use", in a detail's title.
func InUseLabel(c *ui.Context, label string) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Shrink(0).Children(func() {
		InUse(c)
		ui.Text(c, label).FontSize(TextControl).FontWeight(WeightBold).TextColor(Colors(c).OkText).SingleLine()
	})
}
