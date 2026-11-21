package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"tomoru/internal/history"
	"tomoru/internal/lang"
	"tomoru/internal/learn"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

type Learning struct {
	Running     bool
	Done, Total int
	Scene       string
	Paths       []string
	Suggestions []learn.Suggestion
	Failed      int
	Err         error
	Finished    bool
}

type Retranslating struct {
	Running     bool
	Done, Total int
	Err         error
	Finished    bool
}

var ErrNoChatEndpoint = errors.New("no chat endpoint")

type learningRequest struct {
	Paths   []string
	Scene   string
	Context learn.Context
	Ready   func(context.Context) (learn.Asker, error)
	Err     error
}

type retranslationRequest struct {
	Paths   []string
	Terms   []scene.Term
	General bool
	Ready   func(context.Context) (translate.Translator, error)
	Err     error
}

type LearningService struct {
	Learning                Learning
	Retranslating           Retranslating
	work                    *tasks
	changed                 func()
	rewritten               func()
	learning, retranslating *learningOperation
}

type learningOperation struct {
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	stopped bool
}

func newLearningOperation(parent context.Context) *learningOperation {
	ctx, cancel := context.WithCancel(parent)
	return &learningOperation{ctx: ctx, cancel: cancel}
}

func (o *learningOperation) invalidate() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stopped = true
	o.cancel()
}

func (o *learningOperation) valid() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.stopped
}

func (o *learningOperation) rewrite(path string, entries []history.Entry) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stopped || o.ctx.Err() != nil {
		return false, nil
	}
	return true, history.Rewrite(path, entries)
}

func (s *LearningService) notify() {
	if s.changed != nil {
		s.changed()
	}
}

func (s *LearningService) postLearning(op *learningOperation, change func(*Learning)) {
	s.work.Post(func() {
		if s.learning != op || !op.valid() {
			return
		}
		change(&s.Learning)
		s.notify()
	})
}

func (s *LearningService) startLearning(r learningRequest) {
	if s.work.ctx.Err() != nil {
		return
	}
	s.CancelLearning()
	r.Paths = slices.Clone(r.Paths)
	r.Context.Terms = cloneTerms(r.Context.Terms)
	r.Context.Rejected = slices.Clone(r.Context.Rejected)
	s.Learning = Learning{Running: true, Scene: r.Scene, Paths: slices.Clone(r.Paths)}
	defer s.notify()
	if r.Err != nil || r.Ready == nil {
		s.Learning.Running, s.Learning.Finished, s.Learning.Err = false, true, r.Err
		return
	}
	op := newLearningOperation(s.work.ctx)
	s.learning = op
	s.work.Go(func() {
		defer op.cancel()
		m, err := r.Ready(op.ctx)
		if op.ctx.Err() != nil {
			return
		}
		if err != nil {
			s.postLearning(op, func(l *Learning) { l.Running, l.Finished, l.Err = false, true, err })
			return
		}
		var chunks [][]learn.Line
		for _, path := range r.Paths {
			if op.ctx.Err() != nil {
				return
			}
			head, err := history.ReadHead(path)
			entries, readErr := history.Read(path)
			if err = errors.Join(err, readErr); err != nil {
				s.postLearning(op, func(l *Learning) { l.Err = errors.Join(l.Err, err) })
				continue
			}
			if r.Context.SrcLang == "" {
				r.Context.SrcLang, r.Context.TgtLang = head.Src, head.Tgt
			}
			chunks = append(chunks, chunk(entries)...)
		}
		s.postLearning(op, func(l *Learning) { l.Total = len(chunks) })
		for _, ch := range chunks {
			if op.ctx.Err() != nil {
				return
			}
			found, err := learn.Ask(op.ctx, m, r.Context, ch)
			if op.ctx.Err() != nil {
				return
			}
			s.postLearning(op, func(l *Learning) {
				l.Done++
				if err != nil {
					l.Failed, l.Err = l.Failed+1, err
				}
				for _, suggestion := range found {
					l.Suggestions = learn.Merge(l.Suggestions, suggestion)
				}
			})
		}
		s.postLearning(op, func(l *Learning) { l.Running, l.Finished = false, true })
	})
}

func (s *LearningService) CancelLearning() {
	if s.learning != nil {
		s.learning.invalidate()
		s.learning = nil
	}
	if s.Learning.Running {
		s.Learning.Running, s.Learning.Finished = false, true
		s.notify()
	}
}

func (s *LearningService) ForgetLearning() {
	s.CancelLearning()
	s.Learning = Learning{}
	s.notify()
}

func (s *LearningService) postRetranslating(op *learningOperation, change func(*Retranslating)) {
	s.work.Post(func() {
		if s.retranslating != op || !op.valid() {
			return
		}
		change(&s.Retranslating)
		s.notify()
	})
}

