// Package subtitle draws subtitles in the user's style: the overlay's, and the settings' preview.
// It also works out which sentences show.
package subtitle

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"tomoru/internal/lang"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/samples"
	"tomoru/locales"
)

var (
	// FontFamily are the families of the fonts, by key: the first family with a glyph draws it, of
	// macOS's then Windows's. Windows has no rounded one: the system's draws it there.
	FontFamily = map[string]string{
		"serif":   "Songti SC, Hiragino Mincho ProN, AppleMyungjo, SimSun, Yu Mincho, Batang",
		"rounded": "Yuanti SC, Hiragino Maru Gothic ProN",
	}
	// TextColors and EdgeColors are the colors to choose from: light ones for the text, to read
	// over a picture, and dark ones around it; a white edge for dark text.
	TextColors = []string{"#ffffff", "#c8c8c8", "#ffec8c", "#f5a524", "#ffb3cf", "#7fdbff", "#a8f0a0"}
	EdgeColors = []string{"#000000", "#1e2259", "#4a3200", "#4a1030", "#ffffff"}
)

// Scheme is a set of the subtitles' colors that go together, of TextColors and EdgeColors. The
// original stays white, or a gray under the translation, which carries the color: it is what
// people read.
type Scheme struct{ ID, Src, Tgt, Edge string }

// Schemes are the color schemes to choose from, the default first: the translation in ice blue,
// edged in Tomoru's night, which reads over most pictures. "amber" is the lantern's of its brand.
var Schemes = []Scheme{
	{"ice", "#ffffff", "#7fdbff", "#1e2259"},
	{"amber", "#ffffff", "#f5a524", "#1e2259"},
	{"classic", "#ffffff", "#ffec8c", "#000000"},
	{"sakura", "#ffffff", "#ffb3cf", "#4a1030"},
	{"mint", "#ffffff", "#a8f0a0", "#000000"},
	{"focus", "#c8c8c8", "#ffffff", "#000000"},
}

// Entry is a sentence to show.
type Entry struct {
	Key      any // the sentence's ID, which keeps its element as it goes from pending to committed
	Src, Tgt string
	Pending  bool // still being spoken
	DimTgt   bool // Tgt is not this sentence's, or not a translation
	Draft    bool // Tgt is a draft: the translation of an unfinished sentence
	// Swaps says that Tgt changes whole, as a draft does to the next, or to the translation: it
	// cross-fades, where a translation streaming in grows.
	Swaps bool
}

// Visible is what shows of st, oldest first, like scrolling lyrics: the lines since the
// subtitles last went, and what is being said. With one sentence at a time the previous
// translation stays while the next sentence is recognized, dimmed: it is not that sentence's
// translation.
func Visible(st live.State, since int, p prefs.Prefs) []Entry {
	n := int(p.Shown)
	var all []Entry
	for _, l := range st.Lines {
		if l.ID < since {
			continue
		}
		e := Entry{Key: l.ID, Src: l.Src, Tgt: l.Tgt, Swaps: l.Draft != ""}
		switch {
		case l.Draft != "" && !l.Done:
			// A translation streaming in starts shorter than the draft it replaces: keep the draft
			// until the translation is done, rather than going back to a few words.
			e.Tgt, e.Draft = l.Draft, true
		case l.Err != "":
			e.Tgt, e.DimTgt = locales.TranslationFailed(), true
		}
		if p.Display == 0 || e.Tgt != "" {
			all = append(all, e)
		}
	}
	if st.Pending != "" && (p.Display == 0 || st.Draft != "") {
		next := 1
		if k := len(st.Lines); k > 0 {
			next = st.Lines[k-1].ID + 1
		}
		all = append(all, Entry{Key: next, Src: st.Pending, Pending: true, Tgt: st.Draft, Draft: st.Draft != "", Swaps: st.Draft != ""})
	}
	if k := len(all); p.Display == 0 && n == 1 && k >= 2 && all[k-1].Tgt == "" && all[k-2].Tgt != "" {
		cur, prev := all[k-1], all[k-2]
		return []Entry{{Key: cur.Key, Src: cur.Src, Pending: cur.Pending, Tgt: prev.Tgt, DimTgt: true, Draft: prev.Draft, Swaps: true}}
	}
	return all[max(0, len(all)-n):]
}

