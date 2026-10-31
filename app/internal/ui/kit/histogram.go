package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// Bin is a bin of a Histogram: the stretch it counts, from Lo to Hi, its share of all of them, from
// 0 to 1, and what it says chosen.
type Bin struct {
	Lo, Hi, Share float64
	Tip           string
}

// HistMark is a line across a Histogram at a value, named over the plot: Strong in ink, else muted.
type HistMark struct {
	At     float64
	Label  string
	Strong bool
}

// Histogram shows how values spread from 0 to the last bin's Hi: a bar over each bin, as high as
// its share is dense, so that bins of any width compare. The bins to the first mark are of the
// lantern's amber, those to the second paler, and those past it of the lines' color; the marks
// cross the plot, named in a row over it. The pointer, or the arrows once it has the focus, choose
// a bin, *chosen its index, -1 for none, whose tip shows above it. format says the axis's last
// value, with its unit; name it with label.
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
	y := chart.NewLinear(0, top*1.15) // room over the highest for its tip
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
		return p.Lantern.Alpha(.55)
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
					t := ui.Text(c, m.Label).FontSize(TextSmall).FontFeatures(Tabular).TextColor(ink).SingleLine()
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
				if *chosen >= 0 && *chosen != i {
					col = col.Alpha(.45)
				}
				chart.Rect(c.Key(i), chart.Span{Lo: x.At(b.Lo), Hi: x.At(b.Hi)}, y.Span(0, density(b))).Margin(0, 1).
					Radius(RadiusInner / 3).Background(col).Transition(ui.ElementTransition{Duration: 300 * time.Millisecond}).Label(b.Tip)
			}
			for i, m := range marks {
				ink, width := mark(m)
				chart.RuleX(c.Key([2]int{-1, i}), x.At(m.At), width).Background(ink)
			}
			if i := *chosen; i >= 0 && i < len(bins) {
				chart.Anchor(c, xs[i], y.At(density(bins[i])), tipAlign(xs[i]), ui.End, func() {
					ui.Text(c, bins[i].Tip).NoWrap().FontSize(TextCaption).FontWeight(WeightBold).FontFeatures(Tabular).
						TextColor(p.Paper).Background(p.Ink).Padding(Space(1), Space(2)).Radius(RadiusInner/2).Margin(0, 0, Space(1.5), 0)
				})
			}
		}).Label(label).Height(chartHeight).GapY(Space(1.5))
	})
}
