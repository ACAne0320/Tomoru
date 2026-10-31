package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/locales"
)

// DialogWidth is a dialog's width: room for a form, and a list of names learned.
const DialogWidth = 520

// Dialog is a modal dialog on a card, shown while *open: its title, what intro says under it if
// anything, and what body builds. Escape and a click outside close it, as ui.Modal's.
func Dialog(c *ui.Context, open *bool, title, intro string, body func()) ui.Element {
	p := Colors(c)
	return ui.Modal(c, open, func() {
		ui.Column(c).Gap(Space(1.5)).Children(func() {
			ui.Text(c, title).FontSize(TextTitle - 2).FontWeight(WeightHeavy).TextColor(p.Ink)
			if intro != "" {
				Caption(c, intro)
			}
		})
		body()
	}).Width(DialogWidth).Padding(Space(6)).Gap(Space(4)).Radius(RadiusCard).Background(p.Card)
}

// DialogActions is a dialog's buttons, at its end: what fn builds, after a spacer it may put first.
func DialogActions(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Margin(Space(1), 0, 0).Children(fn)
}

// Off disables b, a button, when off, and fades it: it does nothing until what it needs is there.
func Off(b ui.Element, off bool) ui.Element {
	if off {
		return b.Disabled(true).Opacity(.4)
	}
	return b
}

// Result is how a test went, as a dialog says it under its form: in green when ok, else in amber
// with why under it.
func Result(c *ui.Context, ok bool, title, detail string) ui.Element {
	p := Colors(c)
	col := p.OkText
	if !ok {
		col = p.LanternText
	}
	return ui.Column(c).Gap(Space(0.5)).Children(func() {
		ui.Text(c, title).FontSize(TextCaption).FontWeight(WeightBold).TextColor(col)
		if detail != "" {
			ui.Text(c, detail).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(4)
		}
	})
}

// Confirm asks before a deletion, while *open: question, what detail says goes with it if anything,
// Cancel and action in red. It reports whether action was chosen, and closes then.
func Confirm(c *ui.Context, open *bool, question, detail, action string) bool {
	chosen := false
	Dialog(c, open, question, detail, func() {
		DialogActions(c, func() {
			ui.Spacer(c)
			if Button(c, Plain, Small, nil, locales.Cancel()).AutoFocus().Clicked() { // a key pressed keeps it
				*open = false
			}
			if Button(c, Danger, Small, nil, action).Clicked() {
				*open, chosen = false, true
			}
		})
	})
	return chosen
}
