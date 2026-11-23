package kit

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/locales"
)

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
		col := ui.Hex(*v)
		w := ui.ColorWell(c, &col).Label(locales.CustomColor()).Tooltip(locales.CustomColor())
		if custom {
			w = w.Border(2, c.Theme().Accent)
		}
		if w.Changed() {
			*v = hexOf(col)
		}
	})
}

func Slider(c *ui.Context, v *float64, lo, hi float64, unit, label string) ui.Element {
	row := ui.Row(c).Gap(Space(2)).AlignItems(ui.Center)
	if row.IsDisabled() {
		row = row.Opacity(offOpacity)
	}
	widest, _ := c.MeasureText(0, ui.Span{Text: widestSliderValue, Size: TextControl, Weight: WeightBold})
	return row.Children(func() {
		t := *c.Theme()
		t.Accent = Colors(c).Strong
		ui.Themed(c, &t).Background(ui.Transparent).Grow(1).Children(func() {
			ui.Slider(c, v, lo, hi).Step(1).Label(label)
		})
		ui.Text(c, strconv.Itoa(int(*v))+unit).FontSize(TextControl).FontWeight(WeightBold).FontFeatures(Tabular).
			TextColor(Colors(c).InkMuted).MinWidth(float32(math.Ceil(float64(widest)))).TextAlign(ui.End)
	})
}

const widestSliderValue = "000%"

const offOpacity = .45

func hexOf(col ui.Color) string {
	if col.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", col.R, col.G, col.B, col.A)
}

func Tag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.InkWarm).Background(p.Fill).Radius(99).Padding(1, Space(2))
}

func AccentTag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.LanternText).Background(p.LanternTint).Radius(99).Padding(1, Space(2))
}

func OkTag(c *ui.Context, s string) ui.Element {
	p := Colors(c)
	return ui.Text(c, s).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.OkText).Background(p.OkTint).Radius(99).Padding(1, Space(2))
}

func Progress(c *ui.Context, value float64) ui.Element {
	box := ui.Box(c)
	v := box.AnimateTo("value", float32(value), ui.Animation{Duration: 400 * time.Millisecond, Ease: ui.Linear})
	box.Children(func() { ui.Progress(c, float64(v)).AlignSelf(ui.Stretch) })
	return box
}
