package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

type Bar struct {
	Label string
	Value float64
	Tip   string
	Parts []BarPart
}

type BarPart struct {
	Value float64
	Color ui.Color
}

const chartHeight = 180

const maxAxisLabels = 8

func BarChart(c *ui.Context, label string, bars []Bar, format func(float64) string, chosen *int) ui.Element {
	p := Colors(c)
	labels := make([]string, len(bars))
	values := make([]float64, 0, len(bars))
	for i, b := range bars {
		labels[i] = b.Label
		values = append(values, b.Value)
	}
	x := chart.NewBand(labels...).Padding(.3, .1)
	_, hi := chart.Extent(values...)
	if hi <= 0 {
		hi = 1
	}
	y := chart.NewLinear(0, hi).Nice(4)
	ticks := y.Ticks(4)
	unitLast(ticks, format)
	var under []chart.Tick
	every := (len(bars) + maxAxisLabels - 1) / maxAxisLabels
	for i, t := range x.Ticks() {
		if i%max(every, 1) == 0 {
			under = append(under, t)
		}
	}
	xs := make([]float32, len(bars))
	for i := range bars {
		xs[i] = x.At(i)
	}
	axis := func(_ chart.Side, t chart.Tick) {
		ui.Text(c, t.Label).NoWrap().FontSize(TextMicro).FontFeatures(Tabular).TextColor(p.InkMuted)
	}
	return chart.Plot(c, chart.Axes{Left: ticks, Bottom: under, Label: axis}, func(area ui.Element) {
		area.Draw(func(pt *ui.Painter, r ui.Rect) {
			for _, t := range ticks {
				pt.Line(r.X, chart.Y(r, t.At), r.X+r.W, chart.Y(r, t.At), 1, p.LineSoft)
			}
		})
		chart.Select(c, area, chosen, xs, func(i int) string { return bars[i].Tip })
		for i, b := range bars {
			dim := func(col ui.Color) ui.Color {
				if *chosen >= 0 && *chosen != i {
					return p.Dim(col)
				}
				return col
			}
			parts := b.Parts
			if len(parts) == 0 {
				parts = []BarPart{{b.Value, p.Lantern}}
			}
			lo := 0.0
			for j, part := range parts {
				if part.Value <= 0 {
					continue
				}
				r := chart.Rect(c.Key([2]int{i, j}), x.Band(i), y.Span(lo, lo+part.Value)).Radius(RadiusMark).Background(dim(part.Color)).
					Transition(ui.ElementTransition{Duration: 300 * time.Millisecond}).Label(b.Tip)
				if lo > 0 {
					r.BorderWidth(1, 0, 0).BorderColor(p.Card)
				}
				lo += part.Value
			}
		}
		if i := *chosen; i >= 0 && i < len(bars) {
			at, h, margin := x.Band(i).Hi, ui.Start, [4]float32{0, 0, 0, Space(2)}
			if x.At(i) > .5 {
				at, h, margin = x.Band(i).Lo, ui.End, [4]float32{0, Space(2), 0, 0}
			}
			chart.Anchor(c, at, 1, h, ui.Start, func() {
				ui.Text(c, bars[i].Tip).NoWrap().FontSize(TextCaption).FontWeight(WeightBold).FontFeatures(Tabular).
					TextColor(p.Paper).Background(p.Ink).Padding(Space(1), Space(2)).Radius(RadiusMark).Margin(margin[:]...)
			})
		}
	}).Label(label).Height(chartHeight).GapX(Space(2)).GapY(Space(2))
}
