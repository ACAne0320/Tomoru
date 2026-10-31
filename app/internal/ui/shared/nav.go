// Package shared contains application-specific UI presentation and navigation contracts.
// Reusable controls and visual tokens belong to kit.
package shared

import "github.com/egoist/mygo"

// Nav goes from one surface of the UI to another. The shell implements it: a surface knows no
// other.
type Nav interface {
	// OpenPanel shows the menu bar panel: under the tray icon with a nil anchor, else above
	// anchor, a box in the subtitle overlay.
	OpenPanel(anchor *mygo.Rectangle)
	// FitPanel makes the menu bar panel h tall, its content's height.
	FitPanel(h int)
	ShowSettings()
	// ShowSettingsPage shows the settings window at page, as "scenes".
	ShowSettingsPage(page string)
	ShowGuide()
	// FinishGuide closes the first-run guide, starts translating, and shows where the subtitles go.
	FinishGuide()
	// TranslateImage translates a picture, as its shortcut does: a part of the screen the user
	// selects, out of the panel's way, or the picture on the clipboard when the app may not capture
	// the screen.
	TranslateImage()
	// StartAdjust shows the subtitle area's frame for a while, to move it.
	StartAdjust()
	// SetLocked locks the subtitles, which then let clicks through and cannot be moved, or
	// unlocks them.
	SetLocked(locked bool)
	// SetHideFromCapture keeps the subtitles out of screenshots and recordings, or not.
	SetHideFromCapture(hide bool)
	// SetShortcut makes acc the global shortcut that starts and stops translating, in place of the
	// one before, which stays if acc cannot be registered.
	SetShortcut(acc string) error
	// SetImageShortcut makes acc the global shortcut that translates the picture on the clipboard,
	// as SetShortcut does the other.
	SetImageShortcut(acc string) error
	// Focus brings the app to the front, over the app the system gave it to.
	Focus()
	// SetAppearance shows the app light, dark, or as the system is: prefs.Prefs.Appearance's.
	SetAppearance(appearance string)
}
