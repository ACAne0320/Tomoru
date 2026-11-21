package live

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"tomoru/internal/asr"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/segment"
	"tomoru/internal/translate"
	"tomoru/internal/usage"
)

type Line struct {
	ID             int
	Src            string
	BeforeGlossary string
	Lang           string
	AsSaid         bool
	Tgt            string
	Outcome        Outcome
	Err            error
	Draft          string
	Start, End     time.Time
}

type Outcome int

const (
	Translating Outcome = iota
	Translated
	Untranslated
	Failed
	Skipped
	Canceled
)

func (l Line) Done() bool { return l.Outcome != Translating }

var ErrBehind = errors.New("behind")

type Phase string

const (
	PhaseStarting    Phase = "starting"
	PhaseDownloading Phase = "downloading"
	PhaseLoading     Phase = "loading"
	PhaseListening   Phase = "listening"
	PhaseStopped     Phase = "stopped"
)

type State struct {
	Phase       Phase
	Lines       []Line
	Pending     string
	PendingLang string
	Draft       string
	Voice       bool
	Slow        bool
	Err         error
	Stats       Stats
	StatsAt     time.Time
	SpeechEnd   time.Time
}

type Stats = usage.Counts

const MaxLines = 10

type Config struct {
	Context                 context.Context
	Now                     func() time.Time
	OnLine                  func(LineEvent)
	LineMetadata            func() any
	Engine                  string
	Recognize               asr.Factory
	CommitVolatileSentences bool
	FinalsEndUtterances     bool
	Finalizer               func(engine.Event) (through float64, ok bool)
	Src, Tgt                string
	App                     string
	File                    string
	Translator              translate.Translator
	Parallel                int
	Draft                   bool
	Terms                   func(text string) (string, []translate.Term)
	About                   func() string
}

type stopper interface {
	Stop()
	Done() <-chan struct{}
	Err() error
}

type Session struct {
	cfg      Config
	onChange func(State)
	seg      *segment.Segmenter
	proc     finalizer
	procs    []*child
	queue    chan queued
	ctx      context.Context
	cancel   context.CancelFunc

	eventMu       sync.Mutex
	procMu        sync.Mutex
	mu            sync.Mutex
	state         State
	next          int
	active        map[int]activeLine
	stopped       bool
	workers       sync.WaitGroup
	notifications []notification
	wake          chan struct{}
	reportsDone   chan struct{}
	workersDone   chan struct{}
	started       chan struct{}
	reportsClosed bool

	draft drafter

	errMu   sync.Mutex
	lastErr string

	voiceAt float64
	voiceOn bool

	clockT              float64
	clockAt             time.Time
	heardAt             time.Time
	voiceStart, saidEnd time.Time
	heard, heardFinal   string
	billed              interface{ Sent() float64 }
	ends                map[int]endPair
}

const draftEvery = 600 * time.Millisecond

type drafter struct {
	gen     int
	running bool
	letters int
	at      time.Time
	cancel  context.CancelFunc
}

func (d *drafter) commit() {
	d.gen++
	d.letters = 0
	if d.cancel != nil {
		d.cancel()
	}
	d.running, d.cancel = false, nil
}

func (d *drafter) due(letters, step int, now time.Time) bool {
	return !d.running && letters-d.letters >= step && now.Sub(d.at) >= draftEvery
}

func (d *drafter) start(letters int, now time.Time, cancel context.CancelFunc) int {
	d.running, d.letters, d.at, d.cancel = true, letters, now, cancel
	return d.gen
}

func (d *drafter) finish(gen int) bool {
	if gen != d.gen {
		return false
	}
	d.running, d.cancel = false, nil
	return true
}

type endPair struct{ spoke, shown time.Time }

const maxEnds = 64

func (s *Session) pairEndLocked(st *State, seq int, spoke, shown time.Time) {
	p, ok := s.ends[seq]
	if !ok {
		return
	}
	if !spoke.IsZero() {
		p.spoke = spoke
	}
	if !shown.IsZero() {
		p.shown = shown
	}
	if p.spoke.IsZero() || p.shown.IsZero() {
		s.ends[seq] = p
		return
	}
	delete(s.ends, seq)
	st.Stats.Latency.EndToEnd.Add(p.shown.Sub(p.spoke))
}

