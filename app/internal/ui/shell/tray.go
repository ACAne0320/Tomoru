package shell

import (
	"github.com/egoist/mygo"

	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// trayMenu is the menu bar icon's menu, the system's, which its right button opens, as the panel
// its left: what one does at once, without opening the panel, and quitting. It is made as it opens,
// for translating's state and the shortcuts, which it shows: the menu cannot press them, the
// system's shortcuts having them first.
func (s *Shell) trayMenu() *mygo.Menu {
	k := s.core
	toggle := &mygo.MenuItem{Label: shared.StartText(k.Preferences()), Accelerator: k.Preferences().Shortcut,
		Click: func(*mygo.MenuItem, *mygo.Window) { k.Toggle() }}
	if k.Running {
		toggle.Label = shared.StopText(k.Preferences())
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		toggle,
		{Label: locales.TranslateScreen(), Accelerator: k.Preferences().ImageShortcut,
			Click: func(*mygo.MenuItem, *mygo.Window) { s.TranslateImage() }},
		mygo.Separator(),
		{Label: locales.OpenTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettings() }},
		{Label: locales.AboutTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettingsPage("about") }},
		mygo.Separator(),
		{Label: locales.QuitTomoru(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// openTrayMenu opens the menu bar icon's menu, out of the panel's way. It is the icon's only while
// open: the left button opens the panel.
func (s *Shell) openTrayMenu() {
	s.closePanel()
	s.tray.SetMenu(s.trayMenu())
	s.tray.PopUpMenu()
	s.tray.SetMenu(nil)
}
