package kit

import "github.com/egoist/mygo/ui"

// SplitCard is a card of two panes side by side, a list and the detail of what it shows: give it
// a SplitList and a SplitDetail. Group takes it as its card.
func SplitCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return p.CardShadow.Cast(ui.Row(c).AlignItems(ui.Stretch).Radius(RadiusCard).Background(p.Card).Border(1, p.Line).Clip())
}

// SplitList is the list's pane of a SplitCard: two fifths of it, its rows 4 in from the card's
// edges as an OptionsCard's, a rule at its end.
func SplitList(c *ui.Context, fn func()) ui.Element {
	return ui.Column(c).Grow(2).Basis(0).MinWidth(0).Padding(Space(1)).BorderEndWidth(1).BorderColor(Colors(c).Line).Children(fn)
}

// SplitDetail is the detail's pane of a SplitCard, the rest of it, padded as a Card.
func SplitDetail(c *ui.Context, fn func()) ui.Element {
	return ui.Column(c).Grow(3).Basis(0).MinWidth(0).Padding(Space(4), Space(5)).Gap(Space(3)).Children(fn)
}