// Samples are the last of the sample sentences, as many as show at once, in the chosen
// languages: to show the look. Each keeps its key as the number changes, so only the one added or
// removed comes or goes.
func Samples(p prefs.Prefs) []Entry {
	var out []Entry
	from := len(samples.Lines) - min(int(p.Shown), len(samples.Lines))
	for i, s := range samples.Lines[from:] {
		out = append(out, Entry{Key: -1 - from - i, Src: samples.In(s, p.Src), Tgt: samples.In(s, p.Tgt)})
	}
	return out
}

// RollEvery is how often Rolling brings in the next sample.
const RollEvery = 2400 * time.Millisecond

// rollKey keys a sentence of Rolling's stream: the how-manyth since the epoch.
type rollKey int64

// Sample is the n-th of the sample sentences, going round, in p's languages.
func Sample(p prefs.Prefs, n int64) Entry {
	l := int64(len(samples.Lines))
	s := samples.Lines[(n%l+l)%l]
	return Entry{Key: rollKey(n), Src: samples.In(s, p.Src), Tgt: samples.In(s, p.Tgt)}
}

// Rolling are the sample sentences as they would come, in the chosen languages: one more every
// RollEvery, the oldest going, as many as show at once; and how long until the next. To show the
// look in motion.
func Rolling(p prefs.Prefs, now time.Time) ([]Entry, time.Duration) {
	every := RollEvery.Milliseconds()
	n := now.UnixMilli() / every
	var out []Entry
	for j := n - int64(min(int(p.Shown), len(samples.Lines))) + 1; j <= n; j++ {
		out = append(out, Sample(p, j))
	}
	return out, time.Duration(every-now.UnixMilli()%every) * time.Millisecond
}

const lineHeight = 1.35 // of the font size, about

// SlotHeight is the height of a sentence of two lines of source and two of translation, scaled by
// k: the least the overlay's block is, so that the pointer finds it the same size empty.
func SlotHeight(p prefs.Prefs, k float32, boxed bool) float32 {
	h := float32(p.SrcSize*2+p.TgtSize*2)*lineHeight*k + 2*k
	if p.Display == 1 {
		h = float32(p.TgtSize*2) * lineHeight * k
	}
	if boxed && p.Style == 0 {
		h += 12 * k // the caption band's padding
	}
	return h
}

// Align is where the lines of a sentence start. On the left, the words of a sentence being spoken
// come in at a fixed place, and lines do not shift as they grow, as the BBC asks of live
// subtitles; centered looks like a film's.
func Align(p prefs.Prefs) ui.Align {
	if p.Align == 1 {
		return ui.Center
	}
	return ui.Start
}

// Follows only fades colors: what has it follows the text, which moves by itself.
var Follows = ui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond}

// swap cross-fades a translation changing whole: quickly, as the eye is on it.
var swap = ui.ContentTransition{Animation: &ui.Animation{Duration: 220 * time.Millisecond, Ease: ui.EaseOut}}

// NoBlock is Draw's bareBelow with no block to show behind the sentences.
const NoBlock = 1e9

