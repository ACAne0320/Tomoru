package guide

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

const Width, Height = 800, 580

const TitleBar = 36

const (
	stepWelcome = iota
	stepPermission
	stepRecognition
	stepTranslation
	stepTry
	stepShortcuts
	stepPicture
	stepReady
	steps
)

var phases = []struct {
	first int
	name  func() string
}{
	{stepWelcome, locales.GuidePhaseSetup},
	{stepTry, locales.GuidePhaseTry},
	{stepReady, locales.GuidePhaseStart},
}

type Guide struct {
	core      *core.Core
	nav       shared.Nav
	Step      int
	next      int
	try       try
	keys      shared.ShortcutRecorder
	imageKeys shared.ShortcutRecorder
	asked     bool
	sample    sample
	selected  string
	swaps     int
	picks     map[core.Kind]pick
	scroll    ui.ScrollState
	keysStep  int
	keysSince time.Time
	shownAt   time.Time
}

func New(k *core.Core, nav shared.Nav) *Guide {
	s := min(k.Preferences().GuideStep, steps-1)
	if skipped(s) {
		s = after(s, 1)
	}
	return &Guide{core: k, nav: nav, Step: s, next: s}
}

func (g *Guide) View(c *ui.Context) {
	kit.Paper(c)
	c.Root().Background(kit.Colors(c).Paper)
	k := g.core
	if g.Step > stepWelcome {
		k.PrepareASR(k.Preferences().Src)
		k.PrepareASR(k.Preferences().Tgt)
	}
	if g.shownAt.IsZero() {
		g.shownAt = c.Now()
	}
	g.tryStep(c, g.Step == stepTry)
	g.holdKeys()
	bar := c.TitleBar()
	ui.Column(c).Fill().Children(func() {
		top := kit.Space(4)
		if bar.Height > 0 {
			titleBar(c, bar)
			top = 0
		}
		g.steps(c, top)
	})
	if g.next != g.Step {
		g.Step, g.shownAt = g.next, c.Now()
		g.scroll.Y = 0
		c.AnimationFrame()
	}
}

func titleBar(c *ui.Context, bar ui.TitleBar) {
	ui.Row(c).Height(bar.Height).Padding(0, bar.Right, 0, kit.Space(4)+bar.Left).Gap(kit.Space(2)).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Image(c, assets.AppIcon).Size(22, 22).Margin(-2)
		ui.Text(c, locales.GuideTitle()).FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).InkMuted)
	})
}

func (g *Guide) steps(c *ui.Context, top float32) {
	ui.Row(c).Grow(1).Padding(top, kit.Space(4), kit.Space(4)).Gap(kit.Space(4)).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(leftWidth).Padding(kit.Space(6), leftPadR, kit.Space(1), leftPadL).Gap(kit.Space(4)).Children(func() {
			g.progress(c)
			ui.Column(c).Grow(1).MinHeight(0).Justify(ui.Center).Children(func() {
				kit.FadeScroll(c, &g.scroll, func() {
					ui.Column(c).Gap(kit.Space(4)).Padding(0, 0, kit.RiseBy).Children(func() {
						switch g.Step {
						case stepWelcome:
							g.welcome(c)
						case stepPermission:
							g.permission(c)
						case stepRecognition:
							g.recognition(c)
						case stepTranslation:
							g.translation(c)
						case stepTry:
							g.trying(c)
						case stepPicture:
							g.picture(c)
						case stepShortcuts:
							g.shortcuts(c)
						case stepReady:
							g.ready(c)
						}
					})
				})
			})
			ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
				if g.Step > stepWelcome && kit.Button(c, kit.Plain, kit.Medium, nil, locales.Back()).Clicked() {
					g.goTo(c, after(g.Step, -1))
				}
				ui.Spacer(c)
				g.stepButton(c)
			})
		})
		g.stage(c)
	})
}

func (g *Guide) holdKeys() {
	k := g.core
	if g.Step != g.keysStep || g.keysSince.IsZero() {
		g.keysStep, g.keysSince = g.Step, time.Now()
	}
	k.Held[core.StartStop] = g.Step == stepShortcuts
	k.Held[core.PictureShortcut] = g.Step == stepPicture && k.CanSelect() && !g.works(core.PictureShortcut)
}

func (g *Guide) works(s core.Shortcut) bool { return g.core.PressedSince(s, g.keysSince) }

func skipped(s int) bool { return s == stepPermission && !core.AsksAccess() }

func after(s, dir int) int {
	s += dir
	for skipped(s) {
		s += dir
	}
	return s
}

