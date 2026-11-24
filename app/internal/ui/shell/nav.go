package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/guide"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/panel"
	"tomoru/internal/ui/settings"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

func (s *Shell) togglePanel() {
	if s.panel != nil {
		s.panel.Close()
		return
	}
	s.OpenPanel()
}

func (s *Shell) OpenPanel() { s.openPanel(nil) }

func (s *Shell) OpenPanelAbove(r mygo.Rectangle) { s.openPanel(&r) }

func (s *Shell) openPanel(anchor *mygo.Rectangle) {
	if s.panel != nil {
		s.panel.Close()
	}
	opts := mygo.FlyoutOptions{Width: panel.Width, FitContent: true, MinWidth: panel.Width, MaxWidth: panel.Width,
		Focusable: true, Shadow: true, Gap: 6, Content: ui.View(panel.New(s.core, s).View)}
	if anchor == nil {
		opts.Tray, opts.Placement = s.tray, mygo.PlacementBottom
	} else {
		opts.Parent, opts.Anchor, opts.Placement = s.overlay.Window(), *anchor, mygo.PlacementTop
	}
	p := mygo.NewFlyout(opts)
	s.panel = p
	p.OnClosed(func() {
		if s.panel == p {
			s.panel = nil
		}
	})
}

func (s *Shell) TranslateImage() {
	k := s.core
	if k.CanSelect() {
		if s.image != nil {
			s.image.Close()
		}
		s.closePanel()
		k.SelectImage(s.showImage)
		return
	}
	k.TranslateImage(mygo.Clipboard.ReadImage())
	s.showImage()
}

func (s *Shell) closePanel() {
	if s.panel != nil {
		s.panel.Close()
	}
}

func (s *Shell) ShowSettings() { s.openSettings(nil) }

func (s *Shell) ShowSettingsPage(page shared.Page) { s.openSettings(&page) }

func (s *Shell) openSettings(page *shared.Page) {
	s.closePanel()
	if s.guide != nil {
		s.guide.Show()
		return
	}
	if s.settings == nil {
		s.pages = settings.New(s.core, s)
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: locales.SettingsTitle(), Width: settings.Width, Height: settings.Height, MinWidth: settings.MinWidth, MinHeight: settings.MinHeight,
			TitleBarStyle:        mygo.TitleBarHidden,
			TitleBarHeight:       kit.TitleBarHeight,
			TrafficLightPosition: &mygo.Point{X: 20, Y: 20},
			StateKey:             "settings",
			Content:              ui.View(s.pages.View),
		})
		if len(mygo.Theme.WindowButtons().Start) > 0 {
			if err := mygo.Theme.SetGTKCSS(`headerbar.mygo-window-controls { margin: 8px 0 0 8px; }`); err != nil {
				log.Println("settings:", err)
			}
		}
		s.settings = w
		w.OnClosed(func() {
			if s.settings == w {
				s.settings = nil
			}
			s.syncDock()
		})
	}
	if page != nil {
		pages, page := s.pages, *page
		s.settings.Update(func() { pages.Open(page) })
	}
	s.syncDock()
	s.settings.Show()
}

func (s *Shell) ShowGuide() {
	if s.settings != nil {
		s.settings.Close()
	}
	if s.guide == nil {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:                locales.GuideTitle(),
			Width:                guide.ContentWidth,
			Height:               guide.ContentHeight,
			DisableResize:        true,
			DisableMaximize:      true,
			TitleBarStyle:        mygo.TitleBarHidden,
			TitleBarHeight:       kit.TitleBarHeight,
			TrafficLightPosition: &mygo.Point{X: 20, Y: 20},
			Content:              ui.View(guide.New(s.core, s).View),
		})
		s.guide = w
		w.OnClosed(func() {
			if s.guide == w {
				s.guide = nil
			}
			s.core.ReleaseShortcuts()
			if k := s.core; !k.Preferences().Onboarded {
				k.StopSpeaking()
				k.Stop()
			}
			s.syncDock()
		})
	}
	s.syncDock()
	s.guide.Show()
}

func (s *Shell) FinishGuide() {
	if s.guide != nil {
		s.guide.Close()
	}
	if !s.core.Running {
		s.core.Start()
	}
	s.overlay.ShowSample(8 * time.Second)
}

func (s *Shell) StartAdjust() {
	s.closePanel()
	s.overlay.StartAdjust()
}

func (s *Shell) SetLocked(locked bool) {
	if !locked {
		s.closePanel()
	}
	s.overlay.SetLocked(locked)
	s.redraw()
}

func (s *Shell) SetHideFromCapture(hide bool) { s.overlay.SetHideFromCapture(hide) }

func (s *Shell) Focus() { mygo.App.Focus() }

func (s *Shell) SetAppearance(a prefs.Appearance) { mygo.Theme.SetSource(themeSource(a)) }

func themeSource(a prefs.Appearance) mygo.ThemeSource {
	switch a {
	case prefs.AppearanceLight:
		return mygo.ThemeLight
	case prefs.AppearanceDark:
		return mygo.ThemeDark
	}
	return mygo.ThemeSystem
}

func (s *Shell) SetShortcut(acc string) error {
	return bind(&s.shortcut, acc, locales.Shortcut(), func() {
		if !s.core.Press(core.StartStop) {
			s.core.Toggle()
		}
	})
}

func (s *Shell) SetImageShortcut(acc string) error {
	return bind(&s.imageShortcut, acc, locales.ImageShortcut(), func() {
		if !s.core.Press(core.PictureShortcut) {
			s.TranslateImage()
		}
	})
}

func bind(held *string, acc, what string, fn func()) error {
	opts := mygo.GlobalShortcutOptions{Description: what}
	before := *held
	if before != "" {
		mygo.GlobalShortcut.Unregister(before)
	}
	if err := mygo.GlobalShortcut.RegisterWith(acc, opts, fn); err != nil {
		if before != "" && mygo.GlobalShortcut.RegisterWith(before, opts, fn) != nil {
			*held = ""
		}
		return err
	}
	*held = acc
	return nil
}
