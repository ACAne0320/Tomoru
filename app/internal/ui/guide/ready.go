package guide

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// ready is where the app is now, and what to do with it: the menu bar, the subtitles.
func (g *Guide) ready(c *ui.Context) {
	pal := kit.Colors(c)
	g.part(c, 0, func() { title(c, locales.GuideReadyTitle()) })
	// tip is a thing to know, after a lead: a part of its own, coming in after the one before.
	tip := func(i int, lead func(), s string) {
		gap := float32(2) // to make the parts' gap the tips' 16
		if i == 1 {
			gap = 8 // the first apart from the title
		}
		g.part(c, i, func() {
			ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Start).Margin(gap, 0, 0, 0).Children(func() {
				tint(c, 32, lead)
				ui.Text(c, s).FontSize(kit.TextBody).TextColor(pal.Ink).LineHeight(1.4).Grow(1).Margin(6, 0, 0, 0) // its first line beside the lead
			})
		})
	}
	tip(1, func() { template(c, assets.Tray, 16, pal.LanternText) }, shared.OnWindows(locales.GuideReadyMenuBar(), locales.GuideReadyTray()))
	tip(2, func() { ui.Icon(c, assets.Pointer).FontSize(kit.IconSizeButton + 1).TextColor(pal.LanternText) }, locales.GuideReadySubtitles())
}

// readyStage is the menu bar with the app's icon glowing in it, the app, and subtitles where they
// go; on Windows the taskbar, under them.
func (g *Guide) readyStage(c *ui.Context) {
	p := g.core.Preferences()
	pal := kit.Colors(c)
	still := c.Preferences().ReduceMotion
	// As it shows: the icon wakes, her eyes opening, then a subtitle comes, as translating would.
	since := c.Now().Sub(g.shownAt)
	awake, said := still || since >= wakeAt, still || since >= sayAt
	switch {
	case !awake:
		c.After(wakeAt - since)
	case !said:
		c.After(sayAt - since)
	}
	tray := assets.Tray
	if awake {
		tray = assets.TrayAwake
	}
	// bar is the menu bar, or on Windows the taskbar, its icons at its end.
	bar := func() {
		ui.Row(c).Height(26).Padding(0, kit.Space(3)).Gap(kit.Space(3.5)).AlignItems(ui.Center).Justify(ui.End).Background(pal.Shade.Alpha(.28)).Children(func() {
			for range 3 {
				ui.Box(c).Size(14, 10).Radius(kit.Pill(6)).Background(pal.Starlight.Alpha(.28))
			}
			ui.Box(c).Size(22, 18).Center().Draw(func(p *ui.Painter, r ui.Rect) {
				if !awake {
					return // a glow once awake
				}
				k := float32(.5)
				if !still {
					const period = 1800 * time.Millisecond
					t := p.Now().Sub(g.shownAt.Add(wakeAt)).Milliseconds()
					k = float32(t%period.Milliseconds()) / float32(period.Milliseconds())
					p.After(33 * time.Millisecond)
				}
				grow := 2 + 8*k
				p.Fill(ui.Rect{X: r.X - grow, Y: r.Y - grow, W: r.W + 2*grow, H: r.H + 2*grow}, pal.Lantern.Alpha(.45*(1-k)), r.W/2+grow)
			}).Children(func() {
				template(c, tray, 16, pal.Starlight).ContentTransition(ui.ContentTransition{})
			})
			onStage(c, "9:41", kit.TextCaption, .85).FontWeight(kit.WeightBold)
		})
	}
	ui.Column(c).Fill().Children(func() {
		if !shared.Windows {
			bar()
		}
		ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Image(c, assets.AppIcon).Size(128, 128)
		})
		ui.Column(c).Padding(0, kit.Space(5.5), kit.Space(4.5)).Gap(kit.Space(1.5)).Children(func() {
			if !said {
				return
			}
			ui.Column(c.Key("said")).Gap(kit.Space(1.5)).Transition(kit.Rise(0)).Children(func() {
				subtitle.Draw(c, subtitle.Samples(p)[max(0, len(subtitle.Samples(p))-1):], p, .62, 340, subtitle.NoBlock)
			})
		})
		if shared.Windows {
			bar()
		}
	})
}

// The last step's moments, after it shows: the icon wakes, a subtitle comes, and Done glows once,
// for glowFor.
const (
	wakeAt  = 700 * time.Millisecond
	sayAt   = 1100 * time.Millisecond
	glowAt  = 1600 * time.Millisecond
	glowFor = 1000 * time.Millisecond
)

// glow is how much Done glows now, since the step showed: up and down once, at glowAt.
func glow(c *ui.Context, since time.Duration) float32 {
	switch {
	case c.Preferences().ReduceMotion || since >= glowAt+glowFor:
		return 0
	case since < glowAt:
		c.After(glowAt - since)
		return 0
	}
	c.AnimationFrame()
	return float32(math.Sin(math.Pi * float64(since-glowAt) / float64(glowFor)))
}

// recolor paints what it draws in Color, keeping its alpha: a template image, as the menu bar
// icon, in a color.
var recolor = shader.MustCompile(`package main

var Color vec4

func ColorEffect(pos vec2, color vec4) vec4 {
	return vec4(Color.rgb, 1) * Color.a * color.a
}
`)

// template draws a template image, size DIPs tall, in col.
func template(c *ui.Context, b *ui.Bitmap, size float32, col ui.Color) ui.Element {
	w, h := b.Size()
	return ui.Image(c, b).Size(size*float32(w)/float32(h), size).Effect(recolor.Effect(shader.Options{Uniforms: map[string]any{"Color": col}}))
}