func (s *Session) waitEndLocked(seq int) {
	s.ends[seq] = endPair{}
	for len(s.ends) > maxEnds {
		oldest := seq
		for k := range s.ends {
			oldest = min(oldest, k)
		}
		delete(s.ends, oldest)
	}
}

type queued struct {
	Line
	seq int
	at  time.Time
}

func (s *Session) wall(t float64) time.Time {
	if s.clockAt.IsZero() {
		return time.Time{}
	}
	return s.clockAt.Add(time.Duration((t - s.clockT) * float64(time.Second)))
}

func since(t time.Time) (time.Duration, bool) {
	if t.IsZero() {
		return 0, false
	}
	return time.Since(t), true
}

const firstTextWait = 10 * time.Second

type finalizer interface{ Finalize(t float64) error }

func Start(cfg Config, onChange func(State)) (session *Session, err error) {
	s := newStartingSession(cfg, onChange)
	defer func() {
		close(s.started)
		if err != nil {
			s.stop(err)
			s.wait()
		}
	}()
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	locale := lang.Locale(cfg.Src)
	if cfg.Recognize == nil {
		p, err := engine.Listen(cfg.Engine, engine.ListenOptions{Locale: locale, App: cfg.App, File: cfg.File}, s.handle)
		if err != nil {
			return nil, err
		}
		s.installProcesses(p, p)
	} else {
		rec, err := cfg.Recognize(cfg.Src, s.handle)
		if err != nil {
			return nil, err
		}
		s.installProcesses(nil, rec)
		hear, err := engine.Listen(cfg.Engine, engine.ListenOptions{Locale: locale, App: cfg.App, File: cfg.File, NoRecognition: true}, func(ev engine.Event) {
			switch ev.Type {
			case engine.EventAudio:
				rec.Audio(ev)
			case engine.EventVoice:
				if v, ok := rec.(asr.VoiceHearer); ok {
					v.Voice(ev)
				}
			default:
				s.handle(ev)
			}
		})
		if err != nil {
			return nil, err
		}
		s.installProcesses(nil, hear)
		s.procMu.Lock()
		s.billed, _ = rec.(interface{ Sent() float64 })
		s.procMu.Unlock()
	}
	var once sync.Once
	s.procMu.Lock()
	procs := append([]*child(nil), s.procs...)
	s.procMu.Unlock()
	for _, p := range procs {
		go func() {
			<-p.Done()
			once.Do(func() {
				s.stop(p.Err())
			})
		}()
	}
	return s, nil
}

