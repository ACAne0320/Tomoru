package kit

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestHeatmapHover lines the squares up in columns under the weeks' names, the days to come
// too; outlines the square under the pointer and moves none of the others; and picks its day
// clicked.
func TestHeatmapHover(t *testing.T) {
	days := make([]HeatDay, 7*10)
	for i := range days {
		days[i] = HeatDay{Value: float64(1 + i%5), Tip: "day"}
		if i >= 7*9+2 { // this week's, from Wednesday: to come
			days[i].Value = math.NaN()
		}
	}
	weeks := make([]string, 10)
	weeks[8] = "Oct"
	picked := -1
	// Narrow, so that the squares fill the rows below their most, as on a page.
	const width = 224
	sq := (width - 2*Space(4) - Space(8) - 10*heatGap) / 10 // the rows, less their padding, the weekdays and the gaps
	tt := ui.NewTester(func(c *ui.Context) {
		Paper(c)
		ui.Column(c).Fill().Padding(Space(4)).Background(Colors(c).Card).Children(func() {
			Heatmap(c, "days", days, weeks, [7]string{0: "Mon"}, "Less", "More", func(i int) { picked = i })
		})
	}, width, 360)
	tt.Frame()
	mon, ok := tt.Find("Mon")
	if !ok {
		t.Fatalf("no weekday in %q", tt.Texts())
	}
	week := sq + heatGap // a column, and its gap
	x, y := mon.X+Space(8)+heatGap+2*week+sq/2, mon.Y+mon.H/2
	// Where the ninth week's square starts on each day's row, as the card's white gives way to it:
	// the same on each, and where the week's name starts.
	img, card := tt.Image(), Light.Card
	oct, _ := tt.Find("Oct")
	for d := range 7 {
		row, start := int(y+float32(d)*week), -1
		white := func(px int) bool { c := img.RGBAAt(px, row); return c.R == card.R && c.G == card.G && c.B == card.B }
		for px := int(oct.X - week/2); px < int(oct.X+week/2); px++ {
			if white(px-1) && !white(px) {
				start = px
				break
			}
		}
		if diff := float32(start) - oct.X; start < 0 || diff < -1.5 || diff > 1.5 {
			t.Errorf("day %d: the ninth week starts at %d, its name at %g", d, start, oct.X)
		}
	}
	before := image.NewRGBA(tt.Image().Bounds())
	copy(before.Pix, tt.Image().Pix)
	tt.Move(x, y) // the third week's Monday
	tt.Frame()
	after := tt.Image()
	// Every pixel but the hovered square's as it was: no row grows, no square moves.
	changed := 0
	half := sq/2 + 2
	for py := before.Rect.Min.Y; py < before.Rect.Max.Y; py++ {
		for px := before.Rect.Min.X; px < before.Rect.Max.X; px++ {
			fx, fy := float32(px), float32(py)
			if fx > x-half && fx < x+half && fy > y-half && fy < y+half {
				continue
			}
			if before.RGBAAt(px, py) != after.RGBAAt(px, py) {
				changed++
			}
		}
	}
	if changed > 0 {
		t.Errorf("%d pixels beside the square hovered changed", changed)
	}
	tt.ClickAt(x, y)
	if picked != 14 {
		t.Errorf("picked %d, want the third week's Monday, 14", picked)
	}
}

// TestHeatmapFills spans a wide card with a year of squares, as on a maximized window, rather
// than leaving most of it empty.
func TestHeatmapFills(t *testing.T) {
	days := make([]HeatDay, 7*53)
	for i := range days {
		days[i] = HeatDay{Value: float64(i % 5), Tip: "day"}
	}
	const width = 1200
	tt := ui.NewTester(func(c *ui.Context) {
		Paper(c)
		ui.Column(c).Fill().Padding(Space(4)).Background(Colors(c).Card).Children(func() {
			Heatmap(c, "days", days, make([]string, 53), [7]string{0: "Mon"}, "Less", "More", nil)
		})
	}, width, 600)
	tt.Frame()
	mon, ok := tt.Find("Mon")
	if !ok {
		t.Fatalf("no weekday in %q", tt.Texts())
	}
	img, card, row := tt.Image(), Light.Card, int(mon.Y+mon.H/2)
	end := -1
	for px := range width {
		if c := img.RGBAAt(px, row); c.R != card.R || c.G != card.G || c.B != card.B {
			end = px
		}
	}
	if want := width - int(Space(4)); end < want-3 {
		t.Errorf("the squares end at %d, want about %d", end, want)
	}
}
