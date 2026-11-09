package kit

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// The gallery shows each of kit's components in its states, light and dark, to review them by eye:
// TOMORU_GALLERY=<dir> go test ./internal/ui/kit -run Gallery writes it there as pictures.

type gallery struct {
	on, off   bool
	seg, seg2 int
	clicks    int
	bar       int
	tile      int
	bin, day  int
	shown     float64
	text      string
	fade      ui.ScrollState
}

func (g *gallery) view(c *ui.Context) {
	Paper(c)
	p := Colors(c)
	section := func(name string, fn func()) {
		ui.Column(c).Gap(Space(3)).Children(func() {
			ui.Text(c, name).FontSize(TextCaption).FontWeight(WeightBold).TextColor(p.InkMuted)
			ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Wrap().Children(fn)
		})
	}
	ui.Column(c).Fill().Background(p.Paper).Padding(Space(6)).Gap(SectionGap).Children(func() {
		section("Button · Large", func() {
			if Button(c, Primary, Large, assets.Play, "开始翻译").Clicked() {
				g.clicks++
			}
			Button(c, Strong, Large, assets.Stop, "停止翻译")
		})
		section("Button · Medium", func() {
			Button(c, Primary, Medium, nil, "使用这个场景")
			Button(c, Strong, Medium, assets.Plus, "新建场景")
			Button(c, Plain, Medium, nil, "取消")
			Button(c, Soft, Medium, assets.Sparkles, "从字幕记录学习")
			Button(c, Quiet, Medium, nil, "稍后")
		})
		section("Button · Small", func() {
			Button(c, Primary, Small, nil, "改用 阿里云百炼")
			Button(c, Plain, Small, assets.Download, "下载")
			Button(c, Plain, Small, nil, "设置")
			Button(c, Soft, Small, assets.Sparkles, "学习专名")
			Off(c, Button(c, Plain, Small, nil, "测试"), true)
			Off(c, Button(c, Primary, Small, nil, "保存"), true)
		})
		section("IconButton", func() {
			IconButton(c, Plain, Medium, assets.Reset, "恢复默认样式")
			IconButton(c, Plain, Small, assets.Sliders, "管理场景")
			IconButton(c, Plain, Small, assets.Arrow, "查看全部")
			IconButton(c, Strong, Small, assets.Plus, "添加专名")
			IconButton(c, Soft, Small, assets.Sparkles, "学习专名")
			DeleteButton(c, Small, "删除")
			CancelButton(c, "取消下载")
			IconButton(c, Quiet, Small, assets.More, "更多操作")
		})
		section("Chip", func() {
			Chip(c, "通用", false, 0)
			Chip(c, "最终幻想7 重制版", true, 12)
			Chip(c, "雷军演讲", false, 0)
			Chip(c, "Running Man", false, 3)
		})
		section("Switch · Segmented", func() {
			Switch(c, &g.on, "抢先翻译")
			Switch(c, &g.off, "录屏时隐藏")
			Segmented(c, &g.seg, "带底框", "沉浸").Label("背景")
			Segmented(c, &g.seg2, "无", "描边", "阴影").Label("文字边缘")
		})
		section("Caption · Waiting · Warning · Levels", func() {
			Caption(c, "改了就生效")
			Waiting(c, "测试中…")
			Warning(c, "下载失败")
			MenuLevels(c, true)
			MenuLevels(c, false)
		})
		section("FadeScroll", func() {
			Card(c).Width(340).Height(120).Children(func() {
				FadeScroll(c, &g.fade, func() {
					for _, s := range []string{"本机", "Index-Translate", "Hy-MT2", "云端", "Index-Translate 35B", "你选择的云端服务会收到字幕的原文"} {
						Caption(c, s)
					}
				})
			})
		})
		section("BarChart · StatStrip", func() {
			Card(c).Width(340).Children(func() {
				var bars []Bar
				for i, v := range []float64{1.5, 3.2, math.NaN(), 0.4, 2.8, 4.1, 2.2} {
					bar := Bar{Label: fmt.Sprintf("10/%d", 15+i), Value: v, Tip: fmt.Sprintf("10/%d · %.1f 小时", 15+i, v)}
					if i >= 4 { // stacked, by engine
						bar.Parts = []BarPart{{v * .6, p.Series[0]}, {v * .3, p.Series[1]}, {v * .1, p.LineStrong}}
					}
					bars = append(bars, bar)
				}
				BarChart(c, "每天的运行时长", bars, func(v float64) string { return fmt.Sprintf("%gh", v) }, &g.bar)
			})
			StatStrip(c, "延迟", []Stat{{Label: "识别定稿", Value: "0.62", Unit: "秒", Note: "90% 在 1.1 秒内"}, {Label: "翻译首字", Value: "0.41", Unit: "秒"}}).Width(340)
			Card(c).Width(340).Children(func() {
				PickTiles(c, &g.tile, []TileGroup{
					{Label: "你等的", Tiles: []Tile{{Label: "说完到出译文", Value: "1.3", Unit: "秒", Strong: true}}},
					{Label: "识别", Tiles: []Tile{{Label: "识别首字", Value: "0.45", Unit: "秒"}, {Label: "识别定稿", Value: "—"}}},
				}).Label("延迟")
				var bins []Bin
				for i, share := range []float64{0, .02, .05, .1, .16, .2, .15, .1, .08, .06, .04, .02, .02} {
					bins = append(bins, Bin{Lo: float64(i) * .2, Hi: float64(i+1) * .2, Share: share, Tip: fmt.Sprintf("%.0f%%", share*100)})
				}
				Histogram(c, "说完到出译文", bins, []HistMark{{At: 1.1, Label: "一半在 1.1 秒内", Strong: true}, {At: 1.8, Label: "90% 在 1.8 秒内"}},
					func(v float64) string { return fmt.Sprintf("%g 秒", v) }, &g.bin)
				var ws []Whisker
				for i, v := range []float64{1.2, math.NaN(), 1.4, 1.1, 1.3} {
					ws = append(ws, Whisker{Label: fmt.Sprintf("10/%d", 15+i), Mid: v, Hi: v * 1.5, Tip: fmt.Sprintf("%.1f 秒", v)})
				}
				WhiskerChart(c, "每天", ws, func(v float64) string { return fmt.Sprintf("%g 秒", v) }, &g.day)
				ShareRow(c, Share{Color: p.Series[1], Name: "OpenRouter", Sub: "qwen/qwen3-30b-a3b", Value: "610", Unit: "句", Share: .3}, true)
				ShareRow(c, Share{Color: p.LineStrong, Name: "未记录引擎", Value: "210", Unit: "句", Share: .08, Muted: true}, false)
			})
			Card(c).Width(340).Children(func() {
				var days []HeatDay
				for i := range 7 * 8 {
					v := float64((i * 7) % 11)
					if i%5 == 0 || i > 7*8-3 {
						v = 0
					}
					days = append(days, HeatDay{Value: v, Tip: fmt.Sprintf("%d 句", int(v))})
				}
				days[len(days)-1].Value = math.NaN()
				Heatmap(c, "每日句数", days, []string{"9月", "", "", "", "10月", "", "", ""}, [7]string{0: "周一", 2: "周三", 4: "周五"}, "少", "多", func(int) {})
			})
			Facts(c, []Fact{{Label: "译文", Value: "6,039 字"}, {Label: "输入 tokens", Value: "130,423"}, {Label: "输出 tokens", Value: "11,424"}, {Label: "失败", Value: "1 句"}}).Width(340)
		})
		Groups(c, func() {
			Group(c, "外观", nil, RowsCard, func() {
				SettingRow(c, 0, "背景", "", func() { Segmented(c, &g.seg, "带底框", "沉浸").Label("背景") })
				SettingRow(c, 1, "抢先翻译", "话没说完先出大意", func() { Switch(c, &g.on, "抢先翻译 2") })
			}).Grow(1).Basis(300)
			Group(c, "文字", func() { IconButton(c, Plain, Small, assets.Reset, "恢复默认") }, RowsCard, func() {
				SettingRow(c, 0, "字体", "", func() { Picker(c, "系统默认", func(*ui.Menu) {}).Label("字体") })
				SettingRow(c, 1, "同时显示（条）", "", func() { Stepper(c, &g.shown, 1, 4, "少", "多") })
			}).Grow(1).Basis(300)
		})
		Group(c, "翻译", nil, OptionsCard, func() {
			Options(c, []Option{
				{Key: "2b", Title: "Index-Translate 2B", Meta: "1.6\u00a0GB · 需要 8\u00a0GB 内存", Action: func() { DeleteButton(c, Small, "删除 2B") }},
				{Key: "9b", On: true, Title: "Index-Translate 9B", Tag: "推荐", Accent: true, Meta: "6.0\u00a0GB · 需要 16\u00a0GB 内存",
					Action: func() { CancelButton(c, "取消 9B") },
					Below: func() {
						OptionProgress(c, .33, "2.0\u00a0GB / 6.0\u00a0GB")
						Caption(c, "下载完成后自动切换，在那之前继续用 Index-Translate 35B。")
					}},
				{Key: "q", Title: "Qwen3-ASR 1.7B", Tag: "系统自带", Meta: "2.3\u00a0GB · 用 GPU 运行", Action: func() { Button(c, Plain, Small, assets.Download, "下载 Q") }},
			})
			AddRow(c, "添加接口")
		})
		Group(c, "语音识别", nil, SplitCard, func() {
			SplitList(c, func() {
				ChoiceList(c, "语音识别", []Choice{
					{Key: "apple", Head: "本地", Title: "Apple 语音识别", Tag: "系统自带", Use: "使用 Apple", Meta: "延迟低，擅长日英韩", Progress: -1},
					{Key: "sv", On: true, Title: "SenseVoice", Use: "使用 SenseVoice", Meta: "Small", Progress: -1},
					{Key: "q3", Shown: true, Off: true, Title: "Qwen3-ASR", Tag: "推荐", Accent: true, Use: "使用 Qwen3-ASR", Meta: "下载中 · 0.6B", Progress: -1},
					{Key: "hy", Off: true, Title: "Hy-MT2", Use: "使用 Hy-MT2", Meta: "下载中 · 7B", Progress: .3},
					{Key: "qc", Head: "云端", Off: true, Title: "Qwen ASR（阿里云）", Use: "使用 Qwen ASR", Meta: "填好 API Key 后可用", Progress: -1},
					{Key: "fr", Title: "OpenAI", Use: "使用 OpenAI", Meta: "听不懂法语", Warn: true, Progress: -1},
				})
				AddRow(c, "添加接口")
			})
			SplitDetail(c, func() {
				OptionsCard(c).Children(func() {
					Options(c, []Option{
						{Key: "06", Off: true, Title: "0.6B", Meta: "0.9\u00a0GB · 需要 2\u00a0GB 显存", Action: func() { Button(c, Plain, Small, assets.Download, "下载 0.6B") },
							Below: func() { OptionProgress(c, .4, "0.4\u00a0GB / 0.9\u00a0GB") }},
						{Key: "17", On: true, Title: "1.7B", Tag: "推荐", Accent: true, Done: "已下载", Meta: "2.3\u00a0GB · 需要 4\u00a0GB 显存", Action: func() { DeleteButton(c, Small, "删除 1.7B") }},
						{Key: "9b", Off: true, Warn: true, Title: "9B", Meta: "需要 16\u00a0GB 内存，这台电脑只有 8\u00a0GB"},
					})
				})
			})
		})
		ui.Row(c).Gap(CardGap).AlignItems(ui.Start).Children(func() {
			OptionsCard(c).Width(300).Children(func() {
				List(c, []ListItem{
					{Key: 1, On: true, Title: "最终幻想7 重制版", Meta: []string{"日语 → 中文", "12 个专名"}, Trail: func() { InUse(c) }},
					{Key: 2, Title: "雷军演讲", Meta: []string{"中文 → 英语", "4 个专名"}},
					{Key: 3, Title: "Final Fantasy VII Remake, the stream", Meta: []string{"Japanese → Chinese", "412 lines", "Safari"}},
				})
			})
			Card(c).Width(300).Children(func() {
				InUseLabel(c, "正在使用")
				TextField(c, &g.text).Placeholder("deepseek-flash").Label("模型")
			})
		})
	})
}

