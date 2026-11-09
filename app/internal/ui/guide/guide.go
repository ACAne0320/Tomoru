// Package guide is the first-run guide, in three phases of a step or more each. Setting up: the
// languages; the System Audio Recording permission; how what is heard is recognized and
// translated. Trying it: subtitles for real, on a video or on sentences the system's voice says;
// the shortcut that starts and stops them; and, if the user likes, selecting text on the screen to
// translate, which takes Screen Recording. Then where the app is.
// Each step is in a file of its own: its words and controls on the left, and on the right a stage
// showing what it is about, over a picture that moves as a video would.
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

// Width and Height are the window's size, but for TitleBar.
const Width, Height = 800, 580

// TitleBar is the height of the title bar the guide draws itself, under the window's controls,
// where the window's own would not look the app's (Windows): the window is that much taller.
const TitleBar = 36

// The steps.
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

// phases are the guide's phases, as its header names them, by the step each begins at.
var phases = []struct {
	first int
	name  func() string
}{
	{stepWelcome, locales.GuidePhaseSetup},
	{stepTry, locales.GuidePhaseTry},
	{stepReady, locales.GuidePhaseStart},
}

// Guide is the window's content. The shell shows it in a window.
type Guide struct {
	core *core.Core
	nav  shared.Nav
	Step int
	// next is the step to go to. Step follows it a frame later, so that the step leaving, built
	// once more, goes the way the guide goes.
	next      int
	try       try
	keys      shared.ShortcutRecorder // start and stop's
	imageKeys shared.ShortcutRecorder // the picture's
	asked     bool                    // the system was asked for a permission, and has not answered yet
	sample    sample                  // the picture to translate, as the stage shows it
	// selected is the sample's translation once selecting worked: the check stays done, and the
	// translation shows, after the card it came in is closed, which clears the core's picture.
	selected string
	swaps    int                // the languages swapped, for the button to turn as often
	picks    map[core.Kind]pick // the engines the stage shows, to flash as they are chosen
	// keysStep is the step keysSince is of: when it showed, for a shortcut pressed since to be
	// seen working.
	scroll    ui.ScrollState // where the step is scrolled, when it is taller than its room
	keysStep  int
	keysSince time.Time
	shownAt   time.Time // when the step showed, for what it does a while after
}

// New starts the guide at the step it was left at: System Settings has the app reopened to grant
// it permission.
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
		// The languages are chosen: get their recognition ready while the user goes on. The target
		// too, for a swap.
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
		g.scroll.Y = 0 // a step starts at its top
		c.AnimationFrame()
	}
}

// titleBar is the window's title bar, drawn by the guide: the app's icon and the window's title
// on the paper, clear of the window's controls; it drags the window.
func titleBar(c *ui.Context, bar ui.TitleBar) {
	ui.Row(c).Height(bar.Height).Padding(0, bar.Right, 0, kit.Space(4)+bar.Left).Gap(kit.Space(2)).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Image(c, assets.AppIcon).Size(22, 22).Margin(-2)
		ui.Text(c, locales.GuideTitle()).FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).InkMuted)
	})
}

