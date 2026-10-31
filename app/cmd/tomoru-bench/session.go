package main

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"tomoru/internal/live"
)

// target is the language a case is translated into, and its reference's track: Chinese for the
// others and English for Chinese, unless only another translation has a reference.
func target(c Case) (lang, ref string) {
	want := "zh"
	if c.Lang == "zh" {
		want = "en"
	}
	for _, t := range c.Tgt {
		if strings.HasPrefix(t, want) {
			return want, t
		}
	}
	if len(c.Tgt) > 0 {
		return strings.SplitN(c.Tgt[0], "-", 2)[0], c.Tgt[0]
	}
	return want, ""
}

// runCase plays a case's recording through a session, recording the subtitles' changes.
func runCase(c Case, media, tmp string, maxSec float64, tail time.Duration, cfg live.Config) Result {
	tgt, tgtRef := target(c)
	r := Result{ID: c.ID, Lang: c.Lang, Scene: c.Scene, Tgt: tgt, TgtRef: tgtRef}
	wav := filepath.Join(media, c.ID+".wav")
	dur, err := wavSeconds(wav)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	if maxSec > 0 && dur > maxSec {
		cut := filepath.Join(tmp, c.ID+".wav")
		if err := trimWav(wav, cut, maxSec); err != nil {
			r.Err = err.Error()
			return r
		}
		wav, dur = cut, maxSec
	}
	r.Audio = dur
	cfg.Src, cfg.Tgt, cfg.File = c.Lang, tgt, wav

	var mu sync.Mutex
	var at time.Time // when listening began: the recording's start
	listening := make(chan struct{})
	var last live.State
	lines := map[int]live.Line{}
	var stopped error
	ended := make(chan struct{})
	var endOnce sync.Once
	previousOnLine := cfg.OnLine
	cfg.OnLine = func(event live.LineEvent) {
		mu.Lock()
		t := 0.0
		if !at.IsZero() {
			t = round(time.Since(at).Seconds())
		}
		recordLine(&r, lines, t, event.Line)
		mu.Unlock()
		if previousOnLine != nil {
			previousOnLine(event)
		}
	}
	s, err := live.Start(cfg, func(st live.State) {
		mu.Lock()
		defer mu.Unlock()
		if st.Phase == "stopped" {
			stopped = st.Err
			endOnce.Do(func() { close(ended) })
		}
		if at.IsZero() {
			if st.Phase != "listening" {
				return
			}
			at = time.Now()
			close(listening)
		}
		t := round(time.Since(at).Seconds())
		if st.Pending != last.Pending {
			p := st.Pending
			r.Events = append(r.Events, Event{T: t, Pending: &p})
		}
		if st.Voice != last.Voice {
			v := st.Voice
			r.Events = append(r.Events, Event{T: t, Voice: &v})
		}
		if st.Draft != last.Draft {
			d := st.Draft
			r.Events = append(r.Events, Event{T: t, Draft: &d})
		}
		last = st
	})
	if err != nil {
		r.Err = err.Error()
		return r
	}
	select {
	case <-listening:
	case <-ended:
		s.Wait()
		mu.Lock()
		defer mu.Unlock()
		if stopped != nil {
			r.Err = stopped.Error()
		} else {
			r.Err = "stopped before listening"
		}
		return r
	case <-time.After(2 * time.Minute):
		s.Stop()
		s.Wait()
		r.Err = "never listened"
		return r
	}
	select {
	case <-time.After(time.Duration(dur*float64(time.Second)) + tail):
	case <-ended:
	}
	s.Stop()
	s.Wait() // the processes ended count in the run's CPU
	mu.Lock()
	defer mu.Unlock()
	if stopped != nil {
		r.Err = stopped.Error()
	}
	// Retain every sentence completion, including partial translations finalized by Stop.
	// Only display-only changes beyond the recording window are excluded.
	r.Events = slices.DeleteFunc(r.Events, func(e Event) bool { return e.Line == 0 && e.T > dur+tail.Seconds() })
	return r
}

// recordLine consumes reliable lifecycle events; the bounded subtitle snapshot is never a
// source of sentence records. Tracking by ID also avoids duplicates when commit is followed by
// progress or completion with unchanged text.
func recordLine(r *Result, lines map[int]live.Line, t float64, l live.Line) {
	old, seen := lines[l.ID]
	if !seen {
		r.Events = append(r.Events, Event{T: t, Line: l.ID, Src: l.Src, LDraft: l.Draft})
	}
	if old.Tgt != l.Tgt || old.Done != l.Done || old.Err != l.Err {
		tgt := l.Tgt
		r.Events = append(r.Events, Event{T: t, Line: l.ID, Tgt: &tgt, Done: l.Done, Err: l.Err})
	}
	lines[l.ID] = l
}
