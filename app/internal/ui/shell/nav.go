package shell

import (
	"time"

	"github.com/egoist/mygo"

	"tomoru/internal/core"
	"tomoru/internal/ui/guide"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/panel"
	"tomoru/internal/ui/settings"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// The windows, as shared.Nav opens them.

func (s *Shell) togglePanel() {
	if s.panel != nil {
		s.panel.Close()
		return
	}
	s.OpenPanel(nil)
}

// OpenPanel shows the panel below the tray icon, or above anchor, a box in the overlay. The
// overlay's settings button is the way in when the menu bar has no room for the icon: macOS hides
// status items behind the notch, and capturing adds its own recording indicator.
func (s *Shell) OpenPanel(anchor *mygo.Rectangle) {
	if s.panel != nil {
		s.panel.Close()
	}
	// The panel draws its own frame, the night's band over the paper; the system draws its shadow.
	// As tall as it measures: it shows before it fits itself, and changing its height then made it
	// jump as it opened, the first time after launch too, when it had not fitted itself yet.
	opts := mygo.FlyoutOptions{Width: panel.Width, Height: panel.Measure(s.core, s), Focusable: true, Shadow: true, Gap: 6,
		Content: s.view(panel.New(s.core, s).View)}
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

// FitPanel makes the panel h tall, as tall as what it shows.
func (s *Shell) FitPanel(h int) {
	if s.panel != nil {
		s.panel.SetContentSize(panel.Width, h)
	}
}

// TranslateImage translates a picture, as its shortcut asks: a part of the screen the user selects
// when the app may capture it, else the picture on the clipboard, with a card that says how to
// put one there when there is none.
func (s *Shell) TranslateImage() {
	k := s.core
	if k.CanSelect() {
		if s.image != nil {
			s.image.Close() // out of the way of the selection
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
		s.guide.Show() // finish the first-run guide first
		return
	}
	if s.settings == nil {
		s.pages = settings.New(s.core, s)
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: locales.SettingsTitle(), Width: settings.Width, Height: settings.Height, MinWidth: settings.MinWidth, MinHeight: settings.MinHeight,
			TitleBarStyle: mygo.TitleBarHidden, // the frame draws under the window's controls
			StateKey:      "settings",
			Content:       s.view(s.pages.View),
		})
		// Its controls are on the night's band, dark in either appearance: drawn light (Windows).
		w.SetWindowButtonsTheme(mygo.ThemeDark)
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
			Content: s.view(guide.New(s.core, s).View),
		}
		if shared.Windows {
			// Windows' title bar is white: the guide draws its own on the paper, under the controls.
			opts.TitleBarStyle, opts.TitleBarHeight, opts.Height = mygo.TitleBarHidden, guide.TitleBar, guide.Height+guide.TitleBar
		}
		w := mygo.NewWindow(opts)
		s.guide = w
		w.OnClosed(func() {
			if s.guide == w {
				s.guide = nil
			}
			clear(s.core.Held) // the shortcuts it checked do what they do again
			// Closed before it was done: what it tried stops with it.
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
		s.core.Start() // else it runs since the guide tried it
	}
	s.overlay.ShowSample(8 * time.Second) // show where the subtitles go, while nobody speaks
}

func (s *Shell) StartAdjust() {
	s.closePanel()
	s.overlay.StartAdjust()
}

func (s *Shell) SetLocked(locked bool) {
	if !locked {
		s.closePanel() // unlocked to move the subtitles: out of their way
	}
	s.overlay.SetLocked(locked)
	s.redraw()
}

func (s *Shell) SetHideFromCapture(hide bool) { s.overlay.SetHideFromCapture(hide) }

func (s *Shell) Focus() { mygo.App.Focus() }

func (s *Shell) SetAppearance(a string) { mygo.Theme.SetSource(kit.ThemeSource(a)) }

func (s *Shell) SetShortcut(acc string) error {
	return bind(&s.shortcut, acc, func() {
		if !s.core.Press(core.StartStop) {
			s.core.Toggle()
		}
	})
}

func (s *Shell) SetImageShortcut(acc string) error {
	return bind(&s.imageShortcut, acc, func() {
		if !s.core.Press(core.PictureShortcut) {
			s.TranslateImage()
		}
	})
}

// bind makes acc the global shortcut that runs fn, in place of *held, the one registered before
// ("" for none), which stays if acc cannot be registered.
func bind(held *string, acc string, fn func()) error {
	before := *held
	if before != "" {
		mygo.GlobalShortcut.Unregister(before)
	}
	if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
		if before != "" && mygo.GlobalShortcut.Register(before, fn) != nil {
			*held = ""
		}
		return err
	}
	*held = acc
	return nil
}
