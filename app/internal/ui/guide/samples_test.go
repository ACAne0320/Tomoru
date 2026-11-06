package guide

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/samples"
	"tomoru/internal/ui/kit"
)

// TestSamplePictures draws the sample game dialogue the guide translates as a picture, in each
// language, into internal/ui/assets/samples: TOMORU_SAMPLE_PICTURES=1 go test ./internal/ui/guide
// -run SamplePictures writes them again.
func TestSamplePictures(t *testing.T) {
	if os.Getenv("TOMORU_SAMPLE_PICTURES") == "" {
		t.Skip("TOMORU_SAMPLE_PICTURES is not set")
	}
	for lang, d := range samples.Dialogue {
		view := func(c *ui.Context) {
			p := kit.Dark
			c.Root().Background(p.Night)
			ui.Column(c).Fill().Padding(kit.Space(3)).Children(func() {
				ui.Column(c).Grow(1).Padding(kit.Space(4), kit.Space(5)).Gap(kit.Space(2)).Radius(kit.RadiusInner).
					Background(p.Shade.Alpha(.55)).Border(2, p.Starlight.Alpha(.75)).Children(func() {
					ui.Text(c, d[0]).FontSize(kit.TextBody + 2).FontWeight(kit.WeightBold).TextColor(p.SubTgt)
					ui.Text(c, d[1]).FontSize(kit.TextTitle - 2).TextColor(p.Starlight).LineHeight(1.35)
					ui.Spacer(c)
					ui.Text(c, "▼").FontSize(kit.TextCaption).TextColor(p.Starlight.Alpha(.7)).AlignSelf(ui.End)
				})
			})
		}
		tt := ui.NewTester(view, 520, 180)
		tt.SetScale(2)
		tt.Frame()
		f, err := os.Create(filepath.Join("..", "assets", "samples", "dialogue-"+lang+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
