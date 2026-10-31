package core

import (
	"slices"
	"sync"
	"time"

	"tomoru/internal/live"
	"tomoru/internal/usage"
)

// sessionReports retains the runtime's latest snapshot and daily usage independently
// of callbacks queued for the application thread. Stop joins the runtime before
// reading it. Each run owns its own accumulator, so retired callbacks cannot alter
// another run or the saved ledger.
type sessionReports struct {
	mu       sync.Mutex
	latest   live.State
	received bool
	days     usage.Ledger
}

func (r *sessionReports) store(st live.State, now func() time.Time) live.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st.StatsAt.IsZero() { // adapters without a runtime clock
		st.StatsAt = now()
	}
	delta := st.Stats.Since(r.latest.Stats)
	delta.Run = 0 // elapsed time belongs to the application, not the runtime
	speech := delta.Speech
	delta.Speech = 0
	if delta != (usage.Counts{}) {
		r.days.Add(usage.DayOf(st.StatsAt), usage.Engines{}, delta)
	}
	end := st.SpeechEnd
	if end.IsZero() {
		end = st.StatsAt
	}
	r.days.AddSpeech(end, speech, usage.Engines{})
	st.Lines = slices.Clone(st.Lines)
	r.latest, r.received = st, true
	return st
}

func (r *sessionReports) endAt(now time.Time) time.Time {
	if r == nil {
		return now
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.endLocked(now)
}

func (r *sessionReports) endLocked(now time.Time) time.Time {
	if r.received && r.latest.Phase == "stopped" && r.latest.StatsAt.Before(now) {
		return r.latest.StatsAt
	}
	return now
}

func (r *sessionReports) usageSnapshot(now time.Time) (usage.Ledger, time.Time) {
	if r == nil {
		return usage.Ledger{}, now
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.days.Clone(), r.endLocked(now)
}
