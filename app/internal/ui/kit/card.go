package kit

import "github.com/egoist/mygo/ui"

// Card is a group of a page on the paper: white, outlined. Give it children; CardHeader titles it.
func Card(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Card).Border(1, p.Line)
}

// SandCard is a quieter group, on sand: what the home page shows besides the subtitles.
func SandCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Sand)
}

// CardHeader is a card's title, and what trailing builds at its end: an icon button or two.
func CardHeader(c *ui.Context, title string, trailing func()) ui.Element {
	return ui.Row(c).Gap(Space(2.5)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(TextSection).FontWeight(WeightBold).TextColor(Colors(c).Ink).Grow(1).MaxLines(1)
		if trailing != nil {
			trailing()
		}
	})
}
