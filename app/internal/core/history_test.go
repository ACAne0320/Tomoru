package core

import (
	"testing"

	"tomoru/internal/history"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
)

// TestKeep keeps the lines of a session once translated, in order, under the scene in use, and
// those still translating as they are when it stops; nothing while the user did not ask.
func TestKeep(t *testing.T) {
	c, f := scenesCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.Prefs.Onboarded = true
	id := c.AddScene("FF7")
	c.SetScene(id)
	c.Start()
	f.flush(t, 1)
	s := f.sessions[0]
	report := func(lines ...live.Line) {
		s.report(live.State{Phase: "listening", Lines: lines})
		f.flush(t, 1)
	}
	one := live.Line{ID: 1, Src: "メテオ", Heard: "メデオ", Tgt: "陨石", Done: true}
	report(one) // not asked yet
	c.UpdatePrefs(func(p *prefs.Prefs) { p.History = true })
	two := live.Line{ID: 2, Src: "クラウド", Draft: "克劳"}
	three := live.Line{ID: 3, Src: "さらに", Tgt: "还有", Done: true}
	report(one, two, three) // the second still translating holds the third
	rs, _ := history.List(c.deps.HistoryDir)
	if len(rs) != 1 || rs[0].Lines != 1 || rs[0].Head.Scene != id || rs[0].Head.Name != "FF7" || rs[0].Head.Src != "en" {
		t.Fatalf("records %+v", rs)
	}
	c.Stop()
	es, err := history.Read(rs[0].Path)
	if err != nil || len(es) != 3 || es[0].Heard != "メデオ" || es[1].Tgt != "克劳" || es[2].Tgt != "还有" {
		t.Errorf("lines %+v %v", es, err)
	}

	// Listing, reading and deleting, as the settings do.
	c.LoadHistory()
	f.until(t, func() bool { return !c.History.Loading })
	if !c.History.Loaded || len(c.History.Records) != 1 || c.HistoryStale() {
		t.Fatalf("history %+v", c.History)
	}
	c.ShowHistory(rs[0].Path)
	f.flush(t, 1)
	if len(c.History.Lines) != 3 {
		t.Errorf("shown %+v", c.History.Lines)
	}
	undo := c.DeleteHistory([]string{rs[0].Path})
	f.flush(t, 1)
	if len(c.History.Records) != 0 {
		t.Errorf("deleted: %+v", c.History.Records)
	}
	undo()
	f.flush(t, 1)
	if len(c.History.Records) != 1 {
		t.Errorf("undone: %+v", c.History.Records)
	}
}

// TestKeepNothingUnasked keeps no subtitles until the user asks to keep them.
func TestKeepNothingUnasked(t *testing.T) {
	c, f := newCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.Prefs.History = false
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "Hi", Tgt: "嗨", Done: true}}})
	f.flush(t, 1)
	c.Stop()
	if rs, _ := history.List(c.deps.HistoryDir); len(rs) != 0 {
		t.Errorf("kept %+v", rs)
	}
}
