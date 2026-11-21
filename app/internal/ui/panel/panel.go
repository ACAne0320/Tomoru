package panel

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

const Width = 340

type Panel struct {
	core *core.Core
	nav  shared.Nav
}

func New(k *core.Core, nav shared.Nav) *Panel { return &Panel{core: k, nav: nav} }

func (p *Panel) View(c *ui.Context) {
	k := p.core
	if k.Running {
		c.After(time.Second - k.RunFor()%time.Second)
	}
	p.frame(c)
}

func (p *Panel) frame(c *ui.Context) ui.Element {
	return kit.PanelFrame(c, func() { p.head(c) }, func() { p.body(c) }, func() { p.foot(c) })
}

func (p *Panel) head(c *ui.Context) {
	k := p.core
	pal := kit.Colors(c)
	kit.Sticker(c, 56, 36, k.Running)
	ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() {
		ui.Text(c, shared.Title(k.Preferences(), k.Running)).FontSize(kit.TextSection).FontWeight(kit.WeightHeavy).TextColor(pal.NightTitle).MaxLines(1)
		ui.Row(c).Wrap().GapX(kit.Space(1)).FontSize(kit.TextCaption).FontFeatures(kit.Tabular).TextColor(pal.NightText).Children(func() {
			for _, part := range p.state() {
				ui.Text(c, part).MaxLines(3)
			}
		})
	})
	keys := kit.ShortcutText(k.Preferences().Shortcut)
	if k.Running {
		if kit.Button(c, kit.Destructive(kit.Plain), kit.Small, assets.Pause, locales.PauseShort()).Tooltip(keys).Clicked() {
			k.Stop()
		}
	} else if kit.Button(c, kit.Primary, kit.Small, assets.Play, locales.Start()).Tooltip(keys).Clicked() {
		k.Start()
	}
}

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
		run, lines, label = time.Duration(today.RunSeconds*float64(time.Second)), today.Lines, locales.StatToday()
	}
	return []string{label + " " + kit.Clock(run) + " ·", locales.HistoryLines(lines)}
}

func (p *Panel) body(c *ui.Context) {
	k := p.core
	if err := shared.ErrorText(k.Err); err != "" {
		ui.Column(c).Gap(kit.Space(2)).Children(func() {
			ui.Text(c, err).FontSize(kit.TextCaption).TextColor(kit.Colors(c).Danger).MaxLines(3)
			ui.Row(c).Children(func() {
				shared.FixButton(c, k, k.Err, func() { p.nav.ShowSettingsPage(shared.PageEngines) })
			})
		})
	}
	kit.ListCard(c).Children(func() {
		p.scene(c)
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Padding(kit.Space(2), kit.Space(3), kit.Space(3)).Children(func() {
			kit.PickerField(c, shared.LangName(k.Preferences().Src), kit.Choices(k.Preferences().Src, shared.LangChoices(k.SourceLangs()), shared.LangName, k.SetSrc)).Label(locales.SpokenLanguage()).Grow(1).Basis(0)
			off := !k.Preferences().Translates()
			if kit.Off(c, kit.IconButton(c, kit.Quiet, kit.Small, assets.Swap, locales.SwapLanguages()), !k.CanSwap() || off).Clicked() {
				k.Swap()
			}
			kit.Off(c, kit.PickerField(c, shared.LangName(k.Preferences().Tgt), kit.Choices(k.Preferences().Tgt, shared.LangChoices(k.TargetLangs()), shared.LangName, k.SetTgt)).Label(locales.TranslateInto()).Grow(1).Basis(0), off)
		})
		k.ListApps()
		c.After(core.RefreshEvery)
		kit.MenuRow(c, 1, func() {
			if k.Running && k.State.Phase == "listening" {
				kit.MenuLevels(c, k.State.Voice)
			} else {
				kit.MenuIcon(c, assets.Waves)
			}
		}, locales.Source(), shared.SourceName(k), nil, shared.SourceMenu(k))
	})
}

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
	}, shared.SceneMenu(k, false, func() { p.nav.ShowSettingsPage(shared.PageScenes) }))
}

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
