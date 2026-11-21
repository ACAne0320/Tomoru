package shared

import "github.com/egoist/mygo"

type Nav interface {
	OpenPanel(anchor *mygo.Rectangle)
	FitPanel(h int)
	ShowSettings()
	ShowSettingsPage(page string)
	ShowGuide()
	FinishGuide()
	TranslateImage()
	StartAdjust()
	SetLocked(locked bool)
	SetHideFromCapture(hide bool)
	SetShortcut(acc string) error
	SetImageShortcut(acc string) error
	Focus()
	SetAppearance(appearance string)
}
