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

const (
	afterTitle    = 8
	betweenTips   = 2
	tipLineHeight = 1.4
	alignedToTint = (tintSize - kit.TextBody*tipLineHeight) / 2
)

func (g *Guide) ready(c *ui.Context) {
	pal := kit.Colors(c)
	g.part(c, 0, func() { title(c, locales.GuideReadyTitle()) })
	tip := func(i int, lead func(), s string) {
		gap := float32(betweenTips)
		if i == 1 {
			gap = afterTitle
		}
		g.part(c, i, func() {
			ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Start).Margin(gap, 0, 0, 0).Children(func() {
				tint(c, tintSize, lead)
				ui.Text(c, s).FontSize(kit.TextBody).TextColor(pal.Ink).LineHeight(tipLineHeight).Grow(1).Margin(alignedToTint, 0, 0, 0)
			})
		})
	}
	tip(1, func() { template(c, assets.Tray, 16, pal.LanternText) }, shared.OnEach(locales.GuideReadyMenuBar(), locales.GuideReadyTray(), locales.GuideReadyLinux()))
	tip(2, func() { ui.Icon(c, assets.Pointer).FontSize(kit.IconSizeMenu).TextColor(pal.LanternText) }, locales.GuideReadySubtitles())
}

func (g *Guide) readyStage(c *ui.Context) {
	p := g.core.Preferences()
	pal := kit.Colors(c)
	still := c.Preferences().ReduceMotion
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
	bar := func() {
		ui.Row(c).Height(26).Padding(0, kit.Space(3)).Gap(kit.Space(4)).AlignItems(ui.Center).Justify(ui.End).Background(pal.Shade.Alpha(.28)).Children(func() {
			for range 3 {
				ui.Box(c).Size(14, 10).Radius(kit.Pill(6)).Background(pal.Starlight.Alpha(.28))
			}
			ui.Box(c).Size(22, 18).Center().Draw(func(p *ui.Painter, r ui.Rect) {
				if !awake {
					return
				}
				k := float32(.5)
				if !still {
					const period = 1800 * time.Millisecond
					t := p.Now().Sub(g.shownAt.Add(wakeAt)).Milliseconds()
					k = float32(t%period.Milliseconds()) / float32(period.Milliseconds())
					p.After(slowFrame)
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
		ui.Column(c).Padding(0, kit.Space(6), kit.Space(4)).Gap(kit.Space(2)).Children(func() {
			if !said {
				return
			}
			ui.Column(c.Key("said")).Gap(kit.Space(2)).Transition(kit.Rise(0)).Children(func() {
				subtitle.Draw(c, subtitle.Samples(p)[max(0, len(subtitle.Samples(p))-1):], p, .62, 340, subtitle.NoBlock)
			})
		})
		if shared.Windows {
			bar()
		}
	})
}

const (
	wakeAt  = 700 * time.Millisecond
	sayAt   = 1100 * time.Millisecond
	glowAt  = 1600 * time.Millisecond
	glowFor = 1000 * time.Millisecond
)

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

var recolor = shader.MustCompile(`package main

var Color vec4

func ColorEffect(pos vec2, color vec4) vec4 {
	return vec4(Color.rgb, 1) * Color.a * color.a
}
`)

func template(c *ui.Context, b *ui.Bitmap, size float32, col ui.Color) ui.Element {
	w, h := b.Size()
	return ui.Image(c, b).Size(size*float32(w)/float32(h), size).Effect(recolor.Effect(shader.Options{Uniforms: map[string]any{"Color": col}}))
}
