// Package panel is the menu bar panel: translating's state and its numbers, the scene and its
// languages, what it hears, and a row of icons for the subtitles and the app (docs/design-system.md).
package panel

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// Width is the panel's width, Height about its height, which the shell fits to the panel as it
// shows.
const Width, Height = 340, 297

// Panel is the panel's content. The shell shows it in a flyout.
type Panel struct {
	core   *core.Core
	nav    shared.Nav
	source shared.SourceMenu
}

func New(k *core.Core, nav shared.Nav) *Panel { return &Panel{core: k, nav: nav} }

// Measure is how tall the panel is as it would show now, laid out off screen, or Height when it
// cannot tell: the shell opens it so tall. An error makes it taller than Height, so a flyout
// opened Height tall jumped as it fitted itself.
func Measure(k *core.Core, nav shared.Nav) int {
	m := &measure{Nav: nav}
	ui.NewTester(New(k, m).View, Width, 2*Height) // taller than it is: it fits itself
	if m.h == 0 {
		return Height
	}
	return m.h
}

// measure is the shell as the panel measured off screen goes to it: it keeps the height it fits.
type measure struct {
	shared.Nav
	h int
}

func (m *measure) FitPanel(h int) { m.h = h }

func (p *Panel) View(c *ui.Context) {
	k := p.core
	if k.Running {
		c.After(time.Second) // the numbers count on
	}
	panel := kit.PanelFrame(c, func() { p.head(c) }, func() { p.body(c) }, func() { p.foot(c) })
	// The panel's height is known once it is laid out: the next frame fits the flyout to it.
	_, now := c.Size()
	switch h := int(math.Ceil(float64(panel.Bounds().H))); {
	case h == 0:
		c.Invalidate()
	case h != int(now):
		p.nav.FitPanel(h)
	}
}

// head is the night's band: the sticker, whether it translates and its numbers, start or stop.
func (p *Panel) head(c *ui.Context) {
	k := p.core
	pal := kit.Colors(c)
	kit.Sticker(c, 56, 36, k.Running) // awake while translating, as the menu bar icon
	ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() {
		title := locales.HomeIdle()
		if k.Running {
			title = locales.HomeTranslating()
		}
		ui.Text(c, title).FontSize(kit.TextSection).FontWeight(kit.WeightHeavy).TextColor(pal.NightTitle).MaxLines(1)
		// Under it, what it waits for, or the numbers: a line, or two in a wordy language, broken
		// only between the time and the count, never inside one.
		ui.Row(c).Wrap().GapX(kit.Space(1)).FontSize(kit.TextCaption).FontFeatures(kit.Tabular).TextColor(pal.NightText).Children(func() {
			for _, part := range p.state() {
				ui.Text(c, part).MaxLines(3)
			}
		})
	})
	keys := kit.ShortcutText(k.Preferences().Shortcut) // on hovering it, as the foot has no room to say it
	if k.Running {
		if kit.Button(c, kit.Destructive(kit.Plain), kit.Small, assets.Stop, locales.Stop()).Tooltip(keys).Clicked() {
			k.Stop()
		}
	} else if kit.Button(c, kit.Primary, kit.Small, assets.Play, locales.Start()).Tooltip(keys).Clicked() { // short, as Stop: the title says what
		k.Start()
	}
}

// state is what goes under the title: what translating waits for, or its numbers, this run's while
// translating and today's while not, the time and the count apart.
func (p *Panel) state() []string {
	k := p.core
	if k.Running {
		switch k.State.Phase {
		case "downloading":
			return []string{locales.StatusDownloading()}
		case "starting":
			return []string{locales.StatusPreparing()}
		case "loading":
			return []string{locales.StatusLoadingModel()}
		}
	}
	run, lines, label := k.RunFor(), k.State.Stats.Lines, locales.StatRunning()
	if !k.Running {
		today := k.TodayStats()
		run, lines, label = time.Duration(today.Run*float64(time.Second)), today.Lines, locales.StatToday()
	}
	return []string{label + " " + kit.Clock(run) + " ·", locales.HistoryLines(lines)}
}

