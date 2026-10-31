package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Chip is a choice among a few, as a pill: a scene, a filter. Chosen, it is in ink. A count above 0
// shows in a badge after its label: the names a scene has.
func Chip(c *ui.Context, label string, on bool, count int) ui.Element {
	p := Colors(c)
	h := Small.height() + 2
	b := ui.ButtonBase(c).Height(h).Padding(0, Space(3.5)).Gap(Space(2)).Radius(Pill(h)).FontSize(TextSmall).Selected(on).Shrink(0)
	if count > 0 {
		b = b.Padding(0, Space(2), 0, Space(3.5))
	}
	if on {
		b = b.Background(p.Ink).TextColor(p.Paper).FontWeight(WeightBold).Cursor(ui.CursorPointer)
	} else {
		b = press(c, b, Plain)
	}
	return b.Children(func() {
		ui.Text(c, label).SingleLine()
		if count > 0 {
			CountBadge(c, count)
		}
	})
}

// CountBadge is a number in the lantern's amber: how many there are of something.
func CountBadge(c *ui.Context, n int) ui.Element {
	p := Colors(c)
	const h = 20
	return ui.Box(c).Height(h).MinWidth(h).Padding(0, Space(1.5)).Radius(Pill(h)).Background(p.Lantern).Center().Children(func() {
		ui.Text(c, strconv.Itoa(n)).FontSize(TextMicro).FontWeight(WeightHeavy).FontFeatures(Tabular).TextColor(p.LanternInk).SingleLine()
	})
}
