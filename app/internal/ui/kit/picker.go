package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// Picker shows value on the track a segmented control has, as tall, and opens a native menu that
// build fills: "Listen to [Safari]", a setting's font. Name it with Label. Made wider, as in a
// Control, its value keeps to the start and the chevron to the end.
func Picker(c *ui.Context, value string, build func(m *ui.Menu)) ui.Element {
	p := Colors(c)
	const h = 36
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

// Choices fills a menu with options, named by name, value checked, choose taking another. An
// option "" is a separator.
func Choices(value string, options []string, name func(string) string, choose func(string)) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		for _, o := range options {
			if o == "" {
				m.Separator()
				continue
			}
			if m.Item(name(o)).Checked(o == value).Chosen() && o != value {
				choose(o)
			}
		}
	}
}
