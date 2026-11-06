package kit

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Art is the drawing of a page with nothing in it: a small likeness of the page itself, on a
// hatched ground. The lantern marks the next step in it, if there is one: the button that takes it
// on the page, or what the user does outside Tomoru.
type Art int

const (
	// ArtScenes is the scenes' list with only the general scene, the new one's place marked.
	ArtScenes Art = iota
	// ArtHistoryOff is the history's switch, turned on, over an empty list.
	ArtHistoryOff
	// ArtHistory is a video playing, and the history its subtitles will be in.
	ArtHistory
	// ArtStats is a video playing, and the statistics it will count in.
	ArtStats
	// ArtTermsLearn is a session kept, its names learned into the empty glossary.
	ArtTermsLearn
	// ArtTerms is the empty glossary, a name to add by hand, and the history to come.
	ArtTerms
	artCount
)

// arts are the drawings parsed, once each, light and dark.
var arts [artCount][2]*ui.SVG

// Empty is a page's place while it has nothing to show: its drawing, a sentence that says what
// would be there and how it comes, and what action builds below them, a button, if the page has
// no button for it already.
func Empty(c *ui.Context, art Art, text string, action func()) ui.Element {
	dark := 0
	if c.Theme().Dark {
		dark = 1
	}
	if arts[art][dark] == nil {
		arts[art][dark] = ui.MustParseSVG([]byte(drawArt(art, Colors(c))))
	}
	return SandCard(c).AlignItems(ui.Center).Padding(Space(6), Space(5)).Children(func() {
		ui.Image(c, arts[art][dark]).Size(160, 120)
		Caption(c, text).TextAlign(ui.Center)
		if action != nil {
			action()
		}
	})
}

