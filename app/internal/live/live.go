// Package live is a translating session: the engine's results go through the segmenter, and each
// segment through the translator, if there is one, into the lines the subtitles show.
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

// Line is a sentence heard, and its translation as far as it has come.
type Line struct {
	ID  int
	Src string
	// Heard is Src as recognized, when the glossary wrote a name in it right.
	Heard string
	Tgt   string
	Done  bool  // translation finished, failed, was skipped, or was canceled; or none was to be done
	Err   error // why it has no translation: the translator's error, ErrBehind or context.Canceled
	// Draft is the translation of the sentence while it was being spoken. A translation streaming
	// in starts shorter than the draft it replaces, so the draft shows until Done.
	Draft string
}

// ErrBehind is a translation skipped because translating fell behind. Its message is what history
// files keep for it.
var ErrBehind = errors.New("behind")

// State is what the subtitles show.
type State struct {
	// Phase is "starting", "downloading" (recognition for the language), "listening" or "stopped".
	Phase   string
	Lines   []Line // the latest last, MaxLines at most
	Pending string // heard and not committed yet
	Draft   string // a translation of Pending, when Config.Draft
	Voice   bool   // someone is speaking, as the engine's voice detection says
	// Slow tells the recognizer's network is slow: its text comes late, for the user to check it.
	Slow  bool
	Err   error // why the session stopped
	Stats Stats // how much the session has done
	// StatsAt is when this cumulative snapshot was produced, before callbacks are queued.
	// Consumers must use it, rather than callback delivery time, to date statistics.
	StatsAt time.Time
	// SpeechEnd is the audio's wall-clock end of the latest positive Speech increment.
	// The interval starts Speech's increment seconds earlier. It remains unchanged until
	// another increment; consumers use it only when cumulative Speech increases.
	SpeechEnd time.Time
}

// Stats is how much a session has done, and how long its steps took. Its Run is the app's to
// count: a session leaves it 0.
type Stats = usage.Counts

// MaxLines is how many lines the subtitle snapshot keeps; lifecycle events are not limited.
const MaxLines = 10

// Config is what a session needs.
type Config struct {
	// Context owns the session's lifetime. Nil uses context.Background.
	Context context.Context
	// Now supplies statistics timestamps and anchors the engine's audio clock to wall time.
	// Nil uses time.Now. A supplied clock must be safe for concurrent session goroutines.
	Now func() time.Time
	// OnLine receives every committed sentence, translation progress and final result, even
	// after the subtitle snapshot has evicted the sentence. It must return promptly.
	OnLine func(LineEvent)
	// LineMetadata captures caller-owned metadata synchronously when a sentence commits,
	// before its lifecycle notifications are queued. It must be fast and safe from session
	// goroutines, must not call session methods, and must return an immutable value or snapshot.
	// The exact value accompanies every lifecycle event for that sentence; nil leaves it absent.
	LineMetadata func() any
	Engine       string // the engine's path
	// Recognize starts the recognizer of what the engine hears, for a language: one of package
	// asr's. Nil has the engine recognize itself.
	Recognize asr.Factory
	// VolatileSentences commits the sentences the recognizer's volatile text ends: where they
	// hold (asr.Spec).
	VolatileSentences bool
	// FinalsEnd tells the recognizer's final results end utterances (asr.Spec).
	FinalsEnd bool
	// Hint tells a recognizer in the engine when to make its result final, as each of the engine's
	// events comes (asr.Spec's Hint): Apple's. Nil for none.
	Hint     func(engine.Event) (through float64, ok bool)
	Src, Tgt string // languages, of lang.All
	App      string // the app to hear, "" for the whole system
	File     string // a recording to hear instead, at its pace, for benchmarks
	// Translator translates each sentence. Nil translates nothing: the session only recognizes, and
	// each sentence is done as it is committed.
	Translator translate.Translator
	// Parallel is how many sentences may be translated at once, of those waiting; 0 is one. Each
	// streams into its line, whatever the order they finish in.
	Parallel int
	// Draft translates sentences while they are spoken, so that their gist shows before they end.
	Draft bool
	// Terms applies the scene's glossary to text heard: the text with names heard wrong written
	// right, which shows and is translated, and the terms it concerns, which go with it. It is
	// called from the session's goroutines. Nil leaves text as it is.
	Terms func(text string) (string, []translate.Term)
	// About is what is being watched, as the scene's description says, sent with each sentence.
	// It is called from the session's goroutines. Nil sends none.
	About func() string
}

