package core

import (
	"slices"

	"tomoru/internal/asr"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
)

// The engines to choose, of recognition and of translation, on this computer and in the cloud;
// which of them to recommend and to use, and what this computer runs; and choosing one that is not
// downloaded yet, as the guide does, which is used once it is.

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
	for _, s := range CloudSizes {
		es = append(es, Engine{ID: s.ID, Kind: MT, Cloud: true})
	}
	for _, s := range MTServices {
		es = append(es, Engine{ID: s.ID, Kind: MT, Cloud: true})
	}
	for _, e := range endpoints {
		es = append(es, Engine{ID: prefs.CustomPrefix + e.ID, Kind: MT, Cloud: true, Endpoint: &e})
	}
	return es
}

// Family is what e is listed with: its model's family, the model's sizes together; Index-
// Translate's cloud, its sizes together, as "cloud"; else its own ID, an engine of its own.
func (e Engine) Family() string {
	if e.Model != nil {
		return e.Model.Family
	}
	if _, ok := CloudSizeByID(e.ID); ok {
		return "cloud"
	}
	return e.ID
}

// Families are the engines of kind k by family, in the order of Engines: each the sizes of a
// model, or an engine of its own.
func (c *Core) Families(k Kind) [][]Engine {
	var fs [][]Engine
	for _, e := range c.Engines(k) {
		i := slices.IndexFunc(fs, func(f []Engine) bool { return f[0].Family() == e.Family() })
		if i < 0 {
			fs, i = append(fs, nil), len(fs)
		}
		fs[i] = append(fs[i], e)
	}
	return fs
}

// Recommended tells whether e is the one engine of its kind to recommend here: a local family's
// size to recommend where this computer runs it, its GPU having the memory it needs; else, for
// translation, Index-Translate's cloud; for recognition, none.
func (c *Core) Recommended(e Engine) bool { return e.ID != "" && e.ID == c.recommended(e.Kind) }

func (c *Core) recommended(k Kind) string {
	for _, e := range c.Engines(k) {
		if m := e.Model; m != nil && m.Rec && c.gpu() != 0 && c.Shortfall(*m) == (Shortfall{}) {
			return e.ID
		}
	}
	if k == MT {
		return "cloud"
	}
	return ""
}

// Shortfall is what this computer lacks to run a model: Need of memory, where it has Has, of its
// GPU's own if GPU; zero where it has what the model needs, or what it has is not known.
type Shortfall struct {
	Need, Has uint64 // in whole GB
	GPU       bool
}

// Shortfall says what this computer lacks to run m: its memory, else its GPU's, short of what m
// needs. Both in whole GB, as a computer's are told: a card of 8 GB counts as 8, though its driver
// keeps some of it.
func (c *Core) Shortfall(m models.Model) Shortfall {
	var mem uint64
	if c.deps.Memory != nil {
		mem = c.deps.Memory()
	}
	need, has := gb(m.Needs.Memory), gb(mem)
	if mem != 0 && has < need {
		return Shortfall{Need: need, Has: has}
	}
	need, has = gb(m.Needs.VRAM), gb(c.gpu())
	if c.gpu() != 0 && has < need {
		return Shortfall{Need: need, Has: has, GPU: true}
	}
	return Shortfall{}
}

// Runs tells whether this computer is known to run m: its memory found, and its GPU's where m
// needs one, enough. Not found, as a GPU before its devices are listed, it is not known.
func (c *Core) Runs(m models.Model) bool {
	if c.deps.Memory == nil || c.deps.Memory() == 0 || m.Needs.VRAM != 0 && c.gpu() == 0 {
		return false
	}
	return c.Shortfall(m) == Shortfall{}
}

func (c *Core) gpu() uint64 {
	if c.deps.GPU == nil {
		return 0
	}
	return c.deps.GPU()
}

// gb is n bytes in GB, rounded.
func gb(n uint64) uint64 { return (n + 1<<29) >> 30 }

// Engine finds an engine of kind k.
func (c *Core) Engine(k Kind, id string) (Engine, bool) {
	for _, e := range c.Engines(k) {
		if e.ID == id {
			return e, true
		}
	}
	return Engine{}, false
}

// Usable tells whether e can be used now: its model, if it has one, is downloaded and runs on
// this computer; a cloud recognition service has its API key.
func (c *Core) Usable(e Engine) bool {
	if e.Model != nil && c.Shortfall(*e.Model) != (Shortfall{}) {
		return false
	}
	return c.EngineService.usable(e, c.Prefs.APIs())
}

// Pick is the engine of family f to use on choosing the family: the size in use, else its size
// to recommend, else the largest; of those Usable. False where none is.
func (c *Core) Pick(f []Engine) (Engine, bool) {
	var found []Engine
	for _, e := range f {
		if c.Usable(e) {
			found = append(found, e)
		}
	}
	if len(found) == 0 {
		return Engine{}, false
	}
	for _, e := range found {
		if e.ID == c.InUse(e.Kind) {
			return e, true
		}
	}
	for _, e := range found {
		if e.Model != nil && e.Model.Rec {
			return e, true
		}
	}
	return found[len(found)-1], true
}

// Suggested is the size of family f to offer where one is picked for the user, as the guide does:
// its size to recommend if this computer runs it, else the largest it runs, else the smallest.
func (c *Core) Suggested(f []Engine) Engine {
	runs := func(e Engine) bool { return e.Model == nil || c.Shortfall(*e.Model) == (Shortfall{}) }
	for _, e := range f {
		if e.Model != nil && e.Model.Rec && runs(e) {
			return e
		}
	}
	for i := len(f) - 1; i >= 0; i-- {
		if runs(f[i]) {
			return f[i]
		}
	}
	return f[0]
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

// Use uses engine id of kind k from now on, if it is Usable, and tells whether it was.
func (c *Core) Use(k Kind, id string) bool {
	e, ok := c.Engine(k, id)
	if !ok || !c.Usable(e) {
		return false
	}
	c.EngineService.cancelPending(k)
	c.use(e)
	c.settingsChanged()
	return true
}

// Choose chooses engine id of kind k, as the first-run guide does: used at once if it can be, else
// once its model is downloaded, the one in use going on until then.
func (c *Core) Choose(k Kind, id string) {
	e, ok := c.Engine(k, id)
	if !ok {
		return
	}
	if !c.EngineService.choose(e, c.Prefs.APIs()) {
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
	ready := c.EngineService.readyPending(c.Prefs.Endpoints, c.Prefs.APIs())
	for _, e := range ready {
		c.use(e)
	}
	if len(ready) > 0 {
		c.settingsChanged()
	}
}