// body is why translating stopped, if it did, with the action that resolves it under it, for want
// of room beside it; the scene and its languages, and what it hears.
func (p *Panel) body(c *ui.Context) {
	k := p.core
	if err := shared.ErrorText(k.Err); err != "" {
		ui.Column(c).Gap(kit.Space(2)).Children(func() {
			ui.Text(c, err).FontSize(kit.TextCaption).TextColor(kit.Colors(c).Danger).MaxLines(3)
			ui.Row(c).Children(func() {
				shared.FixButton(c, k, k.Err, func() { p.nav.ShowSettingsPage("engines") })
			})
		})
	}
	kit.ListCard(c).Children(func() {
		// The scene, and under it, with no rule between, its languages: the scene keeps them.
		// What it hears is not the scene's, after a rule.
		p.scene(c)
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Padding(kit.Space(2), kit.Space(3), kit.Space(3)).Children(func() {
			kit.PickerField(c, shared.LangName(k.Preferences().Src), kit.Choices(k.Preferences().Src, shared.LangChoices(k.SourceLangs()), shared.LangName, k.SetSrc)).Label(locales.SpokenLanguage()).Grow(1).Basis(0)
			// With the original only, nothing is translated into the target: it stays, greyed.
			off := !k.Preferences().Translates()
			if kit.Off(c, kit.IconButton(c, kit.Quiet, kit.Small, assets.Swap, locales.SwapLanguages()), !k.CanSwap() || off).Clicked() {
				k.Swap()
			}
			kit.Off(c, kit.PickerField(c, shared.LangName(k.Preferences().Tgt), kit.Choices(k.Preferences().Tgt, shared.LangChoices(k.TargetLangs()), shared.LangName, k.SetTgt)).Label(locales.TranslateInto()).Grow(1).Basis(0), off)
		})
		k.ListApps()
		c.After(core.ListEvery) // the apps playing change
		// What it hears, moving as a voice is heard while it listens: that sound comes, with nothing
		// translated yet.
		kit.MenuRow(c, 1, func() {
			if k.Running && k.State.Phase == "listening" {
				kit.MenuLevels(c, k.State.Voice)
			} else {
				kit.MenuIcon(c, assets.Waves)
			}
		}, locales.Source(), shared.SourceName(k), nil, p.source.Build(k))
	})
}

// scene is the scene's row: the one in use, and how many names it has.
func (p *Panel) scene(c *ui.Context) {
	k := p.core
	terms := 0
	if s, ok := k.SceneByID(k.Preferences().Scene); ok {
		terms = len(s.Terms)
	}
	kit.MenuRow(c, 0, func() { kit.MenuIcon(c, assets.Book) }, locales.Scene(), shared.CurrentSceneName(k), func() {
		if terms > 0 {
			kit.CountBadge(c, terms)
		}
	}, shared.SceneMenu(k, false, func() { p.nav.ShowSettingsPage("scenes") }))
}

// foot is icons alone, each named as it is hovered: translating a part of the screen; the
// subtitles, found to move them while nothing shows, and locked or unlocked; the window, and
// quitting, apart from Stop so that none quits by mistake.
func (p *Panel) foot(c *ui.Context) {
	k := p.core
	screen := locales.TranslateScreen()
	if kit.IconButton(c, kit.Quiet, kit.Medium, assets.Scan, screen).
		Tooltip(screen + " " + kit.ShortcutText(k.Preferences().ImageShortcut)).Clicked() {
		p.nav.TranslateImage()
	}
	ui.Box(c).Width(1).Height(16).Margin(0, kit.Space(1)).Background(kit.Colors(c).Line)
	if kit.IconButton(c, kit.Quiet, kit.Medium, assets.Move, locales.MoveSubtitles()).Clicked() {
		p.nav.StartAdjust()
	}
	// The lock shows how the subtitles are: open, or shut on the lantern's tint; it is named for
	// what a click does.
	locked := k.Preferences().Locked
	kind, icon, label := kit.Quiet, assets.Unlock, locales.LockSubtitles()
	if locked {
		kind, icon, label = kit.Soft, assets.Lock, locales.UnlockSubtitles()
	}
	if kit.IconButton(c, kind, kit.Medium, icon, label).Clicked() {
		p.nav.SetLocked(!locked)
	}
	ui.Box(c).Grow(1)
	if kit.IconButton(c, kit.Quiet, kit.Medium, assets.Settings, locales.OpenTomoru()).Clicked() {
		p.nav.ShowSettings()
	}
	if kit.IconButton(c, kit.Close, kit.Medium, assets.Quit, locales.QuitTomoru()).Clicked() {
		mygo.App.Quit()
	}
}
