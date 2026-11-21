package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type HeatDay struct {
	Value float64
	Tip   string
}

const heatLevels = len(Palette{}.Amber) - 1

const (
	heatSize = 12
	heatGap  = 3
)

func HeatWeeks(width float32) int {
	return max(1, int((width-Space(8)-heatGap)/(heatSize+heatGap)))
}

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
			return p.Amber[0]
		}
		return p.Amber[min(heatLevels, max(1, int(math.Ceil(float64(heatLevels)*v/top))))]
	}
	cell := func(e ui.Element) ui.Element { return e.Grow(1).Basis(0).MinWidth(0) }
	side := func(e ui.Element) ui.Element { return e.Width(Space(8)).Shrink(0) }
	return ui.Column(c).Gap(Space(1)).Role(ui.RoleGroup).Label(label).Children(func() {
		ui.Row(c).Gap(heatGap).AlignItems(ui.Start).Children(func() {
			side(ui.Box(c))
			last := ""
			for w, name := range weeks {
				n := float32(len(weeks) - w)
				if lw, _ := c.MeasureText(0, ui.Span{Text: name, Size: TextMicro}); name != "" && lw > n*heatSize+(n-1)*heatGap {
					last, name = name, ""
				}
				cell(ui.Box(c.Key(w))).Children(func() {
					switch {
					case w == len(weeks)-1 && last != "":
						ui.Text(c, last).NoWrap().FontSize(TextMicro).TextColor(p.InkMuted).Absolute().Top(0).Right(0)
					case name != "":
						ui.Text(c, name).NoWrap().FontSize(TextMicro).TextColor(p.InkMuted)
					}
				})
			}
		})
		ui.Column(c).Gap(heatGap).FocusGroup(ui.Horizontal | ui.Vertical).Children(func() {
			for d := range 7 {
				ui.Row(c.Key(d)).Gap(heatGap).AlignItems(ui.Center).Children(func() {
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
							sq.InsetShadow(0, 0, 0, 1.5, p.Ink)
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
				ui.Box(c.Key(n)).Size(10, 10).Radius(3).Background(p.Amber[n])
			}
			ui.Text(c, more).FontSize(TextMicro).TextColor(p.InkMuted).Margin(0, 0, 0, Space(1))
		})
	})
}
