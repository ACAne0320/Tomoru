package kit

import "github.com/egoist/mygo/ui"

type Share struct {
	Color       ui.Color
	Name, Sub   string
	Value, Unit string
	Share       float32
	Muted       bool
}

func ShareRow(c *ui.Context, s Share, on bool) ui.Element {
	p := Colors(c)
	ink := p.Ink
	if s.Muted {
		ink = p.InkMuted
	}
	b := ui.ButtonBase(c).Selected(on).Label(s.Name).Padding(Space(2)).Margin(0, -Space(2)).Radius(RadiusField).
		Gap(Space(3)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	hovered := b.Hovered()
	face := AnimateColor(c, b, "face", Fade, func(p *Palette) ui.Color {
		switch {
		case on:
			return p.Band
		case hovered:
			return p.Paper
		default:
			return ui.Transparent
		}
	})
	return b.Background(face).Children(func() {
		ui.Box(c).Size(12, 12).Radius(4).Background(s.Color).Shrink(0)
		ui.Column(c).Basis(Space(52)).Shrink(1).MinWidth(0).Children(func() {
			ui.Text(c, s.Name).FontSize(TextBody).FontWeight(WeightBold).TextColor(ink).MaxLines(1).Tooltip(s.Name)
			if s.Sub != "" {
				ui.Text(c, s.Sub).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(1).Tooltip(s.Sub)
			}
		})
		ui.Box(c).Grow(1).Height(6).Radius(3).Background(p.LineSoft).Children(func() {
			ui.Box(c).WidthPercent(100 * min(max(s.Share, 0), 1)).Height(6).Radius(3).Background(s.Color)
		})
		ui.Row(c).Width(Space(24)).Justify(ui.End).AlignItems(ui.End).Gap(Space(1)).Shrink(0).Children(func() {
			ui.Text(c, s.Value).FontSize(TextBody).FontWeight(WeightHeavy).FontFeatures(Tabular).TextColor(ink).SingleLine()
			if s.Unit != "" {
				ui.Text(c, s.Unit).FontSize(TextCaption).FontWeight(WeightBold).TextColor(p.InkMuted).SingleLine()
			}
		})
	})
}
