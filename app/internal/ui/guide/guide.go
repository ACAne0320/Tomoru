package guide

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

const ContentWidth, ContentHeight = 800, 580

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
	core              *core.Core
	nav               shared.Nav
	Step              int
	pendingStep       int
	try               try
	startStopRecorder shared.ShortcutRecorder
	pictureRecorder   shared.ShortcutRecorder
	awaitingPrompt    bool
	sample            sample
	sampleTranslation string
	swaps             int
	picks             map[core.Kind]pick
	scroll            ui.ScrollState
	keysShown         keysShown
	shownAt           time.Time
}

func New(k *core.Core, nav shared.Nav) *Guide {
	s := min(k.Preferences().GuideStep, steps-1)
	if skipped(s) {
		s = after(s, 1)
	}
	return &Guide{core: k, nav: nav, Step: s, pendingStep: s}
}

func (g *Guide) View(c *ui.Context) {
	kit.Paper(c)
	c.Root().Background(kit.Colors(c).Paper)
	k := g.core
	if g.Step > stepWelcome {
		k.PrepareLanguages()
	}
	if g.shownAt.IsZero() {
		g.shownAt = c.Now()
	}
	g.tryStep(c, g.Step == stepTry)
	g.holdKeys()
	bar := c.TitleBar()
	top := kit.Space(4)
	if bar.Right > 0 {
		top = max(top, bar.Height)
	}
	ui.Column(c).Fill().Children(func() {
		ui.Box(c).Height(top).Shrink(0).DragWindow()
		g.steps(c, top, bar)
	})
	g.commitStep(c)
}

func (g *Guide) commitStep(c *ui.Context) {
	if g.pendingStep != g.Step {
		g.Step, g.shownAt = g.pendingStep, c.Now()
		g.scroll.Y = 0
		c.AnimationFrame()
	}
}

func (g *Guide) steps(c *ui.Context, top float32, bar ui.TitleBar) {
	head := kit.Space(6)
	switch {
	case bar.Right > 0:
		head = kit.Space(3)
	case bar.Bottom > 0:
		head = max(head, bar.Bottom+kit.Space(4)-top)
	}
	ui.Row(c).Grow(1).Padding(0, kit.Space(4), kit.Space(4)).Gap(kit.Space(4)).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(leftWidth).Padding(0, leftPadR, kit.Space(1), leftPadL).Children(func() {
			ui.Box(c).Height(head).Shrink(0).DragWindow()
			ui.Column(c).Grow(1).MinHeight(0).Gap(kit.Space(4)).Children(func() {
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
		})
		g.stage(c)
	})
}

func (g *Guide) holdKeys() {
	k := g.core
	if g.Step != g.keysShown.step || g.keysShown.since.IsZero() {
		g.keysShown = keysShown{g.Step, time.Now()}
	}
	k.HoldShortcut(core.StartStop, g.Step == stepShortcuts)
	k.HoldShortcut(core.PictureShortcut, g.Step == stepPicture && k.CanSelect() && !g.works(core.PictureShortcut))
}

type keysShown struct {
	step  int
	since time.Time
}

func (g *Guide) works(s core.Shortcut) bool { return g.core.PressedSince(s, g.keysShown.since) }

func skipped(s int) bool { return s == stepPermission && !core.AsksAccess() }

func after(s, dir int) int {
	s += dir
	for skipped(s) {
		s += dir
	}
	return s
}

func (g *Guide) goTo(c *ui.Context, s int) {
	g.pendingStep = s
	if !g.core.Preferences().Onboarded {
		g.core.UpdatePrefs(func(p *prefs.Prefs) { p.GuideStep = s })
	}
	c.AnimationFrame()
}

func (g *Guide) progress(c *ui.Context) {
	p := kit.Colors(c)
	phase := 0
	for i, ph := range phases {
		if g.pendingStep >= ph.first {
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
				on := i == phase
				if on {
					t.FontWeight(kit.WeightBold)
				}
				col := kit.AnimateColor(c, t, "c", ui.Animation{Duration: 240 * time.Millisecond}, func(p *kit.Palette) ui.Color {
					if on {
						return p.Ink
					}
					return p.InkMuted
				})
				t.TextColor(col)
			}
		})
		const h = 4
		track := ui.Box(c).Height(h).Radius(kit.Pill(h)).Background(p.LineStrong).Clip()
		track.Children(func() {
			done := float32(g.pendingStep+1) / steps
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
		case core.PermissionGranted:
			if next(c, locales.Continue()).Clicked() {
				g.goTo(c, stepRecognition)
			}
		case core.PermissionDenied:
			if next(c, locales.OpenSystemSettings()).Clicked() {
				kit.OpenScreenSettings()
			}
		default:
			if kit.Off(c, next(c, locales.Allow()), k.Asking || k.Access == core.PermissionUnchecked).Clicked() {
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
			k.ReleaseShortcuts()
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
