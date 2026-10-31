package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The design tokens of docs/design-system.md: the colors, sizes, corners and motion the surfaces are
// drawn with. Surfaces take them from here, and write no color or size of their own
// (design_test.go keeps them to it).

// Palette is the colors, named for what they are for.
type Palette struct {
	// The night: the frame of the window, its sidebar, the panel's head.
	Night, NightLine, NightText, NightTitle ui.Color
	// NavOn is the ground of the sidebar's page shown: the paper it opens.
	NavOn ui.Color
	// Stage is the night sky that subtitles are previewed on.
	Stage ui.Color
	// The guide's sky, deeper at the top: what it shows is on it, in panes of glass, in starlight.
	SkyTop, SkyBottom ui.Color
	Glass, GlassLine  ui.Color
	Starlight         ui.Color // text and marks on the night, and on colored discs
	Shade             ui.Color // what darkens: a menu bar over the sky, a key's edge
	KeyLit            ui.Color // a key the guide saw pressed right, lit as a keyboard's backlight

	// The paper: the pages, and what is on them.
	Paper ui.Color // a page
	Card  ui.Color // a card on it, and fields
	Sand  ui.Color // a quieter card, and list items
	Fill  ui.Color // the ground of pickers and segmented controls
	// Lines: a card's outline, the rules inside a card, and the outline of a plain button.
	Line, LineSoft, LineStrong ui.Color

	// Ink: text and icons on paper. InkWarm is for times and numbers on sand.
	Ink, InkMuted, InkWarm ui.Color

	// The drawings of empty pages (Empty): the hatching of their ground, what is drawn in pencil
	// (icons, a name in a line) and the shadow under what stands on the ground.
	Hatch, Pencil, Lift ui.Color

	// The lantern: the one main action of a screen, what is chosen, switches that are on.
	Lantern, LanternHover, LanternPressed ui.Color
	LanternInk                            ui.Color // text on the lantern
	LanternTint                           ui.Color // the ground of icons and hints
	LanternText                           ui.Color // amber text and links on paper

	// States: in use and ready (and the On device tag), the Cloud tag, deleting; a close button
	// hovered, the red of title bars', the same in either theme, under Starlight.
	Ok, OkText, OkTint   ui.Color
	CloudText, CloudTint ui.Color
	Danger, CloseHover   ui.Color

	// The default subtitle colors on the stage, the scheme named Lantern.
	SubSrc, SubTgt ui.Color

	// Series tell apart a chart's series of no color of their own, as one's own APIs in the
	// statistics: the lantern's family, apart in lightness.
	Series [4]ui.Color
	// The engines' makers' colors, the engines' in charts: Apple's blue; Qwen's purple; Alibaba's
	// orange, SenseVoice's; and bilibili's pink, Index-Translate's: in the cloud, deeper for 9B,
	// paler for 2B.
	Apple, Qwen, Alibaba ui.Color
	Bilibili             [3]ui.Color
}