// drawArt writes art as an SVG in p's colors. MyGo draws no filters or patterns: shadows are
// rectangles a little lower, the hatching lines clipped to the ground.
func drawArt(art Art, p *Palette) string {
	d := &sketch{p: p}
	d.ground()
	switch art {
	case ArtScenes:
		d.object(16, 20, 128, 80, 11)
		d.add(`<rect x="24.5" y="28.5" width="39" height="17" rx="6" fill="%s" stroke="%s"/>`, hex(p.Sand), hex(p.LineStrong))
		d.bar(29, 32.5, 20, 4, p.LineStrong)
		d.bar(29, 39.5, 26, 3, p.Pencil)
		d.dashed(24, 50, 40, 18, 6)
		d.dashed(24, 72, 40, 18, 6)
		d.cell(70, 28, 66, 64, 8, p.Sand)
		d.bar(76, 34, 30, 5, p.LineStrong)
		d.bar(76, 45, 16, 4, p.LineStrong)
		d.icon(lucideArrow, 98, 47, 7, p.Pencil, 1.3)
		d.bar(104, 45, 16, 4, p.LineStrong)
		d.bar(76, 58, 22, 4, p.LineStrong)
		d.cell(76, 66, 54, 8, 4, p.Card)
		d.cell(76, 77, 54, 8, 4, p.Card)
		d.lantern(lucidePlus, 44, 59, 7)
	case ArtHistoryOff:
		d.object(18, 22, 124, 76, 11)
		d.bar(26, 31, 30, 6, p.LineStrong)
		d.add(`<rect x="86.5" y="27.5" width="47" height="14" rx="7" fill="%s" stroke="%s"/>`, hex(p.Card), hex(p.LineStrong))
		d.bar(93, 32.5, 20, 4, p.LineStrong)
		d.cell(117, 31, 12, 7, 3.5, p.Lantern)
		d.add(`<circle cx="125.5" cy="34.5" r="2.6" fill="%s"/>`, hex(p.Starlight))
		for _, y := range []float32{49, 63, 77} {
			d.cell(26, y, 40, 11, 5, p.Sand)
		}
		d.dashed(72, 49, 62, 39, 7)
		d.icon(lucideHistory, 103, 68.5, 10, p.Pencil, 1.3)
	case ArtHistory:
		d.player()
		d.object(102, 26, 44, 64, 10)
		d.dashed(108, 33, 32, 13, 4)
		d.cell(108, 50, 32, 13, 4, p.Sand)
		d.cell(108, 67, 32, 13, 4, p.Sand)
	case ArtStats:
		d.player()
		d.object(102, 26, 44, 64, 10)
		for _, x := range []float32{108, 125} {
			d.cell(x, 32, 15, 13, 4, p.Sand)
			d.bar(x+4, 36.5, 7, 3, p.LineStrong)
		}
		for i, h := range []float32{10, 17, 8, 22} {
			d.dashed(109+float32(i)*8, 82-h, 6, h, 2)
		}
		d.add(`<line x1="108" y1="82.5" x2="140" y2="82.5" stroke="%s" stroke-linecap="round"/>`, hex(p.LineStrong))
	case ArtTermsLearn:
		d.object(16, 28, 56, 60, 10)
		// Each line as heard and translated, a name in pencil in what was heard.
		for _, l := range []struct{ y, at, w float32 }{{36, 8, 14}, {52, 22, 10}, {68, 4, 12}} {
			d.bar(22, l.y, 40, 3, p.Fill)
			d.bar(22+l.at, l.y, l.w, 3, p.Pencil)
			d.bar(22, l.y+6, 30, 3.5, p.LineStrong)
		}
		d.object(96, 28, 50, 60, 10)
		d.bar(102, 35, 14, 4, p.LineStrong)
		d.bar(124, 35, 14, 4, p.LineStrong)
		d.add(`<line x1="102" y1="44.5" x2="140" y2="44.5" stroke="%s"/>`, hex(p.Line))
		for _, y := range []float32{50, 62, 74} {
			d.cell(102, y, 17, 8, 3, p.Sand)
			d.cell(123, y, 17, 8, 3, p.Sand)
		}
		d.lantern(lucideSparkles, 84, 58, 8)
	case ArtTerms:
		d.object(20, 24, 96, 72, 12)
		d.bar(28, 33, 20, 5, p.LineStrong)
		d.icon(lucideArrow, 58, 35.5, 8, p.Pencil, 1.3)
		d.bar(66, 33, 20, 5, p.LineStrong)
		d.add(`<line x1="28" y1="45.5" x2="108" y2="45.5" stroke="%s"/>`, hex(p.Line))
		for _, y := range []float32{51, 65, 79} {
			d.cell(28, y, 26, 10, 4, p.Sand)
			d.icon(lucideArrow, 58, y+5, 7, p.Pencil, 1.3)
			d.cell(64, y, 44, 10, 4, p.Sand)
		}
		d.dashed(124, 40, 22, 40, 8)
		d.icon(lucideHistory, 135, 60, 9, p.Pencil, 1.3)
		d.lantern(lucidePlus, 104, 35, 7)
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 120" width="160" height="120">` + d.b.String() + `</svg>`
}

// The Lucide icons drawn in the art (lucide-static v0.469.0, ISC; see assets/icons/LICENSE).
const (
	lucidePlay     = `<path d="M6 3 20 12 6 21Z"/>`
	lucidePlus     = `<path d="M5 12h14"/><path d="M12 5v14"/>`
	lucideArrow    = `<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>`
	lucideHistory  = `<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M12 7v5l4 2"/>`
	lucideSparkles = `<path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"/><path d="M20 3v4"/><path d="M22 5h-4"/>`
)

// sketch is an SVG being drawn in a palette's colors, on the 160×120 ground.
type sketch struct {
	b strings.Builder
	p *Palette
}

func (d *sketch) add(format string, args ...any) { fmt.Fprintf(&d.b, format, args...) }

// ground is the hatched ground the art stands on.
func (d *sketch) ground() {
	p := d.p
	d.add(`<clipPath id="ground"><rect width="160" height="120" rx="18"/></clipPath>`)
	d.add(`<rect width="160" height="120" rx="18" fill="%s"/>`, hex(p.Paper))
	d.add(`<g clip-path="url(#ground)" stroke="%s" stroke-width="1">`, hex(p.Hatch))
	for x := -120; x <= 160; x += 7 {
		d.add(`<line x1="%d" y1="120" x2="%d" y2="0"/>`, x, x+120)
	}
	d.add(`</g><rect x="0.5" y="0.5" width="159" height="119" rx="17.5" fill="none" stroke="%s"/>`, hex(p.Line))
}

// object is a card or window standing on the ground: white, outlined, its shadow below.
func (d *sketch) object(x, y, w, h, r float32) {
	p := d.p
	for _, l := range []struct{ dy, spread, alpha float32 }{{4.5, 1.5, .7}, {2.5, .5, .9}, {1, 0, 1}} {
		d.add(`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s" opacity="%.3f"/>`,
			x-l.spread, y+l.dy, w+2*l.spread, h, r+l.spread, hex(p.Lift), float32(p.Lift.A)/255*l.alpha)
	}
	d.add(`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s" stroke="%s"/>`, x+.5, y+.5, w-1, h-1, r-.5, hex(p.Card), hex(p.Line))
}

// bar stands for a line of text: a rounded bar.
func (d *sketch) bar(x, y, w, h float32, col ui.Color) { d.cell(x, y, w, h, h/2, col) }

// cell is a filled place: a list item, a table's cell.
func (d *sketch) cell(x, y, w, h, r float32, col ui.Color) {
	d.add(`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s"/>`, x, y, w, h, r, hex(col))
}

// dashed is a place for what will come: a dashed outline.
func (d *sketch) dashed(x, y, w, h, r float32) {
	d.add(`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="none" stroke="%s" stroke-dasharray="2.5 2"/>`, x+.5, y+.5, w-1, h-1, r, hex(d.p.LineStrong))
}

// icon draws a Lucide icon of size centered at cx, cy, its strokes width wide.
func (d *sketch) icon(paths string, cx, cy, size float32, col ui.Color, width float32) {
	d.add(`<g transform="translate(%g %g) scale(%g)" fill="none" stroke="%s" stroke-width="%g" stroke-linecap="round" stroke-linejoin="round">%s</g>`,
		cx-size/2, cy-size/2, size/24, hex(col), width*24/size, paths)
}

// lantern is the next step: an amber disc, glowing below, with its icon.
func (d *sketch) lantern(paths string, cx, cy, r float32) {
	p := d.p
	d.add(`<circle cx="%g" cy="%g" r="%g" fill="%s" opacity="0.22"/>`, cx, cy+2, r+1, hex(p.Lantern))
	d.add(`<circle cx="%g" cy="%g" r="%g" fill="%s"/>`, cx, cy, r, hex(p.Lantern))
	if paths == lucidePlay {
		cx += .6 // the triangle's weight is left of its box
	}
	d.icon(paths, cx, cy, r*1.05, p.LanternInk, 1.6)
}

// player is a video playing somewhere else, the lantern on its play key, an arrow to what it
// will fill.
func (d *sketch) player() {
	p := d.p
	d.object(14, 30, 70, 56, 10)
	d.cell(20, 36, 58, 34, 6, p.Night)
	for _, s := range []struct{ x, y, a float32 }{{27, 42, .5}, {70, 41, .4}, {62, 63, .35}} {
		d.add(`<circle cx="%g" cy="%g" r="0.8" fill="%s" opacity="%g"/>`, s.x, s.y, hex(p.Starlight), s.a)
	}
	d.bar(20, 76, 58, 3, p.Fill)
	d.bar(20, 76, 9, 3, p.Pencil)
	d.icon(lucideArrow, 93, 58, 9, p.Pencil, 1.3)
	d.lantern(lucidePlay, 49, 53, 8)
}

// hex is col's #rrggbb; its alpha is the opacity given with it.
func hex(col ui.Color) string { return fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B) }
