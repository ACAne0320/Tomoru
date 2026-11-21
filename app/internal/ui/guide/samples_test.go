package guide

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/kit"
)

type dialogue struct{ Speaker, Line string }

var dialogues = map[string]dialogue{
	"zh": {Speaker: "艾露娜", Line: "克劳德，塔顶上那个发光的东西……该不会就是「星之碎片」吧？"},
	"ja": {Speaker: "エルナ", Line: "クロード、あの塔のてっぺんで光ってるもの……まさか「星の欠片」じゃないよね？"},
	"en": {Speaker: "Erna", Line: "Claude, that light on top of the tower… it can't be the Star Fragment, can it?"},
	"ko": {Speaker: "에르나", Line: "클로드, 저 탑 꼭대기에서 빛나는 거… 설마 ‘별의 조각’은 아니겠지?"},
}

func TestSamplePictures(t *testing.T) {
	if os.Getenv("TOMORU_SAMPLE_PICTURES") == "" {
		t.Skip("TOMORU_SAMPLE_PICTURES is not set")
	}
	for lang, d := range dialogues {
		view := func(c *ui.Context) {
			p := kit.Dark
			c.Root().Background(p.Night)
			ui.Column(c).Fill().Padding(kit.Space(3)).Children(func() {
				ui.Column(c).Grow(1).Padding(kit.Space(4), kit.Space(5)).Gap(kit.Space(2)).Radius(kit.RadiusInner).
					Background(p.Shade.Alpha(.55)).Border(2, p.Starlight.Alpha(.75)).Children(func() {
					ui.Text(c, d.Speaker).FontSize(kit.TextBody + 2).FontWeight(kit.WeightBold).TextColor(p.SubTgt)
					ui.Text(c, d.Line).FontSize(kit.TextTitle - 2).TextColor(p.Starlight).LineHeight(1.35)
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
