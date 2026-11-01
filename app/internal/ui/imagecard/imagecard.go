// Package imagecard is the card that shows a picture's translation: the part of the screen selected,
// or the picture on the clipboard, translated as its shortcut asks (docs/image-translation.md), into
// the language chosen on it. It floats over the screen, moved by its title, until Escape or its
// close button closes it. Its size stays as the user leaves it: what it translates scrolls in it,
// rather than the card growing with it, which showed a frame of the card cut short at each line.
package imagecard

import (
	"errors"
	"math"
	"net"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"tomoru/internal/core"
	"tomoru/internal/translate"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// Width and Height are the card's size until the user resizes it; MinWidth and MinHeight the least
// it is resized to.
const (
	Width, Height       = 440, 440
	MinWidth, MinHeight = 340, 300
)

// thumbHeight is the most the picture takes: enough to check a number against, as the
// translation may get one wrong. A short card gives it thumbShare of its height at most.
const (
	thumbHeight = 168
	thumbShare  = .45
)

// The card's parts around the picture and its translation, for FitHeight: the head with its
// padding, the foot, and the bottom padding with the gaps between the four.
const (
	headHeight  = 44
	footHeight  = 28
	frameHeight = 16 + 3*12
	// lineHeight is a line of the translation's; the card makes room for fitLines of them at least.
	lineHeight = kit.TextBody * 1.4
	fitLines   = 4
)

// FitHeight is the height a card w wide needs for a picture pw by ph: its thumbnail, and fitLines
// of translation below, as a game's dialogue box has; MinHeight at least. A card is no taller than
// that, nor than its size: a longer translation scrolls in it, as the card is not resized as it
// comes.
func FitHeight(w, pw, ph int) int {
	thumb := thumbSize(float32(w), pw, ph)
	h := headHeight + thumb + fitLines*lineHeight + footHeight + frameHeight
	return max(int(math.Ceil(float64(h))), MinHeight)
}

// thumbSize is the height of the thumbnail of a picture pw by ph in a card w wide, at most
// thumbHeight.
func thumbSize(w float32, pw, ph int) float32 {
	inner := w - 2*kit.Space(4) - 2*kit.Space(2)
	return min(thumbHeight, inner*float32(ph)/float32(max(pw, 1))) + 2*kit.Space(2)
}

// Card is the card's state.
type Card struct {
	core  *core.Core
	close func()
	pick  func() // has the user select a part of the screen to translate

	// Peek shows the picture whole, as large as it was on the screen, as its thumbnail is clicked.
	Peek func()

	pic    int // the picture shown, as core.ImageView.ID tells it
	bitmap *ui.Bitmap
	copied time.Time        // when Copy was last clicked: it says so for a while
	md     markdown.Options // how the translation comes in: evenly, fading in
	scroll ui.ScrollState   // the translation's, which follows its end as it comes
}

// New is the card of k's picture. close closes it; pick has the user select a part of the screen,
// once the app may capture it.
func New(k *core.Core, close func(), pick func()) *Card {
	return &Card{core: k, close: close, pick: pick, pic: -1}
}

func (d *Card) View(c *ui.Context) {
	kit.Paper(c)
	p := kit.Colors(c)
	c.Root().Background(p.Paper) // the window's corners are the system's
	if id := d.core.Image.ID(); id != d.pic {
		d.pic, d.bitmap, d.scroll.Y = id, nil, 0
		if b, err := ui.DecodeBitmap(d.core.Image.PNG); err == nil {
			d.bitmap = b
		}
	}
	// The head on top, the foot at the bottom, and between, the picture and what it says, which
	// scrolls in what room the card has.
	ui.Column(c).Fill().Padding(0, kit.Space(4), kit.Space(4)).Gap(kit.Space(3)).Children(func() {
		d.head(c)
		if d.core.Image.PNG == nil {
			d.allow(c)
			return
		}
		d.thumbnail(c)
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).Gap(kit.Space(3)).Children(func() { d.translation(c) })
		d.foot(c)
	})
}

// head is the card's title, the language the picture is translated into, and its close button: the
// strip along the card's top edge, which moves the card, but for the buttons. The foot says what
// translates it.
func (d *Card) head(c *ui.Context) {
	k, p := d.core, kit.Colors(c)
	ui.Row(c).Gap(kit.Space(2)).Padding(kit.Space(4), 0, 0).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Icon(c, assets.Picture).FontSize(kit.IconSizeButton).TextColor(p.LanternText)
		ui.Text(c, locales.ImageTitle()).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(p.Ink).SingleLine()
		ui.Spacer(c)
		if k.Image.PNG != nil {
			tgt := k.ImageTgt()
			kit.Picker(c, shared.LangName(tgt), kit.Choices(tgt, shared.LangChoices(k.TargetLangs()), shared.LangName, k.SetImageTgt)).
				Label(locales.TranslateInto())
		}
		if kit.IconButton(c, kit.Close, kit.Small, assets.Close, locales.Close()).Clicked() {
			d.close()
		}
	})
}

