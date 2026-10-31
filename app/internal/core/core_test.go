package core

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// fake is the world of a Core under test: sessions that report when told to, an engine that
// answers from a table, and a main thread that runs what is posted when told to.
type fake struct {
	mu       sync.Mutex // guards posted and ran: the Core's background work posts from goroutines
	posted   []func()
	sessions []*fakeSession
	answers  map[string][]engine.Event // by command
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
		if !l.Done {
			l.Done = true
			l.Err = context.Canceled.Error()
			s.lines[id] = l
			if s.cfg.OnLine != nil {
				s.cfg.OnLine(live.LineEvent{Kind: live.LineCompleted, Line: l})
			}
		}
	}
}
func (s *fakeSession) Wait() {}

func newCore(t *testing.T) (*Core, *fake) {
	f := &fake{answers: map[string][]engine.Event{}}
	p := prefs.Default("zh-Hans")
	p.ASR = "apple" // the engine's own recognition, downloaded or not, on any system: these test the session
	c := New(p, Deps{
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
						committed.Done = false
						cfg.OnLine(live.LineEvent{Kind: live.LineCommitted, Line: committed})
					}
					if cfg.OnLine != nil && l != old {
						kind := live.LineProgress
						if l.Done {
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
		Translator: translate.Cloud(), // never called: sessions are fake
		RunEngine: func(_ context.Context, _ string, args ...string) ([]engine.Event, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.ran = append(f.ran, args[0])
			return f.answers[args[0]], nil
		},
	})
	return c, f
}

// flush runs what was posted, and waits for the background work to post first.
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
	f.flush(t, 1) // the translator is ready
	if !c.Running || c.State.Phase != "starting" || len(f.sessions) != 1 || changes != 2 {
		t.Fatalf("running %v, phase %q, %d sessions, %d changes", c.Running, c.State.Phase, len(f.sessions), changes)
	}
	s := f.sessions[0]
	if s.cfg.Engine != "/engine" || s.cfg.Src != "en" || s.cfg.Tgt != "zh" || !s.cfg.Draft {
		t.Errorf("config %+v", s.cfg)
	}

	s.report(live.State{Phase: "listening"})
	f.flush(t, 1)
	if c.State.Phase != "listening" || c.NoteAt.IsZero() || c.ASR["en"] != "ready" {
		t.Errorf("listening: %+v", c)
	}

	// Changing a language restarts; the old session's reports are dropped.
	c.SetTgt("ja")
	f.flush(t, 1)
	if !s.stopped || len(f.sessions) != 2 || f.sessions[1].cfg.Tgt != "ja" {
		t.Fatalf("restart: stopped %v, %d sessions", s.stopped, len(f.sessions))
	}
	s.report(live.State{Phase: "listening", Pending: "old"})
	f.flush(t, 1)
	if c.State.Pending == "old" {
		t.Error("took a stale report")
	}

	c.Pause()
	if c.Running || !c.Paused || !f.sessions[1].stopped {
		t.Errorf("paused: running %v, paused %v", c.Running, c.Paused)
	}
}

func TestSessionError(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	f.sessions[0].report(live.State{Phase: "stopped", Err: &engine.Error{Code: "no-permission"}})
	f.flush(t, 1)
	var e *engine.Error
	if c.Running || !errors.As(c.Err, &e) || e.Code != "no-permission" {
		t.Errorf("running %v, err %v", c.Running, c.Err)
	}
	// Starting again clears it.
	c.Start()
	if c.Err != nil {
		t.Errorf("err %v", c.Err)
	}
	f.flush(t, 1)
	f.sessions[1].report(live.State{Phase: "stopped"})
	f.flush(t, 1)
	if !IsExited(c.Err) {
		t.Errorf("exited without a word: %v", c.Err)
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

// After nobody spoke for a while, the subtitles start afresh from the next line.
func TestSince(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	report := f.sessions[0].report
	report(live.State{Phase: "listening"})
	report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "a"}}})
	f.flush(t, 2)
	if c.Since != 0 && c.Since != 1 {
		t.Fatalf("since %d", c.Since)
	}
	c.Updated = time.Now().Add(-time.Minute)
	report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "a"}, {ID: 2, Src: "b"}}})
	f.flush(t, 1)
	if c.Since != 2 {
		t.Errorf("since %d, want 2", c.Since)
	}
}

func TestLanguages(t *testing.T) {
	c, _ := newCore(t)
	c.SetSrc("zh") // the target: they swap
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

	// Only the system's recognition gets ready so: Windows has none.
	preparing, ready, ran := "preparing", "ready", []string{"access", "apps", "prepare"}
	if prefs.SystemASR() == "" {
		preparing, ready, ran = "", "", ran[:2]
	}
	c.PrepareASR("ja")
	c.PrepareASR("ja") // once
	c.CheckAccess()
	c.ListApps()
	c.ListApps() // not again so soon
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

// TestSpeak says the samples aloud in the spoken language, and then is done; StopSpeaking stops
// them without an error.
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
	f.flush(t, 1) // the stopped run's end changes nothing
	if c.Speaking || c.SpeakErr != nil {
		t.Errorf("after stopping: speaking %v, error %v", c.Speaking, c.SpeakErr)
	}
}

// TestVoiceKeepsSubtitles: voice detected while nothing new is said does not keep the subtitles
// showing.
func TestVoiceKeepsSubtitles(t *testing.T) {
	c, f := newCore(t)
	c.Start()
	f.flush(t, 1)
	s := f.sessions[0]
	s.report(live.State{Phase: "listening"})
	f.flush(t, 1)
	s.report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "a"}}})
	f.flush(t, 1)
	at := c.Updated
	s.report(live.State{Phase: "listening", Lines: []live.Line{{ID: 1, Src: "a"}}, Voice: true})
	f.flush(t, 1)
	if !c.Updated.Equal(at) || !c.State.Voice {
		t.Errorf("updated moved by voice: %v → %v, voice %v", at, c.Updated, c.State.Voice)
	}
}
