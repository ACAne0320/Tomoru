package kit

import (
	"github.com/egoist/mygo"
)

// ThemeSource is the appearance prefs.Prefs.Appearance names: light, dark, or the system's.
func ThemeSource(appearance string) mygo.ThemeSource {
	switch appearance {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	}
	return mygo.ThemeSystem
}