// Draw draws entries, oldest first, in p's style, scaled by k (the settings preview is smaller),
// in width, each on a closed-caption band if that is the style, but those from bareBelow down in
// the window: there the overlay's hovered block is their background. They go in a column that
// holds them at its bottom. It returns the top of the oldest, as last laid out, in the window.
//
// The newest sentence is at the bottom, where the eye rests; the older ones above, fainter, are
// there to look up to for what was missed. Nothing moves but by as much as the text grows, as
// roll-up live captions go: a sentence wrapping to a new line, or the next one's first line,
// pushes those above up by that line, smoothly, those farther up a moment later, as Apple
// Music's lyrics do. A sentence comes in from a little below, fading in; the oldest goes a
// little up, fading out. Each spans the width, and only goes up and down: a centered sentence
// growing does not move sideways.
func Draw(c *ui.Context, entries []Entry, p prefs.Prefs, k, width, bareBelow float32) (top float32) {
	align := Align(p)
	band := p.Style == 0
	cjk := lang.CJK(p.Src)
	for i, e := range entries {
		age := len(entries) - 1 - i
		roll := ui.ElementTransition{
			Position:  true,
			Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond}, Delay: time.Duration(age) * 30 * time.Millisecond},
			Enter:     &ui.Motion{Y: 14 * k},
			Exit:      &ui.Motion{Y: -14 * k},
		}
		sentence := ui.Column(c.Key(e.Key)).Transition(roll)
		fade := float32(1)
		if age > 0 {
			fade = max(.25, .62-.08*float32(age)) // .54, .46, … down to a quarter
		}
		sentence.Opacity(sentence.Animate("fade", fade, 320*time.Millisecond))
		b := sentence.Bounds()
		if b.H > 0 && (top == 0 || b.Y < top) {
			top = b.Y
		}
		boxed := b.H > 0 && b.Y < bareBelow || bareBelow == NoBlock
		sentence.Children(func() {
			// The band keeps its padding without its color, so that hovering moves no text.
			lines := ui.Column(c).AlignSelf(align).AlignItems(align).Gap(2 * k)
			if !band {
				// Without a band, a line spans the width, aligned by its text: as wide whatever it
				// says, its edge's layer (edgeFor) keeps its size as words come in, rather than
				// being made anew for each, its old one kept on the GPU until Go collects it.
				lines = ui.Column(c).AlignSelf(ui.Stretch).AlignItems(ui.Stretch).Gap(2 * k)
			}
			if band {
				bg := ui.RGBA(0, 0, 0, 0)
				if boxed {
					bg = ui.RGBA(0, 0, 0, .62)
				}
				lines = lines.Background(bg).Radius(8*k).Padding(6*k, 14*k).Transition(Follows)
			}
			lines.Children(func() {
				src := func() {
					if e.Src == "" || p.Display != 0 {
						return
					}
					text, col := e.Src, hex(p.SrcColor, ui.RGBA(255, 255, 255, 1))
					n := 2
					if e.Pending {
						// Still being recognized: one line that rolls, as live captions do, rather
						// than a second line opening and filling word by word.
						col, n = ui.Hex("#cdcdcd"), 1
						text = fitTail(c, p, text, float32(p.SrcSize)*k, width-36*k, cjk)
					}
					line(c, p, text, float32(p.SrcSize)*k, col, n)
				}
				tgt := func() {
					if e.Tgt == "" {
						return
					}
					n := 2
					if age == 0 {
						n = 3
					}
					t := line(c, p, e.Tgt, float32(p.TgtSize)*k, hex(p.TgtColor, ui.Hex("#ffec8c")), n)
					if e.Swaps {
						t.ContentTransition(swap)
					}
					switch {
					case e.DimTgt:
						t.Opacity(.5)
					case e.Draft:
						t.Opacity(.72) // a draft, until the sentence ends
					}
				}
				if p.TgtFirst {
					tgt()
					src()
				} else {
					src()
					tgt()
				}
			})
		})
	}
	return top
}

