package settings

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core/coretest"
	"tomoru/internal/live"
	"tomoru/internal/scene"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/usage"
	"tomoru/locales"
)

// TestHome starts translating from the home page and shows its numbers while it does, not the
// subtitles, which show over the screen; and goes to the statistics from its lines by day.
func TestHome(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	tt := ui.NewTester(s.View, Width, Height)
	for _, want := range []string{locales.HomeIdle(), locales.StatsActivity(), locales.StatToday(), locales.Source(), locales.SpokenLanguage(), locales.TranslateInto()} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	if err := tt.Click(locales.StartTranslating()); err != nil || !k.Started {
		t.Fatalf("start: %v, started %v", err, k.Started)
	}

	// A session, as tests have none: two sentences in.
	k.Err, k.Running = nil, true
	k.State = live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "皆さん、こんばんは。", Tgt: "大家晚上好。", Done: true}},
		Stats: live.Stats{Lines: 12, Speech: 90, Chars: 345}}
	tt.Frame()
	for _, want := range []string{locales.HomeTranslating(), locales.StopTranslating(), locales.StatRunning(), "12", "1.5", "345"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("大家晚上好。") {
		t.Error("the subtitles here too, as they show over the screen")
	}
	if err := tt.Click(locales.HomeOpenStats()); err != nil || s.Tab != "stats" {
		t.Errorf("statistics: %v, tab %q", err, s.Tab)
	}
}

// TestAppearance chooses a dark appearance in Preferences, which keeps it.
func TestAppearance(t *testing.T) {
	k := coretest.New()
	s := New(k, &coretest.Nav{})
	s.Tab = "general"
	tt := ui.NewTester(s.View, Width, Height)
	if err := tt.Click(locales.AppearanceDark()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.Appearance != "dark" {
		t.Errorf("appearance %q", k.Prefs.Appearance)
	}
	if err := tt.Click(locales.AppearanceSystem()); err != nil {
		t.Fatal(err)
	}
	if k.Prefs.Appearance != "" {
		t.Errorf("appearance %q, following the system", k.Prefs.Appearance)
	}
}

// TestHomeScenes switches the scene from its menu over the sentence, whose languages are then the
// scene's, and goes to the scenes' page from it.
func TestHomeScenes(t *testing.T) {
	k := coretest.New()
	id := k.AddScene("Vtube")
	k.UpdateScene(id, func(s *scene.Scene) { s.Src, s.Tgt = "en", "zh" })
	s := New(k, &coretest.Nav{})
	tt := ui.NewTester(s.View, Width, Height)
	if err := tt.Click(locales.SceneGeneral()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Vtube · " + shared.Langs("en", "zh")); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	if k.Prefs.Scene != id || !tt.HasText(shared.LangName("en")) {
		t.Errorf("scene %q: %q", k.Prefs.Scene, tt.Texts())
	}
	if err := tt.Click("Vtube"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.ManageScenes() + "…"); err != nil || s.Tab != "scenes" {
		t.Errorf("manage: %v, tab %q", err, s.Tab)
	}
	// A new scene from the menu is in use, and shown on the scenes' page to be named.
	s.Tab = "home"
	tt.Frame()
	if err := tt.Click("Vtube"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.NewScene() + "…"); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	tt.Frame()
	if k.Prefs.Scene == id || s.Tab != "scenes" || s.sceneEdit.scene != k.Prefs.Scene {
		t.Errorf("in use %q, tab %q, shown %q", k.Prefs.Scene, s.Tab, s.sceneEdit.scene)
	}
}

// TestActivityDays lays the days out a week a column from Monday, today's the last with something,
// and leaves out what is before the first and after today.
func TestActivityDays(t *testing.T) {
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.Local) // a Wednesday
	var l usage.Ledger
	l.Add("2026-10-21", usage.Engines{}, usage.Counts{Lines: 2})
	l.Add("2026-10-19", usage.Engines{}, usage.Counts{Lines: 3})
	l.Add("2026-09-28", usage.Engines{}, usage.Counts{Lines: 4})
	l.Add("2026-09-27", usage.Engines{}, usage.Counts{Lines: 5}) // the Sunday before the first week
	from, lines := activityDays(l, 4, now)
	if from.Format(time.DateOnly) != "2026-09-28" || len(lines) != 28 {
		t.Fatalf("from %s, %d days", from.Format(time.DateOnly), len(lines))
	}
	if lines[0] != 4 || lines[21] != 3 || lines[23] != 2 || lines[22] != 0 {
		t.Errorf("lines %v", lines)
	}
	for i := 24; i < 28; i++ {
		if !math.IsNaN(lines[i]) {
			t.Errorf("day %d after today: %v", i, lines[i])
		}
	}
}

// TestActivityTip shows a day's date and lines with the pointer on its square, besides its label;
// and the day in the statistics, clicked.
func TestActivityTip(t *testing.T) {
	s := New(statsCore(t, sampleLedger(time.Now())), &coretest.Nav{})
	tt := ui.NewTester(s.View, Width, 1400)
	tt.Frame()
	// The days of no lines, each said once, as its square's label: and one again by the pointer.
	none := " · " + locales.HistoryLines(0)
	shown := func() int {
		n := 0
		for _, text := range tt.Texts() {
			if strings.HasSuffix(text, none) {
				n++
			}
		}
		return n
	}
	before := shown()
	if before == 0 {
		t.Fatalf("no day of no lines in %q", tt.Texts())
	}
	mon, ok := tt.Find(i18n.Weekdays(i18n.Locale(), i18n.Abbreviated, true)[time.Monday])
	if !ok {
		t.Fatalf("no weekdays in %q", tt.Texts())
	}
	tt.Move(mon.X+kit.Space(9)+6, mon.Y+mon.H/2) // past the weekdays' column, on the first week's square
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if n := shown(); n != before+1 {
		t.Errorf("%d days of no lines said with the pointer on one, %d before", n, before)
	}
	// Clicked, the day shows in the statistics, of all of them as it is months ago: before the
	// first, so no bar is.
	tt.ClickAt(mon.X+kit.Space(9)+6, mon.Y+mon.H/2)
	if s.Tab != "stats" || s.statsEdit.span != 2 || s.statsEdit.kind != 1 || s.statsEdit.bar != -1 {
		t.Errorf("tab %q, %+v", s.Tab, s.statsEdit)
	}
}

// TestStatsDay opens the statistics on a day of the last week, or of the last 30 days, its bar
// chosen, whatever the hour of the day asked and of now.
func TestStatsDay(t *testing.T) {
	y, m, d := time.Now().Date()
	for _, hour := range []int{0, 9, 15, 23} {
		now := time.Date(y, m, d, hour, 30, 0, 0, time.Local)
		s := New(statsCore(t, sampleLedger(now)), &coretest.Nav{})
		for _, c := range []struct{ ago, span, bar int }{{0, 0, 6}, {6, 0, 0}, {7, 1, 22}, {29, 1, 0}} {
			s.statsDay(now.AddDate(0, 0, -c.ago), now)
			if e := s.statsEdit; s.Tab != "stats" || e.span != c.span || e.bar != c.bar {
				t.Errorf("%d:30, %d days ago: tab %q, span %d, bar %d; want %d, %d", hour, c.ago, s.Tab, e.span, e.bar, c.span, c.bar)
			}
		}
	}
}