// stopper is a process of the session, to stop and wait for.
type stopper interface {
	Stop()
	Done() <-chan struct{}
	Err() error
}

// Session is a running session.
type Session struct {
	cfg      Config
	onChange func(State)
	seg      *segment.Segmenter
	proc     finalizer   // the engine recognizing, as Hint has it finalize; nil for a recognizer of its own
	procs    []stopper   // the engine and the recognizer, to stop; none in tests
	queue    chan queued // segments to translate, in order
	ctx      context.Context
	cancel   context.CancelFunc

	// Events can arrive from both the capture process and an external recognizer.
	eventMu       sync.Mutex
	procMu        sync.Mutex // guards process installation during Start
	mu            sync.Mutex // guards state, active lines, lifecycle and drafts
	state         State
	next          int                // the last line's ID
	active        map[int]activeLine // committed lines still translating, independent of the display
	stopped       bool
	workers       sync.WaitGroup
	notifications []notification // ordered under mu; callbacks run outside it
	wake          chan struct{}
	reportsDone   chan struct{}
	workersDone   chan struct{}
	started       chan struct{}
	reportsClosed bool

	// Drafts: one at a time, of pending text grown by a few words, and not too often. A commit
	// makes the one in flight stale.
	draftGen    int
	draftBusy   bool
	draftLen    int // the letters of the pending text last drafted
	draftAt     time.Time
	draftCancel context.CancelFunc // drops the one in flight

	errMu   sync.Mutex
	lastErr string // the translation error logged last, by translateLoop

	voiceAt float64 // when the voice started, by the audio clock, while it is on: on the engine's goroutine
	voiceOn bool

	// The audio clock as the engine last told it, and when: the wall's time of an audio time.
	clockT  float64
	clockAt time.Time
	// heardAt is when the voice last started, by the wall, until text is recognized after it.
	heardAt time.Time
	billed  interface{ Sent() float64 } // a cloud recognizer, which bills the audio sent
	// ends pair each line's speech end with the first words of its translation, by its segment's
	// Seq, under mu: the first known waits for the other. A line cut from the hypothesis learns its
	// end from the final result after it, often once its translation shows.
	ends map[int]endPair
}

// endPair is when a line's speech ended and when its translation's first words showed, by the
// wall; zero while not known.
type endPair struct{ spoke, shown time.Time }

// maxEnds is the most lines waiting to pair their ends: one whose final result never comes is
// dropped with the oldest.
const maxEnds = 64

