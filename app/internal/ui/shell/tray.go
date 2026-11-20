package shell

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"

	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// menuOnly tells whether the tray icon only has its menu: Linux's (AppIndicator) opens it on a
// click, and tells neither clicks nor where the icon is, so the menu shows the panel, in the middle
// of the screen.
var menuOnly = runtime.GOOS == "linux"

// trayMenu is the menu bar icon's menu, the system's, which its right button opens, as the panel
// its left: what one does at once, without opening the panel, and quitting. It is made as it opens,
// for translating's state and the shortcuts, which it shows: the menu cannot press them, the
// system's shortcuts having them first. Where the icon only has its menu, it shows the panel too.
func (s *Shell) trayMenu() *mygo.Menu {
	k := s.core
	toggle := &mygo.MenuItem{Label: shared.StartText(k.Preferences()), Accelerator: k.Preferences().Shortcut,
		Click: func(*mygo.MenuItem, *mygo.Window) { k.Toggle() }}
	if k.Running {
		toggle.Label = shared.PauseText(k.Preferences())
	}
	var items []*mygo.MenuItem
	if menuOnly {
		items = append(items, &mygo.MenuItem{Label: locales.OpenPanel(), Click: func(*mygo.MenuItem, *mygo.Window) { s.OpenPanel(nil) }})
	}
	return mygo.NewMenu(append(items,
		toggle,
		&mygo.MenuItem{Label: locales.TranslateScreen(), Accelerator: k.Preferences().ImageShortcut,
			Click: func(*mygo.MenuItem, *mygo.Window) { s.TranslateImage() }},
		mygo.Separator(),
		&mygo.MenuItem{Label: locales.OpenTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettings() }},
		&mygo.MenuItem{Label: locales.AboutTomoru(), Click: func(*mygo.MenuItem, *mygo.Window) { s.ShowSettingsPage("about") }},
		mygo.Separator(),
		&mygo.MenuItem{Label: locales.QuitTomoru(), Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	))
}

// openTrayMenu opens the menu bar icon's menu, out of the panel's way. It is the icon's only while
// open: the left button opens the panel.
func (s *Shell) openTrayMenu() {
	s.closePanel()
	s.tray.SetMenu(s.trayMenu())
	s.tray.PopUpMenu()
	s.tray.SetMenu(nil)
}

// setTrayMenu sets the menu of an icon that only has its menu again when what it shows changed,
// as the menu shows it when it opens.
func (s *Shell) setTrayMenu() {
	m := s.trayMenu()
	var b strings.Builder
	for _, it := range m.Items() {
		b.WriteString(it.Label + "\t" + it.Accelerator + "\n")
	}
	if b.String() != s.trayItems {
		s.trayItems = b.String()
		s.tray.SetMenu(m)
	}
}
