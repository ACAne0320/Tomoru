package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type Palette struct {
	Night, NightText, NightTitle ui.Color
	NavLine                      ui.Color
	Stage                        ui.Color
	SkyTop, SkyBottom            ui.Color
	Glass, GlassLine             ui.Color
	Starlight                    ui.Color
	Shade                        ui.Color
	KeyLit                       ui.Color

	Paper                      ui.Color
	Card                       ui.Color
	Sand                       ui.Color
	Fill                       ui.Color
	SegOn                      ui.Color
	Line, LineSoft, LineStrong ui.Color
	EmptyLine                  ui.Color
	Raised, RaisedLine         ui.Color

	Ink, InkMuted, InkWarm, Faint ui.Color

	Hatch, Pencil, Lift ui.Color

	Lantern, LanternHover, LanternPressed ui.Color
	LanternInk                            ui.Color
	LanternTint                           ui.Color
	LanternText                           ui.Color
	Amber                                 [6]ui.Color
	Band, Shown, DotRing, SwitchOff       ui.Color

	Strong, StrongInk ui.Color

	Ok, OkText, OkTint   ui.Color
	CloudText, CloudTint ui.Color
	Danger, CloseHover   ui.Color

	Scrim                               ui.Color
	CardShadow, SegShadow, RaisedShadow Shadow

	SubSrc, SubTgt ui.Color

	Series                         [4]ui.Color
	Apple, Qwen, QwenPale, Alibaba ui.Color
	Bilibili                       [3]ui.Color
}

type Shadow struct {
	X, Y, Blur, Spread float32
	Color              ui.Color
}

func (s Shadow) Cast(e ui.Element) ui.Element { return e.Shadow(s.X, s.Y, s.Blur, s.Spread, s.Color) }

