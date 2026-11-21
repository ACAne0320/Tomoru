package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

type Bin struct {
	Lo, Hi, Share float64
	Tip           string
}

type HistMark struct {
	At     float64
	Label  string
	Strong bool
}

func Histogram(c *ui.Context, label string, bins []Bin, marks []HistMark, format func(float64) string, chosen *int) ui.Element {
	p := Colors(c)
	hi, top := 1.0, 0.0
	if len(bins) > 0 {
		hi = bins[len(bins)-1].Hi
	}
	density := func(b Bin) float64 {
		if b.Hi <= b.Lo {
			return 0
		}
		return b.Share / (b.Hi - b.Lo)
	}
	for _, b := range bins {
		top = max(top, density(b))
	}
	if top <= 0 {
		top = 1
	}
	x := chart.NewLinear(0, hi)
	y := chart.NewLinear(0, top*1.15)
	ticks := x.Ticks(5)
	unitLast(ticks, format)
	xs := make([]float32, len(bins))
	for i, b := range bins {
		xs[i] = x.At((b.Lo + b.Hi) / 2)
	}
	color := func(b Bin) ui.Color {
		switch {
		case len(marks) > 0 && b.Hi <= marks[0].At+1e-9:
			return p.Lantern
		case len(marks) > 1 && b.Lo >= marks[1].At-1e-9:
			return p.LineStrong
		}
		return p.Amber[3]
	}
	mark := func(m HistMark) (ui.Color, float32) {
		if m.Strong {
			return p.Ink, 2
		}
		return p.InkMuted, 1
	}
	axis := func(_ chart.Side, t chart.Tick) {
		ui.Text(c, t.Label).NoWrap().FontSize(TextMicro).FontFeatures(Tabular).TextColor(p.InkMuted)
	}
	return ui.Column(c).Gap(Space(3)).Children(func() {
		ui.Row(c).Gap(Space(5)).Wrap().Children(func() {
			for i, m := range marks {
				ink, width := mark(m)
				ui.Row(c.Key(i)).Gap(Space(2)).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(width).Height(14).Background(ink)
					t := ui.Text(c, m.Label).FontSize(TextControl).FontFeatures(Tabular).TextColor(ink).SingleLine()
					if m.Strong {
						t.FontWeight(WeightBold)
					}
				})
			}
		})
		chart.Plot(c, chart.Axes{Bottom: ticks, Label: axis}, func(area ui.Element) {
			area.Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Line(r.X, r.Y+r.H, r.X+r.W, r.Y+r.H, 1, p.Line)
			})
			chart.Select(c, area, chosen, xs, func(i int) string { return bins[i].Tip })
			for i, b := range bins {
				col := color(b)
				if *chosen != NoPick && *chosen != i {
					col = p.Dim(col)
				}
				chart.Rect(c.Key(i), chart.Span{Lo: x.At(b.Lo), Hi: x.At(b.Hi)}, y.Span(0, density(b))).Margin(0, 1).
					Radius(RadiusMark).Background(col).Transition(ui.ElementTransition{Duration: 300 * time.Millisecond}).Label(b.Tip)
			}
			for i, m := range marks {
				ink, width := mark(m)
				chart.RuleX(c.Key([2]int{-1, i}), x.At(m.At), width).Background(ink)
			}
			if i := *chosen; i >= 0 && i < len(bins) {
				chart.Anchor(c, xs[i], y.At(density(bins[i])), tipAlign(xs[i]), ui.End, func() {
					ui.Text(c, bins[i].Tip).NoWrap().FontSize(TextCaption).FontWeight(WeightBold).FontFeatures(Tabular).
						TextColor(p.Paper).Background(p.Ink).Padding(Space(1), Space(2)).Radius(RadiusMark).Margin(0, 0, Space(2), 0)
				})
			}
		}).Label(label).Height(chartHeight).GapY(Space(2))
	})
}
