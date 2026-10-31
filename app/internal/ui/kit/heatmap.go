package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// HeatDay is a day of a Heatmap: its value, NaN for a day still to come, and what it says.
type HeatDay struct {
	Value float64
	Tip   string
}

// heatLevels are the depths of the lantern's amber a day can take, past none.
const heatLevels = 4

// A square of a Heatmap is about heatSize, and grows to fill a row to heatMax, heatGap from the
// next: small, as a year of days reads at a glance, without the card taking the page.
const (
	heatSize = 12
	heatMax  = 14
	heatGap  = 3
)

// HeatWeeks are the weeks a Heatmap as wide as width shows at about its squares' size.
func HeatWeeks(width float32) int {
	return max(1, int((width-Space(8)-heatGap)/(heatSize+heatGap)))
}

// Heatmap shows a value a day as squares, a week a column from Monday, of deeper amber for more:
// weeks names the columns over them (a month's name where it starts, "" for none) and weekdays the
// rows beside them ("" for none); less and more name the ends of the legend under them. A square
// says its tip on hovering, outlined in ink; clicked, or with Enter once the arrows bring the focus
// to it, it calls pick with its day's index, if pick is not nil. Name it with label.
func Heatmap(c *ui.Context, label string, days []HeatDay, weeks []string, weekdays [7]string, less, more string, pick func(day int)) ui.Element {
	p := Colors(c)
	top := 0.0
	for _, d := range days {
		if !math.IsNaN(d.Value) {
			top = max(top, d.Value)
		}
	}
	level := func(v float64) ui.Color {
		switch {
		case math.IsNaN(v):
			return ui.Transparent
		case v <= 0 || top <= 0:
			return p.LineSoft
		}
		n := min(heatLevels, max(1, int(math.Ceil(heatLevels*v/top))))
		return p.Lantern.Alpha(float32(n) / heatLevels)
	}
	cell := func(e ui.Element) ui.Element { return e.Grow(1).Basis(0).MinWidth(0).MaxWidth(heatMax) }
	side := func(e ui.Element) ui.Element { return e.Width(Space(8)).Shrink(0) }
	return ui.Column(c).Gap(Space(1)).Role(ui.RoleGroup).Label(label).Children(func() {
		ui.Row(c).Gap(heatGap).Children(func() {
			side(ui.Box(c))
			for w, name := range weeks {
				cell(ui.Box(c.Key(w))).Children(func() {
					ui.Text(c, name).NoWrap().FontSize(TextMicro).TextColor(p.InkMuted)
				})
			}
		})
		// The squares, one stop of Tab, among which the arrows move.
		ui.Column(c).Gap(heatGap).FocusGroup(ui.Horizontal | ui.Vertical).Children(func() {
			for d := range 7 {
				ui.Row(c.Key(d)).Gap(heatGap).AlignItems(ui.Center).Children(func() {
					// No taller than the squares, so that the rows are as far apart as the columns
					// (Segoe UI's lines are taller than SF's).
					side(ui.Box(c)).Height(heatSize).Justify(ui.Center).Children(func() {
						ui.Text(c, weekdays[d]).NoWrap().FontSize(TextMicro).TextColor(p.InkMuted)
					})
					for w := range weeks {
						i := 7*w + d
						sq := cell(ui.Box(c.Key(w))).AspectRatio(1).Radius(3)
						if i >= len(days) {
							continue
						}
						sq.Background(level(days[i].Value))
						if math.IsNaN(days[i].Value) {
							continue
						}
						sq.Label(days[i].Tip).Tooltip(days[i].Tip)
						if pick == nil {
							continue
						}
						sq.Focusable().Cursor(ui.CursorPointer)
						if sq.Hovered() || sq.FocusVisible() {
							sq.InsetShadow(0, 0, 0, 1.5, p.Ink) // an outline inside it, which takes no room
						}
						if sq.Clicked() {
							pick(i)
						}
					}
				})
			}
		})
		ui.Row(c).Gap(Space(1)).AlignItems(ui.Center).Margin(Space(1), 0, 0).Children(func() {
			ui.Box(c).Grow(1)
			ui.Text(c, less).FontSize(TextMicro).TextColor(p.InkMuted).Margin(0, Space(1), 0, 0)
			for n := range heatLevels + 1 {
				col := p.LineSoft
				if n > 0 {
					col = p.Lantern.Alpha(float32(n) / heatLevels)
				}
				ui.Box(c.Key(n)).Size(10, 10).Radius(3).Background(col)
			}
			ui.Text(c, more).FontSize(TextMicro).TextColor(p.InkMuted).Margin(0, 0, 0, Space(1))
		})
	})
}
