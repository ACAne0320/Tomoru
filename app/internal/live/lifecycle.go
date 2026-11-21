package live

import (
	"context"
	"slices"
	"sync"
)

type LineEventKind string

const (
	LineCommitted LineEventKind = "committed"
	LineProgress  LineEventKind = "progress"
	LineCompleted LineEventKind = "completed"
)

type LineEvent struct {
	Kind     LineEventKind
	Line     Line
	Metadata any
}

type activeLine struct {
	Line
	metadata any
}

type notification struct {
	line  *LineEvent
	state *State
}

func (s *Session) emitLineLocked(kind LineEventKind, line Line, metadata any) {
	if s.cfg.OnLine == nil {
		return
	}
	event := LineEvent{Kind: kind, Line: line, Metadata: metadata}
	s.notifications = append(s.notifications, notification{line: &event})
	s.signalReports()
}

func (s *Session) emitStateLocked() {
	s.state.StatsAt = s.cfg.Now()
	if s.onChange == nil {
		return
	}
	st := s.state
	st.Lines = slices.Clone(st.Lines)
	s.notifications = append(s.notifications, notification{state: &st})
	s.signalReports()
}

func (s *Session) signalReports() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Session) reportLoop() {
	defer close(s.reportsDone)
	for {
		<-s.wake
		for {
			s.mu.Lock()
			batch, closed := s.notifications, s.reportsClosed
			s.notifications = nil
			s.mu.Unlock()
			for _, report := range batch {
				if report.line != nil {
					s.cfg.OnLine(*report.line)
				} else {
					s.onChange(*report.state)
				}
			}
			if closed {
				return
			}
			if len(batch) == 0 {
				break
			}
		}
	}
}

type child struct {
	stopper
	once sync.Once
}

func (c *child) Stop() { c.once.Do(c.stopper.Stop) }

func (s *Session) installProcesses(fin finalizer, procs ...stopper) {
	s.procMu.Lock()
	if fin != nil {
		s.proc = fin
	}
	var installed []*child
	for _, p := range procs {
		installed = append(installed, &child{stopper: p})
	}
	s.procs = append(s.procs, installed...)
	s.procMu.Unlock()
	if s.ctx.Err() != nil {
		for _, c := range installed {
			c.Stop()
		}
	}
}

func (s *Session) Stop() { s.stop(nil) }

func (s *Session) stop(err error) {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.cancel()
		ids := make([]int, 0, len(s.active))
		for id := range s.active {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			line := s.active[id]
			line.Outcome, line.Err = Canceled, context.Canceled
			lineOf(&s.state, id, func(display *Line) { *display = line.Line })
			s.emitLineLocked(LineCompleted, line.Line, line.metadata)
			delete(s.active, id)
		}
		s.state.Phase, s.state.Err, s.state.Voice = PhaseStopped, err, false
		s.emitStateLocked()
	}
	s.mu.Unlock()

	s.procMu.Lock()
	procs := slices.Clone(s.procs)
	s.procMu.Unlock()
	for _, p := range procs {
		p.Stop()
	}
}

func (s *Session) Close() {
	s.Stop()
	s.wait()
}

func (s *Session) wait() {
	<-s.workersDone
	s.procMu.Lock()
	procs := slices.Clone(s.procs)
	s.procMu.Unlock()
	for _, p := range procs {
		<-p.Done()
	}
	<-s.reportsDone
}
