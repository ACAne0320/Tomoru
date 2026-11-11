package core

import (
	"errors"
	"testing"
	"time"

	"tomoru/internal/history"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
	"tomoru/internal/usage"
)

// stopCount is how many times s was stopped.
func (s *fakeServer) stopCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stops
}

// TestOriginalOnly starts subtitles of the original alone with a translation engine that could
// not translate: nothing is asked of it, llama-server gives its memory back, and switching to and
// from translating restarts, as switching between the two that translate does not.
func TestOriginalOnly(t *testing.T) {
	c, f, _, srv, _ := engineCore(t)
	c.SetEngine("local-2b") // not downloaded
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplaySrc })
	c.Start()
	if c.Err != nil || !c.Running || c.State.Phase != "starting" {
		t.Fatalf("running %v, phase %q, err %v", c.Running, c.State.Phase, c.Err)
	}
	f.flush(t, 1)
	cfg := f.sessions[0].cfg
	if cfg.Translator != nil || cfg.Draft || len(srv.ensured) != 0 || srv.stopCount() != 1 {
		t.Errorf("translator %v, draft %v, ensured %q, %d stops", cfg.Translator, cfg.Draft, srv.ensured, srv.stopCount())
	}
	if e := c.runEngines; e.MT != "" || !e.Untranslated || e.ASR != "apple" {
		t.Errorf("engines %+v", e)
	}
	if h := c.HistoryService.policy.Load().head; !h.Untranslated || h.Tgt != "zh" {
		t.Errorf("history head %+v", h)
	}

	// Translating again: the model, downloaded now, loads.
	c.Download("local-2b")
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplayBoth })
	if len(f.sessions) != 1 || c.State.Phase != "loading" {
		t.Fatalf("%d sessions, phase %q", len(f.sessions), c.State.Phase)
	}
	f.flush(t, 1)
	if _, ok := f.sessions[1].cfg.Translator.(*translate.Endpoint); !ok || !f.sessions[0].stopped || c.serving() != "local-2b" {
		t.Errorf("translator %v, first stopped %v, serving %q", f.sessions[1].cfg.Translator, f.sessions[0].stopped, c.serving())
	}
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplayTgt })
	if len(f.sessions) != 2 || f.sessions[1].stopped {
		t.Errorf("the translation alone restarted: %d sessions", len(f.sessions))
	}

	// The original alone again, while a picture's card has the model: it keeps running, and
	// the subtitles no longer hold it.
	c.ImageService.Image = ImageView{PNG: []byte{1}, Via: "local-2b"}
	stops := srv.stopCount()
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Display = prefs.DisplaySrc })
	f.flush(t, 1)
	if len(f.sessions) != 3 || f.sessions[2].cfg.Translator != nil || srv.stopCount() != stops || c.serving() != "" {
		t.Errorf("%d sessions, %d stops after %d, serving %q", len(f.sessions), srv.stopCount(), stops, c.serving())
	}
}

// TestKeepUntranslated keeps the lines of the original alone as they are done, in a session that
// says it showed no translation.
func TestKeepUntranslated(t *testing.T) {
	c, f := newCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.Prefs.Onboarded, c.Prefs.History, c.Prefs.Display = true, true, prefs.DisplaySrc
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "大家好", Done: true}}})
	f.flush(t, 1)
	c.Stop()
	rs, _ := history.List(c.deps.HistoryDir)
	if len(rs) != 1 || !rs[0].Head.Untranslated || rs[0].Lines != 1 {
		t.Fatalf("records %+v", rs)
	}
	es, err := history.Read(rs[0].Path)
	if err != nil || len(es) != 1 || es[0].Src != "大家好" || es[0].Tgt != "" || es[0].Err != "" {
		t.Errorf("lines %+v %v", es, err)
	}
}

// TestRetranslateUntranslated writes the names right in a session of the original alone, with no
// translator: the engine chosen could not translate.
func TestRetranslateUntranslated(t *testing.T) {
	c, f := scenesCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.deps.Translator = nil
	c.Prefs.MT = "local-2b" // not downloaded
	id := c.AddScene("雷军演讲")
	c.AddTerm(id, scene.Term{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}})
	start := time.Now()
	w := history.NewWriter(c.deps.HistoryDir, history.Head{Scene: id, Src: "zh", Tgt: "en", Start: start, Untranslated: true})
	w.Add(history.Entry{At: start, Src: "在刘北赛道上"})
	w.Add(history.Entry{At: start.Add(time.Second), Src: "你好"})
	rs, _ := history.List(c.deps.HistoryDir)
	path := rs[0].Path
	c.Retranslate([]string{path}, id)
	f.until(t, func() bool { return c.Retranslating.Finished })
	if r := c.Retranslating; r.Total != 1 || r.Done != 1 || r.Err != nil {
		t.Fatalf("retranslating %+v", r)
	}
	es, _ := history.Read(path)
	if es[0].Src != "在纽北赛道上" || es[0].Heard != "在刘北赛道上" || es[0].Tgt != "" || es[1].Src != "你好" {
		t.Errorf("lines %+v", es)
	}

	// A session translated still needs the translator, and says why it is not there.
	translated := keepSession(t, c.deps.HistoryDir, id, start.Add(time.Hour), "在刘北赛道上")
	c.Retranslate([]string{path, translated}, id)
	f.until(t, func() bool { return c.Retranslating.Finished })
	if r := c.Retranslating; !errors.Is(r.Err, ErrModelMissing) {
		t.Errorf("retranslating %+v", r)
	}
}

// TestTodayTranslated counts today's lines translated apart from those of the original only.
func TestTodayTranslated(t *testing.T) {
	c, _ := newCore(t)
	day := usage.DayOf(c.now())
	c.ledger.Add(day, usage.Engines{ASR: "apple", MT: "cloud"}, usage.Counts{Lines: 3})
	c.ledger.Add(day, usage.Engines{ASR: "apple", Untranslated: true}, usage.Counts{Lines: 2})
	if today := c.TodayStats(); today.Lines != 5 || today.Translated != 3 {
		t.Errorf("today %+v", today)
	}
}