// fitTail is s if it fits one line of width at size, else "…" and as much of its end as fits:
// the newest words of a sentence still being spoken matter most. Spaced languages start at a
// word.
func fitTail(c *ui.Context, p prefs.Prefs, s string, size, width float32, cjk bool) string {
	measure := func(t string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: t, Font: FontFamily[p.Font], Size: size, Weight: 700})
		return w
	}
	if measure(s) <= width {
		return s
	}
	r := []rune(s)
	lo, hi := 1, len(r) // the first rune of the longest end that fits after "…"
	for lo < hi {
		mid := (lo + hi) / 2
		if measure("…"+string(r[mid:])) <= width {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	end := string(r[lo:])
	if i := strings.IndexByte(end, ' '); !cjk && i >= 0 && i < 16 {
		end = end[i+1:]
	}
	return "…" + end
}

// line draws one subtitle line with the chosen edge, which the GPU draws from the text as it
// renders (edgeShader).
func line(c *ui.Context, p prefs.Prefs, s string, size float32, col ui.Color, lines int) ui.Element {
	t := ui.Text(c, s).FontSize(size).Bold().TextAlign(Align(p)).TextColor(col).MaxLines(lines)
	if f := FontFamily[p.Font]; f != "" {
		t = t.Font(f)
	}
	if p.Edge == 1 || p.Edge == 2 {
		t = t.Effect(edgeFor(p, size))
	}
	return t
}

// edgeShader draws a line's edge from the line as it renders: an outline Width wide around the
// glyphs (their coverage, the most of it within Width), or a soft shadow down and to the right
// (their coverage moved by Width and blurred, and a faint rim close around them), under the text. Drawing the text once more for
// each side, as before, cost a run of glyphs a copy and left the outline's corners notched.
var edgeShader = shader.MustCompile(`package main

var Edge vec4   // the edge's color
var Width float // the outline's width, or how far the shadow falls, in DIPs
var Mode int    // 1 an outline, 2 a shadow

func LayerEffect(pos vec2) vec4 {
	text := layerSample(pos)
	a := 0.0
	if Mode == 1 {
		for i := 0; i < 16; i++ {
			t := float(i) * 0.39269908
			d := vec2(cos(t), sin(t)) * Width
			a = max(a, max(layerSample(pos+d).a, layerSample(pos+d*0.5).a))
		}
	} else {
		o := pos - vec2(Width, Width)
		sum := 0.0
		for y := -2; y <= 2; y++ {
			for x := -2; x <= 2; x++ {
				w := exp(-float(x*x+y*y) / 4.5)
				a += w * layerSample(o+vec2(float(x), float(y))*Width*0.45).a
				sum += w
			}
		}
		a = min(1, a/sum*1.8)
		for i := 0; i < 8; i++ {
			t := float(i) * 0.78539816
			a = max(a, 0.45*layerSample(pos+vec2(cos(t), sin(t))*Width*0.6).a)
		}
	}
	return text + vec4(Edge.rgb, 1)*Edge.a*a*(1-text.a)
}
`)

// edgeFor is the edge of a line of size in p's style.
func edgeFor(p prefs.Prefs, size float32) ui.Effect {
	w := max(1.2, size*.06)
	reach := w + 1
	if p.Edge == 2 {
		reach = 2*w + 2
	}
	return gpuOnly{edgeShader.Effect(shader.Options{
		Uniforms:        map[string]any{"Edge": hex(p.EdgeColor, ui.RGBA(0, 0, 0, 1)), "Width": w, "Mode": int(p.Edge)},
		MaxSampleOffset: ui.Point{X: reach, Y: reach},
	})}
}

// gpuOnly is an effect drawn on the GPU only: in memory, as in tests, a shader runs pixel by pixel,
// and the content shows as it is.
type gpuOnly struct{ shader.Effect }

func (e gpuOnly) PaintEffect(p *ui.Painter, box ui.Rect, content *ui.Bitmap, inset ui.Point) {
	if p.Accelerated() {
		e.Effect.PaintEffect(p, box, content, inset)
		return
	}
	s := p.Scale()
	w, h := content.Size()
	p.ImageRegion(content, ui.Rect{}, ui.Rect{X: box.X - inset.X/s, Y: box.Y - inset.Y/s, W: float32(w) / s, H: float32(h) / s}, ui.ImageOptions{})
}

// hex parses a color of the settings file, which a person may have edited: ui.Hex panics on bad
// input.
func hex(s string, fallback ui.Color) (c ui.Color) {
	defer func() {
		if recover() != nil {
			c = fallback
		}
	}()
	return ui.Hex(s)
}