func newStartingSession(cfg Config, onChange func(State)) *Session {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &Session{
		cfg:         cfg,
		onChange:    onChange,
		seg:         segment.New(segment.Config{CommitVolatileSentences: cfg.CommitVolatileSentences, FinalsEndUtterances: cfg.FinalsEndUtterances}),
		queue:       make(chan queued, 64),
		ends:        map[int]endPair{},
		state:       State{Phase: PhaseStarting},
		active:      make(map[int]activeLine),
		wake:        make(chan struct{}, 1),
		reportsDone: make(chan struct{}),
		workersDone: make(chan struct{}),
		started:     make(chan struct{}),
	}
	parent := cfg.Context
	if parent == nil {
		parent = context.Background()
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.ctx = translate.WithUsage(s.ctx, func(u translate.Usage) {
		s.update(func(st *State) {
			st.Stats.InTokens, st.Stats.OutTokens = st.Stats.InTokens+u.In, st.Stats.OutTokens+u.Out
		})
	})
	s.mu.Lock()
	if cfg.Translator != nil {
		for range max(1, cfg.Parallel) {
			s.spawnLocked(s.translateLoop)
		}
	}
	s.warmLocked()
	s.mu.Unlock()
	go s.reportLoop()
	go func() {
		<-s.ctx.Done()
		s.stop(parent.Err())
		<-s.started
		s.workers.Wait()
		close(s.workersDone)
		s.mu.Lock()
		s.reportsClosed = true
		s.signalReports()
		s.mu.Unlock()
	}()
	return s
}

func (s *Session) handle(ev engine.Event) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	if s.cfg.Finalizer != nil {
		if through, ok := s.cfg.Finalizer(ev); ok {
			s.procMu.Lock()
			proc := s.proc
			s.procMu.Unlock()
			if proc != nil {
				proc.Finalize(through)
			}
		}
	}
	var acts []segment.Action
	var lat usage.Latency
	switch ev.Type {
	case engine.EventState:
		s.update(func(st *State) { st.Phase = Phase(ev.State) })
		return
	case engine.EventSlow:
		s.update(func(st *State) { st.Slow = ev.On })
		return
	case engine.EventClock:
		s.clockT, s.clockAt = ev.T, s.cfg.Now()
		acts = s.seg.Tick(ev.T)
	case engine.EventVoice:
		acts = s.seg.Voice(ev.On, ev.T)
		spoke := 0.0
		if s.voiceOn && !ev.On {
			spoke = max(0, ev.T-s.voiceAt)
		}
		began := ev.On && !s.voiceOn
		if began {
			s.voiceAt, s.heardAt = ev.T, s.wall(ev.T)
			s.voiceStart = s.heardAt
			if s.voiceStart.IsZero() {
				s.voiceStart = s.cfg.Now()
			}
		}
		s.voiceOn = ev.On
		var speechEnd time.Time
		if spoke > 0 {
			speechEnd = s.wall(ev.T)
			if speechEnd.IsZero() {
				speechEnd = s.cfg.Now()
			}
		}
		s.update(func(st *State) {
			if began {
				s.warmLocked()
			}
			st.Voice, st.Stats.SpeechSeconds = ev.On, st.Stats.SpeechSeconds+spoke
			if spoke > 0 {
				st.SpeechEnd = speechEnd
			}
		})
	case engine.EventResult:
		s.noteHeard(ev)
		if d, ok := since(s.heardAt); ok && strings.TrimSpace(ev.Text()) != "" {
			if d <= firstTextWait {
				lat.FirstText.Add(d)
			}
			s.heardAt = time.Time{}
		}
		if ev.Final {
			var toks []segment.Token
			for _, t := range ev.Tokens {
				if t.Start != nil && t.End != nil {
					toks = append(toks, segment.Token{Text: t.Text, Start: *t.Start, End: *t.End})
				}
			}
			if n := len(toks); n > 0 {
				if d, ok := since(s.wall(toks[n-1].End)); ok {
					lat.Final.Add(d)
				}
			}
			acts = s.seg.Final(toks)
		} else {
			acts = s.seg.Volatile(ev.Text(), ev.Start, ev.End)
		}
	default:
		return
	}
	var added []queued
	var segs []*segment.Segment
	type ended struct {
		seq int
		at  time.Time
	}
	var spoke []ended
	now := time.Now()
	heard := s.spoken(s.heard)
	for _, a := range acts {
		l := heard
		if a.Commit.Final {
			l = s.spoken(s.heardFinal)
		}
		q := queued{Line: Line{Src: a.Commit.Text, Lang: l}, seq: a.Commit.Seq, at: now}
		if src, _ := s.terms(q.Src); src != q.Src {
			q.Src, q.BeforeGlossary = src, q.Src
		}
		if a.Commit.Final {
			spoke = append(spoke, ended{q.seq, s.wall(a.Commit.End)})
		}
		added, segs = append(added, q), append(segs, a.Commit)
	}
	s.said(added, segs, s.cfg.Now())
	for _, e := range s.seg.Ended() {
		spoke = append(spoke, ended{e.Seq, s.wall(e.End)})
	}
	pending, _ := s.terms(s.seg.Pending())
	var sent float64
	s.procMu.Lock()
	billed := s.billed
	s.procMu.Unlock()
	if billed != nil {
		sent = billed.Sent()
	}
	s.update(func(st *State) {
		st.Stats.Latency.FirstText.Merge(lat.FirstText)
		st.Stats.Latency.Final.Merge(lat.Final)
		st.Stats.AudioSeconds = max(st.Stats.AudioSeconds, sent)
		for _, q := range added {
			s.next++
			q.ID = s.next
			q.Draft, st.Draft = st.Draft, ""
			s.draft.commit()
			var metadata any
			if s.cfg.LineMetadata != nil {
				metadata = s.cfg.LineMetadata()
			}
			s.active[q.ID] = activeLine{Line: q.Line, metadata: metadata}
			s.emitLineLocked(LineCommitted, q.Line, metadata)
			if s.cfg.Translator == nil || q.Lang == s.cfg.Tgt {
				q.Outcome, q.AsSaid = Untranslated, s.cfg.Translator != nil
				delete(s.active, q.ID)
				s.emitLineLocked(LineCompleted, q.Line, metadata)
			} else {
				select {
				case s.queue <- q:
					s.waitEndLocked(q.seq)
				default:
					q.Outcome, q.Err = Skipped, ErrBehind
					delete(s.active, q.ID)
					s.emitLineLocked(LineCompleted, q.Line, metadata)
				}
			}
			st.Lines = append(st.Lines, q.Line)
			st.Stats.Lines++
		}
		for _, e := range spoke {
			if !e.at.IsZero() {
				s.pairEndLocked(st, e.seq, e.at, time.Time{})
			}
		}
		if n := len(st.Lines); n > MaxLines {
			st.Lines = st.Lines[n-MaxLines:]
		}
		st.Pending, st.PendingLang = pending, heard
		s.maybeDraftLocked(st)
	})
}

