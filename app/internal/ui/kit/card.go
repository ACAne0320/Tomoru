package kit

import "github.com/egoist/mygo/ui"

// Card is a part of a page on the paper: white, outlined, raised a little by its faint shadow.
// Give it children; Group titles it, above it.
func Card(c *ui.Context) ui.Element {
	p := Colors(c)
	return p.CardShadow.Cast(ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Card).Border(1, p.Line))
}

// RowsCard is a Card of SettingRows, which keep their own height: only its sides padded as a
// Card's, and a little at its ends.
func RowsCard(c *ui.Context) ui.Element {
	return Card(c).Padding(Space(1), Space(5)).Gap(0)
}

// SandCard is a quieter part, sunk into the paper on sand: a note where a list is empty.
func SandCard(c *ui.Context) ui.Element {
	p := Colors(c)
	return ui.Column(c).Padding(Space(4), Space(5)).Gap(Space(3)).Radius(RadiusCard).Background(p.Sand)
}

// Group is a part of a page under its title, outside its card: the title, what trailing builds at
// its end (a segmented control, a button), and under it the card card makes (Card, RowsCard,
// OptionsCard), filled by body and stretched to the group's height. It returns the group, to take
// its share of a row with Grow and Basis.
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

// Groups is a row of Groups side by side, CardGap apart across and down as they wrap, as tall as
// the tallest: what fn builds.
func Groups(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(CardGap).AlignItems(ui.Stretch).Wrap().Children(fn)
}

// groupTitle is the height of a Group's title, as tall as what may stand at its end, a segmented
// control: groups side by side start their cards on one line, whatever their titles hold.
const groupTitle = 36