var Light = Palette{
	Night: ui.Hex("#1b1949"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavLine: ui.Hex("#efe4cc"),
	Stage:   ui.Hex("#15133d"),
	SkyTop:  ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#fbf6ec"), Card: ui.Hex("#ffffff"), Sand: ui.Hex("#f4ead6"), Fill: ui.Hex("#f1e5cb"), SegOn: ui.Hex("#ffffff"),
	Line: ui.Hex("#efe4cc"), LineSoft: ui.Hex("#f3ead8"), LineStrong: ui.Hex("#e6d8bb"),
	EmptyLine: ui.Hex("#d5c5a3"),
	Raised:    ui.Hex("#ffffff"), RaisedLine: ui.Hex("#e6d8bb"),

	Ink: ui.Hex("#232057"), InkMuted: ui.Hex("#6b6590"), InkWarm: ui.Hex("#73644a"), Faint: ui.Hex("#9d9ab9"),
	Hatch: ui.Hex("#efe4cc"), Pencil: ui.Hex("#aaa5c4"), Lift: ui.RGBA(58, 42, 16, .05),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#fff0d1"), LanternText: ui.Hex("#8a5200"),
	Amber: [6]ui.Color{ui.Hex("#f3ead8"), ui.Hex("#fbefd6"), ui.Hex("#fadda9"), ui.Hex("#fcc771"), ui.Hex("#f5a524"), ui.Hex("#d27908")},
	Band:  ui.Hex("#fff3d9"), Shown: ui.Hex("#f4ead6"), DotRing: ui.Hex("#c9b997"), SwitchOff: ui.Hex("#d1c3a6"),

	Strong: ui.Hex("#232057"), StrongInk: ui.Hex("#fbf6ec"),

	Ok: ui.Hex("#3fa463"), OkText: ui.Hex("#2f6b45"), OkTint: ui.Hex("#e7f3ea"),
	CloudText: ui.Hex("#3b3592"), CloudTint: ui.Hex("#e8e6fb"),
	Danger: ui.Hex("#a4472f"), CloseHover: ui.Hex("#c42b1c"),

	Scrim:        ui.RGBA(27, 25, 73, .42),
	CardShadow:   Shadow{0, 10, 24, -14, ui.RGBA(58, 42, 16, .22)},
	SegShadow:    Shadow{0, 1, 3, 0, ui.RGBA(80, 60, 20, .18)},
	RaisedShadow: Shadow{0, 1, 2, 0, ui.RGBA(40, 30, 10, .08)},

	SubSrc: ui.Hex("#f4f2ff"), SubTgt: ui.Hex("#f5a524"),

	Series: [4]ui.Color{ui.Hex("#f5a524"), ui.Hex("#8a5200"), ui.Hex("#e8cf9f"), ui.Hex("#4a3a24")},
	Apple:  ui.Hex("#007aff"), Qwen: ui.Hex("#615ced"), QwenPale: ui.Hex("#aeabf6"), Alibaba: ui.Hex("#ff6a00"),
	Bilibili: [3]ui.Color{ui.Hex("#fb7299"), ui.Hex("#d63f73"), ui.Hex("#fdb2c9")},
}

var Dark = Palette{
	Night: ui.Hex("#0e0a2f"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavLine: ui.Hex("#2a2766"),
	Stage:   ui.Hex("#0d0b2a"),
	SkyTop:  ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#222049"), Card: ui.Hex("#312f61"), Sand: ui.Hex("#171537"), Fill: ui.RGBA(255, 255, 255, .07), SegOn: ui.Hex("#4c4a88"),
	Line: ui.Hex("#414070"), LineSoft: ui.Hex("#3c3c65"), LineStrong: ui.Hex("#545489"),
	EmptyLine: ui.Hex("#545489"),
	Raised:    ui.Hex("#3d3b73"), RaisedLine: ui.Hex("#545489"),

	Ink: ui.Hex("#fbf3e2"), InkMuted: ui.Hex("#bbb9e0"), InkWarm: ui.Hex("#d9c9a3"), Faint: ui.Hex("#8784ac"),
	Hatch: ui.Hex("#28255a"), Pencil: ui.Hex("#7a74b0"), Lift: ui.RGBA(0, 0, 0, .14),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#603d6b"), LanternText: ui.Hex("#ffc760"),
	Amber: [6]ui.Color{ui.RGBA(255, 255, 255, .06), ui.Hex("#53355c"), ui.Hex("#874463"), ui.Hex("#b95a4f"), ui.Hex("#de8035"), ui.Hex("#f5a524")},
	Band:  ui.Hex("#46447f"), Shown: ui.Hex("#3c3c65"), DotRing: ui.Hex("#626396"), SwitchOff: ui.Hex("#555681"),

	Strong: ui.Hex("#5455b6"), StrongInk: ui.Hex("#fbf3e2"),

	Ok: ui.Hex("#4cbf75"), OkText: ui.Hex("#8be0a8"), OkTint: ui.Hex("#21412f"),
	CloudText: ui.Hex("#c9c4ff"), CloudTint: ui.Hex("#37327a"),
	Danger: ui.Hex("#ff8a6b"), CloseHover: ui.Hex("#c42b1c"),

	Scrim:        ui.RGBA(7, 5, 26, .6),
	CardShadow:   Shadow{0, 14, 28, -16, ui.RGBA(0, 0, 0, .7)},
	SegShadow:    Shadow{0, 1, 2, 0, ui.RGBA(0, 0, 0, .45)},
	RaisedShadow: Shadow{0, 1, 2, 0, ui.RGBA(0, 0, 0, .2)},

	SubSrc: ui.Hex("#f4f2ff"), SubTgt: ui.Hex("#f5a524"),

	Series: [4]ui.Color{ui.Hex("#f5a524"), ui.Hex("#ffc760"), ui.Hex("#b8741a"), ui.Hex("#d9c9a3")},
	Apple:  ui.Hex("#0a84ff"), Qwen: ui.Hex("#8f8af7"), QwenPale: ui.Hex("#5d58b8"), Alibaba: ui.Hex("#ff7a1a"),
	Bilibili: [3]ui.Color{ui.Hex("#fb7299"), ui.Hex("#ffa3c0"), ui.Hex("#c24d78")},
}

type themeSurfaceKey struct{}

type themeInkKey struct{}

type themeCacheKey struct{}

type themeCache struct {
	surface, ink float32
	p            Palette
}

func themeSteps(c *ui.Context) (surface, ink float32) {
	target := b2f(c.Theme().Dark)
	root := c.Root()
	return root.AnimateTo(themeSurfaceKey{}, target, themeSurfaceFade),
		root.AnimateTo(themeInkKey{}, target, themeInkFade)
}

func Colors(c *ui.Context) *Palette {
	surface, ink := themeSteps(c)
	switch {
	case surface <= 0 && ink <= 0:
		return &Light
	case surface >= 1 && ink >= 1:
		return &Dark
	}
	cache := ui.Local(c.Root(), themeCacheKey{}, func() themeCache { return themeCache{surface: -1, ink: -1} })
	if cache.surface != surface || cache.ink != ink {
		cache.surface, cache.ink = surface, ink
		cache.p = mixPalette(&Light, &Dark, surface, ink)
	}
	return &cache.p
}

type darkColorKey struct{ key any }

func AnimateColor(c *ui.Context, e ui.Element, key any, a ui.Animation, pick func(*Palette) ui.Color) ui.Color {
	l := e.AnimateColor(key, pick(&Light), a)
	d := e.AnimateColor(darkColorKey{key}, pick(&Dark), a)
	t, _ := themeSteps(c)
	switch {
	case t <= 0:
		return l
	case t >= 1:
		return d
	}
	return mixColor(l, d, t)
}

var duskTint = ui.Hex("#634168")

func mixColor(a, b ui.Color, t float32) ui.Color {
	if a == b || t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return a.MixOklab(b, t)
}

func mixDusk(a, b ui.Color, t float32) ui.Color {
	col := mixColor(a, b, t)
	if a == b || t <= 0 || t >= 1 {
		return col
	}
	bell := 4 * t * (1 - t)
	return col.MixOklab(duskTint, .1*bell)
}

func mixShadow(a, b Shadow, t float32) Shadow {
	lerp := func(x, y float32) float32 { return x + (y-x)*t }
	return Shadow{
		X:      lerp(a.X, b.X),
		Y:      lerp(a.Y, b.Y),
		Blur:   lerp(a.Blur, b.Blur),
		Spread: lerp(a.Spread, b.Spread),
		Color:  mixColor(a.Color, b.Color, t),
	}
}

func mixPalette(a, b *Palette, surface, ink float32) Palette {
	ms := func(x, y ui.Color) ui.Color { return mixColor(x, y, surface) }
	md := func(x, y ui.Color) ui.Color { return mixDusk(x, y, surface) }
	mi := func(x, y ui.Color) ui.Color { return mixColor(x, y, ink) }
	p := Palette{
		Night: ms(a.Night, b.Night), NightText: mi(a.NightText, b.NightText), NightTitle: mi(a.NightTitle, b.NightTitle),
		NavLine: md(a.NavLine, b.NavLine),
		Stage:   ms(a.Stage, b.Stage),
		SkyTop:  ms(a.SkyTop, b.SkyTop), SkyBottom: ms(a.SkyBottom, b.SkyBottom),
		Glass: ms(a.Glass, b.Glass), GlassLine: ms(a.GlassLine, b.GlassLine),
		Starlight: ms(a.Starlight, b.Starlight), Shade: ms(a.Shade, b.Shade), KeyLit: ms(a.KeyLit, b.KeyLit),

		Paper: md(a.Paper, b.Paper), Card: md(a.Card, b.Card), Sand: md(a.Sand, b.Sand), Fill: ms(a.Fill, b.Fill), SegOn: md(a.SegOn, b.SegOn),
		Line: md(a.Line, b.Line), LineSoft: md(a.LineSoft, b.LineSoft), LineStrong: md(a.LineStrong, b.LineStrong),
		EmptyLine: md(a.EmptyLine, b.EmptyLine),
		Raised:    md(a.Raised, b.Raised), RaisedLine: md(a.RaisedLine, b.RaisedLine),

		Ink: mi(a.Ink, b.Ink), InkMuted: mi(a.InkMuted, b.InkMuted), InkWarm: mi(a.InkWarm, b.InkWarm), Faint: mi(a.Faint, b.Faint),
		Hatch: md(a.Hatch, b.Hatch), Pencil: mi(a.Pencil, b.Pencil), Lift: ms(a.Lift, b.Lift),

		Lantern: ms(a.Lantern, b.Lantern), LanternHover: ms(a.LanternHover, b.LanternHover), LanternPressed: ms(a.LanternPressed, b.LanternPressed),
		LanternInk: mi(a.LanternInk, b.LanternInk), LanternTint: ms(a.LanternTint, b.LanternTint), LanternText: mi(a.LanternText, b.LanternText),
		Band: md(a.Band, b.Band), Shown: md(a.Shown, b.Shown), DotRing: ms(a.DotRing, b.DotRing), SwitchOff: ms(a.SwitchOff, b.SwitchOff),

		Strong: ms(a.Strong, b.Strong), StrongInk: mi(a.StrongInk, b.StrongInk),

		Ok: ms(a.Ok, b.Ok), OkText: mi(a.OkText, b.OkText), OkTint: ms(a.OkTint, b.OkTint),
		CloudText: mi(a.CloudText, b.CloudText), CloudTint: ms(a.CloudTint, b.CloudTint),
		Danger: mi(a.Danger, b.Danger), CloseHover: ms(a.CloseHover, b.CloseHover),

		Scrim:        ms(a.Scrim, b.Scrim),
		CardShadow:   mixShadow(a.CardShadow, b.CardShadow, surface),
		SegShadow:    mixShadow(a.SegShadow, b.SegShadow, surface),
		RaisedShadow: mixShadow(a.RaisedShadow, b.RaisedShadow, surface),

		SubSrc: ms(a.SubSrc, b.SubSrc), SubTgt: ms(a.SubTgt, b.SubTgt),

		Apple: ms(a.Apple, b.Apple), Qwen: ms(a.Qwen, b.Qwen), QwenPale: ms(a.QwenPale, b.QwenPale), Alibaba: ms(a.Alibaba, b.Alibaba),
	}
	for i := range p.Amber {
		p.Amber[i] = ms(a.Amber[i], b.Amber[i])
	}
	for i := range p.Series {
		p.Series[i] = ms(a.Series[i], b.Series[i])
	}
	for i := range p.Bilibili {
		p.Bilibili[i] = ms(a.Bilibili[i], b.Bilibili[i])
	}
	return p
}

func Paper(c *ui.Context) {
	p := Colors(c)
	t := *c.Theme()
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = p.Paper, p.Card, p.Fill, p.LineStrong
	t.Border, t.Text, t.TextMuted = p.Line, p.Ink, p.Faint
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.Lantern, p.LanternHover, p.LanternPressed, p.LanternInk
	t.Danger, t.Success = p.Danger, p.Ok
	t.Focus = p.Lantern.Alpha(.6)
	t.Selection = p.Lantern.Alpha(.3)
	t.Scrollbar = p.Ink.Alpha(.22)
	c.SetTheme(&t)
}

const (
	TextDisplay = 32
	TextStat    = 26
	TextTitle   = 22
	TextGroup   = 17
	TextSection = 15
	TextBody    = 14
	TextControl = 13
	TextCaption = 12
	TextMicro   = 11

	WeightMedium = 600
	WeightBold   = 700
	WeightHeavy  = 800
)

const Tabular = "tnum"

func Space(n int) float32 { return 4 * float32(n) }

const (
	CardGap    = 16
	SectionGap = 24
	GroupGap   = 8
)

const (
	RadiusFrame   = 18
	RadiusCard    = 20
	RadiusTile    = 16
	RadiusField   = 14
	RadiusSidebar = 14
	RadiusInner   = 10
	RadiusMark    = 4
)

func Pill(h float32) float32 { return h / 2 }

const (
	IconSizeSmall  = 14
	IconSizeButton = 15
	IconSizeMenu   = 16
	IconSizeNav    = 17
	IconSizeTile   = 18
)

var (
	Spring           = ui.Animation{Spring: ui.SpringOptions{Duration: 460 * time.Millisecond, Bounce: .08}}
	Fade             = ui.Animation{Duration: 200 * time.Millisecond}
	FadeQuick        = ui.Animation{Duration: 120 * time.Millisecond}
	themeSurfaceFade = ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.CubicBezier(.25, .1, .25, 1)}
	themeInkFade     = ui.Animation{Duration: 200 * time.Millisecond, Delay: 85 * time.Millisecond, Ease: ui.CubicBezier(0, 0, .2, 1)}
)

const (
	RiseBy  = 8
	Stagger = 40 * time.Millisecond
)

func Rise(i int) ui.ElementTransition { return RiseAfter(time.Duration(i) * Stagger) }

func Arrive(e ui.Element, i int) ui.Element { return ArriveAfter(e, time.Duration(i)*Stagger) }

func ArriveAfter(e ui.Element, d time.Duration) ui.Element {
	k := e.Keyframes("arrive", []ui.Keyframe{{Value: 0}, {At: d + 420*time.Millisecond, Value: 1, Ease: ui.EaseOut}}, ui.KeyframeOptions{Delay: d})
	return e.Opacity(k).Translate(0, RiseBy*(1-k))
}

func RiseAfter(d time.Duration) ui.ElementTransition {
	a := Spring
	a.Delay = d
	return ui.ElementTransition{Animation: &a, Enter: &ui.Motion{Y: RiseBy}}
}

var Resize = ui.Animation{Spring: ui.SpringOptions{Duration: 340 * time.Millisecond}}

const Settle = 200 * time.Millisecond

func Grow() ui.ElementTransition {
	return ui.ElementTransition{Animation: &Resize, Size: true, Position: true}
}

func (p *Palette) Dim(col ui.Color) ui.Color {
	for i, a := range p.Amber {
		if i > 0 && a == col {
			return p.Amber[max(1, i-2)]
		}
	}
	return col.Mix(p.Card, .55)
}
