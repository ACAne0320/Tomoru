package guide

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// shortcuts is the shortcut that starts and stops translating, set to the default, to change here
// or keep. Pressed here, it only lights its keys on the stage: it is seen working. The picture's is
// chosen on its step, with selecting.
func (g *Guide) shortcuts(c *ui.Context) {
	g.part(c, 0, func() {
		title(c, locales.GuideShortcutsTitle())
		body(c, locales.GuideShortcutsBody())
	})
	g.part(c, 1, func() {
		ui.Column(c).Margin(6, 0, 0, 0).Children(func() {
			shortcutRow(c, assets.Keyboard, locales.Shortcut(), func() {
				shared.ShortcutField(c, g.core, g.nav, core.StartStop, locales.Shortcut(), &g.keys)
				worked(c, g.works(core.StartStop))
			})
		})
	})
}

// worked says the shortcut was seen working, once it was pressed right.
func worked(c *ui.Context, ok bool) {
	pal := kit.Colors(c)
	if !ok {
		return
	}
	ui.Row(c.Key("worked")).Gap(kit.Space(1.5)).AlignItems(ui.Center).Transition(kit.Rise(0)).Children(func() {
		kit.InUse(c)
		ui.Text(c, locales.ShortcutWorks()).FontSize(kit.TextSmall).FontWeight(kit.WeightBold).TextColor(pal.OkText)
	})
}

// shortcutRow is a shortcut: its icon, its name, and its field.
func shortcutRow(c *ui.Context, icon *ui.SVG, name string, field func()) {
	pal := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Start).Children(func() {
		tint(c, 32, func() { ui.Icon(c, icon).FontSize(kit.IconSizeButton + 1).TextColor(pal.LanternText) })
		ui.Column(c).Grow(1).Gap(kit.Space(1.5)).Children(func() {
			ui.Text(c, name).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(pal.Ink)
			field()
		})
	})
}

// shortcutsStage is the shortcut's keys, pressed one after another until the user presses them,
// then lit.
func (g *Guide) shortcutsStage(c *ui.Context) {
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		keycaps(c, g.core.Preferences().Shortcut, g.works(core.StartStop))
	})
}

// keycaps are an accelerator's keys on glass: pressed one after another, again and again, to show
// them, until the user presses them. Each lights up, glowing as a keyboard's backlight, as the user
// holds it: a modifier as it is held, as the window sees it; the key as the shortcut fires, which
// takes it before the window does. Once pressed right (done), they all stay lit.
func keycaps(c *ui.Context, acc string, done bool) {
	pal := kit.Colors(c)
	held := c.Modifiers()
	ms := kit.ShortcutMods(acc)
	demo := !c.Preferences().ReduceMotion && !done && held == 0
	glow := ui.Animation{Duration: 160 * time.Millisecond}
	row := ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center)
	// Pressed right, a light sweeps across them once.
	sweep := float32(0)
	if done {
		sweep = 1
	}
	sweep = row.AnimateTo("sweep", sweep, ui.Animation{Duration: 900 * time.Millisecond, Ease: ui.EaseInOut})
	keys := kit.ShortcutKeys(acc)
	row.Children(func() {
		for i, key := range keys {
			lit := done || ms[i] != 0 && held&ms[i] != 0
			cap := glass(c, ui.Box(c.Key(i)).MinWidth(capWidth(key, ms[i])).Height(54).Padding(0, kit.Space(3)).Center().Clip())
			// Held, it goes down as a key does.
			down := float32(0)
			if lit && !done {
				down = 2
			}
			down = cap.AnimateTo("down", down, kit.FadeQuick)
			cap.Translate(0, down)
			if demo {
				at := time.Duration(i) * 180 * time.Millisecond
				y := cap.Keyframes("press", []ui.Keyframe{
					{Value: 0},
					{At: at + 600*time.Millisecond, Value: 0},
					{At: at + 700*time.Millisecond, Value: 3, Ease: ui.EaseOut},
					{At: 1700 * time.Millisecond, Value: 3},
					{At: 1900 * time.Millisecond, Value: 0, Ease: ui.EaseInOut},
					{At: 2600 * time.Millisecond, Value: 0},
				}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
				cap.Translate(0, y+down)
			}
			face, edge := pal.Glass, pal.GlassLine
			if lit {
				face, edge = pal.KeyLit.Alpha(.32), pal.KeyLit
				cap.Shadow(0, 0, 22, 0, cap.AnimateColor("shine", pal.KeyLit.Alpha(.75), glow)) // the backlight
			} else {
				cap.Shadow(0, 3, 0, 0, pal.Shade.Alpha(.35)) // the key's edge
			}
			// The sweep over each in turn: a soft band of light crossing it, the next as it leaves.
			k := min(max(sweep*float32(len(keys)+1)-float32(i), 0), 1.5) / 1.5
			cap.Background(cap.AnimateColor("face", face, glow)).Border(1.5, cap.AnimateColor("edge", edge, glow)).Children(func() {
				onStage(c, key, kit.TextTitle, .95).FontWeight(kit.WeightBold)
				if k > 0 && k < 1 {
					ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
						const band = 28
						x := r.X - band + (r.W+2*band)*k
						for j, a := range []float32{.1, .22, .4, .22, .1} {
							p.Fill(ui.Rect{X: x + float32(j-2)*band/5, Y: r.Y, W: band / 5, H: r.H}, pal.Starlight.Alpha(a), 0)
						}
					})
				}
			})
		}
	})
}

// capWidth is how wide a key is on the stage, as on a keyboard: a letter's is 54, Shift's about
// two letters', the other modifiers' and the wider keys' between; a longer name widens it more.
func capWidth(key string, m ui.Modifiers) float32 {
	u := float32(1)
	switch {
	case m == ui.Shift:
		u = 2
	case m != 0:
		u = 1.25
	case key == "Space":
		u = 3
	case key == "Enter" || key == "Backspace":
		u = 1.75
	case key == "Tab":
		u = 1.5
	}
	return 54 * u
}
