package kit

import "github.com/egoist/mygo/ui"

func SplitCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return p.CardShadow.Cast(ui.Row(c).AlignItems(ui.Stretch).Radius(RadiusCard).Background(p.Card).Border(1, p.Line).Clip())
}

func SplitList(c *ui.Context, fn func()) ui.Element {
	return ui.Column(c).Grow(2).Basis(0).MinWidth(0).Padding(Space(1)).BorderEndWidth(1).BorderColor(Colors(c).Line).Children(fn)
}

func SplitDetail(c *ui.Context, fn func()) ui.Element {
	return ui.Column(c).Grow(3).Basis(0).MinWidth(0).Padding(Space(4), Space(5)).Gap(Space(3)).Children(fn)
}