// Light is the palette of a light appearance.
var Light = Palette{
	Night: ui.Hex("#1b1949"), NightLine: ui.Hex("#2c2970"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavOn:  ui.Hex("#fbf6ec"),
	Stage:  ui.Hex("#15133d"),
	SkyTop: ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#fbf6ec"), Card: ui.Hex("#ffffff"), Sand: ui.Hex("#f4ead6"), Fill: ui.Hex("#f1e5cb"),
	Line: ui.Hex("#efe4cc"), LineSoft: ui.Hex("#f3ead8"), LineStrong: ui.Hex("#e6d8bb"),

	Ink: ui.Hex("#232057"), InkMuted: ui.Hex("#6b6590"), InkWarm: ui.Hex("#73644a"),
	Hatch: ui.Hex("#efe4cc"), Pencil: ui.Hex("#aaa5c4"), Lift: ui.RGBA(58, 42, 16, .05),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#fff0d1"), LanternText: ui.Hex("#8a5200"),

	Ok: ui.Hex("#3fa463"), OkText: ui.Hex("#2f6b45"), OkTint: ui.Hex("#e7f3ea"),
	CloudText: ui.Hex("#3b3592"), CloudTint: ui.Hex("#e8e6fb"),
	Danger: ui.Hex("#a4472f"), CloseHover: ui.Hex("#c42b1c"),

	SubSrc: ui.Hex("#f4f2ff"), SubTgt: ui.Hex("#f5a524"),

	Series: [4]ui.Color{ui.Hex("#f5a524"), ui.Hex("#8a5200"), ui.Hex("#e8cf9f"), ui.Hex("#4a3a24")},
	Apple:  ui.Hex("#007aff"), Qwen: ui.Hex("#615ced"), Alibaba: ui.Hex("#ff6a00"),
	Bilibili: [3]ui.Color{ui.Hex("#fb7299"), ui.Hex("#d63f73"), ui.Hex("#fdb2c9")},
}

// Dark is the palette of a dark appearance. Provisional: the night stays, the paper turns to a deep
// indigo, and its colors are to be settled once the light ones are in use (docs/design-system.md).
var Dark = Palette{
	Night: ui.Hex("#121033"), NightLine: ui.Hex("#2a2766"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavOn:  ui.Hex("#2f2b62"),
	Stage:  ui.Hex("#0d0b2a"),
	SkyTop: ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#1f1c45"), Card: ui.Hex("#2a2758"), Sand: ui.Hex("#25224f"), Fill: ui.Hex("#34306a"),
	Line: ui.Hex("#3a366f"), LineSoft: ui.Hex("#322e63"), LineStrong: ui.Hex("#46418a"),

	Ink: ui.Hex("#fbf3e2"), InkMuted: ui.Hex("#c3bef0"), InkWarm: ui.Hex("#d9c9a3"),
	Hatch: ui.Hex("#28255a"), Pencil: ui.Hex("#7a74b0"), Lift: ui.RGBA(0, 0, 0, .14),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#4a3a2a"), LanternText: ui.Hex("#ffc760"),

	Ok: ui.Hex("#4cbf75"), OkText: ui.Hex("#8be0a8"), OkTint: ui.Hex("#21412f"),
	CloudText: ui.Hex("#c9c4ff"), CloudTint: ui.Hex("#37327a"),
	Danger: ui.Hex("#ff8a6b"), CloseHover: ui.Hex("#c42b1c"),

	SubSrc: ui.Hex("#f4f2ff"), SubTgt: ui.Hex("#f5a524"),

	Series: [4]ui.Color{ui.Hex("#f5a524"), ui.Hex("#ffc760"), ui.Hex("#b8741a"), ui.Hex("#d9c9a3")},
	Apple:  ui.Hex("#0a84ff"), Qwen: ui.Hex("#8f8af7"), Alibaba: ui.Hex("#ff7a1a"),
	Bilibili: [3]ui.Color{ui.Hex("#fb7299"), ui.Hex("#ffa3c0"), ui.Hex("#c24d78")},
}

// Colors is the palette of the frame's appearance, light or dark.
func Colors(c *ui.Context) *Palette {
	if c.Theme().Dark {
		return &Dark
	}
	return &Light
}

// Paper sets the frame's theme to the palette, for MyGo's own widgets on the paper to match: a
// window's view calls it first.
func Paper(c *ui.Context) {
	p := Colors(c)
	t := *c.Theme()
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = p.Paper, p.Card, p.Fill, p.LineStrong
	t.Border, t.Text, t.TextMuted = p.Line, p.Ink, p.InkMuted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.Lantern, p.LanternHover, p.LanternPressed, p.LanternInk
	t.Danger, t.Success = p.Danger, p.Ok
	t.Focus = p.Lantern.Alpha(.6)
	t.Selection = p.Lantern.Alpha(.3)
	t.Scrollbar = p.Ink.Alpha(.22)
	c.SetTheme(&t)
}

// Text sizes, in points, and weights.
const (
	TextDisplay = 32 // a page's title
	TextStat    = 26 // a number of the run's
	TextTitle   = 22 // the title of what a page shows: a scene, a record
	TextSection = 15 // a card's title
	TextBody    = 14
	TextSmall   = 13 // buttons, chips, tables
	TextCaption = 12 // notes and states
	TextMicro   = 11 // tags, badges

	WeightMedium = 600
	WeightBold   = 700
	WeightHeavy  = 800
)

// Tabular is the font feature that gives digits one width, for numbers that change in place.
const Tabular = "tnum"

// Space is the step of paddings and gaps: Space(4) is 16.
func Space(n float32) float32 { return 4 * n }

// Corners.
const (
	RadiusFrame = 18 // the window's frame
	RadiusCard  = 20
	RadiusTile  = 16 // tiles, list items, option rows
	RadiusField = 14 // fields, segmented controls
	RadiusPaper = 14 // the paper inside the frame
	RadiusInner = 10 // a segment, a small icon's ground
)

// Pill rounds a box of height h into a pill, or a circle if it is as wide.
func Pill(h float32) float32 { return h / 2 }

// Icon sizes.
const (
	IconSizeNav    = 17
	IconSizeButton = 15
	IconSizeTile   = 18
)

// Motion: a spring for what moves or grows, a fade for what changes in place, a quick fade for
// what leaves before what replaces it comes, and Rise for what appears.
var (
	Spring    = ui.Animation{Spring: ui.SpringOptions{Duration: 460 * time.Millisecond, Bounce: .08}}
	Fade      = ui.Animation{Duration: 200 * time.Millisecond}
	FadeQuick = ui.Animation{Duration: 120 * time.Millisecond}
)

// Rising: how far what appears rises as it fades in, and how long each of a group waits after
// the one before it.
const (
	RiseBy  = 8
	Stagger = 40 * time.Millisecond
)

// Rise is how what appears comes in: from RiseBy below, transparent, on the Spring; the i-th of a
// group after i Staggers, so that they come in one after another.
func Rise(i int) ui.ElementTransition { return RiseAfter(time.Duration(i) * Stagger) }

// Arrive is Rise played as e is first built, the i-th of a group after i Staggers: for what comes
// with what holds it, a window opening or a step, where a transition, which needs a frame before
// to come in after, would not play.
func Arrive(e ui.Element, i int) ui.Element { return ArriveAfter(e, time.Duration(i)*Stagger) }

// ArriveAfter is Arrive after d, as what comes with a pane that takes its size first (Settle).
func ArriveAfter(e ui.Element, d time.Duration) ui.Element {
	k := e.Keyframes("arrive", []ui.Keyframe{{Value: 0}, {At: d + 420*time.Millisecond, Value: 1, Ease: ui.EaseOut}}, ui.KeyframeOptions{Delay: d})
	return e.Opacity(k).Translate(0, RiseBy*(1-k))
}

// RiseAfter is Rise after d, as text into a pane that takes its size for it first (Grow, Settle).
func RiseAfter(d time.Duration) ui.ElementTransition {
	a := Spring
	a.Delay = d
	return ui.ElementTransition{Animation: &a, Enter: &ui.Motion{Y: RiseBy}}
}

// Resize is the spring of a pane taking another size, quicker than Spring, so that what comes into
// it, after Settle, never meets its edge; Settle is about when it is there.
var Resize = ui.Animation{Spring: ui.SpringOptions{Duration: 340 * time.Millisecond}}

const Settle = 200 * time.Millisecond

// Grow is how what changes its size goes, as a card whose text takes another line: on Resize, and
// moving there, as its siblings do when it grows.
func Grow() ui.ElementTransition {
	return ui.ElementTransition{Animation: &Resize, Size: true, Position: true}
}
