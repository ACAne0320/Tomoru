package core

import (
	"context"

	"tomoru/internal/learn"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// ChatEndpoints are the user's endpoints that can learn names: those prompted as chat models.
func (c *Core) ChatEndpoints() []prefs.Endpoint {
	var out []prefs.Endpoint
	for _, e := range c.Prefs.Endpoints {
		if formatOf(e) == translate.Chat {
			out = append(out, e)
		}
	}
	return out
}

// LearnFrom snapshots the scene and endpoint before handing the operation to its owner.
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
	// Capture only endpoint data and the credential adapter. No worker reads Core's mutable
	// preferences or scenes, even when credential lookup delays the start of the request.
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

// TakeLearned applies accepted suggestions and remembers rejections in the owning scene.
func (c *Core) TakeLearned(id string, taken []bool) (int, func()) {
	n, undo := c.SceneService.takeLearned(id, c.Learning.Suggestions, taken)
	c.settingsChanged()
	return n, func() {
		if undo() {
			c.settingsChanged()
		}
	}
}

// Retranslate snapshots the glossary and translator factory; the service owns reading,
// translating and writing the selected history sessions.
func (c *Core) Retranslate(paths []string, id string) {
	r := retranslationRequest{Paths: paths}
	if s, ok := c.sceneByID(id); ok {
		r.Terms = s.Terms
		r.Ready, _, r.Err = c.translator(c.Prefs.MT)
	}
	c.LearningService.startRetranslating(r)
}