const galleryW, galleryH = 760, 2800

func TestGallery(t *testing.T) {
	g := &gallery{on: true, seg2: 1, bar: 5, bin: 6, day: -1}
	tt := ui.NewTester(g.view, galleryW, galleryH)
	tt.SetScale(2)
	dir := os.Getenv("TOMORU_GALLERY")
	save := func(name string, img image.Image) {
		if dir == "" {
			return
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Advance(1e9)
		tt.Frame()
		name := "gallery-light.png"
		if dark {
			name = "gallery-dark.png"
		}
		save(name, tt.Image())
	}
	tt.SetDark(false)
	tt.Frame()

	// Each button at rest, hovered and pressed, side by side.
	labels := []string{"开始翻译", "停止翻译", "取消", "从字幕记录学习", "稍后", "最终幻想7 重制版", "通用"}
	var strip *image.RGBA
	const cw, ch, padX, padY = 300, 64, 44, 18
	if dir != "" {
		strip = image.NewRGBA(image.Rect(0, 0, 3*cw*2, len(labels)*ch*2))
		draw.Draw(strip, strip.Bounds(), image.White, image.Point{}, draw.Src)
	}
	for i, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q", label)
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		for j, state := range []string{"rest", "hover", "pressed"} {
			switch state {
			case "rest":
				tt.Move(1, 1)
			case "hover":
				tt.Move(x, y)
			case "pressed":
				tt.Press(x, y)
			}
			tt.Advance(3e8)
			tt.Frame()
			if strip != nil {
				src := image.Rect(int(r.X-padX)*2, int(r.Y-padY)*2, int(r.X+r.W+padX)*2, int(r.Y+r.H+padY)*2)
				at := image.Pt(j*cw*2, i*ch*2)
				draw.Draw(strip, image.Rectangle{Min: at, Max: at.Add(src.Size())}, tt.Image(), src.Min, draw.Src)
			}
			if state == "pressed" {
				tt.Release(x, y)
				tt.Frame()
			}
		}
	}
	if strip != nil {
		save("gallery-states.png", strip)
	}

	// What they do.
	clicks := g.clicks
	if err := tt.Click("开始翻译"); err != nil || g.clicks != clicks+1 {
		t.Errorf("button: %v, clicks %d", err, g.clicks)
	}
	if err := tt.Click("抢先翻译"); err != nil || g.on {
		t.Errorf("switch: %v, on %v", err, g.on)
	}
	if err := tt.Click("沉浸"); err != nil || g.seg != 1 {
		t.Errorf("segmented: %v, %d", err, g.seg)
	}
	if _, ok := tt.Find("恢复默认样式"); !ok {
		t.Error("an icon button is not named")
	}
}

