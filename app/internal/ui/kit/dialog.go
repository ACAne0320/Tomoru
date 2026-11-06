package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/locales"
)

// DialogWidth is a dialog's width: room for a form, and a list of names learned.
const DialogWidth = 520

// Dialog is a modal dialog on a card over the window dimmed in the night's indigo, shown while
// *open: its title, what intro says under it if anything, and what body builds. Escape and a click
// outside close it, as ui.Modal's.
func Dialog(c *ui.Context, open *bool, title, intro string, body func()) ui.Element {
	p := Colors(c)
	w, h := c.Size()
	return ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(p.Scrim)
		panel.Width(DialogWidth).MaxWidth(w-Space(10)).MaxHeight(h-Space(10)).Padding(Space(6)).Gap(Space(4)).
			Radius(RadiusCard).Background(p.Card).Shadow(0, 10, 30, 0, p.Shade.Alpha(.3))
		ui.Column(c).Gap(Space(2)).Children(func() {
			ui.Text(c, title).FontSize(TextTitle).FontWeight(WeightHeavy).TextColor(p.Ink)
			if intro != "" {
				Caption(c, intro)
			}
		})
		body()
	})
}

// DialogActions is a dialog's buttons, at its end: what fn builds, after a spacer it may put first.
func DialogActions(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Margin(Space(1), 0, 0).Children(fn)
}

// Result is how a test went, as a dialog says it under its form: in green when ok, else in amber
// with why under it.
func Result(c *ui.Context, ok bool, title, detail string) ui.Element {
	p := Colors(c)
	col := p.OkText
	if !ok {
		col = p.LanternText
	}
	return ui.Column(c).Gap(Space(1)).Children(func() {
		ui.Text(c, title).FontSize(TextCaption).FontWeight(WeightBold).TextColor(col)
		if detail != "" {
			ui.Text(c, detail).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(4)
		}
	})
}

// ResultMark is a finding under a Result, what icon shows and s says: in green when it is ok, else
// in ink, as "Reads pictures" under a test's translation.
func ResultMark(c *ui.Context, icon *ui.SVG, s string, ok bool) ui.Element {
	p := Colors(c)
	col := p.OkText
	if !ok {
		col = p.Ink
	}
	return ui.Row(c).Gap(Space(1)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).FontSize(TextCaption).TextColor(col)
		ui.Text(c, s).FontSize(TextCaption).FontWeight(WeightMedium).TextColor(col)
	})
}

// Confirm asks before a deletion that can't be undone and changes more than the thing deleted,
// while *open: question, what detail says goes with it if anything, Cancel and action in red. It
// reports whether action was chosen, and closes then.
func Confirm(c *ui.Context, open *bool, question, detail, action string) bool {
	chosen := false
	Dialog(c, open, question, detail, func() {
		confirmActions(c, open, &chosen, action)
	})
	return chosen
}

// PopconfirmWidth is a Popconfirm's width: room for a question and a line under it.
const PopconfirmWidth = 300

// Popconfirm asks by anchor, the button that deletes one thing, before a deletion that can't be
// undone, while *open: question, what detail says goes with it if anything, Cancel and action in
// red. Escape and a click outside close it, and the focus goes back to anchor. It reports whether
// action was chosen, and closes then.
func Popconfirm(c *ui.Context, anchor ui.Element, open *bool, question, detail, action string) bool {
	if !*open {
		return false
	}
	p := Colors(c)
	chosen := false
	ui.Popover(c, anchor, open, func() {
		ui.Column(c).Gap(Space(1)).Children(func() {
			ui.Text(c, question).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink).MaxLines(2)
			if detail != "" {
				Caption(c, detail)
			}
		})
		confirmActions(c, open, &chosen, action)
	}).Width(PopconfirmWidth).Padding(Space(4)).Gap(Space(3)).Radius(RadiusCard).Background(p.Card).Border(1, p.Line)
	return chosen
}

// confirmActions is Confirm's and Popconfirm's buttons: Cancel, focused, and action in red.
func confirmActions(c *ui.Context, open, chosen *bool, action string) {
	DialogActions(c, func() {
		ui.Spacer(c)
		if Button(c, Plain, Small, nil, locales.Cancel()).AutoFocus().Clicked() { // a key pressed keeps it
			*open = false
		}
		if Button(c, Danger, Small, nil, action).Clicked() {
			*open, *chosen = false, true
		}
	})
}

// Guard keeps dialog, a Dialog with changes, open when Escape or a click outside closes it, and
// sets *asking, for the dialog to show Discard in its buttons' place; closed so again while it
// asks, it goes back to editing.
func Guard(dialog ui.Element, open, asking *bool, changed bool) {
	if dialog.Dismissed() && (changed || *asking) {
		*open, *asking = true, !*asking
	}
}

// Discard asks, in a dialog's buttons' place, before its changes are thrown away: Keep editing,
// focused, which unsets *asking, and Discard in red, which closes the dialog.
func Discard(c *ui.Context, open, asking *bool) {
	DialogActions(c, func() {
		ui.Text(c, locales.AskDiscard()).FontSize(TextControl).FontWeight(WeightBold).TextColor(Colors(c).Ink).Grow(1).MaxLines(2)
		if Button(c, Plain, Small, nil, locales.KeepEditing()).AutoFocus().Clicked() {
			*asking = false
		}
		if Button(c, Danger, Small, nil, locales.Discard()).Clicked() {
			*open, *asking = false, false
		}
	})
}
