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

	// The paper and its layers, from deepest: sand sinks into the paper, a card stands on it.
	Paper ui.Color // a page
	Card  ui.Color // a card on it, and fields
	Sand  ui.Color // a quieter card, and list items
	Fill  ui.Color // the track of pickers, segmented controls and steppers
	SegOn ui.Color // the chosen segment, raised on its track
	// Lines: a card's outline, the rules inside a card, and the outline of a plain button.
	Line, LineSoft, LineStrong ui.Color
	// Raised is the face of a plain button, RaisedLine its outline: a step above the card it is on.
	Raised, RaisedLine ui.Color

	// Ink: text and icons on paper. InkWarm is for times and numbers on sand; Faint is for what
	// is not there yet or can't be used: a field's placeholder, a disabled button's label.
	Ink, InkMuted, InkWarm, Faint ui.Color

	// The drawings of empty pages (Empty): the hatching of their ground, what is drawn in pencil
	// (icons, a name in a line) and the shadow under what stands on the ground.
	Hatch, Pencil, Lift ui.Color

	// The lantern: the one main action of a screen, what is chosen, switches that are on.
	Lantern, LanternHover, LanternPressed ui.Color
	LanternInk                            ui.Color // text on the lantern
	LanternTint                           ui.Color // the ground of icons and hints
	LanternText                           ui.Color // amber text and links on paper
	// Amber is the lantern's ramp, opaque, from nothing (Amber[0]) to the most (Amber[5]): a
	// heatmap's days, a chart's bars, what is dimmed beside the one chosen. In dark it warms from
	// the indigo through mauve, never through brown.
	Amber [6]ui.Color
	// Band is the ground of the option chosen in a list; DotRing a radio's ring, SwitchOff a
	// switch's track while off.
	Band, DotRing, SwitchOff ui.Color

	// Strong is the face of a firm action's button and a chosen chip, StrongInk its label.
	Strong, StrongInk ui.Color

	// States: in use and ready (and the On device tag), the Cloud tag, deleting; a close button
	// hovered, the red of title bars', the same in either theme, under Starlight.
	Ok, OkText, OkTint   ui.Color
	CloudText, CloudTint ui.Color
	Danger, CloseHover   ui.Color

	// Scrim dims the window under a dialog: the night's indigo, not grey.
	Scrim ui.Color
	// The shadows: a card's, faint and warm (black in dark); a chosen segment's on its track; a
	// plain button's.
	CardShadow, SegShadow, RaisedShadow Shadow

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

// Shadow is a shadow cast by what is raised: its offset, blur, spread and color.
type Shadow struct {
	X, Y, Blur, Spread float32
	Color              ui.Color
}

// Cast casts s under e.
func (s Shadow) Cast(e ui.Element) ui.Element { return e.Shadow(s.X, s.Y, s.Blur, s.Spread, s.Color) }

