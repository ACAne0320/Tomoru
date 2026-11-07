package kit

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/locales"
)

// Swatches choose a color among colors, or any color with the well after them, which opens a
// color picker. The well shows the color as it is, marked as chosen when it is none of colors.
func Swatches(c *ui.Context, v *string, colors []string) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		custom := true
		for _, col := range colors {
			b := ui.ButtonBase(c).Label(col).Width(22).Height(22).Radius(11).Background(ui.Hex(col)).Border(1, ui.RGBA(0, 0, 0, .25))
			if strings.EqualFold(*v, col) {
				b, custom = b.Border(2, c.Theme().Accent), false
			}
			if b.Clicked() {
				*v = col
			}
		}
		ui.Box(c).Width(1).Height(16).Background(c.Theme().Border)
		col := parseHex(*v, ui.RGB(255, 255, 255))
		w := ui.ColorWell(c, &col).Label(locales.CustomColor()).Tooltip(locales.CustomColor())
		if custom {
			w = w.Border(2, c.Theme().Accent)
		}
		if w.Changed() {
			*v = hexOf(col)
		}
	})
}

// Slider sets *v from lo to hi in whole steps, named label, its value after it at the end: a
// subtitle's size. Grown, as in a Control, the track takes the room. In a disabled row
// (SettingRowOff) it fades.
func Slider(c *ui.Context, v *float64, lo, hi float64, label string) ui.Element {
	row := ui.Row(c).Gap(Space(2)).AlignItems(ui.Center)
	if row.IsDisabled() {
		row = row.Opacity(offOpacity)
	}
	return row.Children(func() {
		ui.Slider(c, v, lo, hi).Step(1).Grow(1).Label(label)
		ui.Text(c, strconv.Itoa(int(*v))).FontSize(TextControl).FontWeight(WeightBold).FontFeatures(Tabular).
			TextColor(Colors(c).InkMuted).MinWidth(Space(6)).TextAlign(ui.End)
	})
}

// offOpacity fades a control of MyGo's own colors, which a disabled row can't make faint.
const offOpacity = .45

// parseHex parses a color of the settings file, which a person may have edited: ui.Hex panics
// on bad input.
func parseHex(s string, fallback ui.Color) (col ui.Color) {
	defer func() {
		if recover() != nil {
			col = fallback
		}
	}()
	return ui.Hex(s)
}

// hexOf is col as the settings keep it: #rrggbb, or #rrggbbaa when it is not opaque.
func hexOf(col ui.Color) string {
	if col.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", col.R, col.G, col.B, col.A)
}

// Tag is a small label beside a name.
func Tag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.InkWarm).Background(p.Fill).Radius(99).Padding(1, Space(2))
}

// AccentTag is a Tag in the lantern's amber: a word for the one to pick, as Recommended.
func AccentTag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.LanternText).Background(p.LanternTint).Radius(99).Padding(1, Space(2))
}

// OkTag is a Tag in green: a word for what is ready, as Downloaded.
func OkTag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.OkText).Background(p.OkTint).Radius(99).Padding(1, Space(2))
}

// Progress is a progress bar that goes smoothly from one value to the next, as a download's comes
// a few times a second.
func Progress(c *ui.Context, value float64) ui.Element {
	box := ui.Box(c)
	v := box.AnimateTo("value", float32(value), ui.Animation{Duration: 400 * time.Millisecond, Ease: ui.Linear})
	box.Children(func() { ui.Progress(c, float64(v)).AlignSelf(ui.Stretch) })
	return box
}
