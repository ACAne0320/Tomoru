package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/locales"
)

const DialogWidth = 520

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

func DialogActions(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Margin(Space(1), 0, 0).Children(fn)
}

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

func Confirm(c *ui.Context, open *bool, question, detail, action string) bool {
	chosen := false
	Dialog(c, open, question, detail, func() {
		confirmActions(c, open, &chosen, action)
	})
	return chosen
}

const PopconfirmWidth = 300

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

func confirmActions(c *ui.Context, open, chosen *bool, action string) {
	DialogActions(c, func() {
		ui.Spacer(c)
		if Button(c, Plain, Small, nil, locales.Cancel()).AutoFocus().Clicked() {
			*open = false
		}
		if Button(c, Danger, Small, nil, action).Clicked() {
			*open, *chosen = false, true
		}
	})
}

type DiscardGuard struct{ Confirming bool }

func (g *DiscardGuard) Guard(dialog ui.Element, open *bool, changed bool) {
	if !dialog.Dismissed() {
		return
	}
	switch {
	case g.Confirming:
		g.Confirming = false
	case changed:
		g.Confirming = true
	default:
		return
	}
	*open = true
}

func (g *DiscardGuard) Actions(c *ui.Context, open *bool) {
	DialogActions(c, func() {
		ui.Text(c, locales.AskDiscard()).FontSize(TextControl).FontWeight(WeightBold).TextColor(Colors(c).Ink).Grow(1).MaxLines(2)
		if Button(c, Plain, Small, nil, locales.KeepEditing()).AutoFocus().Clicked() {
			g.Confirming = false
		}
		if Button(c, Danger, Small, nil, locales.Discard()).Clicked() {
			*open, g.Confirming = false, false
		}
	})
}
