package guide

import (
	"cmp"
	"github.com/egoist/mygo/ui"
	"time"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// sample is the sample picture as the stage shows it: its language's, decoded once.
type sample struct {
	lang   string
	bitmap *ui.Bitmap
}

// samplePNG is the sample game dialogue in the spoken language, which the picture step has the
// user select and translate.
func (g *Guide) samplePNG() []byte {
	if b := assets.SamplePicture(g.core.Preferences().Src); b != nil {
		return b
	}
	return assets.SamplePicture("en")
}

// sampleBitmap is the sample to show.
func (g *Guide) sampleBitmap() *ui.Bitmap {
	if lang := g.core.Preferences().Src; g.sample.lang != lang || g.sample.bitmap == nil {
		b, _ := ui.DecodeBitmap(g.samplePNG())
		g.sample = sample{lang, b}
	}
	return g.sample.bitmap
}

// picture offers selecting text on the screen to translate, for the user to want or skip, as it
// takes Screen Recording. Wanted, it asks for it; macOS has it turned on in System Settings, and the
// app reopened, which comes back to this step. On, the user chooses its shortcut and tries it on the
// sample on the stage, its translation coming under.
func (g *Guide) picture(c *ui.Context) {
	k := g.core
	// Answered, the system's prompt gives the front back to the app before: take it back.
	if k.ScreenAsking {
		g.asked = true
	} else if g.asked {
		g.asked = false
		g.nav.Focus()
	}
	k.CheckScreen() // turned on in System Settings meanwhile
	c.After(core.ListEvery)
	if k.CanSelect() {
		g.pictureTry(c)
		return
	}
	g.part(c, 0, func() { g.pictureAsk(c) })
}

// pictureAsk is the picture step before selecting is on: asking for Screen Recording, or the way
// to turn it on.
func (g *Guide) pictureAsk(c *ui.Context) {
	k := g.core
	title(c, locales.GuideSelectTitle())
	switch {
	case k.Screen == "denied":
		body(c, locales.GuideSelectSettings())
		if kit.Button(c, kit.Primary, kit.Small, nil, locales.OpenSystemSettings()).AlignSelf(ui.Start).Clicked() {
			kit.GuideScreenRecording()
		}
	default:
		body(c, locales.GuideSelectAsk())
		label := locales.ScreenSelectTurnOn()
		if k.ScreenAsking {
			label = locales.ScreenSelectAsking()
		}
		if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, assets.Picture, label), k.ScreenAsking || k.Screen == "").AlignSelf(ui.Start).Clicked() {
			k.RequestScreen()
		}
		note(c, locales.ScreenSelectWhy())
	}
}

// pictureTry is selecting, on: its shortcut, to change or keep, and two checks, done one after
// the other: pressing it, which lights its keys on the stage, then pressing it again to select the
// sample on the stage, which translates it for real, its translation under.
func (g *Guide) pictureTry(c *ui.Context) {
	k := g.core
	keys, works := kit.ShortcutText(k.Preferences().ImageShortcut), g.works(core.PictureShortcut)
	im := k.Image
	if im.PNG != nil && im.Text != "" {
		g.selected = im.Text
	}
	g.part(c, 0, func() {
		title(c, locales.GuideSelectTitle())
		body(c, locales.GuideSelectIntro())
	})
	g.part(c, 1, func() {
		shortcutRow(c, assets.Picture, locales.ImageShortcut(), func() {
			shared.ShortcutField(c, k, g.nav, core.PictureShortcut, locales.ImageShortcut(), &g.imageKeys)
		})
	})
	g.part(c, 2, func() { g.pictureChecks(c, keys, works) })
}

// pictureChecks are the picture step's two checks, done one after the other, and the sample's
// translation under them once selecting it worked.
func (g *Guide) pictureChecks(c *ui.Context, keys string, works bool) {
	k := g.core
	pal := kit.Colors(c)
	im := k.Image
	ui.Column(c).Gap(kit.Space(3)).Margin(6, 0, 0, 0).Children(func() {
		detail := ""
		if works {
			detail = locales.ShortcutWorks()
		}
		check(c, works, false, false, locales.GuideSelectPress(keys), detail)
		if !works {
			return // the next once this is done
		}
		done := g.selected != ""
		switch {
		case im.Running:
			detail = locales.ImageTranslating()
		case done:
			detail = ""
		case im.Err != nil:
			detail = locales.ImageFailed()
		default:
			detail = shared.OnWindows(locales.GuideSelectAllow(), "")
		}
		ui.Column(c.Key("select")).Transition(kit.Rise(0)).Children(func() {
			check(c, done, im.Running && !done, im.Err != nil && !done, locales.GuideSelectFrame(), detail)
		})
		if !done {
			return
		}
		ui.Column(c).Gap(kit.Space(2)).Padding(kit.Space(3), kit.Space(4)).Margin(0, 0, 0, kit.Space(7)).Radius(kit.RadiusTile).Background(pal.Card).Border(1, pal.Line).Children(func() {
			ui.Text(c, g.selected).FontSize(kit.TextBody).TextColor(pal.Ink).LineHeight(1.4).MaxLines(5)
		})
	})
}

