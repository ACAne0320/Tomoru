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

func Colors(c *ui.Context) *Palette {
	if c.Theme().Dark {
		return &Dark
	}
	return &Light
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
	Spring    = ui.Animation{Spring: ui.SpringOptions{Duration: 460 * time.Millisecond, Bounce: .08}}
	Fade      = ui.Animation{Duration: 200 * time.Millisecond}
	FadeQuick = ui.Animation{Duration: 120 * time.Millisecond}
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
