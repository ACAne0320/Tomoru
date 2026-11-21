package settings

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/core/coretest"
	"tomoru/internal/models"
	"tomoru/internal/ui/kit"
	"tomoru/internal/usage"
	"tomoru/locales"
)

func sampleLedger(now time.Time) usage.Ledger {
	var l usage.Ledger
	apple := usage.Engines{ASR: "apple", MT: "cloud"}
	cloud := usage.Engines{ASR: core.QwenCloud, ASRModel: "qwen3-asr-flash-realtime", MT: "custom:or", MTName: "OpenRouter", MTModel: "qwen/qwen3-30b-a3b"}
	for i := 20; i >= 0; i-- {
		if i%6 == 4 {
			continue
		}
		hours := 1 + 2.5*math.Abs(math.Sin(float64(i)))
		var c usage.Counts
		c.Run = hours * 3600
		c.Lines = int(hours * 70)
		c.Speech = c.Run * .55
		c.Chars = c.Lines * 14
		for j := range c.Lines {
			wait := time.Duration(900+300*math.Sin(float64(i+j))) * time.Millisecond
			c.Latency.FirstText.Add(wait / 2)
			c.Latency.Final.Add(wait * 2 / 3)
			c.Latency.FirstTranslation.Add(wait / 3)
			c.Latency.Translation.Add(wait * 3 / 4)
			c.Latency.EndToEnd.Add(wait + 400*time.Millisecond)
		}
		e := apple
		if i < 4 {
			e = cloud
			c.Audio = c.Run * .7
			c.InTokens, c.OutTokens, c.Failed = c.Lines*96, c.Lines*18, 2
		}
		l.Add(usage.DayOf(now.AddDate(0, 0, -i)), e, c)
	}
	l.Add(usage.DayOf(now.AddDate(0, 0, -2)), usage.Engines{}, usage.Counts{Run: 5400, Lines: 210, Chars: 2900})
	l.Add(usage.DayOf(now), usage.Engines{ASR: "apple", MT: "local-2b"}, usage.Counts{Run: 18})
	return l
}

func statsCore(t *testing.T, l usage.Ledger) *core.Core {
	path := filepath.Join(t.TempDir(), "stats.json")
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	return coretest.NewWith(func(d *core.Deps) { d.StatsPath = path })
}

func TestStats(t *testing.T) {
	s := New(statsCore(t, usage.Ledger{}), &coretest.Nav{})
	s.Tab = "stats"
	tt := ui.NewTester(s.View, Width, Height)
	tt.Frame()
	if !tt.HasText(locales.StatsEmpty()) {
		t.Errorf("none yet: %q", tt.Texts())
	}

	s = New(statsCore(t, sampleLedger(time.Now())), &coretest.Nav{})
	s.Tab = "stats"
	tt = ui.NewTester(s.View, Width, 2000)
	tt.Frame()
	has := func(what string, want ...string) {
		t.Helper()
		for _, w := range want {
			if !tt.HasText(w) {
				t.Errorf("%s: no %q in %q", what, w, tt.Texts())
			}
		}
	}
	if vs := strings.TrimSpace(locales.StatsVsBefore(7, "")); !slices.ContainsFunc(tt.Texts(), func(text string) bool { return strings.Contains(text, vs) }) {
		t.Errorf("no %q in %q", vs, tt.Texts())
	}
	has("compared", locales.LatSame(7))
	has("translation", locales.StatLines(), "OpenRouter", "qwen/qwen3-30b-a3b", locales.EngineUnknown(), locales.FactInTokens(), locales.LatEndToEnd())
	split := func() int {
		n := 0
		for _, text := range tt.Texts() {
			if strings.Contains(text, "\n") {
				n++
			}
		}
		return n
	}
	if split() == 0 {
		t.Errorf("no day of two engines in %q", tt.Texts())
	}
	if err := tt.Click("OpenRouter"); err != nil || !s.statsEdit.picked || s.statsEdit.engine != "custom:or" {
		t.Errorf("picked: %v, %+v", err, s.statsEdit)
	}
	tt.Frame()
	if n := split(); n != 0 {
		t.Errorf("%d days of two engines with one picked", n)
	}
	if err := tt.Click("OpenRouter"); err != nil || s.statsEdit.picked {
		t.Errorf("picked again: %v, %+v", err, s.statsEdit)
	}
	tt.Frame()
	if tt.HasText(locales.EngineApple()) {
		t.Errorf("recognition's engines with translation's: %q", tt.Texts())
	}
	if err := tt.Click(locales.EnginesAsr()); err != nil || s.statsEdit.kind != 0 {
		t.Errorf("recognition: %v, kind %d", err, s.statsEdit.kind)
	}
	tt.Frame()
	has("recognition", locales.StatRun(), locales.EngineApple(), locales.EngineQwenAsr(), "qwen3-asr-flash-realtime", locales.FactAudio())
	has("latency", locales.LatEndToEndDescription(), locales.LatYouWait())
	if err := tt.Click(locales.EnginesMt()); err != nil {
		t.Error(err)
	}
	tt.Frame()
	if err := tt.Click(locales.LatFinal()); err != nil || s.statsEdit.lat != 2 {
		t.Errorf("recognition settled: %v, latency %d", err, s.statsEdit.lat)
	}
	tt.Frame()
	has("recognition settled", locales.LatFinalDescription())
	spread, ok := tt.Find(locales.StatsSpread())
	if !ok {
		t.Fatalf("no %q in %q", locales.StatsSpread(), tt.Texts())
	}
	tt.ClickAt(spread.X+spread.W+kit.Space(10), spread.Y+spread.H/2)
	if s.statsEdit.latView != 1 {
		t.Errorf("day by day: view %d", s.statsEdit.latView)
	}
	tt.Frame()
	if err := tt.Click(locales.StatsRangeAll()); err != nil || s.statsEdit.span != 2 {
		t.Errorf("all of them: %v, span %d", err, s.statsEdit.span)
	}
}

