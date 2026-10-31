package core

import (
	"tomoru/internal/asr"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
)

// The engines to choose, of recognition and of translation, on this computer and in the cloud;
// and choosing one that is not downloaded yet, which is used once it is.

// Kind is what an engine does.
type Kind int

const (
	ASR Kind = iota // recognizes speech
	MT              // translates
)

// Engine is an engine to choose.
type Engine struct {
	ID    string // as Prefs.ASR or Prefs.MT names it
	Kind  Kind
	Cloud bool // it runs on a server, which gets what is heard or recognized; else on this computer
	// Model is the model to download before it can be used, or nil.
	Model *models.Model
	// Endpoint is the API the user added that it is, or nil.
	Endpoint *prefs.Endpoint
}

// Engines are the engines of kind k this computer can use: those on it first, the system's and
// then the models to download, then the cloud's, then the APIs the user added.
func (c *Core) Engines(k Kind) []Engine { return engineCatalog(k, c.Prefs.Endpoints) }

func engineCatalog(k Kind, endpoints []prefs.Endpoint) []Engine {
	var es []Engine
	if k == ASR {
		for _, s := range asr.Specs() {
			es = append(es, Engine{ID: s.ID, Kind: ASR, Cloud: s.Cloud, Model: s.Model})
		}
		return es
	}
	for i := range models.All {
		if m := &models.All[i]; !m.ASR {
			es = append(es, Engine{ID: m.ID, Kind: MT, Model: m})
		}
	}
	es = append(es, Engine{ID: "cloud", Kind: MT, Cloud: true})
	for _, e := range endpoints {
		es = append(es, Engine{ID: prefs.CustomPrefix + e.ID, Kind: MT, Cloud: true, Endpoint: &e})
	}
	return es
}

// Recommended tells whether e is the engine to recommend here: a recognition model llama-server
// runs, where a GPU has the memory it needs. Elsewhere SenseVoice stays, which any computer runs.
func (c *Core) Recommended(e Engine) bool {
	if e.Kind != ASR || e.Model == nil || e.Model.Runtime != "llama" || c.deps.GPU == nil {
		return false
	}
	return c.deps.GPU() >= e.Model.Needs.VRAM
}

// Engine finds an engine of kind k.
func (c *Core) Engine(k Kind, id string) (Engine, bool) {
	for _, e := range c.Engines(k) {
		if e.ID == id {
			return e, true
		}
	}
	return Engine{}, false
}

// Usable tells whether e can be used now: its model, if it has one, is downloaded; a cloud
// recognition service has its API key.
func (c *Core) Usable(e Engine) bool {
	return c.EngineService.usable(e, c.Prefs.ASRAPIs)
}

// InUse is the engine of kind k in use.
func (c *Core) InUse(k Kind) string {
	if k == ASR {
		return c.Prefs.ASR
	}
	return c.Prefs.MT
}

// Chosen is the engine of kind k last chosen: the one to use once it is downloaded, else the one in
// use.
func (c *Core) Chosen(k Kind) string {
	return c.EngineService.chosen(k, c.InUse(k))
}

// Choose chooses engine id of kind k: used at once if it can be, else once its model is
// downloaded, the one in use going on until then.
func (c *Core) Choose(k Kind, id string) {
	e, ok := c.Engine(k, id)
	if !ok {
		return
	}
	if !c.EngineService.choose(e, c.Prefs.ASRAPIs) {
		c.settingsChanged()
		return
	}
	c.use(e)
	c.settingsChanged()
}

func (c *Core) use(e Engine) {
	switch e.Kind {
	case MT:
		c.SetEngine(e.ID)
	case ASR:
		if e.ID != c.Prefs.ASR {
			c.Prefs.ASR = e.ID
			c.fitLangs()
			c.RestartIfRunning()
		}
	}
}

// usePending uses the engines chosen whose models came in.
func (c *Core) usePending() {
	ready := c.EngineService.readyPending(c.Prefs.Endpoints, c.Prefs.ASRAPIs)
	for _, e := range ready {
		c.use(e)
	}
	if len(ready) > 0 {
		c.settingsChanged()
	}
}
