package shell

import (
	"runtime"
	"slices"

	"github.com/egoist/mygo"

	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

var menuOnly = runtime.GOOS == "linux"

func (s *Shell) trayMenu() *mygo.Menu {
	k := s.core
	toggle := &mygo.MenuItem{Label: shared.StartText(k.Preferences()), Accelerator: k.Preferences().Shortcut,
		Click: func(*mygo.MenuItem, *mygo.Window) { k.Toggle() }}
	if k.Running {
		toggle.Label = shared.PauseText(k.Preferences())
	}
	var items []*mygo.MenuItem
	if menuOnly {
		items = append(items, &mygo.MenuItem{Label: locales.OpenPanel(), Click: func(*mygo.MenuItem, *mygo.Window) { s.OpenPanel() }})
	}
	return mygo.NewMenu(append(items,
		toggle,
		&mygo.MenuItem{Label: locales.TranslateScreen(), Accelerator: k.Preferences().ImageShortcut,
			Click: func(*mygo.MenuItem, *mygo.Window) { s.TranslateImage() }},
		mygo.Separator(),
		&mygo.MenuItem{Label: locales.OpenTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettings() }},
		&mygo.MenuItem{Label: locales.AboutTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettingsPage(shared.PageAbout) }},
		mygo.Separator(),
		&mygo.MenuItem{Label: locales.QuitTomoru(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	))
}

func (s *Shell) openTrayMenu() {
	s.closePanel()
	s.tray.SetMenu(s.trayMenu())
	s.tray.PopUpMenu()
	s.tray.SetMenu(nil)
}

func (s *Shell) setTrayMenu() {
	m := s.trayMenu()
	var items []trayEntry
	for _, it := range m.Items() {
		items = append(items, trayEntry{it.Label, it.Accelerator})
	}
	if !slices.Equal(items, s.trayItems) {
		s.trayItems = items
		s.tray.SetMenu(m)
	}
}

type trayEntry struct{ label, accelerator string }