// steps is the step, its words and controls on the left and its stage on the right, top below the
// title bar.
func (g *Guide) steps(c *ui.Context, top float32) {
	ui.Row(c).Grow(1).Padding(top, kit.Space(4), kit.Space(4)).Gap(kit.Space(4)).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(leftWidth).Padding(kit.Space(6), leftPadR, kit.Space(1), leftPadL).Gap(kit.Space(4)).Children(func() {
			g.progress(c)
			// The step in the middle of its room; one taller than it, as in a long language,
			// scrolls rather than run into the buttons under it.
			ui.Column(c).Grow(1).MinHeight(0).Justify(ui.Center).Children(func() {
				// Cut at its foot, it fades there, so that it reads as going on.
				kit.FadeScroll(c, &g.scroll, func() {
					// The step's room stays from step to step, for its parts (part) to come into it;
					// under it, room for them to rise from.
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

// holdKeys holds the shortcut a step checks, for pressing it to only light its keys: starting and
// stopping's on its step, the picture's on its step until it is seen working, after which it
// selects.
func (g *Guide) holdKeys() {
	k := g.core
	if g.Step != g.keysStep || g.keysSince.IsZero() {
		g.keysStep, g.keysSince = g.Step, time.Now()
	}
	k.Held[core.StartStop] = g.Step == stepShortcuts
	k.Held[core.PictureShortcut] = g.Step == stepPicture && k.CanSelect() && !g.works(core.PictureShortcut)
}

// works tells whether shortcut s was pressed since the step showed.
func (g *Guide) works(s core.Shortcut) bool { return g.core.PressedSince(s, g.keysSince) }

// skipped tells whether the guide leaves step s out on this system: Windows asks no permission to
// hear what plays.
func skipped(s int) bool { return s == stepPermission && !core.AsksAccess() }

// after is the step dir steps after s (-1, before it) that the guide has.
func after(s, dir int) int {
	s += dir
	for skipped(s) {
		s += dir
	}
	return s
}

// goTo goes to step s: the frame after this one builds the step leaving once more, going the
// right way, then the next.
func (g *Guide) goTo(c *ui.Context, s int) {
	g.next = s
	if !g.core.Preferences().Onboarded {
		g.core.UpdatePrefs(func(p *prefs.Prefs) { p.GuideStep = s })
	}
	c.AnimationFrame()
}

// progress is where the guide is: the phases, the one it is in in ink, and under them a bar that
// fills step by step, in the lantern's amber.
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

// The left column: its width, its padding at either side, and the progress bar's width within it.
const leftWidth = 352

var (
	leftPadL, leftPadR = kit.Space(4), kit.Space(4)
	progressWidth      = leftWidth - leftPadL - leftPadR
)

// stepButton is the step's main button: on to the next step, or what the step waits for (allowing
// the permission, say).
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
			// The system asks only when the engine first records.
			if kit.Off(c, next(c, locales.Allow()), k.Asking || k.Access == "").Clicked() {
				k.RequestAccess()
			}
		}
	case stepRecognition:
		// Without the system's recognition, one is chosen first; a model may still be downloading.
		if kit.Off(c, next(c, locales.Continue()), k.Chosen(core.ASR) == "").Clicked() {
			g.goTo(c, stepTranslation)
		}
	case stepTranslation:
		// The cloud is chosen to begin with; a model chosen may still be downloading.
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
		// Selecting is the user's to want: until it is on, the step can be skipped.
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
			b.Shadow(0, 8, 24+20*v, 0, kit.Colors(c).Lantern.Alpha(.45+.35*v)) // the primary's, glowing
		}
		if b.Clicked() {
			k.StopSpeaking()
			clear(k.Held)
			k.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded, p.GuideStep = true, 0 })
			g.nav.FinishGuide()
		}
	}
}

// next is a step's button on: its main action.
func next(c *ui.Context, label string) ui.Element {
	return kit.Button(c, kit.Primary, kit.Medium, nil, label)
}

// part is a part of the step, the i-th from its top, its things gap apart as the step's are: it
// rises in after those above it as the step comes, the first as the window opens too; the step
// before goes at once, for the two not to show over each other. It rises, not slides: its room
// scrolls, and would cut it at its side.
func (g *Guide) part(c *ui.Context, i int, fn func()) ui.Element {
	return kit.Arrive(ui.Column(c.Key(fmt.Sprintf("%d.%d", g.Step, i))).Gap(kit.Space(4)), i).Children(fn)
}

// title and body are a step's heading and what it says under it.
func title(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(kit.Colors(c).Ink)
}

func body(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(kit.TextBody).TextColor(kit.Colors(c).InkMuted).LineHeight(1.45)
}

// note is a line under a step's controls, in muted ink.
func note(c *ui.Context, s string) ui.Element { return kit.Caption(c, s).LineHeight(1.4) }
