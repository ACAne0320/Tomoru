package kit

import "github.com/egoist/mygo/ui"

// Segmented chooses one of labels, *selected its index: a track of the paper's fill, the chosen
// segment raised on it. Name it with Label; Changed reports a new choice.
func Segmented(c *ui.Context, selected *int, labels ...string) ui.Element {
	p := Colors(c)
	parts := ui.SegmentedBase(c, selected, len(labels))
	const h = 30
	return parts.Track.Padding(3).Gap(2).Radius(RadiusField).Background(p.Fill).Children(func() {
		for i, label := range labels {
			s := parts.Segment(i).Height(h).Padding(0, Space(3.5)).Radius(RadiusInner + 1).FontSize(TextSmall).Cursor(ui.CursorPointer)
			if i == *selected {
				s = s.Background(s.AnimateColor("face", p.Card, Fade)).FontWeight(WeightBold).TextColor(p.Ink).
					Shadow(0, 1, 3, 0, ui.RGBA(80, 60, 20, .18))
			} else {
				col := ui.Transparent
				if s.Hovered() {
					col = p.LineStrong.Alpha(.6)
				}
				s = s.Background(s.AnimateColor("face", col, Fade)).TextColor(p.Ink)
			}
			s.Children(func() { ui.Text(c, label).SingleLine() })
		}
	})
}
