package shell

import (
	"bytes"
	"image"
	_ "image/png" // for the picture's size
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/imagecard"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// cardGap is between the picture's card and what it is placed by.
const cardGap = 8

// showImage shows the picture's card, unless it shows: a panel, which floats over every app and
// takes the keyboard without making Tomoru the active app, and stays until it is closed, to be
// moved by its title. It goes under the part of the screen selected, or over it, by what it
// translates; for the clipboard's picture, by the menu bar icon (the taskbar's). It is as large as
// the user last resized it, as far as the screen has room; until they resize one, as tall as its
// picture needs. It never resizes itself once shown.
func (s *Shell) showImage() {
	if s.image != nil {
		return // it shows the new picture
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
		Content: s.view(d.View)})
	s.image = card
	resized := false // by the user: its size is kept, the screen's room for it not
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

// cardSize is the size of the picture's card: w by h as the user last resized it, else its own,
// as far as work has room for it.
func cardSize(w, h int, work mygo.Rectangle) (int, int) {
	if w <= 0 || h <= 0 {
		w, h = imagecard.Width, imagecard.Height
	}
	w = min(max(w, imagecard.MinWidth), work.Width-2*cardGap)
	h = min(max(h, imagecard.MinHeight), work.Height-2*cardGap)
	return w, h
}

// cardPlace is where a card w by h goes by at, in work: under it, its left edge on at's, or its
// middle on at's, centered; over it where work has no room under it and has over it. In work, as
// far as it fits. above tells it went over.
func cardPlace(at mygo.Rectangle, centered bool, w, h int, work mygo.Rectangle) (x, y int, above bool) {
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
		x = 1 // a window at 0, 0 is centered
	}
	return x, y, above
}

// selected is where the picture was selected, in DIPs: the engine says it in pixels, which are a
// display's DIPs times its scale.
func (s *Shell) selected() (mygo.Rectangle, bool) {
	return toDIPs(s.core.Image.Area, mygo.Screen.Displays())
}

// toDIPs is px, in the pixels of the display it starts on, in DIPs; false when empty.
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

// peek shows the picture whole in a window of its own, in the middle of the card's display, as
// large as it was on the screen, or as the screen has room for: a panel, as the card, that the
// user moves by the picture and resizes, the picture fitting it, and closes with its close button,
// which shows while the pointer is over it, or Escape, or with the card. Clicked again, the thumbnail brings it back to the front.
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
		Content: s.view(func(c *ui.Context) {
			c.Root().Background(kit.Colors(c).Paper)
			pane := ui.Box(c).Fill().DragWindow()
			pane.Children(func() {
				ui.Image(c, bitmap).Fill().Fit(ui.Contain)
				// Only while the pointer is over the picture, for it hides what is in the corner.
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

// pictureBounds is where a picture w by h pixels shows whole, in DIPs: as large as it was on d, or
// as nine tenths of its work area has room for, in its middle.
func pictureBounds(w, h int, d mygo.Display) mygo.Rectangle {
	scale := max(d.ScaleFactor, 1)
	wa := d.WorkArea
	pw, ph := float64(w)/scale, float64(h)/scale
	k := min(1, .9*float64(wa.Width)/pw, .9*float64(wa.Height)/ph)
	dw, dh := int(math.Round(pw*k)), int(math.Round(ph*k))
	return mygo.Rectangle{X: wa.X + (wa.Width-dw)/2, Y: wa.Y + (wa.Height-dh)/2, Width: dw, Height: dh}
}
