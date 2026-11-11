package core

import (
	"path/filepath"
	"testing"
	"time"

	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/history"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/usage"
)

// TestToday counts what each session did into the day's numbers, keeps them for the next launch,
// and starts afresh the next day.
func TestToday(t *testing.T) {
	c, f := newCore(t)
	now := time.Date(2026, 10, 20, 21, 0, 0, 0, time.Local)
	c.deps.Now = func() time.Time { return now }
	c.deps.StatsPath = filepath.Join(t.TempDir(), "stats.json")
	c.Prefs.Onboarded = true

	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "listening", Stats: live.Stats{Lines: 3, Speech: 20, Chars: 40}})
	f.flush(t, 1)
	now = now.Add(90 * time.Second)
	if c.RunFor() != 90*time.Second {
		t.Errorf("ran for %v", c.RunFor())
	}
	if got := c.TodayStats(); got.Run != 90 || got.Lines != 3 || got.Chars != 40 {
		t.Errorf("today while running %+v", got)
	}
	c.Stop()
	if c.RunFor() != 0 {
		t.Errorf("ran for %v, stopped", c.RunFor())
	}

	c.Start()
	f.flush(t, 1)
	now = now.Add(30 * time.Second)
	f.sessions[1].report(live.State{Phase: "stopped", Stats: live.Stats{Lines: 1, Speech: 5, Chars: 10}})
	f.flush(t, 1)
	want := Today{Day: "2026-10-20", Counts: usage.Counts{Run: 120, Lines: 4, Speech: 25, Chars: 50}, Translated: 4}
	if got := c.TodayStats(); got != want {
		t.Errorf("today %+v, want %+v", got, want)
	}
	// By the engines it used.
	if l := c.Usage(); len(l.Days) != 1 || len(l.Days[0].Uses) != 1 || l.Days[0].Uses[0].Engines != (usage.Engines{ASR: c.Prefs.ASR, MT: "cloud"}) {
		t.Errorf("usage %+v", l)
	}

	// The next launch reads them; the next day has none.
	d := New(prefs.Default("zh-Hans"), Deps{Post: func(func()) {}, StatsPath: c.deps.StatsPath, Now: func() time.Time { return now }})
	if got := d.TodayStats(); got != want {
		t.Errorf("read back %+v", got)
	}
	now = now.Add(24 * time.Hour)
	if got := d.TodayStats(); got != (Today{Day: "2026-10-21"}) {
		t.Errorf("next day %+v", got)
	}
}

// TestGuideKeepsNothing tries translating in the first-run guide: the sample is neither counted
// nor kept.
func TestGuideKeepsNothing(t *testing.T) {
	c, f := newCore(t)
	c.deps.StatsPath = filepath.Join(t.TempDir(), "stats.json")
	c.deps.HistoryDir = t.TempDir()
	c.Prefs.History = true
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "stopped", Stats: live.Stats{Lines: 1, Chars: 10},
		Lines: []live.Line{{ID: 1, Src: "Good evening", Tgt: "晚上好", Done: true}}})
	f.flush(t, 1)
	if got := c.TodayStats(); got.Lines != 0 || got.Run != 0 {
		t.Errorf("today %+v", got)
	}
	if rs, _ := history.List(c.deps.HistoryDir); len(rs) != 0 {
		t.Errorf("kept %+v", rs)
	}
}

// TestUsage counts each session under the engines it started with, an endpoint's name and model
// as they were, and shows the one going on, today's.
func TestUsage(t *testing.T) {
	c, f := newCore(t)
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.Local)
	c.deps.Now = func() time.Time { return now }
	c.Prefs.Onboarded = true
	c.Prefs.Endpoints = []prefs.Endpoint{{ID: "or", Name: "OpenRouter", URL: "https://openrouter.ai/api", Model: "qwen/qwen3"}}
	c.Prefs.ASR, c.Prefs.MT = QwenCloud, prefs.CustomPrefix+"or"
	c.Prefs.ASRAPIs = []prefs.Service{{ID: QwenCloud, Key: true}}
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "listening", Stats: live.Stats{Lines: 2, Audio: 12, InTokens: 64}})
	f.flush(t, 1)
	now = now.Add(time.Minute)
	l := c.Usage()
	if len(l.Days) != 1 || len(l.Days[0].Uses) != 1 {
		t.Fatalf("usage while running %+v", l)
	}
	u := l.Days[0].Uses[0]
	want := usage.Engines{ASR: QwenCloud, ASRModel: qwencloud.Models[0], MT: "custom:or", MTName: "OpenRouter", MTModel: "qwen/qwen3"}
	if u.Engines != want || u.Run != 60 || u.Lines != 2 || u.Audio != 12 || u.InTokens != 64 {
		t.Errorf("use %+v", u)
	}
	// Renamed after: what ran is counted as it was.
	c.Prefs.Endpoints[0].Name = "Mine"
	c.Stop()
	if u := c.Usage().Days[0].Uses[0]; u.MTName != "OpenRouter" || u.Run != 60 {
		t.Errorf("ended %+v", u)
	}
}
