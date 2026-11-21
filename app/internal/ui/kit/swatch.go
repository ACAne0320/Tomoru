package kit

import "github.com/egoist/mygo/ui"

const swatchesPerRow = 7

var swatchSize = (ControlWidth - (swatchesPerRow-1)*Space(1)) / swatchesPerRow

func SchemeSwatch(c *ui.Context, name string, ground, src, tgt ui.Color, on bool) ui.Element {
	return swatchDisc(c, name, on).Background(ground).Children(func() {
		ui.RichText(c).FontSize(TextControl).FontWeight(WeightHeavy).LineHeight(1).Children(func() {
			ui.Text(c, "A").TextColor(src)
			ui.Text(c, "a").TextColor(tgt)
		})
	})
}

func CustomSwatch(c *ui.Context, label string, on bool) ui.Element {
	stops := make([]ui.GradientStop, 0, 13)
	for i := range 13 {
		stops = append(stops, ui.GradientStop{Color: ui.Oklch(.76, .15, 30*float32(i)), At: float32(i) / 12})
	}
	return swatchDisc(c, label, on).ConicGradient(ui.ConicGradient{Stops: stops, Oklab: true})
}

func swatchDisc(c *ui.Context, label string, on bool) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Selected(on).Size(swatchSize, swatchSize).Shrink(0).Radius(Pill(swatchSize)).Label(label).Tooltip(label).
		Column().Justify(ui.Center).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if on {
		return b.Border(2, p.Lantern)
	}
	return b.Border(1, p.LineStrong)
}
