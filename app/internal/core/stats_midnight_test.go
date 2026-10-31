package core

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/usage"
)

func TestOvernightIdleSessionKeepsYesterdaySubtitles(t *testing.T) {
	c, f := newCore(t)
	zone := time.FixedZone("UTC+8", 8*60*60)
	start := time.Date(2026, 10, 21, 19, 11, 0, 0, zone)
	now := start
	c.deps.Now = func() time.Time { return now }
	c.deps.StatsPath = filepath.Join(t.TempDir(), "stats.json")
	c.Prefs.Onboarded = true
	c.Start()
	f.flush(t, 1)
	now = time.Date(2026, 10, 21, 19, 21, 0, 0, zone)
	f.sessions[0].report(live.State{Phase: "listening", Stats: live.Stats{Lines: 184, Chars: 3200, Speech: 502}})
	f.flush(t, 1)

	// No further speech, results or UI callbacks arrive while the session stays open.
	now = time.Date(2026, 10, 22, 0, 45, 0, 0, zone)
	wantToday := Today{Day: "2026-10-22", Counts: usage.Counts{Run: 45 * 60}}
	if got := c.TodayStats(); got != wantToday {
		t.Fatalf("yesterday's subtitles leaked into today: %+v", got)
	}
	wantYesterday := usage.Counts{Run: (4*60 + 49) * 60, Lines: 184, Chars: 3200, Speech: 502}
	preview := c.Usage()
	if got := preview.Day("2026-10-21"); got != wantYesterday {
		t.Fatalf("yesterday while running: %+v", got)
	}
	if c.RunFor() != now.Sub(start) {
		t.Fatal("the session's total elapsed time changed")
	}
	c.Stop()
	c.Stop()
	if got := c.Usage(); !reflect.DeepEqual(got, preview) {
		t.Fatalf("stopping changed daily attribution: %+v", got)
	}
	d := New(prefs.Default("zh-Hans"), Deps{Post: func(func()) {}, StatsPath: c.deps.StatsPath, Now: func() time.Time { return now }})
	defer d.Shutdown()
	if got := d.TodayStats(); got != wantToday {
		t.Fatalf("reopening changed today: %+v", got)
	}
	if got := d.Usage().Day("2026-10-21"); got != wantYesterday {
		t.Fatalf("reopening changed yesterday: %+v", got)
	}

	// A new silent five-second session must not copy the old subtitle totals again.
	c.Start()
	f.flush(t, 1)
	now = now.Add(5 * time.Second)
	c.Stop()
	wantToday.Run += 5
	if got := c.TodayStats(); got != wantToday {
		t.Fatalf("new silent run reused old counts: %+v", got)
	}
}

func TestUsageUsesEventDatesBeforeDelayedDispatchAndStop(t *testing.T) {
	c, f := newCore(t)
	start := time.Date(2026, 10, 21, 23, 59, 40, 0, time.UTC)
	now := start
	c.deps.Now = func() time.Time { return now }
	c.Prefs.Onboarded = true
	c.Start()
	f.flush(t, 1)
	old := f.sessions[0]
	first := live.Stats{Lines: 2, Chars: 8, Audio: 4, InTokens: 10, OutTokens: 3}
	first.Latency.Translation.Add(100 * time.Millisecond)

	// Both the runtime reporter and UI dispatcher can be delayed across midnight.
	now = start.Add(40 * time.Second)
	old.report(live.State{Phase: "listening", StatsAt: start.Add(10 * time.Second), Stats: first})
	second := first
	second.Lines++
	second.Chars += 6
	second.Failed++
	second.Audio += 2
	second.InTokens += 7
	second.OutTokens += 2
	second.Latency.Translation.Add(300 * time.Millisecond)
	old.report(live.State{Phase: "listening", StatsAt: start.Add(30 * time.Second), Stats: second})
	wantToday := second.Since(first)
	wantToday.Run = 20
	wantYesterday := first
	wantYesterday.Run = 20
	if got := c.TodayStats().Counts; got != wantToday {
		t.Fatalf("pending UI callbacks changed dates: %+v", got)
	}
	// Reading snapshots, stopping, and later draining stale UI reports count each delta once.
	copy := c.Usage()
	copy.Days[0].Uses[0].Lines = 999
	c.Stop()
	f.flush(t, 2)
	if got := c.TodayStats().Counts; got != wantToday {
		t.Fatalf("late UI callbacks counted again: %+v", got)
	}
	if got := c.Usage().Day("2026-10-21"); got != wantYesterday {
		t.Fatalf("old day's counts changed: %+v", got)
	}
	c.Start()
	f.flush(t, 1)
	old.report(live.State{Phase: "listening", Stats: live.Stats{Lines: 100, Chars: 1000}})
	f.flush(t, 1)
	c.Stop()
	if got := c.TodayStats().Counts; got != wantToday {
		t.Fatalf("retired runtime changed the new session: %+v", got)
	}
}

func TestSpeechCrossingMidnightUsesAudioEndNotReportTime(t *testing.T) {
	c, f := newCore(t)
	now := time.Date(2026, 10, 21, 23, 59, 50, 0, time.UTC)
	c.deps.Now = func() time.Time { return now }
	c.Prefs.Onboarded = true
	c.Start()
	f.flush(t, 1)
	end := now.Add(15 * time.Second)
	now = now.Add(20 * time.Second)
	f.sessions[0].report(live.State{Phase: "listening", StatsAt: now, SpeechEnd: end, Stats: live.Stats{Speech: 10}})
	c.Stop()
	if got := c.Usage().Day("2026-10-21"); got != (usage.Counts{Run: 10, Speech: 5}) {
		t.Fatalf("speech before midnight: %+v", got)
	}
	if got := c.TodayStats().Counts; got != (usage.Counts{Run: 10, Speech: 5}) {
		t.Fatalf("speech after midnight: %+v", got)
	}
}

func TestStoppedRuntimeDoesNotAccrueTimeWhileUIIsDelayed(t *testing.T) {
	c, f := newCore(t)
	start := time.Date(2026, 10, 21, 23, 59, 10, 0, time.UTC)
	now := start
	c.deps.Now = func() time.Time { return now }
	c.Prefs.Onboarded = true
	c.Start()
	f.flush(t, 1)
	now = start.Add(10 * time.Minute)
	f.sessions[0].report(live.State{Phase: "stopped", StatsAt: start.Add(40 * time.Second), Stats: live.Stats{Lines: 1}})
	if got := c.RunFor(); got != 40*time.Second {
		t.Fatalf("counted UI queue time: %v", got)
	}
	if got := c.TodayStats().Counts; got != (usage.Counts{}) {
		t.Fatalf("ended session created usage today: %+v", got)
	}
	f.flush(t, 1)
	if got := c.Usage().Day("2026-10-21"); got != (usage.Counts{Run: 40, Lines: 1}) {
		t.Fatalf("stopped event lost its own date: %+v", got)
	}
}
