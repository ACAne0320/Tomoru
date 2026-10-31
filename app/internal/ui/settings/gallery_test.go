package settings

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/history"
	"tomoru/internal/scene"
)

// TestGallery pictures the pages of the scenes and the history with names and sessions in them,
// and of the engines on a computer with a GPU, light and dark, to review them by eye: TOMORU_GALLERY=<dir> go test ./internal/ui/settings -run
// Gallery writes them there.
func TestGallery(t *testing.T) {
	dir := coretest.GalleryDir(t)
	if dir == "" {
		t.Skip("TOMORU_GALLERY is not set")
	}
	k := coretest.NewWith(func(d *core.Deps) {
		d.HistoryDir = t.TempDir()
		d.GPU = func() uint64 { return 8 << 30 }
	})
	ff7 := k.AddScene("最终幻想7 重制版")
	k.UpdateScene(ff7, func(sc *scene.Scene) {
		sc.About = "《最终幻想7 重制版》的游戏直播。主角是克劳德，同伴有蒂法、爱丽丝、巴雷特。"
	})
	for _, t := range []scene.Term{
		{Src: "クラウド", Tgt: "克劳德", Heard: []string{"黒ウド", "クロウド"}, Auto: true},
		{Src: "ティファ", Tgt: "蒂法"},
		{Src: "エアリス", Tgt: "爱丽丝", Heard: []string{"エアリ"}, Auto: true},
		{Src: "ミッドガル", Tgt: "米德加", Heard: []string{"ミッドガール"}},
	} {
		k.AddTerm(ff7, t)
	}
	k.AddScene("雷军演讲")
	k.SetScene(ff7)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 21, 4, 0, 0, time.Local)
	k.History = core.HistoryView{Loaded: true, Records: []history.Record{
		{Path: "/h/a.jsonl", Head: history.Head{Scene: ff7, Src: "ja", Tgt: "zh", Start: start, App: "Safari"}, Lines: 412},
		{Path: "/h/b.jsonl", Head: history.Head{Scene: "", Src: "en", Tgt: "zh", Start: start.Add(-6 * time.Hour), App: "Chrome"}, Lines: 96},
		{Path: "/h/c.jsonl", Head: history.Head{Scene: ff7, Src: "ja", Tgt: "zh", Start: start.AddDate(0, 0, -1)}, Lines: 704},
	}}
	lines := []history.Entry{
		{At: start.Add(12 * time.Second), Src: "皆さん、こんばんは。", Tgt: "大家晚上好。"},
		{At: start.Add(62 * time.Second), Src: "まずはクラウドの新しい武器を見ていきましょう。", Heard: "まずは黒ウドの新しい武器を見ていきましょう。", Tgt: "先来看看克劳德的新武器吧。"},
		{At: start.Add(135 * time.Second), Src: "じゃあミッドガルに向かいます。", Heard: "じゃあミッドガールに向かいます。", Tgt: "那我们出发去米德加。"},
		{At: start.Add(151 * time.Second), Src: "コメントありがとうございます！", Tgt: "谢谢大家的留言！"},
	}
	s := New(k, &coretest.Nav{})
	tt := ui.NewTester(s.View, Width, Height)
	tt.SetScale(2)
	for _, page := range []string{"scenes", "history", "engines"} {
		s.Tab = page
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
			k.History.Lines = lines // as read
			tt.Advance(time.Second)
			tt.Frame()
			name := page + "-light.png"
			if dark {
				name = page + "-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