func (s *LearningService) startRetranslating(r retranslationRequest) {
	if s.work.ctx.Err() != nil {
		return
	}
	s.CancelRetranslating()
	r.Paths = slices.Clone(r.Paths)
	r.Terms = cloneTerms(r.Terms)
	s.Retranslating = Retranslating{Running: true}
	defer s.notify()
	if r.Err == nil && r.Ready == nil {
		s.Retranslating = Retranslating{Finished: true}
		return
	}
	op := newLearningOperation(s.work.ctx)
	s.retranslating = op
	glossary := scene.NewGlossary(r.Terms, r.General)
	s.work.Go(func() {
		defer op.cancel()
		type session struct {
			path    string
			head    history.Head
			entries []history.Entry
		}
		var sessions []session
		var failed error
		total, translates := 0, false
		for _, path := range r.Paths {
			if op.ctx.Err() != nil {
				return
			}
			head, err := history.ReadHead(path)
			entries, readErr := history.Read(path)
			if err = errors.Join(err, readErr); err != nil {
				failed = errors.Join(failed, err)
				continue
			}
			for _, entry := range entries {
				if fix(glossary, entry, head) {
					total++
					translates = translates || !head.Untranslated
				}
			}
			sessions = append(sessions, session{path: path, head: head, entries: entries})
		}
		var tr translate.Translator
		if translates {
			err := r.Err
			if err == nil {
				tr, err = r.Ready(op.ctx)
			}
			if op.ctx.Err() != nil {
				return
			}
			if err != nil {
				s.postRetranslating(op, func(v *Retranslating) { v.Running, v.Finished, v.Err = false, true, err })
				return
			}
		}
		s.postRetranslating(op, func(v *Retranslating) { v.Total = total })
		written := 0
		for _, session := range sessions {
			changed := false
			for i, entry := range session.entries {
				if op.ctx.Err() != nil {
					return
				}
				if !fix(glossary, entry, session.head) {
					continue
				}
				raw := heard(entry)
				src, terms := glossary.Apply(raw)
				var out string
				var err error
				untranslated := session.head.Untranslated || entry.InTargetLang
				if !untranslated {
					out, err = tr.Translate(op.ctx, translate.Request{Src: spokenIn(entry, session.head), Tgt: session.head.Tgt, Text: src, Terms: terms}, nil)
				}
				if op.ctx.Err() != nil {
					return
				}
				if err == nil && (out != "" || untranslated) {
					x := &session.entries[i]
					x.Src, x.Tgt, x.Err, x.BeforeGlossary = src, out, "", ""
					if src != raw {
						x.BeforeGlossary = raw
					}
					changed = true
				} else if err != nil {
					failed = errors.Join(failed, err)
				}
				s.postRetranslating(op, func(v *Retranslating) { v.Done++ })
			}
			if changed {
				accepted, err := op.rewrite(session.path, session.entries)
				if !accepted {
					return
				}
				if err != nil {
					failed = errors.Join(failed, err)
				} else {
					written++
				}
			}
		}
		if op.ctx.Err() != nil {
			return
		}
		s.postRetranslating(op, func(v *Retranslating) {
			v.Running, v.Finished, v.Err = false, true, failed
			if written > 0 && s.rewritten != nil {
				s.rewritten()
			}
		})
	})
}

func (s *LearningService) CancelRetranslating() {
	if s.retranslating != nil {
		s.retranslating.invalidate()
		s.retranslating = nil
	}
	if s.Retranslating.Running {
		s.Retranslating.Running, s.Retranslating.Finished = false, true
		s.notify()
	}
}

func (s *LearningService) ForgetRetranslating() {
	s.CancelRetranslating()
	s.Retranslating = Retranslating{}
	s.notify()
}

func chunk(entries []history.Entry) [][]learn.Line {
	var out [][]learn.Line
	var current []learn.Line
	var start time.Time
	for _, entry := range entries {
		if len(current) > 0 && (entry.KeptAt.Sub(start) >= learn.Chunk || len(current) == 60) {
			out, current = append(out, current), nil
		}
		if len(current) == 0 {
			start = entry.KeptAt
		}
		if strings.TrimSpace(entry.Src) != "" {
			current = append(current, learn.Line{Src: entry.Src, Tgt: entry.Tgt})
		}
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

func fix(g *scene.Glossary, entry history.Entry, head history.Head) bool {
	src, terms := g.Apply(heard(entry))
	if head.Untranslated {
		return src != entry.Src
	}
	return len(terms) > 0
}

func heard(entry history.Entry) string {
	if entry.BeforeGlossary != "" {
		return entry.BeforeGlossary
	}
	return entry.Src
}

func spokenIn(e history.Entry, h history.Head) string {
	if e.HeardLang != "" || h.Src == lang.Auto {
		return e.HeardLang
	}
	return h.Src
}
