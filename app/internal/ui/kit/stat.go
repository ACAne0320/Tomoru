package kit

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"tomoru/locales"
)

type Stat struct{ Label, Value, Unit, Note string }

func StatStrip(c *ui.Context, label string, stats []Stat) ui.Element {
	p := Colors(c)
	return p.CardShadow.Cast(ui.Row(c).Wrap().AlignItems(ui.Stretch).Radius(RadiusCard).Background(p.Card).Border(1, p.Line)).Role(ui.RoleGroup).Label(label).Children(func() {
		for i, s := range stats {
			col := ui.Column(c).Grow(1).Basis(150).Gap(Space(1)).Padding(Space(4), Space(5))
			if i > 0 {
				col = col.BorderStartWidth(1).BorderColor(p.LineSoft)
			}
			col.Children(func() {
				ui.Text(c, s.Label).FontSize(TextCaption).TextColor(p.InkMuted).SingleLine()
				ui.Row(c).Gap(Space(1)).AlignItems(ui.Baseline).Children(func() {
					ui.Text(c, s.Value).FontSize(TextStat).FontWeight(WeightHeavy).FontFeatures(Tabular).TextColor(p.Ink).SingleLine()
					if s.Unit != "" {
						ui.Text(c, s.Unit).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.InkMuted).SingleLine()
					}
				})
				if s.Note != "" {
					ui.Text(c, s.Note).FontSize(TextCaption).FontFeatures(Tabular).TextColor(p.InkMuted).SingleLine()
				}
			})
		}
	})
}

type Fact struct{ Label, Value string }

func Facts(c *ui.Context, facts []Fact) ui.Element {
	p := Colors(c)
	return ui.Row(c).Wrap().GapX(Space(6)).GapY(Space(2)).Children(func() {
		for i, f := range facts {
			ui.Column(c.Key(i)).Gap(Space(1)).Children(func() {
				ui.Text(c, f.Label).FontSize(TextCaption).TextColor(p.InkMuted).SingleLine()
				ui.Text(c, f.Value).FontSize(TextControl).FontWeight(WeightBold).FontFeatures(Tabular).TextColor(p.Ink).SingleLine()
			})
		}
	})
}

func Number(v float64) string {
	v = math.Round(v*1e6) / 1e6
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return Count(int(v))
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func unitLast(ticks []chart.Tick, format func(float64) string) {
	for i := range ticks {
		ticks[i].Label = Number(ticks[i].Value)
	}
	if n := len(ticks); n > 0 {
		ticks[n-1].Label = format(ticks[n-1].Value)
	}
}

func Clock(d time.Duration) string {
	s := int(d.Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func Count(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

func SecondsValue(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.2f", d.Seconds())
	}
	return fmt.Sprintf("%.1f", d.Seconds())
}

func Seconds(d time.Duration) string { return locales.Seconds(SecondsValue(d)) }
