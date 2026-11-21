package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/overlay"
	"tomoru/internal/ui/settings"
	"tomoru/locales"
)

type Shell struct {
	core      *core.Core
	tray      *mygo.Tray
	trayOn    bool
	trayItems string
	overlay   *overlay.Overlay

	shortcut      string
	imageShortcut string

	panel    *mygo.Window
	image    *mygo.Window
	picture  *mygo.Window
	settings *mygo.Window
	pages    *settings.Settings
	guide    *mygo.Window
}

func New(k *core.Core) *Shell {
	mygo.Theme.SetSource(kit.ThemeSource(k.Preferences().Appearance))
	s := &Shell{core: k}
	s.overlay = overlay.New(k, s)
	icon, template := trayIcon(assets.TrayIdle)
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: template, ToolTip: "Tomoru"})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	s.tray = tray
	if menuOnly {
		s.setTrayMenu()
	} else {
		tray.OnClick(s.togglePanel)
		tray.OnRightClick(s.openTrayMenu)
	}
	if err := s.SetShortcut(k.Preferences().Shortcut); err != nil {
		log.Println("shortcut:", err)
	}
	if err := s.SetImageShortcut(k.Preferences().ImageShortcut); err != nil {
		log.Println("picture's shortcut:", err)
	}
	k.CheckScreen()
	k.ListLangs()
	k.OnChange(s.redraw)
	mygo.App.OnLocaleChanged(func(string) { s.relabel() })
	return s
}

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

func (s *Shell) redraw() {
	if on := s.core.Running; on != s.trayOn {
		s.trayOn = on
		icon := assets.TrayIdle
		if on {
			icon = assets.TrayOn
		}
		s.tray.SetIcon(trayIcon(icon))
	}
	if menuOnly {
		s.setTrayMenu()
	}
	for _, w := range []*mygo.Window{s.panel, s.image, s.settings, s.guide} {
		if w != nil {
			w.Update(func() {})
		}
	}
	s.overlay.Redraw()
}

func (s *Shell) relabel() {
	if s.settings != nil {
		s.settings.SetTitle(locales.SettingsTitle())
	}
	if s.guide != nil {
		s.guide.SetTitle(locales.GuideTitle())
	}
	s.redraw()
}

func (s *Shell) syncDock() {
	if s.settings != nil || s.guide != nil {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
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