// allow asks for Screen Recording, which selecting a part of the screen takes, when there was no
// picture to translate: the app may not capture the screen, and the clipboard had none. Once it may,
// the user selects one.
func (d *Card) allow(c *ui.Context) {
	k, p := d.core, kit.Colors(c)
	k.CheckScreen() // turned on in System Settings meanwhile
	c.After(core.ListEvery)
	// In the middle of the card, which keeps its size for the picture to come.
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(kit.Space(1.5)).Padding(0, kit.Space(4), kit.Space(8)).Children(func() {
		ui.Text(c, locales.GuideSelectTitle()).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(p.Ink)
		switch {
		case k.CanSelect():
			if kit.Button(c, kit.Primary, kit.Small, assets.Picture, locales.TranslateScreen()).Margin(kit.Space(1.5), 0, 0).Clicked() {
				d.pick()
			}
		case k.Screen == "denied":
			kit.Caption(c, locales.GuideSelectSettings()).TextAlign(ui.Center)
			if kit.Button(c, kit.Primary, kit.Small, nil, locales.OpenSystemSettings()).Margin(kit.Space(1.5), 0, 0).Clicked() {
				kit.OpenScreenSettings()
			}
		default:
			kit.Caption(c, locales.ScreenSelectWhy()).TextAlign(ui.Center)
			label := locales.ScreenSelectTurnOn()
			if k.ScreenAsking {
				label = locales.ScreenSelectAsking()
			}
			if kit.Off(kit.Button(c, kit.Primary, kit.Small, assets.Picture, label), k.ScreenAsking || k.Screen == "").Margin(kit.Space(1.5), 0, 0).Clicked() {
				k.RequestScreen()
			}
		}
	})
}

// thumbnail is the picture, as large as the card's width allows, thumbHeight tall at most, and less
// in a short card. Clicked, it shows whole, its text as legible as it was on the screen (Peek).
func (d *Card) thumbnail(c *ui.Context) {
	if d.bitmap == nil {
		return
	}
	p := kit.Colors(c)
	w, ch := c.Size()
	pw, ph := d.bitmap.Size()
	h := min(thumbSize(w, pw, ph), thumbShare*ch) - 2*kit.Space(2)
	b := ui.ButtonBase(c).Label(locales.ImageZoomIn()).Tooltip(locales.ImageZoomIn()).Cursor(ui.CursorPointer).
		Radius(kit.RadiusInner).Background(p.Sand).Padding(kit.Space(2)).Clip()
	b.Children(func() {
		ui.Image(c, d.bitmap).Height(h).FillWidth().Fit(ui.Contain)
		badge(c, assets.ZoomIn).Opacity(b.AnimateTo("hint", hint(b.Hovered()), kit.Fade))
	})
	if b.Clicked() && d.Peek != nil {
		d.Peek()
	}
}

// badge is the round icon in the picture's corner that tells what clicking it does.
func badge(c *ui.Context, icon *ui.SVG) ui.Element {
	p := kit.Colors(c)
	return ui.Box(c).Absolute().Right(kit.Space(3)).Bottom(kit.Space(3)).Size(24, 24).Radius(kit.Pill(24)).
		Background(p.Paper.Alpha(.92)).Center().Children(func() {
		ui.Icon(c, icon).FontSize(kit.TextCaption).TextColor(p.Ink)
	})
}

// hint is how plain the thumbnail's zoom badge is: a little, and fully while the pointer is on it.
func hint(hovered bool) float32 {
	if hovered {
		return 1
	}
	return .7
}

