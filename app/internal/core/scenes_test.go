package core

import (
	"reflect"
	"testing"

	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

func scenesCore(t *testing.T) (*Core, *fake) {
	c, f := newCore(t)
	c.deps.ScenesPath = t.TempDir() + "/scenes.json"
	c.useGlossary()
	return c, f
}

// TestSceneLanguages gives each scene its languages: a scene takes those it is used in, and gives
// them back when used again; the general one too.
func TestSceneLanguages(t *testing.T) {
	c, f := scenesCore(t)
	id := c.AddScene(" FF7 ")
	if s, _ := c.SceneByID(id); s.Name != "FF7" || s.Src != "en" || s.Tgt != "zh" || c.Prefs.Scene != "" {
		t.Fatalf("added %+v, in use %q", s, c.Prefs.Scene)
	}
	c.SetScene(id)
	c.SetSrc("ja")
	c.Start()
	f.flush(t, 1)
	c.SetScene("") // back to English: translating restarts
	f.flush(t, 1)
	if c.Prefs.Src != "en" || c.Prefs.Tgt != "zh" || len(f.sessions) != 2 || f.sessions[1].cfg.Src != "en" {
		t.Errorf("general: %s→%s, %d sessions", c.Prefs.Src, c.Prefs.Tgt, len(f.sessions))
	}
	c.SetScene(id)
	if c.Prefs.Src != "ja" || c.Prefs.Tgt != "zh" {
		t.Errorf("FF7: %s→%s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetScene("gone")
	if c.Prefs.Scene != id {
		t.Errorf("in use %q", c.Prefs.Scene)
	}
}

// TestGlossary applies the glossary of the scene in use to what sessions hear, and none without
// scenes.
func TestGlossary(t *testing.T) {
	c, f := scenesCore(t)
	id := c.AddScene("FF7")
	c.AddTerm(id, scene.Term{Src: " クラウド ", Tgt: "克劳德", Heard: []string{"クラウト", "クラウド", ""}})
	c.AddTerm(id, scene.Term{Src: "クラウド", Tgt: "克劳德", Heard: []string{"グラウド"}}) // the same name: replaced, heard kept
	s, _ := c.SceneByID(id)
	if len(s.Terms) != 1 || !reflect.DeepEqual(s.Terms[0].Heard, []string{"グラウド", "クラウト"}) || s.Terms[0].Added.IsZero() {
		t.Fatalf("terms %+v", s.Terms)
	}
	c.Start()
	f.flush(t, 1)
	apply := f.sessions[0].cfg.Terms
	if text, terms := apply("クラウトだ"); text != "クラウトだ" || terms != nil {
		t.Errorf("general: %q %v", text, terms)
	}
	c.SetScene(id) // no restart: the glossary changes from the next line
	want := []translate.Term{{Src: "クラウド", Tgt: "克劳德"}}
	if text, terms := apply("クラウトだ"); text != "クラウドだ" || !reflect.DeepEqual(terms, want) || len(f.sessions) != 1 {
		t.Errorf("FF7: %q %v, %d sessions", text, terms, len(f.sessions))
	}

	// A view edits a term: it applies, and is kept, once the views built.
	c.UpdateScene(id, func(s *scene.Scene) { s.Terms[0].Tgt = "克劳德·斯特莱夫" })
	if _, terms := apply("クラウド"); terms[0].Tgt != "克劳德·斯特莱夫" {
		t.Errorf("edited: %v", terms)
	}
	if ss, err := scene.Load(c.deps.ScenesPath); err != nil || len(ss) != 2 || ss[1].Terms[0].Tgt != "克劳德·斯特莱夫" {
		t.Errorf("kept %+v %v", ss, err)
	}

	undo := c.DeleteTerm(id, 0)
	if _, terms := apply("クラウド"); terms != nil {
		t.Errorf("deleted: %v", terms)
	}
	undo()
	if _, terms := apply("クラウド"); len(terms) != 1 {
		t.Errorf("undone: %v", terms)
	}
}

// TestSceneAbout sends the scene's description with each sentence, as it is now: chosen or edited
// while translating, from the next one; not to the 2B model, which made up names from it.
func TestSceneAbout(t *testing.T) {
	c, f := scenesCore(t)
	id := c.AddScene("药屋")
	c.UpdateScene(id, func(s *scene.Scene) { s.About = " 药屋少女的呢喃，角色有猫猫、壬氏 " })
	c.Start()
	f.flush(t, 1)
	about := f.sessions[0].cfg.About
	if about == nil || about() != "" {
		t.Fatalf("general: set %v", about != nil)
	}
	c.SetScene(id)
	if got := about(); got != "药屋少女的呢喃，角色有猫猫、壬氏" || len(f.sessions) != 1 {
		t.Errorf("chosen: %q, %d sessions", got, len(f.sessions))
	}
	c.UpdateScene(id, func(s *scene.Scene) { s.About = "药屋少女的呢喃 第3季" })
	if got := about(); got != "药屋少女的呢喃 第3季" {
		t.Errorf("edited: %q", got)
	}

	c.Stop()
	c.Prefs.MT = "local-2b"
	c.Start()
	f.flush(t, 1)
	if s := f.sessions[len(f.sessions)-1]; s.cfg.About != nil {
		t.Errorf("2B: %q", s.cfg.About())
	}
}

// TestDeleteScene deletes the scene in use: translating goes on in the general one, and undoing
// uses it again.
func TestDeleteScene(t *testing.T) {
	c, _ := scenesCore(t)
	id := c.AddScene("FF7")
	c.SetScene(id)
	undo := c.DeleteScene(id)
	if c.Prefs.Scene != "" || len(c.Scenes) != 1 {
		t.Fatalf("in use %q, %d scenes", c.Prefs.Scene, len(c.Scenes))
	}
	if c.DeleteScene(""); len(c.Scenes) != 1 {
		t.Error("deleted the general scene")
	}
	undo()
	if c.Prefs.Scene != id || len(c.Scenes) != 2 {
		t.Errorf("undone: in use %q, %d scenes", c.Prefs.Scene, len(c.Scenes))
	}
}

// TestLoadScenes uses the general scene when the one the settings name is gone.
func TestLoadScenes(t *testing.T) {
	c, _ := scenesCore(t)
	c.deps.ScenesPath = ""
	c.Prefs.Scene = "gone"
	c.loadScenes()
	if c.Prefs.Scene != "" || len(c.Scenes) != 1 || c.Scene().ID != "" {
		t.Errorf("in use %q, %+v", c.Prefs.Scene, c.Scenes)
	}
}
