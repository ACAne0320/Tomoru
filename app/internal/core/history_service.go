package core

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"tomoru/internal/history"
	"tomoru/internal/live"
)

// HistoryService owns recording, listing and reading. Recording consumes reliable
// sentence events; its correctness never depends on the subtitle window size.
type HistoryService struct {
	History        HistoryView
	dir            string
	work           *tasks
	changed        func()
	policy         atomic.Pointer[historyPolicy]
	recording      *historyRun
	retryRuns      []*historyRun // retired sessions with writes awaiting recovery
	hist           *history.Writer
	writing        *historyPolicy
	writingRun     *historyRun
	histGen        int
	listed, purged bool
}

type historyPolicy struct {
	enabled bool
	head    history.Head
	epoch   uint64 // changes when recording consent changes, independently of scene changes
}
type historyLine struct {
	line   live.Line
	policy *historyPolicy
}
type historyRun struct {
	mu       sync.Mutex
	next     int
	lastKept int
	lines    map[int]historyLine
}

func (s *HistoryService) configure(dir string, enabled bool, head history.Head) {
	s.dir = dir
	p := historyPolicy{enabled: enabled && dir != "", head: head}
	if old := s.policy.Load(); old != nil {
		p.epoch = old.epoch
		if old.enabled != p.enabled {
			p.epoch++
		}
		if *old == p {
			return
		}
	}
	s.policy.Store(&p)
	if !p.enabled {
		s.closeHistory()
	}
}

func (s *HistoryService) begin() *historyRun {
	s.closeHistory()
	if old := s.recording; old != nil {
		old.mu.Lock()
		if len(old.lines) > 0 {
			s.retryRuns = append(s.retryRuns, old)
		}
		old.mu.Unlock()
	}
	s.recording = &historyRun{next: 1, lines: map[int]historyLine{}}
	return s.recording
}

// accept is safe from the live dispatcher. Policy is captured at commitment, so
// changing scenes cannot relabel an earlier sentence still being translated.
func (r *historyRun) accept(ev live.LineEvent, policy *historyPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ev.Line.ID < r.next {
		return
	}
	entry, ok := r.lines[ev.Line.ID]
	if !ok {
		entry.policy = policy
	}
	entry.line = ev.Line
	r.lines[ev.Line.ID] = entry
}

func (s *HistoryService) drain() {
	// A restart must not throw away the previous run's failed writes. Keep runs distinct even
	// when they used the same scene, and retry older writes before recording newer sessions.
	for len(s.retryRuns) > 0 {
		r := s.retryRuns[0]
		if !s.drainRun(r) {
			return
		}
		r.mu.Lock()
		empty := len(r.lines) == 0
		r.mu.Unlock()
		if !empty {
			return
		}
		s.retryRuns = s.retryRuns[1:]
	}
	if s.recording != nil {
		s.drainRun(s.recording)
	}
}

// drainRun acknowledges one sentence only after it is written or deliberately discarded by
// recording consent. A failed append leaves this sentence and all later ones available to retry.
func (s *HistoryService) drainRun(r *historyRun) bool {
	for {
		r.mu.Lock()
		entry, ok := r.lines[r.next]
		if !ok || !entry.line.Done {
			r.mu.Unlock()
			return true
		}
		alreadyKept := entry.line.ID <= r.lastKept
		r.mu.Unlock()

		p, l := entry.policy, entry.line
		current := s.policy.Load()
		keep := !alreadyKept && p != nil && p.enabled && current != nil && current.enabled && p.epoch == current.epoch
		if keep {
			if s.hist == nil || s.writing != p || s.writingRun != r {
				s.closeHistory()
				head := p.head
				head.Start = time.Now()
				s.hist = history.NewWriter(s.dir, head)
				s.writing, s.writingRun = p, r
			}
			tgt, errText := l.Tgt, ""
			switch {
			case errors.Is(l.Err, context.Canceled):
				if tgt == "" {
					tgt = l.Draft
				}
			case l.Err != nil:
				errText = l.Err.Error()
			}
			if err := s.hist.Add(history.Entry{At: time.Now(), Src: l.Src, Heard: l.Heard, Tgt: tgt, Err: errText}); err != nil {
				s.History.Err = err
				log.Println("history:", err)
				// Writer errors are sticky. A new writer can recover on the next drain; the
				// previous file retains every entry already acknowledged successfully.
				s.closeHistory()
				return false
			}
			s.History.Err = nil
		}
		r.mu.Lock()
		delete(r.lines, r.next)
		r.next++
		if keep {
			r.lastKept = l.ID
		}
		r.mu.Unlock()
	}
}

func (s *HistoryService) closeHistory() {
	if s.hist == nil {
		return
	}
	if err := s.hist.Close(); err != nil {
		s.History.Err = err
		log.Println("history:", err)
	}
	s.hist, s.writing, s.writingRun = nil, nil, nil
	s.histChanged()
}

// backfill preserves the existing opt-in behavior: when recording is enabled,
// the currently visible committed sentences are included once. Subsequent
// recording uses lifecycle events exclusively.
func (s *HistoryService) backfill(lines []live.Line) {
	r := s.recording
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range lines {
		if l.ID <= r.lastKept {
			continue
		}
		if l.ID < r.next {
			r.next = l.ID
		}
		old, ok := r.lines[l.ID]
		if !ok {
			old.line = l
		}
		old.policy = s.policy.Load()
		r.lines[l.ID] = old
	}
}
