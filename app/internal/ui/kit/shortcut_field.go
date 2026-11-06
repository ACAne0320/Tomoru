package kit

import (
	"github.com/egoist/mygo/ui"
	"tomoru/locales"
)

// ShortcutRecorder is the state of a ShortcutField.
type ShortcutRecorder struct {
	on    bool   // recording: the next keys pressed are the shortcut
	focus bool   // to give the field the focus, as recording starts
	err   string // why the keys pressed last are not the shortcut
}

// ShortcutField is the field of a global shortcut, which: its keys as keycaps, named label. A click
// records new ones, until keys that make a shortcut are pressed, or Escape, a click or the focus
// going elsewhere stops it. The shortcut works meanwhile, and pressing it does what it does: the
// field gets keys no app has. The other shortcut's keys are not taken.
func ShortcutField(c *ui.Context, acc, other, label string, register func(string) error, changed func(string), r *ShortcutRecorder) {
	t := c.Theme()
	// It fills the column it is in, as a Control's: a row's other controls end where it ends.
	ui.Column(c).Grow(1).Gap(Space(1)).Children(func() {
		border := t.Border
		if r.on {
			border = t.Accent
		}
		b := ui.ButtonBase(c).Label(label).Tooltip(locales.ShortcutChange()).Row().AlignItems(ui.Center).Justify(ui.Center).MinHeight(36).Padding(0, Space(3)).
			Radius(RadiusField).Border(1, border).Background(t.Surface).AlignSelf(ui.Stretch).Children(func() {
			if r.on {
				ui.Text(c, locales.ShortcutRecording()).FontSize(TextCaption).TextColor(Colors(c).InkMuted).Padding(0, 4)
			} else {
				KeyCaps(c, acc)
			}
		}).HandleInput(func(ev ui.InputEvent) bool {
			if !r.on || ev.Kind != ui.InputKeyDown {
				return false
			}
			key := ev.PhysicalKey // the key codes shortcuts register are by position
			if key == ui.KeyUnknown {
				key = ev.Key
			}
			switch {
			case key == ui.KeyEscape && ev.Mods == 0:
				r.on, r.err = false, ""
			case key == ui.KeyTab && ev.Mods&^ui.Shift == 0:
				return false // the focus goes on, which stops recording
			case !ShortcutKey(key):
				// A modifier alone, as the keys are held one by one.
			default:
				keys, ok := Accelerator(ev.Mods, key)
				if !ok {
					r.err = locales.ShortcutNeedsModifier()
					if !macKeys {
						r.err = locales.ShortcutNeedsModifierWin()
					}
					return true
				}
				r.on, r.err = false, ""
				switch {
				case keys == acc:
				case keys == other:
					r.err = locales.ShortcutDuplicate(ShortcutText(keys))
				case register(keys) != nil:
					r.err = locales.ShortcutTaken(ShortcutText(keys))
				default:
					changed(keys)
				}
			}
			return true
		})
		switch {
		case b.Clicked():
			r.on, r.focus, r.err = !r.on, !r.on, ""
		case r.on && r.focus:
			r.focus = false
		case r.on && !b.Focused():
			r.on = false
		}
		if r.on && r.focus {
			b.Focus()
		}
		if r.err != "" {
			ui.Text(c, r.err).FontSize(12).TextColor(ui.Hex("#c77700"))
		}
	})
}