// TestEmptyGallery draws each Art in its Empty, light and dark, as the pages have them: every one
// parses in either theme. TOMORU_GALLERY=<dir> writes empty-light.png and empty-dark.png there.
func TestEmptyGallery(t *testing.T) {
	view := func(c *ui.Context) {
		Paper(c)
		ui.Box(c).Fill().Background(Colors(c).Paper).Padding(Space(6)).Grid().ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(CardGap).Children(func() {
			Empty(c, ArtScenes, "还没有自己的场景。给常看的节目、游戏或会议建一个，分别设置语言和专名。", nil)
			Empty(c, ArtHistoryOff, "字幕记录没有开启。开启后，翻译过的字幕会保存在这台电脑上。", func() {
				Button(c, Primary, Small, nil, "开启保存记录")
			})
			Empty(c, ArtHistory, "去放一段视频或音频，翻译过的字幕会出现在这里。", nil)
			Empty(c, ArtStats, "还没有数据。翻译时放一段视频或音频，这里会显示用了多久、翻了多少、有多快。", nil)
			Empty(c, ArtTermsLearn, "还没有专名。可以从这个场景的字幕记录里学习，找出容易听错的名字。", nil)
			Empty(c, ArtTerms, "还没有专名。可以手动添加；这个场景有了字幕记录后，也能从记录里学习。", nil)
		})
	}
	tt := ui.NewTester(view, 1000, 640)
	tt.SetScale(2)
	dir := os.Getenv("TOMORU_GALLERY")
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		if dir == "" {
			continue
		}
		name := "empty-light.png"
		if dark {
			name = "empty-dark.png"
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