// translation is the picture's translation as it streams in, which can be selected, or why there
// is none.
func (d *Card) translation(c *ui.Context) {
	k, p := d.core, kit.Colors(c)
	im := k.Image
	switch {
	case im.Text != "":
		// Markdown, as a model may write it of a page it reads, shown as MyGo's stream of a model's
		// reply: rather than in the bursts the network brings, at an even pace, a character at a
		// time, as Chinese and Japanese have no spaces between words, each fading in. Finished, the
		// same view shows the rest of it coming in.
		d.md.Theme = &markdown.Theme{FontSize: kit.TextBody, Text: p.Ink, LineHeight: 1.4}
		d.md.Extensions = parser.GFM // no math: a dialogue's dollars are money
		d.md.Reveal = &markdown.Reveal{Unit: markdown.ByCharacter}
		d.md.ShareSelection = true       // copied as the text it shows
		if d.scroll.Y >= d.scroll.MaxY { // at the end: it stays there, unless the user scrolled up
			d.scroll.Y = math.MaxFloat32
		}
		ui.Scroll(c).TrackScroll(&d.scroll).Shrink(1).MinHeight(0).Children(func() {
			ui.Column(c).Selectable().Children(func() {
				if src := lines(im.Text); im.Running {
					markdown.Stream(c, src, &d.md)
				} else {
					markdown.View(c, src, &d.md)
				}
			})
		})
	case im.Running:
		waiting(c)
	}
	if im.Err == nil {
		return
	}
	ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(kit.Space(0.5)).Children(func() {
			if errors.Is(im.Err, translate.ErrEmpty) {
				kit.Warning(c, locales.ImageNoText())
				return
			}
			via, _ := k.Engine(core.MT, im.Via)
			why := im.Err.Error()
			var ne net.Error
			switch {
			case errors.Is(im.Err, core.ErrModelMissing) || errors.Is(im.Err, core.ErrNoEndpoint) || errors.As(im.Err, new(core.LocalError)):
				why = shared.ErrorText(im.Err) // as the subtitles say it
			case via.Cloud && (errors.As(im.Err, &ne) && ne.Timeout() || errors.Is(im.Err, translate.ErrStalled)):
				why = locales.TranslationFailedCloud() // the free service is down at times
			}
			if via.Endpoint != nil && !errors.Is(im.Err, core.ErrNoEndpoint) {
				// The user's API may read no pictures: no other engine is asked.
				kit.Warning(c, locales.ImageEndpointFailed(shared.EngineName(via)))
			} else {
				kit.Warning(c, locales.ImageFailed())
			}
			kit.Caption(c, why).MaxLines(2)
		})
		if kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retry()).Clicked() {
			k.TranslateImage(im.PNG)
		}
	})
}

// waiting is the translation to come: what is under way, and lines that breathe where its text
// will be.
func waiting(c *ui.Context) {
	kit.Waiting(c, locales.ImageTranslating())
	ui.Column(c).Gap(kit.Space(2.5)).Padding(kit.Space(1), 0, 0).Children(func() {
		for i, w := range []float32{100, 86, 58} {
			bar := ui.Box(c.Key(i)).WidthPercent(w).Height(kit.TextBody * .8).Radius(kit.Pill(kit.TextBody * .8)).Background(kit.Colors(c).Sand)
			bar.Opacity(bar.Keyframes("breathe", []ui.Keyframe{
				{Value: .55},
				{At: 900 * time.Millisecond, Value: 1, Ease: ui.EaseInOut},
			}, ui.KeyframeOptions{Repeat: ui.RepeatForever, Alternate: true, Delay: time.Duration(i) * 150 * time.Millisecond}))
		}
	})
}

// lines is a translation as Markdown, as models write it, that keeps its lines apart, as a game's
// dialogue has them, one a speaker: a break at the end of each, two spaces rather than a backslash,
// which would show as the text comes in, but in code blocks. A piece of a translation is a piece of
// what all of it is, as it streams in.
func lines(text string) string {
	var b strings.Builder
	code := false // in a fenced code block
	for line := range strings.SplitAfterSeq(text, "\n") {
		l, ended := strings.CutSuffix(line, "\n")
		switch t := strings.TrimLeft(l, " "); {
		case ended && len(l)-len(t) < 4 && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")):
			code = !code // a fence, opening or closing
		case ended && !code:
			line = l + "  \n"
		}
		b.WriteString(line)
	}
	return b.String()
}

// note says what translates the picture: the engine chosen. Index-Translate in it links to its
// page.
func note(k *core.Core) string {
	via, _ := k.Engine(core.MT, k.Image.Via)
	switch {
	case via.Model != nil:
		return locales.ImageNoteLocal()
	case via.Endpoint != nil:
		return locales.ImageNoteEndpoint(shared.EngineName(via))
	}
	return locales.ImageNote()
}

// foot says where the picture goes, and copies the translation.
func (d *Card) foot(c *ui.Context) {
	k := d.core
	ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center).Children(func() {
		kit.CaptionLink(c, note(k), "Index-Translate", core.CloudRepo).Grow(1).MaxLines(2).Tooltip(locales.ImageKept())
		label, icon := locales.Copy(), assets.Copy
		if left := d.copied.Add(1500 * time.Millisecond).Sub(c.Now()); left > 0 {
			label, icon = locales.Copied(), assets.Check
			c.After(left)
		}
		if kit.Off(kit.Button(c, kit.Plain, kit.Small, icon, label), k.Image.Text == "").Clicked() {
			c.Services().WriteClipboard(k.Image.Text)
			d.copied = c.Now()
		}
	})
}
