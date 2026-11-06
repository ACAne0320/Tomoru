package kit

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// A SettingRow's heights: a name alone, and a name over its note. Every row is one of the two, so
// that the rules of cards side by side line up.
const (
	rowHeight     = 52
	rowHeightNote = 64
)

// ControlWidth is the column the controls of a RowsCard's rows line up in, at their end.
const ControlWidth = 220

// ControlCardWidth is the least width of a RowsCard whose rows, named labels, each have a control
// in the Control column: Groups side by side take it as their Basis, to wrap before a name is cut.
func ControlCardWidth(c *ui.Context, labels ...string) float32 {
	w := float32(0)
	for _, l := range labels {
		lw, _ := c.MeasureText(0, ui.Span{Text: l, Size: TextBody})
		w = max(w, lw)
	}
	// The card's sides and outline, the name, the gap after it and the column.
	return float32(math.Ceil(float64(2*Space(5) + 2 + w + Space(3) + ControlWidth)))
}

// Control is a SettingRow's control in that column, what fn builds from its start: a Picker, a
// Segmented or a Slider grown to fill it, a field. Switches and buttons keep to the row's end
// without it.
func Control(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Width(ControlWidth).Shrink(0).AlignItems(ui.Center).Children(fn)
}

// SettingRow is a setting of a RowsCard: its name, and what desc says it does under it on one
// line, at the start; the control control builds at the end, centred on them. Rows after the
// first, i above 0, have a rule above them. A row with no name is a note, desc, beside its control.
func SettingRow(c *ui.Context, i int, label, desc string, control func()) ui.Element {
	return settingRow(c, i, label, "", "", desc, false, control, nil)
}

// SettingRowOff is a SettingRow that stays in its place, greyed, while off, as a setting that does
// nothing for now: its name faint, its control disabled, which a kit control inside shows.
func SettingRowOff(c *ui.Context, i int, label, desc string, off bool, control func()) ui.Element {
	return settingRow(c, i, label, "", "", desc, off, control, nil)
}

// SettingRowLink is a SettingRow with a link in its label: link, a part of label, opens url.
func SettingRowLink(c *ui.Context, i int, label, link, url, desc string, control func()) ui.Element {
	return settingRow(c, i, label, link, url, desc, false, control, nil)
}

// SettingRowUnder is a SettingRow with what under builds below its name, note and control, across
// the row, as a move's progress: the row grows for it, its name and control staying where they are.
func SettingRowUnder(c *ui.Context, i int, label, desc string, control, under func()) ui.Element {
	return settingRow(c, i, label, "", "", desc, false, control, under)
}

func settingRow(c *ui.Context, i int, label, link, url, desc string, off bool, control, under func()) ui.Element {
	p := Colors(c)
	h := float32(rowHeight)
	if label != "" && desc != "" {
		h = rowHeightNote
	}
	ink := p.Ink
	if off {
		ink = p.Faint
	}
	col := ui.Column(c).Disabled(off)
	if i > 0 {
		col = col.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
	}
	return col.Children(func() {
		ui.Row(c).Height(h).Gap(Space(3)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
				if label != "" {
					Linked(c, label, link, url, TextBody, ink).MaxLines(1)
				}
				if desc != "" {
					ui.Text(c, desc).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(1).Tooltip(desc)
				}
			})
			if control != nil {
				control()
			}
		})
		if under != nil {
			ui.Column(c).Gap(Space(1)).Padding(0, 0, Space(3)).Children(under)
		}
	})
}

// Stepper counts *v from lo to hi by one, as − and + either side of it, named less and more:
// how many sentences show at once.
func Stepper(c *ui.Context, v *float64, lo, hi float64, less, more string) ui.Element {
	p := Colors(c)
	const h, in = 36, 4
	return ui.Row(c).Height(h).Padding(in).Gap(Space(1)).Radius(RadiusField).Background(p.Fill).AlignItems(ui.Center).Children(func() {
		step := func(icon *ui.SVG, label string, d float64, off bool) {
			b := ui.ButtonBase(c).Size(h-2*in, h-2*in).Radius(RadiusInner).Label(label).Tooltip(label).Disabled(off).TextColor(p.Ink)
			switch {
			case off:
				b = b.TextColor(p.Faint)
			case b.Hovered():
				b = b.Background(p.SegOn).Cursor(ui.CursorPointer)
			}
			if b.Children(func() { ui.Icon(c, icon).FontSize(TextControl) }).Clicked() {
				*v += d
			}
		}
		step(assets.Minus, less, -1, *v <= lo)
		ui.Text(c, strconv.Itoa(int(*v))).FontSize(TextControl).FontWeight(WeightBold).FontFeatures(Tabular).TextColor(p.Ink).MinWidth(Space(6)).TextAlign(ui.Center)
		step(assets.Plus, more, 1, *v >= hi)
	})
}