func (s *Session) noteHeard(ev engine.Event) {
	if ev.Lang == "" && strings.TrimSpace(ev.Text()) == "" {
		return
	}
	s.heard = ev.Lang
	if ev.Final {
		s.heardFinal = ev.Lang
	}
}

func (s *Session) said(lines []queued, segs []*segment.Segment, now time.Time) {
	var untimed []int
	total := 0
	for i, seg := range segs {
		start, end := s.wall(seg.Start), s.wall(seg.End)
		if !seg.Final || start.IsZero() || start.After(end) {
			untimed = append(untimed, i)
			total += max(1, utf8.RuneCountInString(seg.Text))
			continue
		}
		lines[i].Start, lines[i].End = start, end
		s.saidEnd = end
	}
	if len(untimed) == 0 {
		return
	}
	start := s.voiceStart
	if start.Before(s.saidEnd) {
		start = s.saidEnd
	}
	if start.IsZero() || start.After(now) {
		start = now
	}
	span, done := now.Sub(start), 0
	for _, i := range untimed {
		lines[i].Start = start.Add(span * time.Duration(done) / time.Duration(total))
		done += max(1, utf8.RuneCountInString(segs[i].Text))
		lines[i].End = start.Add(span * time.Duration(done) / time.Duration(total))
	}
	s.saidEnd = now
}

func (s *Session) spoken(heard string) string {
	if heard != "" || s.cfg.Src == lang.Auto {
		return heard
	}
	return s.cfg.Src
}

func draftStep(code string) int {
	switch {
	case lang.Chinese(code):
		return 4
	case code == "ja":
		return 6
	case code == "ko":
		return 5
	}
	return 12
}

