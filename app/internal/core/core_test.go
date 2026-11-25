package core

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

type fake struct {
	mu       sync.Mutex
	posted   []func()
	sessions []*fakeSession
	answers  map[string][]engine.Event
	ran      []string
}

type fakeSession struct {
	cfg     live.Config
	report  func(live.State)
	stopped bool
	lines   map[int]live.Line
}

func (s *fakeSession) Stop() {
	s.stopped = true
	for id, l := range s.lines {
		if !l.Done() {
			l.Outcome, l.Err = live.Canceled, context.Canceled
			s.lines[id] = l
			if s.cfg.OnLine != nil {
				s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: l})
			}
		}
	}
}
func (s *fakeSession) Close() { s.Stop() }

func newCore(t *testing.T) (*Core, *fake) {
	p := prefs.Default("zh-Hans")
	p.ASR = "apple"
	return newCoreWith(t, p, func(*Deps) {})
}

func newCoreWith(t *testing.T, p prefs.Prefs, with func(*Deps)) (*Core, *fake) {
	f := &fake{answers: map[string][]engine.Event{}}
	d := Deps{
		Post: func(fn func()) {
			f.mu.Lock()
			f.posted = append(f.posted, fn)
			f.mu.Unlock()
		},
		EnginePath: func() (string, error) { return "/engine", nil },
		StartSession: func(cfg live.Config, report func(live.State)) (Session, error) {
			s := &fakeSession{cfg: cfg, lines: map[int]live.Line{}}
			s.report = func(st live.State) {
				for _, l := range st.Lines {
					old, seen := s.lines[l.ID]
					if cfg.OnLine != nil && !seen {
						committed := l
						committed.Outcome = live.Translating
						cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: committed})
					}
					if cfg.OnLine != nil && l != old {
						kind := live.LineProgress
						if l.Done() {
							kind = live.LineCompleted
						}
						cfg.OnLine(live.LineEvent{Kind: kind, Line: l})
					}
					s.lines[l.ID] = l
				}
				report(st)
			}
			f.sessions = append(f.sessions, s)
			return s, nil
		},
		Translator: translate.Cloud(),
		RunEngine: func(_ context.Context, _ string, args ...string) ([]engine.Event, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.ran = append(f.ran, args[0])
			return f.answers[args[0]], nil
		},
	}
	with(&d)
	return New(p, d), f
}

func (f *fake) flush(t *testing.T, n int) {
	t.Helper()
	var posted []func()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		f.mu.Lock()
		if len(f.posted) >= n {
			posted, f.posted = f.posted, nil
		}
		got := len(f.posted)
		f.mu.Unlock()
		if posted != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d posted, want %d", got, n)
		}
	}
	for _, fn := range posted {
		fn()
	}
}

func TestStartStop(t *testing.T) {
	c, f := newCore(t)
	changes := 0
	c.OnChange(func() { changes++ })
	c.Start()
	f.flush(t, 1)
	if !c.Running || c.State.Phase != live.PhaseStarting || len(f.sessions) != 1 || changes != 2 {
		t.Fatalf("running %v, phase %q, %d sessions, %d changes", c.Running, c.State.Phase, len(f.sessions), changes)
	}
	s := f.sessions[0]
	if s.cfg.Engine != "/engine" || s.cfg.Src != "en" || s.cfg.Tgt != "zh" || !s.cfg.Draft {
		t.Errorf("config %+v", s.cfg)
	}

	s.report(live.State{Phase: live.PhaseListening})
	f.flush(t, 1)
	if c.State.Phase != live.PhaseListening || c.NoteAt.IsZero() || c.ASR["en"] != "ready" || changes != 3 {
		t.Errorf("listening: %d changes, %+v", changes, c)
	}

	c.SetTgt("ja")
	f.flush(t, 1)
	if !s.stopped || len(f.sessions) != 2 || f.sessions[1].cfg.Tgt != "ja" {
		t.Fatalf("restart: stopped %v, %d sessions", s.stopped, len(f.sessions))
	}
	s.report(live.State{Phase: live.PhaseListening, Pending: "old"})
	f.flush(t, 1)
	if c.State.Pending == "old" {
		t.Error("took a stale report")
	}

	c.Stop()
	if c.Running || !f.sessions[1].stopped {
		t.Errorf("paused: running %v", c.Running)
	}
}

