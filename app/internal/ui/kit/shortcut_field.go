package kit

import (
	"github.com/egoist/mygo/ui"
	"tomoru/locales"
)

type ShortcutRecorder struct {
	recording bool
	wantFocus bool
	err       string
}

func ShortcutField(c *ui.Context, acc, other, label string, register func(string) error, changed func(string), r *ShortcutRecorder) {
	t := c.Theme()
	ui.Column(c).Grow(1).Gap(Space(1)).Children(func() {
		border := t.Border
		if r.recording {
			border = t.Accent
		}
		b := ui.ButtonBase(c).Label(label).Tooltip(locales.ShortcutChange()).Row().AlignItems(ui.Center).Justify(ui.Center).MinHeight(36).Padding(0, Space(3)).
			Radius(RadiusField).Border(1, border).Background(t.Surface).AlignSelf(ui.Stretch).Children(func() {
			if r.recording {
				ui.Text(c, locales.ShortcutRecording()).FontSize(TextCaption).TextColor(Colors(c).InkMuted).Padding(0, 4)
			} else {
				KeyCaps(c, acc)
			}
		}).HandleInput(func(ev ui.InputEvent) bool {
			if !r.recording || ev.Kind != ui.InputKeyDown {
				return false
			}
			key := ev.PhysicalKey
			if key == ui.KeyUnknown {
				key = ev.Key
			}
			switch {
			case key == ui.KeyEscape && ev.Mods == 0:
				r.recording, r.err = false, ""
			case key == ui.KeyTab && ev.Mods&^ui.Shift == 0:
				return false
			case !ShortcutKey(key):
			default:
				keys, ok := Accelerator(ev.Mods, key)
				if !ok {
					r.err = locales.ShortcutNeedsModifier()
					if !macKeys {
						r.err = locales.ShortcutNeedsModifierWin()
					}
					return true
				}
				r.recording, r.err = false, ""
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
			r.recording, r.wantFocus, r.err = !r.recording, !r.recording, ""
		case r.recording && r.wantFocus:
			r.wantFocus = false
		case r.recording && !b.Focused():
			r.recording = false
		}
		if r.recording && r.wantFocus {
			b.Focus()
		}
		if r.err != "" {
			Warning(c, r.err)
		}
	})
}
