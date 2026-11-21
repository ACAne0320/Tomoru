package kit

import "github.com/egoist/mygo/ui"

func Card(c *ui.Context) ui.Element {
	p := Colors(c)
	return p.CardShadow.Cast(ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Card).Border(1, p.Line))
}

func RowsCard(c *ui.Context) ui.Element {
	return Card(c).Padding(Space(1), Space(5)).Gap(0)
}

func SandCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Sand)
}

func Group(c *ui.Context, title string, trailing func(), card func(*ui.Context) ui.Element, body func()) ui.Element {
	return ui.Column(c).Gap(GroupGap).MinWidth(0).Children(func() {
		ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Height(groupTitle).Padding(0, Space(1)).Children(func() {
			ui.Text(c, title).FontSize(TextSection).FontWeight(WeightBold).TextColor(Colors(c).Ink).Grow(1).MaxLines(1)
			if trailing != nil {
				trailing()
			}
		})
		card(c).Grow(1).Children(body)
	})
}

func Groups(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(CardGap).AlignItems(ui.Stretch).Wrap().Children(fn)
}

const groupTitle = 36
