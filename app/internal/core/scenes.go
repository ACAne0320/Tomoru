package core

import (
	"log"
	"slices"

	"tomoru/internal/asr"
	"tomoru/internal/lang"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
)

func (c *Core) loadScenes() {
	selected, err := c.SceneService.load(c.deps.ScenesPath, c.Prefs.Scene)
	if err != nil {
		log.Println("scenes:", err)
	}
	c.Prefs.Scene = selected
}

func (c *Core) currentScene() *scene.Scene { return c.SceneService.current(c.Prefs.Scene) }

func (c *Core) Scene() *scene.Scene {
	return &scene.Clone([]scene.Scene{*c.currentScene()})[0]
}

func (c *Core) SetScene(id string) {
	if id == c.Prefs.Scene || c.sceneIndex(id) < 0 {
		return
	}
	c.closeHistory()
	was := c.activeEngines()
	names := c.hints().Names
	src, tgt := c.SceneService.selectScene(c.Prefs.Scene, id, c.Prefs.Src, c.Prefs.Tgt)
	before := [2]string{c.Prefs.Src, c.Prefs.Tgt}
	c.Prefs.Scene, c.Prefs.Src, c.Prefs.Tgt = id, src, tgt
	c.fitLangs()
	spec, _ := asr.ByID(c.Active(ASR))
	if before != [2]string{c.Prefs.Src, c.Prefs.Tgt} || c.activeEngines() != was || spec.TakesScene && !slices.Equal(names, c.hints().Names) {
		c.RestartIfRunning()
	}
	c.settingsChanged()
}

func (c *Core) keepLangs() { c.SceneService.setLanguages(c.Prefs.Scene, c.Prefs.Src, c.Prefs.Tgt) }

func (c *Core) AddScene(name string) string {
	id := c.SceneService.addScene(name, c.Prefs.Src, c.Prefs.Tgt)
	c.settingsChanged()
	return id
}

func (c *Core) DeleteScene(id string) func() {
	if c.sceneIndex(id) <= 0 {
		return func() {}
	}
	inUse := c.Prefs.Scene == id
	if inUse {
		c.SetScene(scene.GeneralID)
	}
	undo, removed := c.SceneService.deleteScene(id)
	if removed {
		c.settingsChanged()
	}
	return func() {
		if !undo() {
			return
		}
		if inUse {
			c.SetScene(id)
		}
		c.settingsChanged()
	}
}

func (c *Core) UpdateScene(id string, edit func(*scene.Scene)) {
	draft, changed := c.SceneService.updateScene(id, edit)
	if !changed {
		return
	}
	if id == c.Prefs.Scene && draft.Src != "" && draft.Tgt != "" && draft.Src != draft.Tgt && (draft.Src != c.Prefs.Src || draft.Tgt != c.Prefs.Tgt) {
		c.Prefs.Src, c.Prefs.Tgt = draft.Src, draft.Tgt
		c.fitLangs()
		c.RestartIfRunning()
	}
	c.settingsChanged()
}

func (c *Core) AddTerm(id string, t scene.Term) {
	if c.SceneService.addTerm(id, t) {
		c.settingsChanged()
	}
}

func (c *Core) DeleteTerm(id string, i int) func() {
	undo, removed := c.SceneService.deleteTerm(id, i)
	if removed {
		c.settingsChanged()
	}
	return func() {
		if undo() {
			c.settingsChanged()
		}
	}
}

func (c *Core) useGlossary() { c.SceneService.activate(c.Prefs.Scene) }

func (c *Core) SceneLangs(id string) (src, tgt string) {
	if id == c.Prefs.Scene {
		return c.Prefs.Src, c.Prefs.Tgt
	}
	if sc, ok := c.sceneByID(id); ok {
		return sc.Src, sc.Tgt
	}
	return "", ""
}

func (c *Core) SetSceneSrc(id, l string) {
	if id == c.Prefs.Scene {
		c.SetSrc(l)
		return
	}
	c.UpdateScene(id, func(sc *scene.Scene) { lang.Choose(&sc.Src, &sc.Tgt, l) })
}

func (c *Core) SetSceneTgt(id, l string) {
	if id == c.Prefs.Scene {
		c.SetTgt(l)
		return
	}
	c.UpdateScene(id, func(sc *scene.Scene) { lang.Choose(&sc.Tgt, &sc.Src, l) })
}

func (c *Core) SceneEngine(id string, k Kind) string {
	sc, ok := c.sceneByID(id)
	switch {
	case !ok:
		return ""
	case k == ASR:
		return sc.ASR
	}
	return sc.MT
}

func (c *Core) SceneActive(id string, k Kind) string {
	if chosen := c.SceneEngine(id, k); chosen != "" {
		if e, ok := c.Engine(k, chosen); ok && c.Usable(e) {
			return e.ID
		}
	}
	return c.InUse(k)
}

func (c *Core) Active(k Kind) string { return c.SceneActive(c.Prefs.Scene, k) }

func (c *Core) activeEngines() [2]string { return [2]string{c.Active(ASR), c.Active(MT)} }

func (c *Core) refit(was [2]string) bool {
	now := c.activeEngines()
	if now == was {
		return false
	}
	if now[0] != was[0] {
		c.fitLangs()
	}
	c.RestartIfRunning()
	return true
}

func (c *Core) effective() prefs.Prefs {
	p := c.Preferences()
	p.ASR, p.MT = c.Active(ASR), c.Active(MT)
	return p
}

func (c *Core) UsableEngines(k Kind) []Engine {
	return slices.DeleteFunc(c.Engines(k), func(e Engine) bool { return !c.Usable(e) })
}

func (c *Core) SetSceneEngine(id string, k Kind, engine string) {
	if _, ok := c.Engine(k, engine); engine != "" && !ok {
		return
	}
	was := c.activeEngines()
	_, changed := c.SceneService.updateScene(id, func(sc *scene.Scene) {
		if k == ASR {
			sc.ASR = engine
		} else {
			sc.MT = engine
		}
	})
	if !changed {
		return
	}
	c.refit(was)
	c.settingsChanged()
}

func (c *Core) SceneSourceLangs(id string) []string { return c.spoken(c.SceneActive(id, ASR)) }
