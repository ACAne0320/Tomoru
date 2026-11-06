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
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
)

// TestGallery pictures every page, with scenes, names, sessions and three weeks of statistics in
// them, on a computer with a GPU and local models, light and dark, to review them by eye:
// TOMORU_GALLERY=<dir> go test ./internal/ui/settings -run Gallery writes them there.
func TestGallery(t *testing.T) {
	dir := coretest.GalleryDir(t)
	if dir == "" {
		t.Skip("TOMORU_GALLERY is not set")
	}
	now := time.Now()
	stats := filepath.Join(t.TempDir(), "stats.json")
	if err := sampleLedger(now).Save(stats); err != nil {
		t.Fatal(err)
	}
	k := coretest.NewWith(func(d *core.Deps) {
		d.HistoryDir = t.TempDir()
		d.GPU = func() uint64 { return 8 << 30 }
		d.StatsPath = stats
		d.Models = &coretest.Models{St: map[string]models.Status{
			"sensevoice-small": {State: models.Ready},
			"local-2b":         {State: models.Ready},
			"local-9b":         {State: models.Downloading, Done: 2 << 30, Total: 6 << 30},
		}}
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
	start := time.Date(now.Year(), now.Month(), now.Day(), 21, 4, 0, 0, time.Local)
	records := core.HistoryView{Loaded: true, Records: []history.Record{
		{Path: "/h/a.jsonl", Head: history.Head{Scene: ff7, Src: "ja", Tgt: "zh", Start: start, App: "Safari"}, Lines: 412},
		{Path: "/h/b.jsonl", Head: history.Head{Scene: "", Src: "en", Tgt: "zh", Start: start.Add(-6 * time.Hour), App: "Chrome"}, Lines: 96},
		{Path: "/h/c.jsonl", Head: history.Head{Scene: ff7, Src: "ja", Tgt: "zh", Start: start.AddDate(0, 0, -1)}, Lines: 704},
	}}
	k.History = records
	lines := []history.Entry{
		{At: start.Add(12 * time.Second), Src: "皆さん、こんばんは。", Tgt: "大家晚上好。"},
		{At: start.Add(62 * time.Second), Src: "まずはクラウドの新しい武器を見ていきましょう。", Heard: "まずは黒ウドの新しい武器を見ていきましょう。", Tgt: "先来看看克劳德的新武器吧。"},
		{At: start.Add(135 * time.Second), Src: "じゃあミッドガルに向かいます。", Heard: "じゃあミッドガールに向かいます。", Tgt: "那我们出发去米德加。"},
		{At: start.Add(151 * time.Second), Src: "コメントありがとうございます！", Tgt: "谢谢大家的留言！"},
	}
	s := New(k, &coretest.Nav{})
	tt := ui.NewTester(s.View, Width, Height)
	tt.SetScale(2)
	// Each picture: its name, a height the whole page fits in, and what it shows, after the last.
	for _, shot := range []struct {
		name   string
		height int
		show   func()
	}{
		{"home", 1000, func() { s.Tab = "home" }},
		{"subtitles", 1300, func() { s.Tab = "subtitles" }},
		{"subtitles-tgt-only", 1300, func() { k.Prefs.Display = 1 }},
		{"subtitles-custom", 1500, func() { k.Prefs.Display, k.Prefs.SrcColor, k.Prefs.TgtColor = 0, "#ffec8c", "#7fdbff" }},
		{"engines", 1200, func() { s.Tab, s.sides[core.ASR], s.sides[core.MT] = "engines", true, true }},
		{"engines-local", 1200, func() { s.sides[core.ASR], s.sides[core.MT] = false, false; k.SetImageEngine("local-2b") }},
		{"endpoint", Height, func() { s.sides[core.MT] = true; k.SetImageEngine("cloud"); s.editEndpoint(prefs.Endpoint{}) }},
		{"endpoint-tested", Height, func() {
			s.editEndpoint(prefs.Endpoint{Name: "Qwen-VL", URL: "https://dashscope.aliyuncs.com/compatible-mode", Model: "qwen-vl-max"})
			k.Tests[s.edit.test] = core.TestResult{Text: "大家晚上好。今天的直播会比平时长一点。", Took: 820 * time.Millisecond, Vision: prefs.VisionYes}
		}},
		{"endpoint-text", Height, func() {
			k.Tests[s.edit.test] = core.TestResult{Text: "大家晚上好。今天的直播会比平时长一点。", Took: 640 * time.Millisecond, Vision: prefs.VisionNo}
		}},
		{"scenes", Height, func() { s.edit.open, s.Tab = false, "scenes" }},
		{"history", Height, func() { s.Tab = "history" }},
		{"history-empty", Height, func() { k.History = core.HistoryView{Loaded: true} }},
		{"stats", 1900, func() { k.History, s.Tab = records, "stats" }},
		{"general", 1000, func() { s.Tab = "general" }},
		{"about", Height, func() { s.Tab = "about" }},
		{"scene-new", Height, func() { s.Tab, s.sceneEdit.scene = "scenes", k.AddScene("") }},
		{"home-running", 1000, func() {
			s.Tab, k.Err, k.Running = "home", nil, true
			k.State = live.State{Phase: "listening", Stats: live.Stats{Lines: 412}}
		}},
	} {
		shot.show()
		tt.SetSize(Width, shot.height)
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
			k.History.Lines = lines // as read
			tt.Advance(time.Second)
			tt.Frame()
			name := shot.name + "-light.png"
			if dark {
				name = shot.name + "-dark.png"
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