// selectCorners draws the corners of a selection drawing themselves around r, over and over, in
// col: what to select, as selecting it will look. Still, they are there.
func selectCorners(p *ui.Painter, r ui.Rect, col ui.Color, still bool) {
	const (
		period = 2800 * time.Millisecond
		draw   = 520 * time.Millisecond
		fade   = 2200 * time.Millisecond
		gone   = 2600 * time.Millisecond
		arm    = 18
		width  = 2.5
	)
	k, alpha := float32(1), float32(1)
	if !still {
		t := time.Duration(p.Now().UnixMilli()%period.Milliseconds()) * time.Millisecond
		switch {
		case t < draw:
			k = float32(t) / float32(draw)
			k = 1 - (1-k)*(1-k)*(1-k) // easing out
		case t >= gone:
			p.After(period - t)
			return
		case t >= fade:
			alpha = 1 - float32(t-fade)/float32(gone-fade)
		}
		p.After(16 * time.Millisecond)
	}
	x0, y0, x1, y1 := r.X+width/2, r.Y+width/2, r.X+r.W-width/2, r.Y+r.H-width/2
	for _, c := range [][3]float32{{x0, y0, 1}, {x1, y0, -1}, {x1, y1, -1}, {x0, y1, 1}} {
		up := float32(1)
		if c[1] == y1 {
			up = -1
		}
		corner := (&ui.Path{}).MoveTo(c[0], c[1]+up*arm).LineTo(c[0], c[1]).LineTo(c[0]+c[2]*arm, c[1])
		p.StrokePath(corner.Trim(.5-k/2, .5+k/2), width, col.Alpha(alpha)) // from the corner out
	}
}

// pictureStage is the shortcut's keys while it is checked, else the sample game dialogue, as a game
// would show it on the screen, with a note over it to select it until it is.
func (g *Guide) pictureStage(c *ui.Context) {
	if g.core.CanSelect() && !g.works(core.PictureShortcut) {
		ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			keycaps(c, g.core.Preferences().ImageShortcut, false)
		})
		return
	}
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Padding(kit.Space(6)).Children(func() {
		b := g.sampleBitmap()
		if b == nil {
			return
		}
		const w = 340
		pw, ph := b.Size()
		pal := kit.Colors(c)
		translated := cmp.Or(g.core.Image.Text, g.selected)
		if translated == "" {
			onStage(c, locales.GuideSelectHere(), kit.TextControl, .9).FontWeight(kit.WeightBold).
				Background(pal.Glass).Radius(kit.Pill(28)).Padding(kit.Space(2), kit.Space(4)).Margin(0, 0, kit.Space(3), 0)
		}
		// Room around the dialogue for the corners showing what to select.
		ui.Box(c).Padding(kit.Space(2)).Draw(func(p *ui.Painter, r ui.Rect) {
			if translated == "" {
				selectCorners(p, r, pal.Lantern, c.Preferences().ReduceMotion)
			}
		}).Children(func() {
			glass(c, ui.Box(c).Padding(kit.Space(2))).Children(func() {
				ui.Image(c, b).Size(w, w*float32(ph)/float32(max(pw, 1))).Radius(kit.RadiusInner)
			})
		})
		if translated == "" {
			return
		}
		// Its translation unfolds under it, as it comes.
		glass(c, ui.Column(c.Key("translated")).Width(w+2*kit.Space(2)).Padding(kit.Space(3), kit.Space(4)).Clip()).
			Transition(ui.ElementTransition{Animation: &kit.Resize, Size: true, Enter: &ui.Motion{Collapse: true}}).Children(func() {
			kit.ArriveAfter(ui.Text(c, translated).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(pal.SubTgt).LineHeight(1.4).MaxLines(4), kit.Settle)
		})
	})
}
