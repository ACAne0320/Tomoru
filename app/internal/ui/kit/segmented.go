package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Segmented chooses one of labels, *selected its index: a track of the paper's fill, the chosen
// segment raised on it, every segment as wide as the widest. Name it with Label; Changed reports a
// new choice.
func Segmented(c *ui.Context, selected *int, labels ...string) ui.Element {
	p := Colors(c)
	parts := ui.SegmentedBase(c, selected, len(labels))
	const h, in, gap = 28, 4, 2
	w := float32(0)
	for _, label := range labels {
		lw, _ := c.MeasureText(0, ui.Span{Text: label, Size: TextControl, Weight: WeightMedium})
		w = max(w, lw)
	}
	w = float32(math.Ceil(float64(w))) + 2*Space(3)
	return parts.Track.Padding(in).Gap(gap).Radius(RadiusField).Background(p.Fill).Children(func() {
		for i, label := range labels {
			s := parts.Segment(i).Height(h).Width(w).Padding(0, Space(3)).Radius(RadiusInner).FontSize(TextControl).Cursor(ui.CursorPointer)
			if i == *selected {
				s = p.SegShadow.Cast(s.Background(s.AnimateColor("face", p.SegOn, Fade)).FontWeight(WeightMedium).TextColor(p.Ink))
			} else {
				col := p.SegOn.Alpha(0)
				if s.Hovered() {
					col = p.Fill.Mix(p.SegOn, .5)
				}
				s = s.Background(s.AnimateColor("face", col, Fade)).TextColor(p.Ink)
			}
			s.Children(func() { ui.Text(c, label).SingleLine() })
		}
	})
}
