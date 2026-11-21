package subtitle

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"tomoru/internal/lang"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/samples"
	"tomoru/internal/translate"
	"tomoru/locales"
)

var (
	presets = map[prefs.Font]string{
		prefs.FontSerif:   "Songti SC, Hiragino Mincho ProN, AppleMyungjo, SimSun, Yu Mincho, Batang, Noto Serif CJK SC",
		prefs.FontRounded: "Yuanti SC, Hiragino Maru Gothic ProN",
	}
	TextColors = []string{"#ffffff", "#c8c8c8", "#ffec8c", "#f5a524", "#ffb3cf", "#7fdbff", "#a8f0a0"}
	EdgeColors = []string{"#000000", "#1e2259", "#4a3200", "#4a1030", "#ffffff"}
)

type Scheme struct{ ID, Src, Tgt, Edge string }

var Schemes = []Scheme{
	{"ice", "#ffffff", "#7fdbff", "#1e2259"},
	{"amber", "#ffffff", "#f5a524", "#1e2259"},
	{"classic", "#ffffff", "#ffec8c", "#000000"},
	{"sakura", "#ffffff", "#ffb3cf", "#4a1030"},
	{"mint", "#ffffff", "#a8f0a0", "#000000"},
	{"focus", "#c8c8c8", "#ffffff", "#000000"},
}

type Entry struct {
	Key      EntryKey
	Src, Tgt string
	Lang     string
	Pending  bool
	TgtKind  TgtKind
	Earlier  bool
	Swaps    bool
	Grows    bool
}

type EntryKey struct {
	rolled bool
	n      int64
}

func LineKey(id int) EntryKey { return EntryKey{n: int64(id)} }

func rollKey(n int64) EntryKey { return EntryKey{rolled: true, n: n} }

type TgtKind int

const (
	TgtTranslation TgtKind = iota
	TgtDraft
	TgtNote
)

func (e Entry) DimTgt() bool { return e.Earlier || e.TgtKind == TgtNote }

const appLang = ""

func Visible(st live.State, since int, p prefs.Prefs) []Entry {
	n := int(p.Shown)
	var all []Entry
	for _, l := range st.Lines {
		if l.ID < since {
			continue
		}
		e := Entry{Key: LineKey(l.ID), Src: l.Src, Tgt: l.Tgt, Lang: spoken(l.Lang, p), Swaps: l.Draft != ""}
		switch {
		case p.Display == prefs.DisplaySrc:
			e.Tgt, e.Swaps = "", false
		case l.AsSaid:
			e.Src, e.Tgt = "", l.Src
		case l.Draft != "" && !l.Done():
			e.Tgt, e.TgtKind = l.Draft, TgtDraft
		case l.Err != nil:
			e.Tgt, e.TgtKind = Failure(l.Err), TgtNote
		default:
			e.Grows = !l.Done()
		}
		if p.Display != prefs.DisplayTgt || e.Tgt != "" {
			all = append(all, e)
		}
	}
	draft := st.Draft
	if p.Display == prefs.DisplaySrc {
		draft = ""
	}
	if st.Pending != "" {
		next := 1
		if k := len(st.Lines); k > 0 {
			next = st.Lines[k-1].ID + 1
		}
		e := Entry{Key: LineKey(next), Src: st.Pending, Lang: spoken(st.PendingLang, p), Pending: true, Tgt: draft, Swaps: draft != ""}
		if draft != "" {
			e.TgtKind = TgtDraft
		}
		if st.PendingLang == p.Tgt && p.Display != prefs.DisplaySrc {
			e.Src, e.Tgt = "", st.Pending
		}
		if p.Display != prefs.DisplayTgt || e.Tgt != "" {
			all = append(all, e)
		}
	}
	if k := len(all); p.Display == prefs.DisplayBoth && n == 1 && k >= 2 && all[k-1].Tgt == "" && all[k-2].Tgt != "" {
		cur, prev := all[k-1], all[k-2]
		return []Entry{{Key: cur.Key, Src: cur.Src, Lang: cur.Lang, Pending: cur.Pending, Tgt: prev.Tgt, TgtKind: prev.TgtKind, Earlier: true, Swaps: true, Grows: prev.Grows}}
	}
	return all[max(0, len(all)-n):]
}