// pairEnd notes when segment seq's speech ended, or its translation's first words showed, under
// mu; once both are known, the wait between them is the line's EndToEnd. A line not waiting (never
// translated, or dropped) is left alone.
func (s *Session) pairEnd(st *State, seq int, spoke, shown time.Time) {
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

// waitEnd has segment seq's line wait to pair its ends, under mu.
func (s *Session) waitEnd(seq int) {
	s.ends[seq] = endPair{}
	for len(s.ends) > maxEnds {
		oldest := seq
		for k := range s.ends {
			oldest = min(oldest, k)
		}
		delete(s.ends, oldest)
	}
}

// queued is a sentence to translate: its segment's Seq, and when it was committed.
type queued struct {
	Line
	seq int
	at  time.Time
}

// wall is the time by the wall of t, a time of the audio clock; zero before the engine told it.
func (s *Session) wall(t float64) time.Time {
	if s.clockAt.IsZero() {
		return time.Time{}
	}
	return s.clockAt.Add(time.Duration((t - s.clockT) * float64(time.Second)))
}

// since is the time since t, if it is not zero.
func since(t time.Time) (time.Duration, bool) {
	if t.IsZero() {
		return 0, false
	}
	return time.Since(t), true
}

// firstTextWait is the longest wait for text after the voice starts that counts as recognition's:
// past it, the voice was not speech (music, a cough), and the text is of later speech.
const firstTextWait = 10 * time.Second

// finalizer is what the session asks of the engine: a *engine.Process, or a fake in tests.
type finalizer interface{ Finalize(t float64) error }

// Start starts the engine, recognizer and translating session. onChange and Config.OnLine run
// serially on a reporting goroutine, outside session locks. They must return promptly; they may
// call Stop, but must not call Wait (which waits for callbacks to finish).
func Start(cfg Config, onChange func(State)) (session *Session, err error) {
	s := newStartingSession(cfg, onChange)
	defer func() {
		close(s.started)
		if err != nil {
			// A recognizer can emit synchronously during startup. Finish those line events and
			// every started worker/child before the caller discards a failed session's handle.
			s.stop(err)
			s.Wait()
		}
	}()
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	locale := lang.Locale(cfg.Src)
	if cfg.Recognize == nil {
		p, err := engine.Listen(cfg.Engine, locale, cfg.App, cfg.File, false, s.handle)
		if err != nil {
			return nil, err
		}
		s.installProcesses(p, p)
	} else {
		// The engine hears, and its audio goes on to the recognizer.
		rec, err := cfg.Recognize(cfg.Src, s.handle)
		if err != nil {
			return nil, err
		}
		s.installProcesses(nil, rec)
		hear, err := engine.Listen(cfg.Engine, locale, cfg.App, cfg.File, true, func(ev engine.Event) {
			switch ev.Type {
			case "audio":
				rec.Audio(ev)
			case "voice":
				// The recognizer takes the engine's voice detection: the cloud's, to hang up while
				// nobody speaks, a local model's, to cut speech into stretches. Each reports it on.
				if v, ok := rec.(interface{ Voice(engine.Event) }); ok {
					v.Voice(ev)
				}
			default:
				s.handle(ev)
			}
		})
		if err != nil {
			rec.Stop()
			return nil, err
		}
		s.installProcesses(nil, hear)
		s.procMu.Lock()
		s.billed, _ = rec.(interface{ Sent() float64 })
		s.procMu.Unlock()
	}
	// The first to end ends the session, and says why.
	var once sync.Once
	s.procMu.Lock()
	procs := append([]stopper(nil), s.procs...)
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
		seg:         segment.New(segment.Config{VolatileSentences: cfg.VolatileSentences, FinalsEnd: cfg.FinalsEnd}),
		queue:       make(chan queued, 64),
		ends:        map[int]endPair{},
		state:       State{Phase: "starting"},
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
	// The tokens of the translations, drafts too.
	s.ctx = translate.WithUsage(s.ctx, func(u translate.Usage) {
		s.update(func(st *State) {
			st.Stats.InTokens, st.Stats.OutTokens = st.Stats.InTokens+u.In, st.Stats.OutTokens+u.Out
		})
	})
	if cfg.Translator != nil {
		for range max(1, cfg.Parallel) {
			s.workers.Add(1)
			go func() {
				defer s.workers.Done()
				s.translateLoop()
			}()
		}
	}
	s.warm()
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

// handle serializes capture and recognizer events before using the segmenter and audio clock.
func (s *Session) handle(ev engine.Event) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	if s.cfg.Hint != nil {
		if through, ok := s.cfg.Hint(ev); ok {
			s.procMu.Lock()
			proc := s.proc
			s.procMu.Unlock()
			if proc != nil {
				proc.Finalize(through)
			}
		}
	}
	var acts []segment.Action
	var lat usage.Latency // the steps that ended with ev
	switch ev.Type {
	case "state":
		s.update(func(st *State) { st.Phase = ev.State })
		return
	case "slow": // a cloud recognizer's, as its network is slow, or quick again
		s.update(func(st *State) { st.Slow = ev.On })
		return
	case "clock":
		s.clockT, s.clockAt = ev.T, s.cfg.Now()
		acts = s.seg.Tick(ev.T)
	case "voice":
		acts = s.seg.Voice(ev.On, ev.T)
		spoke := 0.0
		if s.voiceOn && !ev.On {
			spoke = max(0, ev.T-s.voiceAt)
		}
		began := ev.On && !s.voiceOn
		if began {
			s.voiceAt, s.heardAt = ev.T, s.wall(ev.T)
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
				s.warm() // a sentence to translate is coming
			}
			st.Voice, st.Stats.Speech = ev.On, st.Stats.Speech+spoke
			if spoke > 0 {
				st.SpeechEnd = speechEnd
			}
		})
	case "result":
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
	type ended struct {
		seq int
		at  time.Time
	}
	var spoke []ended
	now := time.Now()
	for _, a := range acts {
		q := queued{Line: Line{Src: a.Commit.Text}, seq: a.Commit.Seq, at: now}
		if src, _ := s.terms(q.Src); src != q.Src {
			q.Src, q.Heard = src, q.Src
		}
		if a.Commit.Timed {
			spoke = append(spoke, ended{q.seq, s.wall(a.Commit.End)})
		}
		added = append(added, q)
	}
	// The ends of lines cut from the hypothesis, as the final results after them tell.
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
		st.Stats.Audio = max(st.Stats.Audio, sent)
		for _, q := range added {
			s.next++
			q.ID = s.next
			// The sentence keeps the draft of it until its own translation is done.
			q.Draft, st.Draft = st.Draft, ""
			s.draftGen++
			s.draftLen = 0
			if s.draftCancel != nil {
				// The draft in flight is of this sentence, and no use now: dropped, so that the
				// next sentence's need not wait for it.
				s.draftCancel()
				s.draftCancel, s.draftBusy = nil, false
			}
			var metadata any
			if s.cfg.LineMetadata != nil {
				metadata = s.cfg.LineMetadata()
			}
			s.active[q.ID] = activeLine{Line: q.Line, metadata: metadata}
			s.emitLine(LineCommitted, q.Line, metadata)
			if s.cfg.Translator == nil { // nothing to translate it with: it is done
				q.Done = true
				delete(s.active, q.ID)
				s.emitLine(LineCompleted, q.Line, metadata)
			} else {
				select {
				case s.queue <- q:
					s.waitEnd(q.seq)
				default: // translation is far behind: this line goes without
					q.Done, q.Err = true, ErrBehind
					delete(s.active, q.ID)
					s.emitLine(LineCompleted, q.Line, metadata)
				}
			}
			st.Lines = append(st.Lines, q.Line)
			st.Stats.Lines++
		}
		for _, e := range spoke {
			if !e.at.IsZero() {
				s.pairEnd(st, e.seq, e.at, time.Time{})
			}
		}
		if n := len(st.Lines); n > MaxLines {
			st.Lines = st.Lines[n-MaxLines:]
		}
		st.Pending = pending
		s.maybeDraft(st)
	})
}

