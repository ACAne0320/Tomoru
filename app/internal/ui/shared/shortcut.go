package shared

import (
	"github.com/egoist/mygo/ui"
	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
)

func held(k *core.Core, nav Nav, which core.Shortcut) (acc string, other string, register func(string) error) {
	p := k.Preferences()
	if which == core.PictureShortcut {
		return p.ImageShortcut, p.Shortcut, nav.SetImageShortcut
	}
	return p.Shortcut, p.ImageShortcut, nav.SetShortcut
}

type ShortcutRecorder = kit.ShortcutRecorder

func ShortcutField(c *ui.Context, k *core.Core, nav Nav, which core.Shortcut, label string, r *ShortcutRecorder) {
	acc, other, register := held(k, nav, which)
	kit.ShortcutField(c, acc, other, label, register, func(keys string) {
		k.UpdatePrefs(func(p *prefs.Prefs) {
			if which == core.PictureShortcut {
				p.ImageShortcut = keys
			} else {
				p.Shortcut = keys
			}
		})
	}, r)
}
