package core

import (
	"context"

	"tomoru/internal/learn"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func (c *Core) ChatEndpoints() []prefs.Endpoint {
	var out []prefs.Endpoint
	for _, e := range c.Prefs.Endpoints {
		if formatOf(e) == translate.Chat {
			out = append(out, e)
		}
	}
	return out
}

func (c *Core) LearnFrom(paths []string, sceneID, endpointID string) {
	r := learningRequest{Paths: paths, Scene: sceneID}
	e, ok := c.Prefs.Endpoint(endpointID)
	if !ok || formatOf(*e) != translate.Chat {
		r.Err = ErrNoChatEndpoint
		c.LearningService.startLearning(r)
		return
	}
	c.Prefs.Learn = endpointID
	c.SavePrefs()
	if s, ok := c.sceneByID(sceneID); ok {
		r.Context = learn.Context{Name: s.Name, About: s.About, Terms: s.Terms, Rejected: s.Rejected}
	}
	ep, secrets := *e, c.deps.Secrets
	r.Ready = func(ctx context.Context) (learn.Asker, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := ""
		if secrets != nil {
			if b, err := secrets.Get(keyName(ep.ID)); err == nil {
				key = string(b)
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return &translate.Endpoint{URL: ep.URL, Model: ep.Model, APIKey: key, Format: translate.Chat}, nil
	}
	c.LearningService.startLearning(r)
}

func (c *Core) TakeLearned(id string, taken []bool) (int, func()) {
	n, undo := c.SceneService.takeLearned(id, c.Learning.Suggestions, taken)
	c.settingsChanged()
	return n, func() {
		if undo() {
			c.settingsChanged()
		}
	}
}

func (c *Core) Retranslate(paths []string, id string) {
	r := retranslationRequest{Paths: paths}
	if s, ok := c.sceneByID(id); ok {
		r.Terms, r.General = s.Terms, s.IsGeneral()
		r.Ready, r.Err = c.retranslator(c.SceneActive(id, MT))
	}
	c.LearningService.startRetranslating(r)
}

func (c *Core) retranslator(id string) (func(context.Context) (translate.Translator, error), error) {
	if tr := c.deps.Translator; tr != nil {
		return func(context.Context) (translate.Translator, error) { return tr, nil }, nil
	}
	ready, _, err := c.EngineService.translator(id, c.Prefs.Endpoints, c.Prefs.MTAPIs, true)
	if _, local := models.ByID(id); !local || err != nil {
		return ready, err
	}
	return func(ctx context.Context) (translate.Translator, error) {
		acquiring, cancel := context.WithCancel(ctx)
		defer cancel()
		claim := &serverClaim{id: id, yield: cancel}
		claimed := make(chan bool, 1)
		c.work.Post(func() {
			if c.serverBusy(id) {
				claimed <- false
				return
			}
			c.claim = claim
			claimed <- true
		})
		select {
		case ok := <-claimed:
			if !ok {
				return nil, ErrModelBusy
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		tr, err := ready(acquiring)
		c.work.Post(func() {
			if c.claim == claim {
				c.claim = nil
			}
		})
		if err != nil && acquiring.Err() != nil && ctx.Err() == nil {
			err = ErrModelBusy
		}
		return tr, err
	}, nil
}

type serverClaim struct {
	id    string
	yield context.CancelFunc
}

func (c *Core) yieldServer(to string) {
	if claim := c.claim; claim != nil && (to == "" || claim.id != to) {
		claim.yield()
		c.claim = nil
	}
}

func (c *Core) serverBusy(id string) bool {
	if serving := c.serving(); serving != "" && serving != id {
		return true
	}
	return c.ImageService.usesServer() && c.Image.Via != id
}
