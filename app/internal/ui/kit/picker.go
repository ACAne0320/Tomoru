package kit

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// Picker is a pill in a sentence showing value, which opens a native menu that build fills:
// "Listen to [Safari]". Name it with Label.
func Picker(c *ui.Context, value string, build func(m *ui.Menu)) ui.Element {
	p := Colors(c)
	const h = 36
	b := ui.ButtonBase(c).Height(h).Padding(0, Space(2.5), 0, Space(3.5)).Gap(Space(1.5)).Radius(Pill(h)).
		FontSize(TextBody + 1).FontWeight(WeightBold).Shrink(0).Cursor(ui.CursorPointer)
	col := p.Fill
	if b.Hovered() || b.Pressed() {
		col = p.LineStrong
	}
	return b.Background(b.AnimateColor("face", col, hover)).TextColor(p.Ink).Menu(build).Children(func() {
		ui.Text(c, value).SingleLine()
		ui.Icon(c, assets.Chevron).FontSize(TextSmall).TextColor(p.InkWarm)
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
