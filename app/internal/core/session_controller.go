package core

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"time"

	"tomoru/internal/live"
	"tomoru/internal/translate"
)

// SessionController owns one session's preparation, lifetime and display state.
// Inputs are resolved by the application facade; hooks coordinate history and usage.
// Methods and hooks run on the application thread. Runtime callbacks retain dated
// reports in a synchronized accumulator, then enqueue display updates.
type SessionController struct {
	Running bool
	Paused  bool
	Err     error
	State   live.State
	NoteAt  time.Time
	Since   int
	Updated time.Time
	Started bool

	work         *tasks
	startSession func(live.Config, func(live.State)) (Session, error)
	changed      func()
	now          func() time.Time
	hideAfter    func() time.Duration
	onState      func(live.State)
	onStopped    func()
	session      Session
	prepare      context.CancelFunc
	reports      *sessionReports
	run          int
}

type sessionRequest struct {
	Config  live.Config
	Prepare func(context.Context, *live.Config) error
	Ready   func(context.Context) (translate.Translator, error)
	Local   bool
	Err     error
}

func (s *SessionController) start(r sessionRequest) {
	if s.work.ctx.Err() != nil {
		return
	}
	s.stop()
	s.run++
	run := s.run
	s.Paused, s.Err, s.Started = false, r.Err, true
	s.State, s.Since = live.State{Phase: "starting"}, 0
	defer s.changed()
	if r.Err != nil {
		return
	}
	if r.Local {
		s.State.Phase = "loading"
	}
	ctx, cancel := context.WithCancel(s.work.ctx)
	s.prepare, s.Running = cancel, true
	reports := &sessionReports{}
	s.reports = reports
	r.Config.Context = ctx
	s.work.Go(func() {
		var tr translate.Translator
		err := ctx.Err()
		if err == nil && r.Prepare != nil {
			err = r.Prepare(ctx, &r.Config)
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			tr, err = r.Ready(ctx)
		}
		s.work.Post(func() {
			if s.run != run || ctx.Err() != nil {
				return
			}
			defer s.changed()
			if err == nil {
				r.Config.Translator = tr
				if ep, ok := tr.(*translate.Endpoint); ok {
					r.Config.Parallel = ep.Parallel
				}
				s.session, err = s.startSession(r.Config, func(st live.State) {
					st = reports.store(st, s.now)
					s.work.Post(func() {
						if s.run == run {
							s.apply(st)
						}
					})
				})
			}
			if err != nil {
				// A failed startup may already have queued its own terminal callback. Retire
				// its token before cancellation so that callback cannot replace the real error.
				s.run++
				cancel()
				s.onStopped()
				s.prepare, s.reports, s.Running, s.Err, s.State = nil, nil, false, err, live.State{}
			}
		})
	})
}

func (s *SessionController) stop() bool {
	if !s.Running {
		return false
	}
	s.run++
	if s.prepare != nil {
		s.prepare()
	}
	if s.session != nil {
		s.session.Stop()
		s.session.Wait()
	}
	s.onStopped()
	s.session, s.prepare, s.reports, s.Running = nil, nil, nil, false
	s.State, s.NoteAt = live.State{}, time.Now()
	return true
}

func (s *SessionController) apply(st live.State) {
	prev := s.State
	s.State = st
	s.onState(st)
	switch {
	case st.Phase == "stopped":
		// Wait is safe here: runtime reports enqueue callbacks and never wait for this thread.
		if s.session != nil {
			s.session.Wait()
		}
		if s.prepare != nil {
			s.prepare()
		}
		s.onStopped()
		s.session, s.prepare, s.reports, s.Running = nil, nil, nil, false
		// An engine that ends without a word, or is ended by the system (a crash, killed), exited:
		// its exit status means nothing to the user.
		s.Err = st.Err
		if exit := (*exec.ExitError)(nil); s.Err == nil || errors.As(s.Err, &exit) {
			s.Err = errExited
		}
	case st.Phase == "listening" && prev.Phase != "listening":
		s.NoteAt = time.Now()
	case st.Phase == prev.Phase && !sameText(st, prev):
		if after := s.hideAfter(); after > 0 && time.Since(s.Updated) >= after {
			s.Since = lastID(prev.Lines) + 1
		}
		s.Updated = time.Now()
	}
	s.changed()
}

var errExited = exitedError{}

type exitedError struct{}

func (exitedError) Error() string { return "the engine exited" }
func IsExited(err error) bool     { return err == errExited }
func sameText(a, b live.State) bool {
	return a.Pending == b.Pending && a.Draft == b.Draft && slices.Equal(a.Lines, b.Lines)
}
func lastID(lines []live.Line) int {
	if len(lines) == 0 {
		return 0
	}
	return lines[len(lines)-1].ID
}
