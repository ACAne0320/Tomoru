package shared

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

func SourceName(k *core.Core) string {
	p := k.Preferences()
	if p.HearsAll() {
		return locales.SourceAll()
	}
	return p.SourceName
}

func SourceMenu(k *core.Core) func(menu *ui.Menu) {
	return func(menu *ui.Menu) {
		p := k.Preferences()
		if menu.Item(locales.SourceAll()).ID("all").Checked(p.HearsAll()).Chosen() {
			k.SetSource("", "")
		}
		running := false
		section := func(title string, playing bool) {
			first := true
			for _, a := range k.Apps {
				if a.Playing != playing {
					continue
				}
				if first {
					menu.Separator()
					menu.Item(title).Disabled(true)
					first = false
				}
				running = running || a.ID == p.Source
				if menu.Item(a.Name).ID("app:" + a.ID).Checked(a.ID == p.Source).Chosen() {
					k.SetSource(a.ID, a.Name)
				}
			}
		}
		section(locales.SourcePlaying(), true)
		section(locales.SourceOtherApps(), false)
		if !running && !p.HearsAll() {
			menu.Separator()
			menu.Item(locales.SourceNotRunning(p.SourceName)).ID("gone").Checked(true)
		}
	}
}

func Langs(src, tgt string) string { return LangName(src) + " → " + LangName(tgt) }

func LangsOf(p prefs.Prefs) string {
	if !p.Translates() {
		return LangName(p.Src)
	}
	return Langs(p.Src, p.Tgt)
}

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

func StartText(p prefs.Prefs) string {
	if p.Translates() {
		return locales.StartTranslating()
	}
	return locales.StartSubtitles()
}

func PauseText(p prefs.Prefs) string {
	if p.Translates() {
		return locales.Pause()
	}
	return locales.PauseSubtitles()
}

func CurrentSceneName(k *core.Core) string {
	if sc, ok := k.SceneByID(k.Preferences().Scene); ok {
		return SceneName(*sc)
	}
	return locales.SceneGeneral()
}

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
