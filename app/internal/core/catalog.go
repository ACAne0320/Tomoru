package core

import (
	"slices"

	"tomoru/internal/asr"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
)

type Kind int

const (
	ASR Kind = iota
	MT
)

type Engine struct {
	ID       string
	Kind     Kind
	Cloud    bool
	Model    *models.Model
	Endpoint *prefs.Endpoint
}

type Source int

const (
	SourceSystem Source = iota
	SourceLocal
	SourceIndexCloud
	SourceService
	SourceUserAPI
)

func (e Engine) Source() Source {
	switch {
	case e.Model != nil:
		return SourceLocal
	case e.Endpoint != nil:
		return SourceUserAPI
	case e.Family() == "cloud":
		return SourceIndexCloud
	case e.Cloud:
		return SourceService
	}
	return SourceSystem
}

func (c *Core) Engines(k Kind) []Engine { return c.EngineService.catalog(k, c.Prefs.Endpoints) }

func (s *EngineService) catalog(k Kind, endpoints []prefs.Endpoint) []Engine {
	var es []Engine
	if k == ASR {
		for _, s := range asr.Specs() {
			es = append(es, Engine{ID: s.ID, Kind: ASR, Cloud: s.Cloud, Model: s.Model})
		}
		return es
	}
	for i := range models.All {
		if m := &models.All[i]; !m.ASR && (!m.Legacy || s.ModelStatus(m.ID).State == models.Ready) {
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
		es = append(es, Engine{ID: prefs.EndpointEngine(e.ID), Kind: MT, Cloud: true, Endpoint: &e})
	}
	return es
}

func (e Engine) Family() string {
	if e.Model != nil {
		return e.Model.Family
	}
	if _, ok := CloudSizeByID(e.ID); ok {
		return "cloud"
	}
	return e.ID
}

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

func (c *Core) Recommended(e Engine) bool { return e.ID != "" && e.ID == c.recommended(e.Kind) }

func (c *Core) recommended(k Kind) string {
	vram, _ := c.gpu()
	for _, e := range c.Engines(k) {
		if m := e.Model; m != nil && m.Recommended && vram != 0 && c.Shortfall(*m) == (Shortfall{}) {
			return e.ID
		}
	}
	if k == MT {
		return "cloud"
	}
	return ""
}

type Shortfall struct {
	NeedGB, HasGB uint64
	GPU           bool
}

func (c *Core) Shortfall(m models.Model) Shortfall {
	mem, memKnown := c.memory()
	need, has := gb(m.Needs.Memory), gb(mem)
	if memKnown && has < need {
		return Shortfall{NeedGB: need, HasGB: has}
	}
	vram, vramKnown := c.gpu()
	need, has = gb(m.Needs.VRAM), gb(vram)
	if vramKnown && vram != 0 && has < need {
		return Shortfall{NeedGB: need, HasGB: has, GPU: true}
	}
	return Shortfall{}
}

func (c *Core) Runs(m models.Model) bool {
	_, memKnown := c.memory()
	vram, vramKnown := c.gpu()
	if !memKnown || m.Needs.VRAM != 0 && (!vramKnown || vram == 0) {
		return false
	}
	return c.Shortfall(m) == Shortfall{}
}

func (c *Core) memory() (bytes uint64, known bool) {
	if c.deps.Memory == nil {
		return 0, false
	}
	return c.deps.Memory()
}

func (c *Core) gpu() (bytes uint64, known bool) {
	if c.deps.GPU == nil {
		return 0, false
	}
	return c.deps.GPU()
}

func gb(n uint64) uint64 { return (n + 1<<29) >> 30 }

func (c *Core) Engine(k Kind, id string) (Engine, bool) {
	for _, e := range c.Engines(k) {
		if e.ID == id {
			return e, true
		}
	}
	return Engine{}, false
}

func (c *Core) Usable(e Engine) bool {
	if e.Model != nil && c.Shortfall(*e.Model) != (Shortfall{}) {
		return false
	}
	return c.EngineService.usable(e, c.Prefs.APIs())
}

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
		if e.Model != nil && e.Model.Recommended {
			return e, true
		}
	}
	for i := len(found) - 1; i >= 0; i-- {
		if !legacy(found[i]) {
			return found[i], true
		}
	}
	return found[len(found)-1], true
}

func (c *Core) Suggested(f []Engine) Engine {
	runs := func(e Engine) bool { return e.Model == nil || c.Shortfall(*e.Model) == (Shortfall{}) }
	for _, e := range f {
		if e.Model != nil && e.Model.Recommended && runs(e) && !legacy(e) {
			return e
		}
	}
	for i := len(f) - 1; i >= 0; i-- {
		if runs(f[i]) && !legacy(f[i]) {
			return f[i]
		}
	}
	return f[0]
}

func legacy(e Engine) bool { return e.Model != nil && e.Model.Legacy }

func (c *Core) InUse(k Kind) string {
	if k == ASR {
		return c.Prefs.ASR
	}
	return c.Prefs.MT
}

func (c *Core) Chosen(k Kind) string {
	return c.EngineService.chosen(k, c.InUse(k))
}

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
			was := c.activeEngines()
			c.Prefs.ASR = e.ID
			c.refit(was)
		}
	}
}

func (c *Core) usePending() {
	ready := c.EngineService.readyPending(c.Prefs.Endpoints, c.Prefs.APIs())
	for _, e := range ready {
		c.use(e)
	}
	if len(ready) > 0 {
		c.settingsChanged()
	}
}
