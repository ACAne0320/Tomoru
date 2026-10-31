package core

import (
	"log"
	"slices"

	"tomoru/internal/asr"
	"tomoru/internal/scene"
)

// loadScenes composes scene storage with the preference naming the selected scene.
func (c *Core) loadScenes() {
	selected, err := c.SceneService.load(c.deps.ScenesPath, c.Prefs.Scene)
	if err != nil {
		log.Println("scenes:", err)
	}
	c.Prefs.Scene = selected
}

func (c *Core) currentScene() *scene.Scene { return c.SceneService.current(c.Prefs.Scene) }

// Scene returns a detached copy of the scene in use.
func (c *Core) Scene() *scene.Scene {
	return &scene.Clone([]scene.Scene{*c.currentScene()})[0]
}

// SetScene coordinates scene selection with history and a possible language restart.
func (c *Core) SetScene(id string) {
	if id == c.Prefs.Scene || c.sceneIndex(id) < 0 {
		return
	}
	c.closeHistory()
	names := c.hints().Names
	src, tgt := c.SceneService.selectScene(c.Prefs.Scene, id, c.Prefs.Src, c.Prefs.Tgt)
	before := [2]string{c.Prefs.Src, c.Prefs.Tgt}
	c.Prefs.Scene, c.Prefs.Src, c.Prefs.Tgt = id, src, tgt
	c.fitLangs()
	// A recognizer told the scene's names hears the new scene's from a new session.
	spec, _ := asr.ByID(c.Prefs.ASR)
	if before != [2]string{c.Prefs.Src, c.Prefs.Tgt} || spec.Hints && !slices.Equal(names, c.hints().Names) {
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
		c.SetScene("")
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

// UpdateScene commits a detached edit, coordinating changed active languages with a session.
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
