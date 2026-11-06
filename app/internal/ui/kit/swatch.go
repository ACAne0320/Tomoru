package kit

import "github.com/egoist/mygo/ui"

// swatchSize is a color scheme's disc: seven, Space(1) apart, fill the Control column.
const swatchSize = 28

// SchemeSwatch is a color scheme's disc: "Aa" on its edge's color, the ground, as outlined text
// reads: "A" in src's color, "a" in tgt's. Named name in its tooltip, ringed in the lantern's amber
// when chosen.
func SchemeSwatch(c *ui.Context, name string, ground, src, tgt ui.Color, on bool) ui.Element {
	return swatchDisc(c, name, on).Background(ground).Children(func() {
		ui.RichText(c).FontSize(TextControl).FontWeight(WeightHeavy).LineHeight(1).Children(func() {
			ui.Text(c, "A").TextColor(src)
			ui.Text(c, "a").TextColor(tgt)
		})
	})
}

// CustomSwatch is the last disc of a row of color schemes: colors of one's own, every hue around
// it, as large as the schemes' discs, named label in its tooltip.
func CustomSwatch(c *ui.Context, label string, on bool) ui.Element {
	stops := make([]ui.GradientStop, 0, 13)
	for i := range 13 {
		stops = append(stops, ui.GradientStop{Color: ui.Oklch(.76, .15, 30*float32(i)), At: float32(i) / 12})
	}
	return swatchDisc(c, label, on).ConicGradient(ui.ConicGradient{Stops: stops, Oklab: true})
}

// swatchDisc is a disc of a row of color schemes, named label: outlined, or ringed in the lantern's
// amber when chosen.
func swatchDisc(c *ui.Context, label string, on bool) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Selected(on).Size(swatchSize, swatchSize).Shrink(0).Radius(Pill(swatchSize)).Label(label).Tooltip(label).
		Column().Justify(ui.Center).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if on {
		return b.Border(2, p.Lantern)
	}
	return b.Border(1, p.LineStrong)
}
