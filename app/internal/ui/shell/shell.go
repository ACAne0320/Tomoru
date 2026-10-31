// Package shell puts the UI together: the menu bar icon, the subtitle overlay, and the panel,
// settings and guide windows, which it opens and closes. It implements shared.Nav, by which the
// surfaces go to each other, and redraws them as the core changes.
package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/overlay"
	"tomoru/internal/ui/settings"
	"tomoru/locales"
)

// Shell is the UI. It is used on the main thread.
type Shell struct {
	core    *core.Core
	tray    *mygo.Tray
	trayOn  bool
	overlay *overlay.Overlay

	shortcut      string // the global shortcut registered, "" for none
	imageShortcut string // the picture's

	panel    *mygo.Window       // the panel's flyout, while it shows
	image    *mygo.Window       // the picture's translation card, while it shows
	picture  *mygo.Window       // the card's picture, whole, while it shows
	settings *mygo.Window       // the settings window, while it shows
	pages    *settings.Settings // its content, whose page ShowSettingsPage turns to
	guide    *mygo.Window       // the first-run guide, while it shows
}

// New makes the menu bar icon, the overlay and the global shortcut.
func New(k *core.Core) *Shell {
	s := &Shell{core: k}
	s.overlay = overlay.New(k, s)
	icon, template := trayIcon(assets.TrayIdle)
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: template, ToolTip: "Tomoru"})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	s.tray = tray
	// A click opens the panel, Tomoru's own, what one changes while watching; the right button the
	// system's menu, as the icons of the menu bar and of Windows' taskbar have it.
	tray.OnClick(s.togglePanel)
	tray.OnRightClick(s.openTrayMenu)
	if err := s.SetShortcut(k.Preferences().Shortcut); err != nil {
		log.Println("shortcut:", err)
	}
	if err := s.SetImageShortcut(k.Preferences().ImageShortcut); err != nil {
		log.Println("picture's shortcut:", err)
	}
	k.CheckScreen() // whether the picture's shortcut selects a part of the screen
	k.ListLangs()   // the languages the system's recognition hears
	k.OnChange(s.redraw)
	mygo.App.OnLocaleChanged(func(string) { s.relabel() })
	return s
}

// Launch does what launching the app asks: the first-run guide on a first launch, else translating
// when the settings or start say so, else saying it is ready where the subtitles go. Opened at
// login, it waits: the user did not ask for subtitles yet.
func (s *Shell) Launch(start bool) {
	k := s.core
	switch {
	case !k.Preferences().Onboarded:
		s.ShowGuide()
	case start || k.Preferences().AutoStart && !mygo.App.WasOpenedAtLogin():
		k.Start()
	default:
		k.NoteAt = time.Now()
		s.redraw()
	}
}

// redraw redraws the windows, which show the core's state, and sets the menu bar icon.
func (s *Shell) redraw() {
	if on := s.core.Running; on != s.trayOn {
		s.trayOn = on
		icon := assets.TrayIdle
		if on {
			icon = assets.TrayOn
		}
		s.tray.SetIcon(trayIcon(icon))
	}
	for _, w := range []*mygo.Window{s.panel, s.image, s.settings, s.guide} {
		if w != nil {
			w.Update(func() {})
		}
	}
	s.overlay.Redraw()
}

// view refreshes the overlay after a surface renders. Application actions own persistence.
func (s *Shell) view(fn func(*ui.Context)) *ui.Content {
	return ui.View(func(c *ui.Context) {
		fn(c)
		s.overlay.Redraw()
	})
}

// relabel sets again what windows took once, in the language the app now shows.
func (s *Shell) relabel() {
	if s.settings != nil {
		s.settings.SetTitle(locales.SettingsTitle())
	}
	if s.guide != nil {
		s.guide.SetTitle(locales.GuideTitle())
	}
	s.redraw()
}

// syncDock shows the Dock icon while the settings window or the guide is open, as menu bar apps
// do, and brings the app to the front for them.
func (s *Shell) syncDock() {
	if s.settings != nil || s.guide != nil {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		// A run without an app bundle has no icon of its own. AppKit sets the new tile's icon after
		// the policy changes, over one set at once or 300 ms later; 1.5 s later it holds.
		time.AfterFunc(1500*time.Millisecond, func() {
			if s.core.Context().Err() == nil {
				mygo.App.Dock.SetIcon(assets.AppIconPNG)
			}
		})
		mygo.App.Focus()
	} else {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}
}