func (g *Guide) goTo(c *ui.Context, s int) {
	g.next = s
	if !g.core.Preferences().Onboarded {
		g.core.UpdatePrefs(func(p *prefs.Prefs) { p.GuideStep = s })
	}
	c.AnimationFrame()
}

func (g *Guide) progress(c *ui.Context) {
	p := kit.Colors(c)
	phase := 0
	for i, ph := range phases {
		if g.next >= ph.first {
			phase = i
		}
	}
	ui.Column(c).Gap(kit.Space(3)).Children(func() {
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
			for i, ph := range phases {
				if i > 0 {
					kit.Caption(c, "›")
				}
				t := ui.Text(c, ph.name()).FontSize(kit.TextControl).SingleLine()
				col := p.InkMuted
				if i == phase {
					col = p.Ink
					t.FontWeight(kit.WeightBold)
				}
				t.TextColor(t.AnimateColor("c", col, ui.Animation{Duration: 240 * time.Millisecond}))
			}
		})
		const h = 4
		track := ui.Box(c).Height(h).Radius(kit.Pill(h)).Background(p.LineStrong).Clip()
		track.Children(func() {
			done := float32(g.next+1) / steps
			bar := ui.Box(c).Height(h).Radius(kit.Pill(h)).Background(p.Lantern)
			bar.Width(bar.AnimateTo("w", done*progressWidth, kit.Spring))
		})
	})
}

const leftWidth = 352

var (
	leftPadL, leftPadR = kit.Space(4), kit.Space(4)
	progressWidth      = leftWidth - leftPadL - leftPadR
)

func (g *Guide) stepButton(c *ui.Context) {
	k := g.core
	switch g.Step {
	case stepWelcome:
		if next(c, locales.Continue()).Clicked() {
			g.goTo(c, after(stepWelcome, 1))
			k.CheckAccess()
			k.CheckScreen()
		}
	case stepPermission:
		switch k.Access {
		case "granted":
			if next(c, locales.Continue()).Clicked() {
				g.goTo(c, stepRecognition)
			}
		case "denied":
			if next(c, locales.OpenSystemSettings()).Clicked() {
				kit.OpenScreenSettings()
			}
		default:
			if kit.Off(c, next(c, locales.Allow()), k.Asking || k.Access == "").Clicked() {
				k.RequestAccess()
			}
		}
	case stepRecognition:
		if kit.Off(c, next(c, locales.Continue()), k.Chosen(core.ASR) == "").Clicked() {
			g.goTo(c, stepTranslation)
		}
	case stepTranslation:
		if next(c, locales.Continue()).Clicked() {
			g.goTo(c, stepTry)
		}
	case stepTry:
		if g.try.works() {
			if next(c, locales.Continue()).Clicked() {
				g.goTo(c, stepShortcuts)
			}
		} else if kit.Button(c, kit.Plain, kit.Medium, nil, locales.Skip()).Clicked() {
			g.goTo(c, stepShortcuts)
		}
	case stepShortcuts:
		if next(c, locales.Continue()).Clicked() {
			g.goTo(c, stepPicture)
		}
	case stepPicture:
		if k.CanSelect() {
			if next(c, locales.Continue()).Clicked() {
				g.goTo(c, stepReady)
			}
		} else if kit.Button(c, kit.Plain, kit.Medium, nil, locales.Skip()).Clicked() {
			g.goTo(c, stepReady)
		}
	case stepReady:
		b := next(c, locales.Done())
		if v := glow(c, c.Now().Sub(g.shownAt)); v > 0 {
			b.Shadow(0, 8, 24+20*v, 0, kit.Colors(c).Lantern.Alpha(.45+.35*v))
		}
		if b.Clicked() {
			k.StopSpeaking()
			clear(k.Held)
			k.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded, p.GuideStep = true, 0 })
			g.nav.FinishGuide()
		}
	}
}

func next(c *ui.Context, label string) ui.Element {
	return kit.Button(c, kit.Primary, kit.Medium, nil, label)
}

func (g *Guide) part(c *ui.Context, i int, fn func()) ui.Element {
	return kit.Arrive(ui.Column(c.Key(fmt.Sprintf("%d.%d", g.Step, i))).Gap(kit.Space(4)), i).Children(fn)
}

func title(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(kit.Colors(c).Ink)
}

func body(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(kit.TextBody).TextColor(kit.Colors(c).InkMuted).LineHeight(1.45)
}

func note(c *ui.Context, s string) ui.Element { return kit.Caption(c, s).LineHeight(1.4) }
