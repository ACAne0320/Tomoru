package live

import (
	"context"
	"slices"
)

// LineEventKind describes a committed sentence's lifecycle.
type LineEventKind string

const (
	LineCommitted LineEventKind = "committed"
	LineProgress  LineEventKind = "progress"
	LineCompleted LineEventKind = "completed"
)

// LineEvent is a complete copy of one sentence at a lifecycle transition. Each committed ID
// receives exactly one completion, including a skipped translation (Err is ErrBehind) or a
// cancellation (Err is context.Canceled). Progress and completion retain Src, Heard and
// Draft even after State.Lines evicts the sentence. Events are serialized in transition order;
// queue overflow can complete newer IDs before older translations. Consumers that write ordered
// records should buffer by ID until preceding sentences complete.
type LineEvent struct {
	Kind LineEventKind
	Line Line
	// Metadata is the immutable value returned by Config.LineMetadata at commitment.
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

// emitLine, emitState and signalReports run under s.mu.
func (s *Session) emitLine(kind LineEventKind, line Line, metadata any) {
	if s.cfg.OnLine == nil {
		return
	}
	event := LineEvent{Kind: kind, Line: line, Metadata: metadata}
	s.notifications = append(s.notifications, notification{line: &event})
	s.signalReports()
}

func (s *Session) emitState() {
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

// reportLoop gives state and sentence consumers one ordered stream, without running consumer
// code under the state lock. Reports are never dropped; callbacks must return promptly.
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

// installProcesses also covers cancellation while a process is still starting. Constructors
// may synchronously report events, so no session lock is held while calling them.
func (s *Session) installProcesses(fin finalizer, procs ...stopper) {
	s.procMu.Lock()
	s.proc = fin
	s.procs = append(s.procs, procs...)
	s.procMu.Unlock()
	if s.ctx.Err() != nil {
		for _, p := range procs {
			p.Stop()
		}
	}
}

// Stop ends the session. Every unfinished committed sentence completes with its last translation
// and draft preserved. It cancels translation and drafting, and requests all child processes to
// stop. Stop is idempotent and can be called by a reporting callback.
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
			line.Done, line.Err = true, context.Canceled
			lineOf(&s.state, id, func(display *Line) { *display = line.Line })
			s.emitLine(LineCompleted, line.Line, line.metadata)
			delete(s.active, id)
		}
		s.state.Phase, s.state.Err, s.state.Voice = "stopped", err, false
		s.emitState()
	}
	s.mu.Unlock()

	// installProcesses stops children added after cancellation. Stop is also called by the
	// context watcher; child Stop implementations are required to be idempotent.
	s.procMu.Lock()
	procs := slices.Clone(s.procs)
	s.procMu.Unlock()
	for _, p := range procs {
		p.Stop()
	}
}

// Wait waits for cancellation, all translation and draft workers, child processes, and the final
// reporting callback. Call Stop first, or cancel Config.Context. A translator must honor its
// context. Wait must not be called from a reporting callback.
func (s *Session) Wait() {
	<-s.workersDone
	s.procMu.Lock()
	procs := slices.Clone(s.procs)
	s.procMu.Unlock()
	for _, p := range procs {
		<-p.Done()
	}
	<-s.reportsDone
}
