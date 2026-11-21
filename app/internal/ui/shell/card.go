package shell

import (
	"bytes"
	"image"
	_ "image/png"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/imagecard"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

const cardGap = 8

func (s *Shell) showImage() {
	if s.image != nil {
		return
	}
	s.closePanel()
	at, selected := s.selected()
	centered := !selected
	if !selected {
		at = s.tray.Bounds()
	}
	work := mygo.Screen.DisplayMatching(at).WorkArea
	prefs := s.core.Preferences()
	w, h := cardSize(prefs.ImageW, prefs.ImageH, work)
	if pic, _, err := image.DecodeConfig(bytes.NewReader(s.core.Image.PNG)); err == nil && prefs.ImageH <= 0 {
		h = min(h, imagecard.FitHeight(w, pic.Width, pic.Height))
	}
	x, y, _ := cardPlace(at, centered, w, h, work)
	var card *mygo.Window
	d := imagecard.New(s.core, func() { card.Close() }, s.TranslateImage)
	d.Peek = func() { s.peek(card) }
	card = mygo.NewWindow(mygo.WindowOptions{Title: locales.ImageTitle(), X: x, Y: y, Width: w, Height: h,
		MinWidth: imagecard.MinWidth, MinHeight: imagecard.MinHeight,
		UseContentSize: true, Panel: true, TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true,
		Content: ui.View(d.View)})
	s.image = card
	resized := false
	card.OnResize(func() {
		if cw, ch := card.ContentSize(); cw != w || ch != h {
			w, h, resized = cw, ch, true
		}
	})
	card.OnClosed(func() {
		if resized {
			s.core.SetImageSize(w, h)
		}
		if s.picture != nil {
			s.picture.Close()
		}
		if s.image == card {
			s.image = nil
			s.core.CloseImage()
		}
	})
}

func cardSize(w, h int, work mygo.Rectangle) (int, int) {
	if w <= 0 || h <= 0 {
		w, h = imagecard.Width, imagecard.Height
	}
	w = min(max(w, imagecard.MinWidth), work.Width-2*cardGap)
	h = min(max(h, imagecard.MinHeight), work.Height-2*cardGap)
	return w, h
}

func cardPlace(at mygo.Rectangle, centered bool, w, h int, work mygo.Rectangle) (x, y int, above bool) {
	if at.Width == 0 && at.Height == 0 {
		return work.X + (work.Width-w)/2, work.Y + (work.Height-h)/2, false
	}
	x = at.X
	if centered {
		x = at.X + at.Width/2 - w/2
	}
	y = at.Y + at.Height + cardGap
	if y+h > work.Y+work.Height && at.Y-cardGap-h >= work.Y {
		y, above = at.Y-cardGap-h, true
	}
	x = max(work.X, min(x, work.X+work.Width-w))
	y = max(work.Y, min(y, work.Y+work.Height-h))
	if x == 0 && y == 0 {
		x = 1
	}
	return x, y, above
}

func (s *Shell) selected() (mygo.Rectangle, bool) {
	return toDIPs(s.core.Image.Area, mygo.Screen.Displays())
}

func toDIPs(px image.Rectangle, displays []mygo.Display) (mygo.Rectangle, bool) {
	if px.Empty() {
		return mygo.Rectangle{}, false
	}
	scale := 1.0
	for _, d := range displays {
		k := max(d.ScaleFactor, 1)
		x, y := float64(px.Min.X)/k, float64(px.Min.Y)/k
		b := d.Bounds
		if x >= float64(b.X) && x < float64(b.X+b.Width) && y >= float64(b.Y) && y < float64(b.Y+b.Height) {
			scale = k
			break
		}
	}
	dip := func(v int) int { return int(math.Round(float64(v) / scale)) }
	return mygo.Rectangle{X: dip(px.Min.X), Y: dip(px.Min.Y), Width: dip(px.Dx()), Height: dip(px.Dy())}, true
}

func (s *Shell) peek(card *mygo.Window) {
	if s.picture != nil {
		s.picture.Close()
	}
	png := s.core.Image.PNG
	pic, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		return
	}
	bitmap, err := ui.DecodeBitmap(png)
	if err != nil {
		return
	}
	d := mygo.Screen.DisplayMatching(card.ContentBounds())
	r := pictureBounds(pic.Width, pic.Height, d)
	var win *mygo.Window
	win = mygo.NewWindow(mygo.WindowOptions{Title: locales.ImageTitle(), X: r.X, Y: r.Y, Width: r.Width, Height: r.Height,
		MinWidth: 160, MinHeight: 100, UseContentSize: true, Panel: true, TitleBarStyle: mygo.TitleBarHidden,
		HideWindowButtons: true, DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true,
		Content: ui.View(func(c *ui.Context) {
			c.Root().Background(kit.Colors(c).Paper)
			pane := ui.Box(c).Fill().DragWindow()
			pane.Children(func() {
				ui.Image(c, bitmap).Fill().Fit(ui.Contain)
				if kit.PictureClose(c, assets.Close, locales.Close(), pane.Hovered()).Absolute().Top(kit.Space(2)).Right(kit.Space(2)).Clicked() {
					win.Close()
				}
			})
		})})
	s.picture = win
	win.OnClosed(func() {
		if s.picture == win {
			s.picture = nil
		}
	})
}

func pictureBounds(w, h int, d mygo.Display) mygo.Rectangle {
	scale := max(d.ScaleFactor, 1)
	wa := d.WorkArea
	pw, ph := float64(w)/scale, float64(h)/scale
	k := min(1, .9*float64(wa.Width)/pw, .9*float64(wa.Height)/ph)
	dw, dh := int(math.Round(pw*k)), int(math.Round(ph*k))
	return mygo.Rectangle{X: wa.X + (wa.Width-dw)/2, Y: wa.Y + (wa.Height-dh)/2, Width: dw, Height: dh}
}
