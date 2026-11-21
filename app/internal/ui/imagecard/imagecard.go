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

const (
	Width, Height       = 440, 440
	MinWidth, MinHeight = 340, 300
)

const (
	thumbHeight = 168
	thumbShare  = .45
)

const (
	headHeight  = 44
	footHeight  = 28
	frameHeight = 16 + 3*12
	lineSpacing = 1.4
	lineHeight  = kit.TextBody * lineSpacing
	fitLines    = 4
)

func FitHeight(w, pw, ph int) int {
	thumb := thumbSize(float32(w), pw, ph)
	h := headHeight + thumb + fitLines*lineHeight + footHeight + frameHeight
	return max(int(math.Ceil(float64(h))), MinHeight)
}

func thumbSize(w float32, pw, ph int) float32 {
	inner := w - 2*kit.Space(4) - 2*kit.Space(2)
	return min(thumbHeight, inner*float32(ph)/float32(max(pw, 1))) + 2*kit.Space(2)
}

type Card struct {
	core  *core.Core
	close func()
	pick  func()

	Peek func()

	pic    int
	bitmap *ui.Bitmap
	copied time.Time
	md     markdown.Options
	scroll ui.ScrollState
}

func New(k *core.Core, close func(), pick func()) *Card {
	return &Card{core: k, close: close, pick: pick, pic: -1}
}

func (d *Card) View(c *ui.Context) {
	kit.Paper(c)
	p := kit.Colors(c)
	c.Root().Background(p.Paper)
	if id := d.core.Image.ID(); id != d.pic {
		d.pic, d.bitmap, d.scroll.Y = id, nil, 0
		if b, err := ui.DecodeBitmap(d.core.Image.PNG); err == nil {
			d.bitmap = b
		}
	}
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

func (d *Card) allow(c *ui.Context) {
	k, p := d.core, kit.Colors(c)
	k.CheckScreen()
	c.After(core.RefreshEvery)
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(kit.Space(2)).Padding(0, kit.Space(4), kit.Space(8)).Children(func() {
		ui.Text(c, locales.GuideSelectTitle()).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(p.Ink)
		switch {
		case k.CanSelect():
			if kit.Button(c, kit.Primary, kit.Small, assets.Picture, locales.TranslateScreen()).Margin(kit.Space(2), 0, 0).Clicked() {
				d.pick()
			}
		case k.Screen == core.PermissionDenied:
			kit.Caption(c, locales.GuideSelectSettings()).TextAlign(ui.Center)
			if kit.Button(c, kit.Primary, kit.Small, nil, locales.OpenSystemSettings()).Margin(kit.Space(2), 0, 0).Clicked() {
				kit.GuideScreenRecording()
			}
		default:
			kit.Caption(c, shared.OnEach(locales.ScreenSelectWhy(), locales.ScreenSelectWhy(), locales.ScreenSelectWhyLinux())).TextAlign(ui.Center)
			label := locales.ScreenSelectTurnOn()
			if k.ScreenAsking {
				label = locales.ScreenSelectAsking()
			}
			if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, assets.Picture, label), k.ScreenAsking || k.Screen == core.PermissionUnchecked).Margin(kit.Space(2), 0, 0).Clicked() {
				k.RequestScreen()
			}
		}
	})
}

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

func badge(c *ui.Context, icon *ui.SVG) ui.Element {
	p := kit.Colors(c)
	return ui.Box(c).Absolute().Right(kit.Space(3)).Bottom(kit.Space(3)).Size(24, 24).Radius(kit.Pill(24)).
		Background(p.Paper.Alpha(.92)).Center().Children(func() {
		ui.Icon(c, icon).FontSize(kit.TextCaption).TextColor(p.Ink)
	})
}

func hint(hovered bool) float32 {
	if hovered {
		return 1
	}
	return .7
}

func (d *Card) translation(c *ui.Context) {
	k, p := d.core, kit.Colors(c)
	im := k.Image
	switch {
	case im.Text != "":
		d.md.Theme = &markdown.Theme{FontSize: kit.TextBody, Text: p.Ink, LineHeight: 1.4}
		d.md.Extensions = parser.GFM
		d.md.Reveal = &markdown.Reveal{Unit: markdown.ByCharacter}
		d.md.ShareSelection = true
		if d.scroll.Y >= d.scroll.MaxY {
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
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(kit.Space(1)).Children(func() {
			if errors.Is(im.Err, translate.ErrEmpty) {
				kit.Warning(c, locales.ImageNoText())
				return
			}
			via, _ := k.Engine(core.MT, im.Via)
			why := im.Err.Error()
			var ne net.Error
			switch {
			case errors.Is(im.Err, core.ErrModelMissing) || errors.Is(im.Err, core.ErrModelsMoving) || errors.Is(im.Err, core.ErrModelBusy) || errors.Is(im.Err, core.ErrNoEndpoint) || errors.Is(im.Err, core.ErrNoVision) || errors.As(im.Err, new(core.LocalError)):
				why = shared.ErrorText(im.Err)
			case via.Cloud && (errors.As(im.Err, &ne) && ne.Timeout() || errors.Is(im.Err, translate.ErrStalled)):
				why = locales.TranslationFailedCloud()
			}
			if via.Endpoint != nil && !errors.Is(im.Err, core.ErrNoEndpoint) {
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

func waiting(c *ui.Context) {
	kit.Waiting(c, locales.ImageTranslating())
	ui.Column(c).Gap(kit.Space(3)).Padding(kit.Space(1), 0, 0).Children(func() {
		for i, w := range []float32{100, 86, 58} {
			bar := ui.Box(c.Key(i)).WidthPercent(w).Height(kit.TextCaption).Radius(kit.Pill(kit.TextCaption)).Background(kit.Colors(c).Sand)
			bar.Opacity(bar.Keyframes("breathe", []ui.Keyframe{
				{Value: .55},
				{At: 900 * time.Millisecond, Value: 1, Ease: ui.EaseInOut},
			}, ui.KeyframeOptions{Repeat: ui.RepeatForever, Alternate: true, Delay: time.Duration(i) * 150 * time.Millisecond}))
		}
	})
}

func lines(text string) string {
	var b strings.Builder
	code := false
	for line := range strings.SplitAfterSeq(text, "\n") {
		l, ended := strings.CutSuffix(line, "\n")
		switch t := strings.TrimLeft(l, " "); {
		case ended && len(l)-len(t) < 4 && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")):
			code = !code
		case ended && !code:
			line = l + "  \n"
		}
		b.WriteString(line)
	}
	return b.String()
}

func note(k *core.Core) string {
	id := k.Image.Via
	if id == "" {
		id = k.Preferences().ImageMT
	}
	via, _ := k.Engine(core.MT, id)
	switch {
	case via.Model != nil:
		return locales.ImageNoteLocal()
	case via.Endpoint != nil:
		return locales.ImageNoteEndpoint(shared.EngineName(via))
	}
	return locales.ImageNote()
}

func (d *Card) foot(c *ui.Context) {
	k := d.core
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
		kit.CaptionLink(c, note(k), "Index-Translate", core.CloudRepo).Grow(1).MaxLines(2).Tooltip(locales.ImageKept())
		label, icon := locales.Copy(), assets.Copy
		if left := d.copied.Add(1500 * time.Millisecond).Sub(c.Now()); left > 0 {
			label, icon = locales.Copied(), assets.Check
			c.After(left)
		}
		if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, icon, label), k.Image.Text == "").Clicked() {
			c.Services().WriteClipboard(k.Image.Text)
			d.copied = c.Now()
		}
	})
}
