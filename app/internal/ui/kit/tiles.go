package kit

import "github.com/egoist/mygo/ui"

// Tile is a choice of PickTiles: what it is, and its value with a unit. Strong sets apart the one
// that matters most, in a bigger value.
type Tile struct {
	Label, Value, Unit string
	Strong             bool
}

// TileGroup is tiles under a name: a stage of something, as recognition.
type TileGroup struct {
	Label string
	Tiles []Tile
}

// PickTiles chooses one of the tiles of groups, *selected its index counted through all of them: a
// row of the groups, each its name over its tiles, the one chosen on the band. A tile's name takes
// two lines where one is too narrow, the values staying on one line along the tiles' foot. The
// arrows move the choice while it has the focus; name it with Label, and Changed reports a new
// choice.
func PickTiles(c *ui.Context, selected *int, groups []TileGroup) ui.Element {
	p := Colors(c)
	n := 0
	for _, g := range groups {
		n += len(g.Tiles)
	}
	parts := ui.SegmentedBase(c, selected, n)
	return parts.Track.Gap(Space(3)).AlignItems(ui.Stretch).Shrink(1).Children(func() {
		i := 0
		for _, g := range groups {
			ui.Column(c).Gap(Space(2)).Grow(float32(len(g.Tiles))).Basis(0).MinWidth(0).Children(func() {
				ui.Text(c, g.Label).FontSize(TextMicro).TextColor(p.InkMuted).Padding(0, Space(1)).SingleLine()
				ui.Row(c).Gap(Space(2)).Grow(1).AlignItems(ui.Stretch).Children(func() {
					for _, t := range g.Tiles {
						s := parts.Segment(i).Grow(1).Basis(0).MinWidth(0).Padding(Space(3)).Radius(RadiusField).
							Cursor(ui.CursorPointer).Label(t.Label + " " + t.Value + " " + t.Unit)
						face := p.Paper
						switch {
						case i == *selected:
							face = p.Band
						case s.Hovered():
							face = p.Paper.Mix(p.Band, .5)
						}
						size := float32(TextTitle)
						if t.Strong {
							size = TextStat
						}
						s.Background(s.AnimateColor("face", face, Fade)).Children(func() {
							ui.Column(c).Grow(1).AlignSelf(ui.Stretch).Gap(Space(1)).Justify(ui.SpaceBetween).Children(func() {
								ui.Text(c, t.Label).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(2)
								ui.Row(c).Gap(Space(1)).AlignItems(ui.End).Children(func() {
									ui.Text(c, t.Value).FontSize(size).FontWeight(WeightHeavy).FontFeatures(Tabular).TextColor(p.Ink).SingleLine()
									if t.Unit != "" {
										ui.Text(c, t.Unit).FontSize(TextMicro).FontWeight(WeightBold).TextColor(p.InkMuted).SingleLine()
									}
								})
							})
						})
						i++
					}
				})
			})
		}
	})
}
