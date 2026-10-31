package guide

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/live"
)

// TestGallery pictures the guide's steps, light and dark, one under another, to review them by
// eye: TOMORU_GALLERY=<dir> go test ./internal/ui/guide -run Gallery writes them there.
func TestGallery(t *testing.T) {
	dir := coretest.GalleryDir(t)
	if dir == "" {
		t.Skip("TOMORU_GALLERY is not set")
	}
	for _, dark := range []bool{false, true} {
		sheet := image.NewRGBA(image.Rect(0, 0, Width*2, Height*2*steps))
		for step := range steps {
			k := coretest.New()
			if !strings.HasPrefix(i18n.Locale(), "zh") {
				k.Prefs.Tgt = "en" // the stage's sample in the pictures' language, as the README shows them
			}
			k.Access = "granted"
			k.Screen = "unknown"
			switch step {
			case stepTry:
				k.Err, k.Running = nil, true
				k.State = live.State{Phase: "listening", Voice: true}
			case stepPicture:
				k.Image = core.ImageView{PNG: []byte{0}, Text: "艾露娜\n克劳德，塔顶上那个发光的东西……该不会就是「星之碎片」吧？"}
			}
			g := New(k, &coretest.Nav{})
			g.Step, g.next = step, step
			tt := ui.NewTester(g.View, Width, Height)
			tt.SetScale(2)
			tt.SetDark(dark)
			tt.Advance(2 * time.Second)
			tt.Frame()
			at := image.Pt(0, step*Height*2)
			draw.Draw(sheet, image.Rectangle{Min: at, Max: at.Add(tt.Image().Bounds().Size())}, tt.Image(), image.Point{}, draw.Src)
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("guide-%s.png", map[bool]string{false: "light", true: "dark"}[dark])))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, sheet); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