// draftStep is how many letters pending text grows by before it is drafted again: about two
// words, as a Chinese or Japanese character carries more than a letter.
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

// maybeDraft starts translating the pending text when it has grown by a few words since the last
// draft, one at a time and not more often than every 0.6 s. It runs under s.mu.
func (s *Session) maybeDraft(st *State) {
	if !s.cfg.Draft || s.cfg.Translator == nil || s.ctx.Err() != nil {
		return
	}
	n := segment.Letters(st.Pending)
	if n == 0 {
		st.Draft, s.draftLen = "", 0
		return
	}
	if s.draftBusy || n-s.draftLen < draftStep(s.cfg.Src) || time.Since(s.draftAt) < 600*time.Millisecond {
		return
	}
	s.draftBusy, s.draftLen, s.draftAt = true, n, time.Now()
	gen, text := s.draftGen, st.Pending
	// In languages that space words, the last one may be half heard ("tips" as "ti"), and the model
	// would guess the rest: leave it out.
	if lang.Spaced(s.cfg.Src) {
		if i := strings.LastIndexByte(text, ' '); i > 0 && !strings.HasSuffix(text, " ") {
			text = text[:i]
		}
	}
	_, terms := s.terms(text)
	ctx, cancel := context.WithCancel(s.ctx)
	s.draftCancel = cancel
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer cancel()
		out, err := s.cfg.Translator.Translate(ctx, translate.Request{Src: s.cfg.Src, Tgt: s.cfg.Tgt, Text: text, Draft: true, Terms: terms, About: s.about()}, nil)
		s.update(func(st *State) {
			if gen != s.draftGen {
				return // a commit dropped it
			}
			s.draftBusy, s.draftCancel = false, nil
			if err != nil || out == "" {
				return
			}
			st.Draft = out
			s.maybeDraft(st) // the pending text may have grown meanwhile
		})
	}()
}

