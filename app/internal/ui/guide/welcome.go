package guide

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// welcome chooses the languages, which the stage's subtitles show at once.
func (g *Guide) welcome(c *ui.Context) {
	// What shows comes in one after another, as the window opens.
	g.part(c, 0, func() { ui.Image(c, assets.AppIcon).Size(76, 76).Margin(-6) })
	g.part(c, 1, func() {
		ui.Column(c).Gap(kit.Space(2)).Children(func() {
			title(c, locales.GuideTitle())
			body(c, locales.GuideWelcomeBody())
		})
	})
	g.part(c, 2, func() { g.languages(c) })
}

// language is a language's field, as one of the two: swapped, each goes to where the other was.
type language string

// languages are the two languages, and swapping them, its button turning half a turn each time.
func (g *Guide) languages(c *ui.Context) {
	k := g.core
	ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
		ui.Column(c).Grow(1).Gap(kit.Space(2.5)).Children(func() {
			lang := func(label, value string, options []string, choose func(string)) {
				ui.Column(c).Gap(kit.Space(1)).Children(func() {
					kit.Caption(c, label)
					kit.PickerField(c.Key(language(value)), shared.LangName(value), kit.Choices(value, shared.LangChoices(options), shared.LangName, choose)).Label(label).
						Transition(ui.ElementTransition{Animation: &kit.Spring, Match: language(value)})
				})
			}
			lang(locales.SpokenLanguage(), k.Preferences().Src, k.SourceLangs(), k.SetSrc)
			lang(locales.TranslateInto(), k.Preferences().Tgt, k.TargetLangs(), k.SetTgt)
		})
		b := kit.IconButton(c, kit.Plain, kit.Small, assets.Swap, locales.SwapLanguages()).Margin(18, 0, 0, 0)
		b.Rotate(b.AnimateTo("turn", 90+180*float32(g.swaps), kit.Spring))
		if kit.Off(b, !k.CanSwap()).Clicked() {
			g.swaps++
			k.Swap()
		}
	})
}

// welcomeStage is a video playing, with subtitles in the languages chosen rolling in.
func (g *Guide) welcomeStage(c *ui.Context) {
	p := g.core.Preferences()
	entries, next := subtitle.Rolling(p, c.Now())
	c.After(next)
	ui.Column(c).Fill().Justify(ui.End).Padding(kit.Space(5), kit.Space(5.5), kit.Space(4.5)).Gap(kit.Space(3.5)).Children(func() {
		// In other languages, the subtitles come in anew.
		ui.Column(c.Key(p.Src + ">" + p.Tgt)).Gap(kit.Space(1.5)).Transition(kit.Rise(0)).Children(func() {
			subtitle.Draw(c, entries, p, .62, 340, subtitle.NoBlock)
		})
		scrubber(c)
	})
}

// scrubber is a video's progress, going on.
func scrubber(c *ui.Context) {
	still := c.Preferences().ReduceMotion
	light := kit.Colors(c).Starlight
	const h = 3
	ui.Box(c).Height(h).Radius(kit.Pill(h)).Background(light.Alpha(.18)).Draw(func(p *ui.Painter, r ui.Rect) {
		k := float32(.38)
		if !still {
			const period = 90 * time.Second
			k = .2 + .6*float32(p.Now().UnixMilli()%period.Milliseconds())/float32(period.Milliseconds())
			p.After(250 * time.Millisecond)
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * k, H: r.H}, light.Alpha(.75), kit.Pill(h))
	})
}
