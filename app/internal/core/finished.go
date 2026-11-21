package core

import "tomoru/internal/models"

type Finish int

const (
	ModelReady Finish = iota
	ModelsMoved
	HistoryExported
)

type Finished struct {
	What Finish
	Kind Kind
	ID   string
	Err  error
}

type finishedLog struct {
	list        []Finished
	modelStates map[string]models.State
	wasMoving   bool
}

func (l *finishedLog) add(f Finished) { l.list = append(l.list, f) }

func (c *Core) TakeFinished() []Finished {
	out := c.finished.list
	c.finished.list = nil
	return out
}

func (c *Core) noteFinished() {
	l := &c.finished
	if l.modelStates == nil {
		l.modelStates = map[string]models.State{}
	}
	for _, kind := range []Kind{ASR, MT} {
		for _, e := range c.Engines(kind) {
			if e.Model == nil {
				continue
			}
			st := c.ModelStatus(e.ID).State
			if was, ok := l.modelStates[e.ID]; ok && st == models.Ready && (was == models.Downloading || was == models.Verifying) {
				l.add(Finished{What: ModelReady, Kind: kind, ID: e.ID})
			}
			l.modelStates[e.ID] = st
		}
	}
	mv := c.ModelsMoving()
	if l.wasMoving && !mv.Active && mv.Err == nil {
		l.add(Finished{What: ModelsMoved})
	}
	l.wasMoving = mv.Active
}
