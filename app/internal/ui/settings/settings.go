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

const Width, Height = 1040, 720

const MinWidth, MinHeight = 860, 600

type Settings struct {
	core         *core.Core
	nav          shared.Nav
	Tab          string
	source       shared.SourceMenu
	edit         endpointEdit
	models       map[string]models.State
	modelsMoving bool
	keys         shared.ShortcutRecorder
	imageKeys    shared.ShortcutRecorder
	customColors bool
	navWidth     float32
	enginesEdit
	sceneEdit
	historyEdit
	learn     learnEdit
	confirm   confirmEdit
	feedback  feedbackEdit
	fonts     fontEdit
	deleting  string
	statsEdit statsEdit
}

func New(k *core.Core, nav shared.Nav) *Settings {
	return &Settings{core: k, nav: nav, Tab: "home", models: map[string]models.State{}, enginesEdit: enginesEdit{shown: map[core.Kind]string{}},
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

func (s *Settings) Open(page string) {
	if page == "scenes" {
		s.sceneEdit.id = false
	}
	s.Tab = page
}