func TestStatsBefore(t *testing.T) {
	all := func(usage.Use) bool { return true }
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.Local)
	var l usage.Ledger
	l.Add("2026-10-21", usage.Engines{}, usage.Counts{Lines: 1})
	l.Add("2026-10-15", usage.Engines{}, usage.Counts{Lines: 2})
	l.Add("2026-10-14", usage.Engines{}, usage.Counts{Lines: 4})
	l.Add("2026-10-08", usage.Engines{}, usage.Counts{Lines: 8})
	l.Add("2026-10-07", usage.Engines{}, usage.Counts{Lines: 16})
	if sum, ok := statsBefore(l, 7, now, all); !ok || sum.Lines != 12 {
		t.Errorf("the week before: %d lines, %v", sum.Lines, ok)
	}
	if _, ok := statsBefore(l, 0, now, all); ok {
		t.Error("before all of them")
	}
	if _, ok := statsBefore(l, 30, now.AddDate(0, 0, -14), all); ok {
		t.Error("before the first day")
	}
	for _, c := range []struct {
		now, then float64
		want      string
	}{{118, 100, "+18%"}, {88, 100, "−12%"}, {100, 100, "+0%"}} {
		if got := change(c.now, c.then); got != c.want {
			t.Errorf("change(%g, %g) = %q, want %q", c.now, c.then, got, c.want)
		}
	}
}

func TestEngineColors(t *testing.T) {
	p := &kit.Light
	mt := func(u usage.Use) string { return u.MT }
	a, b := usage.Use{Engines: usage.Engines{MT: "cloud"}}, usage.Use{Engines: usage.Engines{MT: "custom:or"}}
	ab, ba := engineColors(p, []usage.Use{a, b}, mt), engineColors(p, []usage.Use{b, a}, mt)
	if ab[0] != ba[1] || ab[1] != ba[0] || ab[0] != p.Bilibili[0] {
		t.Errorf("by rank: %v, %v", ab, ba)
	}
	if !slices.Contains(p.Series[:], engineColor(p, "custom:or")) || engineColor(p, "") != p.LineStrong {
		t.Error("one's own API, or none noted")
	}
	for id, want := range map[string]ui.Color{
		"local-2b": p.Bilibili[2], "local-9b": p.Bilibili[1],
		"qwen3-asr-0.6b": p.QwenPale, "qwen3-asr-1.7b": p.Qwen, core.QwenCloud: p.Qwen, "sensevoice-small": p.Alibaba,
	} {
		if got := engineColor(p, id); got != want {
			t.Errorf("%s: %v, want its maker's %v", id, got, want)
		}
	}
	for i, a := range models.All {
		for _, b := range models.All[i+1:] {
			if a.Family == b.Family && engineColor(p, a.ID) == engineColor(p, b.ID) {
				t.Errorf("%s and %s alike", a.ID, b.ID)
			}
		}
	}
}

