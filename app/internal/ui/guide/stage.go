package guide

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"tomoru/internal/core"
	"tomoru/internal/ui/kit"
)

var (
	night = shader.MustCompile(`package main

var Time float
var Top vec4
var Bottom vec4
var Glow vec4

func light(uv vec2, at vec2, aspect float, spread float) float {
	d := (uv - at) * vec2(aspect, 1)
	return exp(-dot(d, d) * spread)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	col := mix(Top.rgb, Bottom.rgb, uv.y)
	lantern := vec2(0.5+0.06*sin(Time*0.31), 0.98)
	col += Glow.rgb * (0.42 + 0.08*sin(Time*0.9)) * light(uv, lantern, aspect, 4.5)
	violet := vec2(0.28+0.18*sin(Time*0.17), 0.16+0.07*cos(Time*0.23))
	col += vec3(0.32, 0.22, 0.70) * 0.32 * light(uv, violet, aspect, 6.0)
	v := uv - 0.5
	col *= 1.0 - 0.55*dot(v, v)
	return vec4(col, 1)
}
`)
	nightStart = time.Now()
)

func (g *Guide) stage(c *ui.Context) {
	still := c.Preferences().ReduceMotion
	pal := kit.Colors(c)
	ui.Box(c).Grow(1).Radius(kit.RadiusTile).Clip().Gradient(pal.SkyTop, pal.SkyBottom, 180).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if !p.Accelerated() {
				return
			}
			t := float32(0)
			if !still {
				t = float32(p.Now().Sub(nightStart).Seconds())
				p.After(33 * time.Millisecond)
			}
			night.Draw(p, r, shader.Options{Radius: kit.RadiusTile, Uniforms: map[string]any{
				"Time": t, "Top": pal.SkyTop, "Bottom": pal.SkyBottom, "Glow": pal.Lantern,
			}})
		}).Children(func() {
		fade := ui.ElementTransition{Animation: &kit.Spring, Enter: &ui.Motion{Scale: .97}}
		key := g.Step
		if key == stepTranslation {
			key = stepRecognition
		}
		ui.Column(c.Key(key)).Absolute().Left(0).Right(0).Top(0).Bottom(0).Transition(fade).Children(func() {
			switch g.Step {
			case stepWelcome:
				g.welcomeStage(c)
			case stepPermission:
				g.permissionStage(c)
			case stepRecognition:
				g.enginesStage(c, core.ASR)
			case stepTranslation:
				g.enginesStage(c, core.MT)
			case stepTry:
				g.tryStage(c)
			case stepPicture:
				g.pictureStage(c)
			case stepShortcuts:
				g.shortcutsStage(c)
			case stepReady:
				g.readyStage(c)
			}
		})
	})
}

func glass(c *ui.Context, e ui.Element) ui.Element {
	p := kit.Colors(c)
	return e.Background(p.Glass).Border(1, p.GlassLine).Radius(kit.RadiusField)
}

func onStage(c *ui.Context, s string, size float32, alpha float32) ui.Element {
	return ui.Text(c, s).FontSize(size).TextColor(kit.Colors(c).Starlight.Alpha(alpha))
}

func tint(c *ui.Context, d float32, icon func()) ui.Element {
	return ui.Box(c).Size(d, d).Radius(kit.RadiusInner).Background(kit.Colors(c).Lantern.Alpha(.16)).Center().Shrink(0).Children(icon)
}
