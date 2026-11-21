package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
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
	s.OpenPanel(nil)
}

func (s *Shell) OpenPanel(anchor *mygo.Rectangle) {
	if s.panel != nil {
		s.panel.Close()
	}
	opts := mygo.FlyoutOptions{Width: panel.Width, Height: panel.Measure(s.core, s), Focusable: true, Shadow: true, Gap: 6,
		Content: ui.View(panel.New(s.core, s).View)}
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

func (s *Shell) FitPanel(h int) {
	if s.panel != nil {
		s.panel.SetContentSize(panel.Width, h)
	}
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

func (s *Shell) ShowSettings() { s.ShowSettingsPage("") }

func (s *Shell) ShowSettingsPage(page string) {
	s.closePanel()
	if s.guide != nil {
		s.guide.Show()
		return
	}
	if s.settings == nil {
		s.pages = settings.New(s.core, s)
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: locales.SettingsTitle(), Width: settings.Width, Height: settings.Height, MinWidth: settings.MinWidth, MinHeight: settings.MinHeight,
			TitleBarStyle: mygo.TitleBarHidden,
			StateKey:      "settings",
			Content:       ui.View(s.pages.View),
		})
		w.SetWindowButtonsTheme(mygo.ThemeDark)
		if err := mygo.Theme.SetGTKCSS(`headerbar.mygo-window-controls button.titlebutton { color: #f6efe4; }`); err != nil {
			log.Println("settings:", err)
		}
		s.settings = w
		w.OnClosed(func() {
			if s.settings == w {
				s.settings = nil
			}
			s.syncDock()
		})
	}
	if page != "" {
		pages := s.pages
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
		opts := mygo.WindowOptions{
			Title: locales.GuideTitle(), Width: guide.Width, Height: guide.Height, DisableResize: true, DisableMaximize: true,
			Content: ui.View(guide.New(s.core, s).View),
		}
		if shared.Windows {
			opts.TitleBarStyle, opts.TitleBarHeight, opts.Height = mygo.TitleBarHidden, guide.TitleBar, guide.Height+guide.TitleBar
		}
		w := mygo.NewWindow(opts)
		s.guide = w
		w.OnClosed(func() {
			if s.guide == w {
				s.guide = nil
			}
			clear(s.core.Held)
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

func (s *Shell) SetAppearance(a string) { mygo.Theme.SetSource(kit.ThemeSource(a)) }

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
