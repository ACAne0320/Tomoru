package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// SettingRow is a setting of a card: its name, and what desc says it does, at the start; the
// control control builds at the end. Rows after the first, i above 0, have a rule above them. A
// row with no name is a note, desc, beside its control.
func SettingRow(c *ui.Context, i int, label, desc string, control func()) ui.Element {
	return SettingRowLink(c, i, label, "", "", desc, control)
}

// SettingRowLink is a SettingRow with a link in its label: link, a part of label, opens url.
func SettingRowLink(c *ui.Context, i int, label, link, url, desc string, control func()) ui.Element {
	p := Colors(c)
	row := ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Padding(Space(2), 0).MinHeight(Space(11))
	if i > 0 {
		row = row.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
	}
	return row.Children(func() {
		ui.Column(c).Grow(1).Gap(Space(0.5)).Children(func() {
			if label != "" {
				Linked(c, label, link, url, TextBody, p.Ink)
			}
			if desc != "" {
				ui.Text(c, desc).FontSize(TextCaption).TextColor(p.InkMuted)
			}
		})
		control()
	})
}

// Stepper counts *v from lo to hi by one, as − and + either side of it, named less and more:
// how many sentences show at once.
func Stepper(c *ui.Context, v *float64, lo, hi float64, less, more string) ui.Element {
	p := Colors(c)
	const h = 36
	return ui.Row(c).Height(h).Padding(Space(0.75)).Gap(Space(1)).Radius(RadiusField).Background(p.Fill).AlignItems(ui.Center).Children(func() {
		step := func(icon *ui.SVG, label string, d float64, off bool) {
			b := ui.ButtonBase(c).Size(h-6, h-6).Radius(RadiusInner + 1).Label(label).Tooltip(label).Disabled(off).TextColor(p.Ink)
			if b.Hovered() && !off {
				b = b.Background(p.LineStrong.Alpha(.6))
			}
			if off {
				b = b.Opacity(.35)
			}
			if b.Children(func() { ui.Icon(c, icon).FontSize(TextSmall) }).Clicked() {
				*v += d
			}
		}
		step(assets.Minus, less, -1, *v <= lo)
		ui.Text(c, strconv.Itoa(int(*v))).FontSize(TextSmall).FontWeight(WeightBold).FontFeatures(Tabular).TextColor(p.Ink).MinWidth(Space(6)).TextAlign(ui.Center)
		step(assets.Plus, more, 1, *v >= hi)
	})
}

// SettingBlock is a setting whose control is wide, as a row of swatches: its name above, the
// control under it across the card. Blocks after the first, i above 0, have a rule above them.
func SettingBlock(c *ui.Context, i int, label string, control func()) ui.Element {
	p := Colors(c)
	col := ui.Column(c).Gap(Space(2.5)).Padding(Space(3), 0)
	if i > 0 {
		col = col.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
	}
	return col.Children(func() {
		ui.Text(c, label).FontSize(TextBody).TextColor(p.Ink)
		control()
	})
}
