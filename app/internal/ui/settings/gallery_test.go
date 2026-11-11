package settings

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
		d.GPU, d.Memory = func() uint64 { return 8 << 30 }, func() uint64 { return 16 << 30 }
		d.StatsPath = stats
		d.Version, d.OS = "0.1.4", "macOS 26.6.2"
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
	var mt string // the translation engine before the report's own
	// Each picture: its name, a height the whole page fits in, and what it shows, after the last.
	for _, shot := range []struct {
		name   string
		height int
		show   func()
	}{
		{"home", 1000, func() { s.Tab = "home" }},
		{"subtitles", 1300, func() { s.Tab = "subtitles" }},
		{"subtitles-tgt-only", 1300, func() { k.Prefs.Display = 1 }},
		{"subtitles-src-only", 1300, func() { k.Prefs.Display = prefs.DisplaySrc }},
		{"subtitles-custom", 1500, func() { k.Prefs.Display, k.Prefs.SrcColor, k.Prefs.TgtColor = 0, "#ffec8c", "#7fdbff" }},
		{"engines", 1900, func() {
			s.Tab, s.shown[core.ASR], s.shown[core.MT] = "engines", "Qwen3-ASR", "Index-Translate"
			k.SetEngine("local-2b")
		}},
		{"engines-cloud", 1900, func() {
			s.shown[core.ASR], s.shown[core.MT] = core.QwenCloud, "cloud"
			k.SetImageEngine("cloud")
		}},
		{"engines-service", 1900, func() { s.shown[core.MT] = core.HyMTCloud }},
		{"engines-service-set", 1900, func() { k.SaveAPI(prefs.Service{ID: core.HyMTCloud}, "sk"); s.svc[core.MT] = serviceEdit{} }},
		{"engines-api", 1900, func() {
			id := k.SaveEndpoint(prefs.Endpoint{Name: "Qwen-VL", URL: "https://dashscope.aliyuncs.com/compatible-mode", Model: "qwen-vl-max"}, "sk")
			s.show(core.MT, prefs.CustomPrefix+id)
			p := k.Preferences()
			ep, _ := p.Endpoint(id)
			s.editAPI(&s.api, *ep)
			k.Tests[s.api.test] = core.TestResult{Text: "大家晚上好。今天的直播会比平时长一点。", Took: 820 * time.Millisecond, Vision: prefs.VisionYes}
		}},
		{"engines-add", 1900, func() {
			s.show(core.MT, "")
			s.addingAPI = true
			s.editAPI(&s.api, prefs.Endpoint{})
		}},
		{"engines-add-narrow", 1900, func() {}}, // at the window's least width
		{"scenes", Height, func() { s.closeAPI(); s.Tab = "scenes" }},
		{"scenes-narrow", MinHeight, func() {}},
		{"history", Height, func() { s.Tab = "history" }},
		{"history-narrow", MinHeight, func() {}},
		{"history-empty", Height, func() { k.History = core.HistoryView{Loaded: true} }},
		{"stats", 1900, func() { k.History, s.Tab = records, "stats" }},
		{"general", 1000, func() { s.Tab = "general" }},
		{"about", Height, func() { s.Tab = "about" }},
		{"report", Height, func() { s.feedback = feedbackEdit{open: true} }},
		{"report-open", MinHeight, func() { // at the window's least height
			s.feedback = feedbackEdit{open: true, shown: true, what: "Watching Frieren on Crunchyroll, the subtitles stopped after ten minutes"}
		}},
		{"report-idea", Height, func() { s.feedback = feedbackEdit{open: true, kind: 1} }},
		{"report-narrow", Height, func() { // an API of one's own, its model's long name of no spaces
			s.feedback.shown, mt = false, k.Prefs.MT
			k.Prefs.Endpoints = append(k.Prefs.Endpoints, prefs.Endpoint{ID: "long", Name: "OpenRouter", Model: "qwen/qwen3-235b-a22b-instruct-2507:free-tier-preview-build-20261109"})
			k.Prefs.MT = "custom:long"
		}},
		{"scene-new", Height, func() {
			s.feedback.open, k.Prefs.MT = false, mt
			s.Tab, s.sceneEdit.scene = "scenes", k.AddScene("")
		}},
		{"home-error", 1000, func() { s.Tab, k.Err = "home", core.ErrMTCloudKey }},
		{"home-running", 1000, func() {
			s.Tab, k.Err, k.Running = "home", nil, true
			k.State = live.State{Phase: "listening", Stats: live.Stats{Lines: 412}}
		}},
	} {
		shot.show()
		width := Width
		if strings.HasSuffix(shot.name, "-narrow") {
			width = MinWidth
		}
		tt.SetSize(width, shot.height)
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