func TestSessionError(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: live.PhaseStopped, Err: &engine.Error{Code: "no-permission"}})
	f.flush(t, 1)
	var e *engine.Error
	if c.Running || !errors.As(c.Err, &e) || e.Code != "no-permission" {
		t.Errorf("running %v, err %v", c.Running, c.Err)
	}
	c.Start()
	if c.Err != nil {
		t.Errorf("err %v", c.Err)
	}
	f.flush(t, 1)
	f.sessions[1].report(live.State{Phase: live.PhaseStopped})
	f.flush(t, 1)
	if !IsExited(c.Err) {
		t.Errorf("exited without a word: %v", c.Err)
	}
	c.Start()
	f.flush(t, 1)
	f.sessions[2].report(live.State{Phase: live.PhaseStopped, Err: &exec.ExitError{}})
	f.flush(t, 1)
	if !IsExited(c.Err) {
		t.Errorf("killed: %v", c.Err)
	}
}

func TestEngineMissing(t *testing.T) {
	c, _ := newCore(t)
	c.deps.EnginePath = func() (string, error) { return "", errors.New("no engine") }
	c.Start()
	if c.Running || c.Err == nil {
		t.Errorf("running %v, err %v", c.Running, c.Err)
	}
}

func TestSince(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	report := f.sessions[0].report
	report(live.State{Phase: live.PhaseListening})
	report(live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a"}}})
	f.flush(t, 2)
	if c.Since != 0 && c.Since != 1 {
		t.Fatalf("since %d", c.Since)
	}
	c.Updated = time.Now().Add(-time.Minute)
	report(live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a"}, {ID: 2, Src: "b"}}})
	f.flush(t, 1)
	if c.Since != 2 {
		t.Errorf("since %d, want 2", c.Since)
	}
	c.Prefs.KeepShown = true
	c.Updated = time.Now().Add(-time.Minute)
	report(live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a"}, {ID: 2, Src: "b"}, {ID: 3, Src: "c"}}})
	f.flush(t, 1)
	if c.Since != 2 {
		t.Errorf("kept: since %d, want 2", c.Since)
	}
}

func TestLanguages(t *testing.T) {
	c, _ := newCore(t)
	c.SetSrc("zh")
	if c.Prefs.Src != "zh" || c.Prefs.Tgt != "en" {
		t.Errorf("%s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.Swap()
	if c.Prefs.Src != "en" || c.Prefs.Tgt != "zh" {
		t.Errorf("%s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
}

func TestSystem(t *testing.T) {
	c, f := newCore(t)
	f.answers["prepare"] = []engine.Event{{Type: "state", State: "downloading"}, {Type: "state", State: "ready"}}
	f.answers["access"] = []engine.Event{{Type: "access", State: "unknown"}}
	f.answers["apps"] = []engine.Event{{Type: "app", ID: "com.apple.Safari", Name: "Safari", Playing: true}}

	preparing, ready, ran := ASRPreparing, ASRReady, []string{"access", "apps", "prepare"}
	if prefs.SystemASR() == "" {
		preparing, ready, ran = ASRUnprepared, ASRUnprepared, ran[:2]
	}
	c.PrepareASR("ja")
	c.PrepareASR("ja")
	c.CheckAccess()
	c.ListApps()
	c.ListApps()
	if c.ASR["ja"] != preparing || !c.Checking {
		t.Errorf("asr %q, checking %v", c.ASR["ja"], c.Checking)
	}
	f.flush(t, len(ran))
	slices.Sort(f.ran)
	if !slices.Equal(f.ran, ran) {
		t.Errorf("ran %q", f.ran)
	}
	if c.ASR["ja"] != ready || c.Checking || c.Access != "unknown" || len(c.Apps) != 1 || c.Apps[0].Name != "Safari" {
		t.Errorf("asr %q, checking %v, access %q, apps %+v", c.ASR["ja"], c.Checking, c.Access, c.Apps)
	}
}

func TestSavePrefs(t *testing.T) {
	c, _ := newCore(t)
	c.deps.PrefsPath = t.TempDir() + "/settings.json"
	c.Prefs.Shown = 5
	c.SavePrefs()
	p, err := prefs.Load(c.deps.PrefsPath, "zh-Hans")
	if err != nil || p.Shown != 5 {
		t.Errorf("%+v, %v", p, err)
	}
}

func TestSpeak(t *testing.T) {
	c, f := newCore(t)
	c.Prefs.Src = "ja"
	c.Speak()
	if !c.Speaking {
		t.Fatal("not speaking")
	}
	f.flush(t, 1)
	if c.Speaking || c.SpeakErr != nil || !slices.Equal(f.ran, []string{"speak"}) {
		t.Errorf("speaking %v, error %v, ran %q", c.Speaking, c.SpeakErr, f.ran)
	}
	c.Speak()
	c.StopSpeaking()
	if c.Speaking {
		t.Error("still speaking")
	}
	f.flush(t, 1)
	if c.Speaking || c.SpeakErr != nil {
		t.Errorf("after stopping: speaking %v, error %v", c.Speaking, c.SpeakErr)
	}
	c.Prefs.Src, c.Prefs.Tgt = "auto", "zh"
	if !c.HasSample() || c.sampleLang() != "ja" {
		t.Errorf("auto->zh: has=%v lang=%q", c.HasSample(), c.sampleLang())
	}
	c.Prefs.Tgt = "ja"
	if !c.HasSample() || c.sampleLang() != "en" {
		t.Errorf("auto->ja: has=%v lang=%q", c.HasSample(), c.sampleLang())
	}
}

func TestVoiceKeepsSubtitles(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	s := f.sessions[0]
	s.report(live.State{Phase: live.PhaseListening})
	f.flush(t, 1)
	s.report(live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a"}}})
	f.flush(t, 1)
	at := c.Updated
	s.report(live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a"}}, Voice: true})
	f.flush(t, 1)
	if !c.Updated.Equal(at) || !c.State.Voice {
		t.Errorf("updated moved by voice: %v → %v, voice %v", at, c.Updated, c.State.Voice)
	}
}

func TestClockRedrawsNothing(t *testing.T) {
	base := live.State{Phase: live.PhaseListening, Lines: []live.Line{{ID: 1, Src: "a", Tgt: "あ", Outcome: live.Translated}}, Stats: live.Stats{Lines: 1, SpeechSeconds: 2, Chars: 1}}
	listening := func(t *testing.T) (report func(live.State), changes *int) {
		c, f := newCore(t)
		c.Start()
		f.flush(t, 1)
		s := f.sessions[0]
		s.report(base)
		f.flush(t, 1)
		changes = new(int)
		c.OnChange(func() { *changes++ })
		return func(st live.State) { s.report(st); f.flush(t, 1) }, changes
	}

	report, changes := listening(t)
	for i := range 3 {
		tick := base
		tick.StatsAt, tick.SpeechEnd = time.Now(), time.Now()
		tick.Stats.AudioSeconds, tick.Stats.InTokens, tick.Stats.OutTokens = float64(i+1), 40*i, 50*i
		tick.Stats.Latency.Final.Add(time.Duration(i+1) * 300 * time.Millisecond)
		report(tick)
	}
	if *changes != 0 {
		t.Errorf("%d changes for clock ticks", *changes)
	}

	for name, edit := range map[string]func(*live.State){
		"phase":    func(st *live.State) { st.Phase = live.PhaseStopped },
		"line":     func(st *live.State) { st.Lines = append(slices.Clone(st.Lines), live.Line{ID: 2, Src: "b"}) },
		"pending":  func(st *live.State) { st.Pending = "b" },
		"language": func(st *live.State) { st.PendingLang = "ja" },
		"draft":    func(st *live.State) { st.Draft = "び" },
		"voice":    func(st *live.State) { st.Voice = true },
		"slow":     func(st *live.State) { st.Slow = true },
		"error":    func(st *live.State) { st.Err = errors.New("lost") },
		"lines":    func(st *live.State) { st.Stats.Lines++ },
		"speech":   func(st *live.State) { st.Stats.SpeechSeconds++ },
		"chars":    func(st *live.State) { st.Stats.Chars++ },
	} {
		t.Run(name, func(t *testing.T) {
			report, changes := listening(t)
			st := base
			edit(&st)
			report(st)
			if *changes != 1 {
				t.Errorf("%d changes", *changes)
			}
		})
	}
}