// translateLoop translates the lines one by one, in order, streaming each into its line;
// Config.Parallel of them take the lines in turn.
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
			// The terms as the glossary is now: one learned since the line was heard goes with it.
			_, terms := s.terms(l.Src)
			r := translate.Request{Src: s.cfg.Src, Tgt: s.cfg.Tgt, Text: l.Src, Terms: terms, About: s.about()}
			shown := false // the first words of the translation showed, and were timed
			first := func(st *State) {
				if shown {
					return
				}
				shown = true
				st.Stats.Latency.FirstTranslation.Add(time.Since(l.at))
				s.pairEnd(st, l.seq, time.Time{}, time.Now())
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
					s.emitLine(LineProgress, x.Line, x.metadata)
				})
			})
			s.update(func(st *State) {
				x, ok := s.active[l.ID]
				if !ok {
					return
				}
				x.Done = true
				if err != nil {
					x.Err = err
				} else {
					x.Tgt = out
				}
				delete(s.active, l.ID)
				lineOf(st, l.ID, func(line *Line) { *line = x.Line })
				s.emitLine(LineCompleted, x.Line, x.metadata)
				switch {
				case err == nil:
					first(st) // a translation that came whole
					st.Stats.Latency.Translation.Add(time.Since(l.at))
					st.Stats.Chars += utf8.RuneCountInString(strings.Join(strings.Fields(out), ""))
				case s.ctx.Err() == nil:
					st.Stats.Failed++
				}
				if !shown {
					delete(s.ends, l.seq) // no translation to wait for
				}
			})
			s.logErr(err)
		}
	}
}

// logErr logs a translation's error, once while lines keep failing alike: when the service is
// down, every line fails so.
func (s *Session) logErr(err error) {
	if s.ctx.Err() != nil {
		return // stopped, not failed
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

// warm has the translator open its connection, when it does (the cloud), ahead of a sentence to
// translate. It runs as the session starts, or under s.mu while it runs, as the session's
// goroutines must not be added to once it waits for them.
func (s *Session) warm() {
	w, ok := s.cfg.Translator.(interface{ Warm(context.Context) })
	if !ok || s.ctx.Err() != nil {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		w.Warm(s.ctx)
	}()
}

// about is what is being watched, with Config.About.
func (s *Session) about() string {
	if s.cfg.About == nil {
		return ""
	}
	return s.cfg.About()
}

// terms applies the glossary to text, with Config.Terms.
func (s *Session) terms(text string) (string, []translate.Term) {
	if s.cfg.Terms == nil || text == "" {
		return text, nil
	}
	return s.cfg.Terms(text)
}

// lineOf changes the line id of st, with f, if it still has it.
func lineOf(st *State, id int, f func(*Line)) {
	for i := range st.Lines {
		if st.Lines[i].ID == id {
			f(&st.Lines[i])
		}
	}
}

// update serializes state changes and queues immutable reports in the same order. Once stopped,
// even a translator or recognizer that returns late cannot change the session again.
func (s *Session) update(f func(*State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.ctx.Err() != nil {
		return
	}
	f(&s.state)
	s.emitState()
}