// Light is the palette of a light appearance.
var Light = Palette{
	Night: ui.Hex("#1b1949"), NightLine: ui.Hex("#2c2970"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavOn:  ui.Hex("#fbf6ec"),
	Stage:  ui.Hex("#15133d"),
	SkyTop: ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#fbf6ec"), Card: ui.Hex("#ffffff"), Sand: ui.Hex("#f4ead6"), Fill: ui.Hex("#f1e5cb"), SegOn: ui.Hex("#ffffff"),
	Line: ui.Hex("#efe4cc"), LineSoft: ui.Hex("#f3ead8"), LineStrong: ui.Hex("#e6d8bb"),
	Raised: ui.Hex("#ffffff"), RaisedLine: ui.Hex("#e6d8bb"),

	Ink: ui.Hex("#232057"), InkMuted: ui.Hex("#6b6590"), InkWarm: ui.Hex("#73644a"), Faint: ui.Hex("#9d9ab9"),
	Hatch: ui.Hex("#efe4cc"), Pencil: ui.Hex("#aaa5c4"), Lift: ui.RGBA(58, 42, 16, .05),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#fff0d1"), LanternText: ui.Hex("#8a5200"),
	Amber: [6]ui.Color{ui.Hex("#f3ead8"), ui.Hex("#fbefd6"), ui.Hex("#fadda9"), ui.Hex("#fcc771"), ui.Hex("#f5a524"), ui.Hex("#d27908")},
	Band:  ui.Hex("#fff3d9"), DotRing: ui.Hex("#c9b997"), SwitchOff: ui.Hex("#d1c3a6"),

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
	Apple:  ui.Hex("#007aff"), Qwen: ui.Hex("#615ced"), Alibaba: ui.Hex("#ff6a00"),
	Bilibili: [3]ui.Color{ui.Hex("#fb7299"), ui.Hex("#d63f73"), ui.Hex("#fdb2c9")},
}

// Dark is the palette of a dark appearance: the night deepens, sand sinks below the paper and a
// card stands above it, each a step of lightness apart; nothing is the light palette turned over.
var Dark = Palette{
	Night: ui.Hex("#0e0a2f"), NightLine: ui.Hex("#2a2766"), NightText: ui.Hex("#d3cff6"), NightTitle: ui.Hex("#fbf3e2"),
	NavOn:  ui.Hex("#222049"),
	Stage:  ui.Hex("#0d0b2a"),
	SkyTop: ui.Hex("#13123a"), SkyBottom: ui.Hex("#33215c"),
	Glass: ui.RGBA(255, 255, 255, .08), GlassLine: ui.RGBA(255, 255, 255, .14),
	Starlight: ui.RGB(255, 255, 255), Shade: ui.RGB(0, 0, 0), KeyLit: ui.Hex("#4d8dff"),

	Paper: ui.Hex("#222049"), Card: ui.Hex("#312f61"), Sand: ui.Hex("#171537"), Fill: ui.Hex("#181639"), SegOn: ui.Hex("#4c4a88"),
	Line: ui.Hex("#414070"), LineSoft: ui.Hex("#3c3c65"), LineStrong: ui.Hex("#545489"),
	Raised: ui.Hex("#3d3b73"), RaisedLine: ui.Hex("#545489"),

	Ink: ui.Hex("#fbf3e2"), InkMuted: ui.Hex("#bbb9e0"), InkWarm: ui.Hex("#d9c9a3"), Faint: ui.Hex("#8784ac"),
	Hatch: ui.Hex("#28255a"), Pencil: ui.Hex("#7a74b0"), Lift: ui.RGBA(0, 0, 0, .14),

	Lantern: ui.Hex("#f5a524"), LanternHover: ui.Hex("#f8b445"), LanternPressed: ui.Hex("#e0931a"),
	LanternInk: ui.Hex("#232057"), LanternTint: ui.Hex("#603d6b"), LanternText: ui.Hex("#ffc760"),
	Amber: [6]ui.Color{ui.Hex("#181639"), ui.Hex("#53355c"), ui.Hex("#874463"), ui.Hex("#b95a4f"), ui.Hex("#de8035"), ui.Hex("#f5a524")},
	Band:  ui.Hex("#4c3155"), DotRing: ui.Hex("#626396"), SwitchOff: ui.Hex("#555681"),

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
	// MyGo draws a field's placeholder in TextMuted; kit draws its own muted text in InkMuted.
	t.Border, t.Text, t.TextMuted = p.Line, p.Ink, p.Faint
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.Lantern, p.LanternHover, p.LanternPressed, p.LanternInk
	t.Danger, t.Success = p.Danger, p.Ok
	t.Focus = p.Lantern.Alpha(.6)
	t.Selection = p.Lantern.Alpha(.3)
	t.Scrollbar = p.Ink.Alpha(.22)
	c.SetTheme(&t)
}

// The type scale, in points: eight steps, each named for its use. A size between them is a ninth
// step to add here, not arithmetic on these.
const (
	TextDisplay = 32 // a page's title
	TextStat    = 26 // a number of the run's
	TextTitle   = 22 // the title of what a page shows: a scene, a record; a dialog's, the guide's
	TextSection = 15 // a group's title; a large button's label
	TextBody    = 14 // body text, a setting's and an option's name
	TextControl = 13 // buttons, segments, pickers, chips, tables
	TextCaption = 12 // notes, states, paths
	TextMicro   = 11 // tags, badges, a chart's ticks

	WeightMedium = 600
	WeightBold   = 700
	WeightHeavy  = 800
)

// Tabular is the font feature that gives digits one width, for numbers that change in place.
const Tabular = "tnum"

// Space is the step of paddings and gaps, whole steps of 4: Space(4) is 16. Inside a label, 4
// (Space(1)) parts a name from the note under it; 8 parts buttons; 12 the parts of a row; 16
// the cards (CardGap) and a card's top and bottom; 20 a card's sides; 24 a page's parts; 32 its
// margins.
func Space(n int) float32 { return 4 * float32(n) }

// The gaps of a page: between cards, across and down alike; between its parts, a title and the
// groups under it; and a group's title above its card.
const (
	CardGap    = 16
	SectionGap = 24
	GroupGap   = 8
)

// Corners. One inside another is the outer less the padding between them: a segment (10) in its
// track (14) 4 in, a row (14) in a list's card (20) 6 in.
const (
	RadiusFrame = 18 // the window's frame
	RadiusCard  = 20 // cards, the stage, dialogs
	RadiusTile  = 16 // tiles, list items
	RadiusField = 14 // fields, tracks, pickers; the rows in a list's card
	RadiusPaper = 14 // the paper inside the frame
	RadiusInner = 10 // a segment, a stepper's button, a small icon's ground, a table's cell
	RadiusMark  = 4  // a chart's bar and its tip, a mark in a line of text
)

// Pill rounds a box of height h into a pill, or a circle if it is as wide.
func Pill(h float32) float32 { return h / 2 }

// Icon sizes.
const (
	IconSizeSmall  = 14 // in a small button
	IconSizeButton = 15 // in a page's header's buttons
	IconSizeMenu   = 16 // a menu row's, a group's icon on its tint
	IconSizeNav    = 17
	IconSizeTile   = 18 // a tile's, a large button's
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

// Dim is col faded beside what is chosen, still opaque: the lantern's amber two steps down its
// ramp, any other color most of the way to the card.
func (p *Palette) Dim(col ui.Color) ui.Color {
	for i, a := range p.Amber {
		if i > 0 && a == col {
			return p.Amber[max(1, i-2)]
		}
	}
	return col.Mix(p.Card, .55)
}
