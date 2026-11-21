package main

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"tomoru/internal/lang"
	"tomoru/internal/live"
)

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
	if cfg.Src != lang.Auto {
		cfg.Src = c.Lang
	}
	cfg.Tgt, cfg.File = tgt, wav

	var mu sync.Mutex
	var at time.Time
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
	s.Wait()
	mu.Lock()
	defer mu.Unlock()
	if stopped != nil {
		r.Err = stopped.Error()
	}
	r.Events = slices.DeleteFunc(r.Events, func(e Event) bool { return e.Line == 0 && e.T > dur+tail.Seconds() })
	return r
}

func recordLine(r *Result, lines map[int]live.Line, t float64, l live.Line) {
	old, seen := lines[l.ID]
	if !seen {
		r.Events = append(r.Events, Event{T: t, Line: l.ID, Src: l.Src, Lang: l.Lang, LDraft: l.Draft})
	}
	if old.Tgt != l.Tgt || old.Done != l.Done || old.Err != l.Err {
		tgt, why := l.Tgt, ""
		if l.Err != nil {
			why = l.Err.Error()
		}
		r.Events = append(r.Events, Event{T: t, Line: l.ID, Tgt: &tgt, Done: l.Done, Err: why})
	}
	lines[l.ID] = l
}
