// Package settings is the settings window: a sidebar of pages, each in a file of its own.
package settings

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/models"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// Width and Height are the window's size at first; it remembers the size the user gives it.
const Width, Height = 1040, 720

// MinWidth and MinHeight are the least the user can make it.
const MinWidth, MinHeight = 860, 600

// Settings is the window's content. The shell shows it in a window.
type Settings struct {
	core      *core.Core
	nav       shared.Nav
	Tab       string // the page: "home", "general", "scenes", "history", "stats", "subtitles", "engines" or "about"
	source    shared.SourceMenu
	edit      endpointEdit
	cloud     shared.ASRCloudEdit
	edits     int                     // the endpoint dialogs opened, to tell their tests apart
	models    map[string]models.State // the local models' states, as last shown: one coming in says so
	sides     map[core.Kind]bool      // the side of the engines page looked at, the cloud's if true; else the chosen engine's
	keys      shared.ShortcutRecorder // start and stop's
	imageKeys shared.ShortcutRecorder // the picture's
	// customColors shows the subtitles' colors of one's own, chosen as a scheme's swatch is.
	customColors bool
	navWidth     float32 // the sidebar's, as the language shown has it
	sceneEdit
	historyEdit
	learn     learnEdit
	confirm   confirmEdit
	deleting  string // the local model asked about by its button before it is deleted
	statsEdit statsEdit
}

func New(k *core.Core, nav shared.Nav) *Settings {
	return &Settings{core: k, nav: nav, Tab: "home", models: map[string]models.State{}, sides: map[core.Kind]bool{},
		statsEdit: statsEdit{kind: 1, bar: -1, latBar: -1}}
}

func (s *Settings) View(c *ui.Context) {
	k := s.core
	top := []kit.NavEntry{
		{ID: "home", Icon: assets.Home, Label: locales.TabHome()},
		{ID: "subtitles", Icon: assets.Captions, Label: locales.TabSubtitles()},
		{ID: "engines", Icon: assets.Languages, Label: locales.TabEngines()},
		{ID: "scenes", Icon: assets.Book, Label: locales.TabScenes()},
	}
	if k.HistoryOn() {
		top = append(top, kit.NavEntry{ID: "history", Icon: assets.History, Label: locales.TabHistory()})
	}
	top = append(top, kit.NavEntry{ID: "stats", Icon: assets.Chart, Label: locales.TabStats()})
	bottom := []kit.NavEntry{
		{ID: "general", Icon: assets.Sliders, Label: locales.TabGeneral()},
		{ID: "about", Icon: assets.Info, Label: locales.TabAbout()},
	}
	s.navWidth = kit.NavWidth(c, top, bottom)
	kit.AppFrame(c, &s.Tab, top, bottom, func() {
		switch s.Tab {
		case "home":
			s.home(c)
		case "general":
			s.general(c)
		case "scenes":
			s.scenes(c)
		case "history":
			s.history(c)
		case "stats":
			s.stats(c)
		case "subtitles":
			s.subtitles(c)
		case "engines":
			s.engines(c)
		case "about":
			s.about(c)
		}
		s.confirmDialog(c)
	})
}

// Open shows page; the scenes' shows the scene in use.
func (s *Settings) Open(page string) {
	if page == "scenes" {
		s.sceneEdit.id = false
	}
	s.Tab = page
}