func (s *Session) maybeDraftLocked(st *State) {
	if !s.cfg.Draft || s.cfg.Translator == nil || s.ctx.Err() != nil {
		return
	}
	n := segment.Letters(st.Pending)
	if n == 0 || st.PendingLang == s.cfg.Tgt {
		st.Draft, s.draft.letters = "", 0
		return
	}
	if !s.draft.due(n, draftStep(st.PendingLang), time.Now()) {
		return
	}
	text, src := st.Pending, st.PendingLang
	if lang.Spaced(src) {
		if i := strings.LastIndexByte(text, ' '); i > 0 && !strings.HasSuffix(text, " ") {
			text = text[:i]
		}
	}
	_, terms := s.terms(text)
	ctx, cancel := context.WithCancel(s.ctx)
	gen := s.draft.start(n, time.Now(), cancel)
	s.spawnLocked(func() {
		defer cancel()
		out, err := s.cfg.Translator.Translate(ctx, translate.Request{Src: src, Tgt: s.cfg.Tgt, Text: text, Draft: true, Terms: terms, About: s.about()}, nil)
		s.update(func(st *State) {
			if !s.draft.finish(gen) {
				return
			}
			if err != nil || out == "" {
				return
			}
			st.Draft = out
			s.maybeDraftLocked(st)
		})
	})
}

func (s *Session) translateLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case l := <-s.queue:
			if s.ctx.Err() != nil {
				return
			}
			s.update(func(st *State) { st.Stats.Latency.Queue.Add(time.Since(l.at)) })
			_, terms := s.terms(l.Src)
			r := translate.Request{Src: l.Lang, Tgt: s.cfg.Tgt, Text: l.Src, Terms: terms, About: s.about()}
			shown := false
			first := func(st *State) {
				if shown {
					return
				}
				shown = true
				st.Stats.Latency.FirstTranslation.Add(time.Since(l.at))
				s.pairEndLocked(st, l.seq, time.Time{}, time.Now())
			}
			out, err := s.cfg.Translator.Translate(s.ctx, r, func(partial string) {
				tgt := strings.TrimSpace(partial)
				s.update(func(st *State) {
					if tgt != "" {
						first(st)
					}
					x, ok := s.active[l.ID]
					if !ok || x.Tgt == tgt {
						return
					}
					x.Tgt = tgt
					s.active[l.ID] = x
					lineOf(st, l.ID, func(line *Line) { *line = x.Line })
					s.emitLineLocked(LineProgress, x.Line, x.metadata)
				})
			})
			s.update(func(st *State) {
				x, ok := s.active[l.ID]
				if !ok {
					return
				}
				if err != nil {
					x.Outcome, x.Err = Failed, err
				} else {
					x.Outcome, x.Tgt = Translated, out
				}
				delete(s.active, l.ID)
				lineOf(st, l.ID, func(line *Line) { *line = x.Line })
				s.emitLineLocked(LineCompleted, x.Line, x.metadata)
				switch {
				case err == nil:
					first(st)
					st.Stats.Latency.Translation.Add(time.Since(l.at))
					st.Stats.Chars += utf8.RuneCountInString(strings.Join(strings.Fields(out), ""))
				case s.ctx.Err() == nil:
					st.Stats.Failed++
				}
				if !shown {
					delete(s.ends, l.seq)
				}
			})
			s.logErr(err)
		}
	}
}

func (s *Session) logErr(err error) {
	if s.ctx.Err() != nil {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if msg != "" && msg != s.lastErr {
		log.Printf("translate: %s", msg)
	}
	s.lastErr = msg
}

func (s *Session) warmLocked() {
	if w, ok := s.cfg.Translator.(interface{ Warm(context.Context) }); ok {
		s.spawnLocked(func() { w.Warm(s.ctx) })
	}
}

func (s *Session) spawnLocked(f func()) {
	if s.stopped || s.ctx.Err() != nil {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		f()
	}()
}

func (s *Session) about() string {
	if s.cfg.About == nil {
		return ""
	}
	return s.cfg.About()
}

func (s *Session) terms(text string) (string, []translate.Term) {
	if s.cfg.Terms == nil || text == "" {
		return text, nil
	}
	return s.cfg.Terms(text)
}

func lineOf(st *State, id int, f func(*Line)) {
	for i := range st.Lines {
		if st.Lines[i].ID == id {
			f(&st.Lines[i])
		}
	}
}

func (s *Session) update(f func(*State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.ctx.Err() != nil {
		return
	}
	f(&s.state)
	s.emitStateLocked()
}
