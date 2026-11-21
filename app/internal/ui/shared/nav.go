package shared

import (
	"github.com/egoist/mygo"

	"tomoru/internal/prefs"
)

type Nav interface {
	OpenPanel()
	OpenPanelAbove(r mygo.Rectangle)
	ShowSettings()
	ShowSettingsPage(page Page)
	ShowGuide()
	FinishGuide()
	TranslateImage()
	StartAdjust()
	SetLocked(locked bool)
	SetHideFromCapture(hide bool)
	SetShortcut(acc string) error
	SetImageShortcut(acc string) error
	Focus()
	SetAppearance(appearance prefs.Appearance)
}

type Page string

const (
	PageHome      Page = "home"
	PageSubtitles Page = "subtitles"
	PageEngines   Page = "engines"
	PageScenes    Page = "scenes"
	PageHistory   Page = "history"
	PageStats     Page = "stats"
	PageGeneral   Page = "general"
	PageAbout     Page = "about"
)
