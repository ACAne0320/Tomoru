package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

type Whisker struct {
	Label   string
	Mid, Hi float64
	Empty   bool
	Tip     string
}

func WhiskerChart(c *ui.Context, label string, ws []Whisker, format func(float64) string, chosen *int) ui.Element {
	p := Colors(c)
	labels := make([]string, len(ws))
	var values []float64
	for i, w := range ws {
		labels[i] = w.Label
		if !w.Empty {
			values = append(values, w.Hi)
		}
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
	every := (len(ws) + maxAxisLabels - 1) / maxAxisLabels
	for i, t := range x.Ticks() {
		if i%max(every, 1) == 0 {
			under = append(under, t)
		}
	}
	xs := make([]float32, len(ws))
	for i := range ws {
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
		chart.Select(c, area, chosen, xs, func(i int) string { return ws[i].Tip })
		for i, w := range ws {
			if w.Empty {
				continue
			}
			dot, line := p.Lantern, p.Amber[3]
			if *chosen != NoPick && *chosen != i {
				dot, line = p.Dim(dot), p.Dim(line)
			}
			chart.Rect(c.Key([2]int{i, 0}), x.Band(i), y.Span(w.Mid, w.Hi)).Children(func() {
				ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Stretch).Children(func() { ui.Box(c).Width(2).Radius(1).Background(line) })
			})
			chart.Point(c.Key([2]int{i, 1}), xs[i], y.At(w.Mid), 10, 10).Radius(Pill(10)).Background(dot).Border(2, p.Card).Label(w.Tip)
		}
		if i := *chosen; i >= 0 && i < len(ws) {
			v := ws[i].Hi
			if ws[i].Empty {
				v = 0
			}
			chart.Anchor(c, xs[i], y.At(v), tipAlign(xs[i]), ui.End, func() {
				ui.Text(c, ws[i].Tip).NoWrap().FontSize(TextCaption).FontWeight(WeightBold).FontFeatures(Tabular).
					TextColor(p.Paper).Background(p.Ink).Padding(Space(1), Space(2)).Radius(RadiusMark).Margin(0, 0, Space(2), 0)
			})
		}
	}).Label(label).Height(chartHeight).GapX(Space(2)).GapY(Space(2))
}

func tipAlign(x float32) ui.Align {
	switch {
	case x < .2:
		return ui.Start
	case x > .8:
		return ui.End
	}
	return ui.Center
}
