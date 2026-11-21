package core

import (
	"context"

	"tomoru/internal/learn"
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
		r.Terms, r.General = s.Terms, s.ID == ""
		r.Ready, _, r.Err = c.translator(c.Prefs.MT)
	}
	c.LearningService.startRetranslating(r)
}
