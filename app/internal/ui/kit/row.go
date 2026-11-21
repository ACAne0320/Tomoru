package kit

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

const (
	rowHeight     = 52
	rowHeightNote = 64
)

const ControlWidth = 220

func ControlCardWidth(c *ui.Context, labels ...string) float32 {
	w := float32(0)
	for _, l := range labels {
		lw, _ := c.MeasureText(0, ui.Span{Text: l, Size: TextBody})
		w = max(w, lw)
	}
	return float32(math.Ceil(float64(2*Space(5) + 2 + w + Space(3) + ControlWidth)))
}

func Control(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Width(ControlWidth).Shrink(0).AlignItems(ui.Center).Children(fn)
}

func SettingRow(c *ui.Context, i int, label, desc string, control func()) ui.Element {
	return settingRow(c, i, label, desc, false, control, nil)
}

func SettingRowOff(c *ui.Context, i int, label, desc string, off bool, control func()) ui.Element {
	return settingRow(c, i, label, desc, off, control, nil)
}

func SettingRowUnder(c *ui.Context, i int, label, desc string, control, under func()) ui.Element {
	return settingRow(c, i, label, desc, false, control, under)
}

func settingRow(c *ui.Context, i int, label, desc string, off bool, control, under func()) ui.Element {
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
					ui.Text(c, label).FontSize(TextBody).TextColor(ink).MaxLines(1)
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
