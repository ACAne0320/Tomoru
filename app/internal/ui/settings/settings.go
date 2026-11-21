package settings

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
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
	learnAPI         endpointEdit
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
	later                 []func()
}

func New(k *core.Core, nav shared.Nav) *Settings {
	k.TakeFinished()
	return &Settings{core: k, nav: nav, Tab: shared.PageHome, enginesEdit: enginesEdit{shown: map[core.Kind]string{}}, sceneEdit: sceneEdit{scene: k.Preferences().Scene},
		statsEdit: statsEdit{kind: kindTranslation, bar: kit.NoPick, latBar: kit.NoPick}}
}

func (s *Settings) View(c *ui.Context) {
	k := s.core
	defer s.runLater()
	s.toastFinished(c)
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
		s.sceneEdit.scene = s.core.Preferences().Scene
	}
	s.Tab = page
}

func (s *Settings) toastFinished(c *ui.Context) {
	k := s.core
	for _, f := range k.TakeFinished() {
		switch f.What {
		case core.ModelReady:
			e, ok := k.Engine(f.Kind, f.ID)
			if !ok {
				continue
			}
			name, kind, id := shared.EngineName(e), f.Kind, f.ID
			switch {
			case k.InUse(kind) == id:
				c.Toast(locales.ModelReadyInUse(name))
			case k.Usable(e):
				c.ToastAction(locales.ModelDownloaded(name), locales.Use(), func() { k.Use(kind, id) })
			default:
				c.Toast(locales.ModelDownloaded(name))
			}
		case core.ModelsMoved:
			c.Toast(locales.ModelsMoved())
		case core.HistoryExported:
			if f.Err != nil {
				c.Toast(locales.ExportFailed(f.Err.Error()))
			} else {
				c.Toast(locales.Exported())
			}
		}
	}
}

func (s *Settings) afterFrame(fn func()) { s.later = append(s.later, fn) }

func (s *Settings) runLater() {
	for len(s.later) > 0 {
		fn := s.later[0]
		s.later = s.later[1:]
		fn()
	}
}