func spoken(heard string, p prefs.Prefs) string {
	if heard == "" && p.Src != lang.Auto {
		return p.Src
	}
	return heard
}

func Failure(err error) string {
	var status *translate.StatusError
	var netErr net.Error
	var opErr *net.OpError
	switch {
	case errors.Is(err, live.ErrBehind):
		return locales.TranslationSkipped()
	case errors.As(err, &status) && (status.Status == http.StatusUnauthorized || status.Status == http.StatusForbidden):
		return locales.TranslationKeyRefused()
	case errors.As(err, &status) && (status.Status == http.StatusPaymentRequired || status.Status == http.StatusTooManyRequests):
		return locales.TranslationLimited()
	case errors.Is(err, translate.ErrStalled), errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &netErr) && netErr.Timeout(), errors.As(err, &opErr):
		return locales.TranslationNotAnswering()
	}
	return locales.TranslationFailed()
}

func Samples(p prefs.Prefs) []Entry {
	var out []Entry
	from := len(samples.Lines) - min(int(p.Shown), len(samples.Lines))
	for i, s := range samples.Lines[from:] {
		out = append(out, Entry{Key: EntryKey{n: int64(-1 - from - i)}, Src: samples.In(s, sampleSrc(p)), Lang: sampleSrc(p), Tgt: samples.In(s, p.Tgt)})
	}
	return out
}

func sampleSrc(p prefs.Prefs) string {
	switch {
	case p.Src != lang.Auto:
		return p.Src
	case p.Tgt == "en":
		return "ja"
	}
	return "en"
}

const RollEvery = 2400 * time.Millisecond

func Sample(p prefs.Prefs, n int64) Entry {
	l := int64(len(samples.Lines))
	s := samples.Lines[(n%l+l)%l]
	return Entry{Key: rollKey(n), Src: samples.In(s, sampleSrc(p)), Lang: sampleSrc(p), Tgt: samples.In(s, p.Tgt)}
}

func MixedSample(p prefs.Prefs) Entry { return Sample(p, samples.MixedScript) }

func Rolling(p prefs.Prefs, now time.Time) ([]Entry, time.Duration) {
	every := RollEvery.Milliseconds()
	n := now.UnixMilli() / every
	var out []Entry
	for j := n - int64(min(int(p.Shown), len(samples.Lines))) + 1; j <= n; j++ {
		out = append(out, Sample(p, j))
	}
	return out, time.Duration(every-now.UnixMilli()%every) * time.Millisecond
}

const lineHeight = 1.35

const bandPadY, bandPadX = 6, 14

func SlotHeight(p prefs.Prefs, k float32, boxed bool) float32 {
	h := float32(p.SrcSize*2+p.TgtSize*2)*lineHeight*k + 2*k
	switch p.Display {
	case prefs.DisplayTgt:
		h = float32(p.TgtSize*2) * lineHeight * k
	case prefs.DisplaySrc:
		h = float32(p.SrcSize*2) * lineHeight * k
	}
	if boxed && p.Style == prefs.StyleBox {
		h += 2 * bandPadY * k
	}
	return h
}

func Align(p prefs.Prefs) ui.Align {
	if p.Align == prefs.AlignCenter {
		return ui.Center
	}
	return ui.Start
}

var Follows = ui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond}

var swap = ui.ContentTransition{Animation: &ui.Animation{Duration: 220 * time.Millisecond, Ease: ui.EaseOut}}

const NoBlock = 1e9