func TestStatsBuckets(t *testing.T) {
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.Local)
	var l usage.Ledger
	l.Add("2026-10-15", usage.Engines{}, usage.Counts{Lines: 1})
	l.Add("2026-10-21", usage.Engines{}, usage.Counts{Lines: 2})
	l.Add("2026-06-02", usage.Engines{}, usage.Counts{Lines: 4})
	bs, byMonth := statsBuckets(l, 7, now)
	if len(bs) != 7 || byMonth || bs[0].label != "10/15" || len(bs[0].days) != 1 || len(bs[6].days) != 1 || bs[6].label != "10/21" {
		t.Errorf("7 days: %+v", bs)
	}
	bs, byMonth = statsBuckets(l, 0, now)
	if !byMonth || len(bs) != 5 || bs[0].label != "2026/6" || len(bs[4].days) != 2 {
		t.Errorf("all of them: %+v", bs)
	}
}

func TestStatsGallery(t *testing.T) {
	dir := os.Getenv("TOMORU_GALLERY")
	if dir == "" {
		t.Skip("TOMORU_GALLERY is not set")
	}
	s := New(statsCore(t, sampleLedger(time.Now())), &coretest.Nav{})
	s.Tab = "stats"
	s.statsEdit.span, s.statsEdit.bar = 1, 27
	tt := ui.NewTester(s.View, Width, 1500)
	tt.SetScale(2)
	for kind, name := range []string{"asr", "mt"} {
		s.statsEdit.kind = kind
		for _, theme := range []string{"light", "dark"} {
			tt.SetDark(theme == "dark")
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, "stats-"+name+"-"+theme+".png"))
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

func TestStatsImages(t *testing.T) {
	now := time.Now()
	var l usage.Ledger
	l.Add(usage.DayOf(now), usage.Engines{ASR: "apple", MT: "cloud"}, usage.Counts{Run: 3600, Lines: 120, Chars: 1500})
	l.Add(usage.DayOf(now), usage.Engines{MT: "cloud"}, usage.Counts{Images: 4})
	s := New(statsCore(t, l), &coretest.Nav{})
	s.Tab = "stats"
	tt := ui.NewTester(s.View, Width, 1400)
	if tt.HasText(locales.EngineUnknown()) {
		t.Errorf("a recognizer's row for the pictures: %q", tt.Texts())
	}
	if err := tt.Click(locales.EnginesMt()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !hasTextContaining(tt, locales.ImageTitle()+" "+locales.UseImages(4)) || !tt.HasText(locales.UseImages(4)) || tt.HasText(locales.EngineUnknown()) {
		t.Errorf("texts %q", tt.Texts())
	}
}

func TestStatsUntranslated(t *testing.T) {
	now := time.Now()
	var l usage.Ledger
	l.Add(usage.DayOf(now), usage.Engines{ASR: "apple", MT: "cloud"}, usage.Counts{Run: 3600, Lines: 120, Chars: 1500})
	l.Add(usage.DayOf(now), usage.Engines{ASR: "apple", Untranslated: true}, usage.Counts{Run: 1800, Lines: 50})
	s := New(statsCore(t, l), &coretest.Nav{})
	s.Tab = "stats"
	tt := ui.NewTester(s.View, Width, 1400)
	if err := tt.Click(locales.EnginesAsr()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("1.5") || !tt.HasText(locales.EngineApple()) {
		t.Errorf("listening: %q", tt.Texts())
	}
	if err := tt.Click(locales.EnginesMt()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(kit.Count(120)) || tt.HasText(kit.Count(170)) || tt.HasText(locales.EngineUnknown()) {
		t.Errorf("translation: %q", tt.Texts())
	}
}
