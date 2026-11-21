package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

func Picker(c *ui.Context, value string, build func(m *ui.Menu)) ui.Element {
	p := Colors(c)
	const h = trackHeight
	b := ui.ButtonBase(c).Height(h).Padding(0, Space(3)).Gap(Space(2)).Radius(RadiusField).
		FontSize(TextControl).FontWeight(WeightMedium).Shrink(0).Cursor(ui.CursorPointer)
	col := p.Fill
	if b.Hovered() || b.Pressed() {
		col = p.Fill.Mix(p.LineStrong, .6)
	}
	return b.Background(b.AnimateColor("face", col, hover)).TextColor(p.Ink).Menu(build).Children(func() {
		ui.Text(c, value).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, assets.Chevron).FontSize(TextControl).TextColor(p.InkWarm)
	})
}

func Choices(value string, groups [][]string, name func(string) string, choose func(string)) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		for i, group := range groups {
			if i > 0 {
				m.Separator()
			}
			for _, o := range group {
				if m.Item(name(o)).Checked(o == value).Chosen() && o != value {
					choose(o)
				}
			}
		}
	}
}