func Draw(c *ui.Context, entries []Entry, p prefs.Prefs, k, width, bareBelow float32) (top float32) {
	align := Align(p)
	band := p.Style == prefs.StyleBox
	opacity := float32(p.TextOpacity) / 100
	for i, e := range entries {
		srcLang := e.Lang
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
			fade = float32(p.OlderOpacity) / 100 * max(.46, 1-.15*float32(age-1))
		}
		sentence.Opacity(sentence.Animate("fade", fade, 320*time.Millisecond))
		b := sentence.Bounds()
		if b.H > 0 && (top == 0 || b.Y < top) {
			top = b.Y
		}
		boxed := b.H > 0 && b.Y < bareBelow || bareBelow == NoBlock
		sentence.Children(func() {
			lines := ui.Column(c).AlignSelf(align).AlignItems(align).Gap(2 * k)
			if !band {
				lines = ui.Column(c).AlignSelf(ui.Stretch).AlignItems(ui.Stretch).Gap(2 * k)
			}
			if band {
				bg := ui.RGBA(0, 0, 0, 0)
				if boxed {
					bg = ui.RGBA(0, 0, 0, float32(p.BoxOpacity)/100)
				}
				lines = lines.Background(bg).Radius(8*k).Padding(bandPadY*k, bandPadX*k).Transition(Follows)
			}
			lines.Children(func() {
				src := func() {
					if e.Src == "" || p.Display == prefs.DisplayTgt {
						return
					}
					text, n, dim := e.Src, 2, float32(1)
					if p.Display == prefs.DisplaySrc && age == 0 {
						n = 3
					}
					if e.Pending {
						n, dim = 1, .8
						text = fitTail(c, p, text, srcLang, float32(p.SrcSize)*k, width-36*k)
					}
					line(c, p, text, srcLang, float32(p.SrcSize)*k, ui.Hex(p.SrcColor), n, !e.Pending, width-28*k).Opacity(dim * opacity)
				}
				tgt := func() {
					if e.Tgt == "" || p.Display == prefs.DisplaySrc {
						return
					}
					n := 2
					if age == 0 {
						n = 3
					}
					tgtLang := p.Tgt
					if e.TgtKind == TgtNote {
						tgtLang = appLang
					}
					t := line(c, p, e.Tgt, tgtLang, float32(p.TgtSize)*k, ui.Hex(p.TgtColor), n, !e.Pending && !e.Grows, width-28*k)
					if e.Swaps {
						t.ContentTransition(swap)
					}
					dim := float32(1)
					switch {
					case e.DimTgt():
						dim = .5
					case e.TgtKind == TgtDraft:
						dim = .72
					}
					t.Opacity(dim * opacity)
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

func family(p prefs.Prefs) string {
	if f, ok := presets[p.Font]; ok {
		return f
	}
	if p.Font == prefs.FontSystem {
		return ""
	}
	return string(p.Font)
}

func fitTail(c *ui.Context, p prefs.Prefs, s, code string, size, width float32) string {
	cjk := lang.CJK(code)
	measure := func(t string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: t, Font: family(p), Size: size, Weight: 700, Lang: code})
		return w
	}
	if measure(s) <= width {
		return s
	}
	r := []rune(s)
	lo, hi := 1, len(r)
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

func line(c *ui.Context, p prefs.Prefs, s, lang string, size float32, col ui.Color, lines int, balance bool, width float32) ui.Element {
	t := ui.Text(c, s).Lang(lang).FontSize(size).Bold().TextAlign(Align(p)).TextColor(col).MaxLines(lines)
	if f := family(p); f != "" {
		t = t.Font(f)
	}
	switch {
	case balance && p.Style == prefs.StyleBox:
		t = t.MaxWidth(balanced(c, ui.Span{Text: s, Font: family(p), Size: size, Weight: 700, Lang: lang}, width, lines))
	case balance:
		t = t.Balance()
	}
	if p.Edge == prefs.EdgeOutline || p.Edge == prefs.EdgeShadow {
		t = t.Effect(edgeFor(p, size))
	}
	return t
}

func balanced(c *ui.Context, text ui.Span, width float32, lines int) float32 {
	one, lineH := c.MeasureText(0, text)
	if one <= width {
		return width
	}
	_, h := c.MeasureText(width, text)
	if n := int(h/lineH + .5); n > lines {
		return width
	}
	lo, hi := one/float32(lines)/2, width
	for hi-lo > .5 {
		mid := (lo + hi) / 2
		if _, mh := c.MeasureText(mid, text); mh > h {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

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

func edgeFor(p prefs.Prefs, size float32) ui.Effect {
	w := max(1.2, size*.06)
	reach := w + 1
	if p.Edge == prefs.EdgeShadow {
		reach = 2*w + 2
	}
	return gpuOnly{edgeShader.Effect(shader.Options{
		Uniforms:        map[string]any{"Edge": ui.Hex(p.EdgeColor), "Width": w, "Mode": int(p.Edge)},
		MaxSampleOffset: ui.Point{X: reach, Y: reach},
	})}
}

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
