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
	core             *core.Core
	nav              shared.Nav
	Tab              shared.Page
	source           shared.SourceMenu
	learnAPI         endpointEdit
	models           map[string]models.State
	modelsMoving     bool
	startStopKeys    shared.ShortcutRecorder
	imageKeys        shared.ShortcutRecorder
	customColorsOpen bool
	navWidth         float32
	enginesEdit
	sceneEdit
	historyEdit
	learn                 learnEdit
	confirm               confirmEdit
	feedback              feedbackEdit
	fonts                 fontEdit
	confirmingDeleteModel string
	statsEdit             statsEdit
}

func New(k *core.Core, nav shared.Nav) *Settings {
	return &Settings{core: k, nav: nav, Tab: shared.PageHome, models: map[string]models.State{}, enginesEdit: enginesEdit{shown: map[core.Kind]string{}},
		statsEdit: statsEdit{kind: kindTranslation, bar: kit.NoPick, latBar: kit.NoPick}}
}

func (s *Settings) View(c *ui.Context) {
	k := s.core
	top := []kit.NavEntry{
		{ID: string(shared.PageHome), Icon: assets.Home, Label: locales.TabHome()},
		{ID: string(shared.PageSubtitles), Icon: assets.Captions, Label: locales.TabSubtitles()},
		{ID: string(shared.PageEngines), Icon: assets.Languages, Label: locales.TabEngines()},
		{ID: string(shared.PageScenes), Icon: assets.Book, Label: locales.TabScenes()},
	}
	if k.HistoryOn() {
		top = append(top, kit.NavEntry{ID: string(shared.PageHistory), Icon: assets.History, Label: locales.TabHistory()})
	}
	top = append(top, kit.NavEntry{ID: string(shared.PageStats), Icon: assets.Chart, Label: locales.TabStats()})
	bottom := []kit.NavEntry{
		{ID: string(shared.PageGeneral), Icon: assets.Sliders, Label: locales.TabGeneral()},
		{ID: string(shared.PageAbout), Icon: assets.Info, Label: locales.TabAbout()},
	}
	s.navWidth = kit.NavWidth(c, top, bottom)
	kit.AppFrame(c, (*string)(&s.Tab), top, bottom, func() {
		switch s.Tab {
		case shared.PageHome:
			s.home(c)
		case shared.PageGeneral:
			s.general(c)
		case shared.PageScenes:
			s.scenes(c)
		case shared.PageHistory:
			s.history(c)
		case shared.PageStats:
			s.stats(c)
		case shared.PageSubtitles:
			s.subtitles(c)
		case shared.PageEngines:
			s.engines(c)
		case shared.PageAbout:
			s.about(c)
		}
		s.confirmDialog(c)
	})
}

func (s *Settings) Open(page shared.Page) {
	if page == shared.PageScenes {
		s.sceneEdit.id = false
	}
	s.Tab = page
}
