package core

import (
	"context"
	"fmt"
	"testing"

	"tomoru/internal/history"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
)

func recordingCore(t *testing.T) (*Core, *fake, *fakeSession) {
	t.Helper()
	c, f := newCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded = true; p.History = true })
	c.Start()
	f.until(t, func() bool { return len(f.sessions) == 1 })
	return c, f, f.sessions[0]
}

func recordedEntries(t *testing.T, c *Core) []history.Entry {
	t.Helper()
	rs, err := history.List(c.deps.HistoryDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []history.Entry
	for _, r := range rs {
		es, err := history.Read(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, es...)
	}
	return out
}

func TestHistoryRecordsEventsBeyondDisplayWindowInOrder(t *testing.T) {
	c, _, s := recordingCore(t)
	// No State.Lines snapshot is needed to retain all twenty committed sentences.
	for i := 1; i <= 20; i++ {
		s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: i, Src: fmt.Sprint(i)}})
	}
	for i := 20; i >= 1; i-- {
		s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: i, Src: fmt.Sprint(i), Tgt: "translated", Done: true}})
	}
	c.Stop()
	es := recordedEntries(t, c)
	if len(es) != 20 {
		t.Fatalf("kept %d of 20", len(es))
	}
	for i, e := range es {
		if e.Src != fmt.Sprint(i+1) {
			t.Fatalf("entry %d: %+v", i, e)
		}
	}
	// Late/duplicate completion of the retired session cannot be written into the next.
	c.Start()
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 21, Src: "stale", Done: true}})
	c.HistoryService.drain()
	if len(recordedEntries(t, c)) != 20 {
		t.Fatal("stale completion was kept")
	}
	c.Shutdown()
}

func TestHistoryShutdownKeepsPartialCommittedLine(t *testing.T) {
	c, _, s := recordingCore(t)
	s.lines[1] = live.Line{ID: 1, Src: "unfinished", Draft: "partial draft"}
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: s.lines[1]})
	c.Shutdown()
	es := recordedEntries(t, c)
	if len(es) != 1 || es[0].Tgt != "partial draft" || es[0].Err != "" {
		t.Fatalf("partial: %+v", es)
	}
}

func TestHistorySceneAttributionAndConsentEpoch(t *testing.T) {
	c, _, s := recordingCore(t)
	id := c.AddScene("second")
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: 1, Src: "first scene"}})
	c.SetScene(id)
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 1, Src: "first scene", Done: true}})
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: 2, Src: "second scene"}})
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 2, Src: "second scene", Done: true}})
	c.HistoryService.drain()
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: 3, Src: "discarded pending"}})
	c.UpdatePrefs(func(p *prefs.Prefs) { p.History = false })
	c.UpdatePrefs(func(p *prefs.Prefs) { p.History = true })
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 3, Src: "discarded pending", Done: true, Err: context.Canceled}})
	c.Stop()
	rs, err := history.List(c.deps.HistoryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("scene files %+v", rs)
	}
	for _, r := range rs {
		es, err := history.Read(r.Path)
		if err != nil || len(es) != 1 {
			t.Fatalf("lines %+v %v", es, err)
		}
		want := "first scene"
		if r.Head.Scene == id {
			want = "second scene"
		}
		if es[0].Src != want {
			t.Fatalf("scene %q has %q", r.Head.Scene, es[0].Src)
		}
	}
}

func TestHistoryPolicyUsesCommitMetadataBeforeDelayedDelivery(t *testing.T) {
	c, _, s := recordingCore(t)
	metadata := s.cfg.LineMetadata()
	id := c.AddScene("after commitment")
	c.SetScene(id)
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: live.Line{ID: 1, Src: "old scene"}, Metadata: metadata})
	s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 1, Src: "old scene", Done: true}, Metadata: metadata})
	c.Stop()
	rs, err := history.List(c.deps.HistoryDir)
	if err != nil || len(rs) != 1 || rs[0].Head.Scene != "" {
		t.Fatalf("queued commitment relabelled: %+v %v", rs, err)
	}
}
