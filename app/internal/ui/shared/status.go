package shared

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// SourceName is what translating hears: all the system's sound, or an app's name.
func SourceName(k *core.Core) string {
	p := k.Preferences()
	if p.Source == "" {
		return locales.SourceAll()
	}
	return p.SourceName
}

// SourceMenu chooses what to hear: all the system's sound, or one app's, those playing sound
// first. A native menu, as macOS's pop-up buttons: it is not cut off by the window, and scrolls
// when the apps are many. The app chosen stays a choice while it is not running. A view keeps
// one per menu it shows.
type SourceMenu struct {
	apps []core.App // the apps the menu shows, as it opened
	at   time.Time  // when the menu was last built
}

// Build fills a menu with what to hear, for a button of any look: a Picker's, a MenuRow's. Call
// k.ListApps while it shows.
func (m *SourceMenu) Build(k *core.Core) func(menu *ui.Menu) {
	return func(menu *ui.Menu) {
		p := k.Preferences()
		// The menu is built as it opens and again for the choice, which finds its item by its
		// place: it keeps the apps it opened with, though they are listed anew meanwhile.
		if time.Since(m.at) > 15*time.Second {
			m.apps = slices.Clone(k.Apps)
		}
		m.at = time.Now()
		if menu.Item(locales.SourceAll()).Checked(p.Source == "").Chosen() {
			k.SetSource("", "")
		}
		running := false
		section := func(title string, playing bool) {
			first := true
			for _, a := range m.apps {
				if a.Playing != playing {
					continue
				}
				if first {
					menu.Separator()
					menu.Item(title).Disabled(true)
					first = false
				}
				running = running || a.ID == p.Source
				if menu.Item(a.Name).Checked(a.ID == p.Source).Chosen() {
					k.SetSource(a.ID, a.Name)
				}
			}
		}
		section(locales.SourcePlaying(), true)
		section(locales.SourceOtherApps(), false)
		if !running && p.Source != "" {
			menu.Separator()
			menu.Item(locales.SourceNotRunning(p.SourceName)).Checked(true)
		}
	}
}

// Langs is a pair of languages as menus show them: 日语 → 中文.
func Langs(src, tgt string) string { return LangName(src) + " → " + LangName(tgt) }

// LangsOf is the languages of the subtitles as p has them: the spoken one alone when they show
// the original only.
func LangsOf(p prefs.Prefs) string {
	if !p.Translates() {
		return LangName(p.Src)
	}
	return Langs(p.Src, p.Tgt)
}

// The words of starting and stopping follow what runs: translating, or, with the original only,
// subtitles alone.

// Title is what the app is doing, as it runs or not.
func Title(p prefs.Prefs, running bool) string {
	switch {
	case running && p.Translates():
		return locales.HomeTranslating()
	case running:
		return locales.HomeSubtitling()
	case p.Translates():
		return locales.HomeIdle()
	}
	return locales.HomeSubtitlesOff()
}

// StartText is the action that starts.
func StartText(p prefs.Prefs) string {
	if p.Translates() {
		return locales.StartTranslating()
	}
	return locales.StartSubtitles()
}

// StopText is the action that stops.
func StopText(p prefs.Prefs) string {
	if p.Translates() {
		return locales.StopTranslating()
	}
	return locales.StopSubtitles()
}

// PauseText is the action that pauses, from the subtitles.
func PauseText(p prefs.Prefs) string {
	if p.Translates() {
		return locales.Pause()
	}
	return locales.PauseSubtitles()
}

// CurrentSceneName is the name of the scene in use.
func CurrentSceneName(k *core.Core) string {
	if sc, ok := k.SceneByID(k.Preferences().Scene); ok {
		return SceneName(*sc)
	}
	return locales.SceneGeneral()
}

// SceneMenu switches the scene: the scenes, the one in use checked, each with the languages it
// keeps (none until it is first used); with languages, the scene's languages as submenus and
// swapping them; a new scene, made and switched to; and manage, which shows the scene in use on
// the scenes' page, where a new one is named.
func SceneMenu(k *core.Core, languages bool, manage func()) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		p := k.Preferences()
		for _, sc := range k.SceneList() {
			label := SceneName(sc)
			if sc.ID == p.Scene {
				label += " · " + LangsOf(p)
			} else if sc.Src != "" && sc.Tgt != "" {
				label += " · " + Langs(sc.Src, sc.Tgt)
			}
			if m.Item(label).Checked(sc.ID == p.Scene).Chosen() && sc.ID != p.Scene {
				k.SetScene(sc.ID)
			}
		}
		if languages {
			m.Separator()
			m.Submenu(locales.SpokenLanguage(), kit.Choices(p.Src, LangChoices(k.SourceLangs()), LangName, k.SetSrc))
			m.Submenu(locales.TranslateInto(), kit.Choices(p.Tgt, LangChoices(k.TargetLangs()), LangName, k.SetTgt)).Disabled(!p.Translates())
			if m.Item(locales.SwapLanguages()).Disabled(!k.CanSwap() || !p.Translates()).Chosen() {
				k.Swap()
			}
		}
		m.Separator()
		if m.Item(locales.NewScene() + "…").Chosen() {
			k.SetScene(k.AddScene(""))
			manage()
		}
		if m.Item(locales.ManageScenes() + "…").Chosen() {
			manage()
		}
	}
}
