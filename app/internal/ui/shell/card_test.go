package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo"

	"tomoru/internal/ui/imagecard"
)

// TestCardPlace puts the card under what it translates, over it at the work area's foot, and in
// the work area.
func TestCardPlace(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}
	for _, c := range []struct {
		name     string
		at       mygo.Rectangle
		centered bool
		x, y     int
		above    bool
	}{
		{"under", mygo.Rectangle{X: 100, Y: 200, Width: 300, Height: 80}, false, 100, 288, false},
		{"over, at the foot", mygo.Rectangle{X: 100, Y: 700, Width: 300, Height: 120}, false, 100, 372, true},
		{"in from the right edge", mygo.Rectangle{X: 1300, Y: 200, Width: 100, Height: 50}, false, 1000, 258, false},
		{"under the menu bar icon", mygo.Rectangle{X: 1200, Y: 0, Width: 24, Height: 24}, true, 992, 32, false},
		{"no room either way", mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}, false, 0, 580, false},
	} {
		x, y, above := cardPlace(c.at, c.centered, 440, 320, work)
		if x != c.x || y != c.y || above != c.above {
			t.Errorf("%s: %d, %d, above %v; want %d, %d, %v", c.name, x, y, above, c.x, c.y, c.above)
		}
	}
}

// TestToDIPs divides the pixels by the scale of the display they are on.
func TestToDIPs(t *testing.T) {
	displays := []mygo.Display{
		{Bounds: mygo.Rectangle{X: 0, Y: 0, Width: 1440, Height: 900}, ScaleFactor: 2},
		{Bounds: mygo.Rectangle{X: 1440, Y: 0, Width: 1920, Height: 1080}, ScaleFactor: 1},
	}
	if r, ok := toDIPs(image.Rect(200, 400, 800, 640), displays); !ok || r != (mygo.Rectangle{X: 100, Y: 200, Width: 300, Height: 120}) {
		t.Errorf("Retina: %v %v", r, ok)
	}
	if r, _ := toDIPs(image.Rect(3000, 100, 3300, 200), displays); r != (mygo.Rectangle{X: 3000, Y: 100, Width: 300, Height: 100}) {
		t.Errorf("second display: %v", r)
	}
	if _, ok := toDIPs(image.Rectangle{}, displays); ok {
		t.Error("no area placed")
	}
}

// TestCardSize sizes the card as the user last resized it, its own size before, within the least it
// is resized to and the room of the screen.
func TestCardSize(t *testing.T) {
	work := mygo.Rectangle{Width: 1440, Height: 875}
	for _, c := range []struct{ w, h, wantW, wantH int }{
		{0, 0, imagecard.Width, imagecard.Height},
		{600, 700, 600, 700},
		{100, 100, imagecard.MinWidth, imagecard.MinHeight},
		{2000, 2000, 1440 - 2*cardGap, 875 - 2*cardGap},
	} {
		if w, h := cardSize(c.w, c.h, work); w != c.wantW || h != c.wantH {
			t.Errorf("%dx%d: %dx%d, want %dx%d", c.w, c.h, w, h, c.wantW, c.wantH)
		}
	}
}

// TestPictureBounds shows a picture as large as it was on a Retina display, in the middle of the
// work area, and one larger than the screen at nine tenths of it, its shape kept.
func TestPictureBounds(t *testing.T) {
	d := mygo.Display{ScaleFactor: 2, WorkArea: mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}}
	if r := pictureBounds(1040, 360, d); r != (mygo.Rectangle{X: 460, Y: 372, Width: 520, Height: 180}) {
		t.Errorf("as it was: %+v", r)
	}
	if r := pictureBounds(5120, 2880, d); r != (mygo.Rectangle{X: 72, Y: 98, Width: 1296, Height: 729}) {
		t.Errorf("the whole screen: %+v", r)
	}
}
