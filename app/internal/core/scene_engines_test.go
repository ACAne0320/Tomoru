package core

import (
	"slices"
	"testing"

	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func sceneEngineCore(t *testing.T) (*Core, *fake, *fakeModels) {
	c, f, m, _, _ := engineCore(t)
	c.deps.ScenesPath = t.TempDir() + "/scenes.json"
	c.deps.ASRPath = func() (string, error) { return "/asr", nil }
	c.useGlossary()
	return c, f, m
}

func TestSceneEnginesFollowDefault(t *testing.T) {
	c, _, m := sceneEngineCore(t)
	id := c.AddScene("FF7")
	c.SetScene(id)
	if c.Active(MT) != "cloud" || c.Active(ASR) != c.Prefs.ASR || c.SceneEngine(id, MT) != "" {
		t.Fatalf("follows the default: %q %q", c.Active(ASR), c.Active(MT))
	}
	m.ready["local-2b"] = true
	c.SetEngine("local-2b")
	if c.Active(MT) != "local-2b" || c.SceneActive("", MT) != "local-2b" {
		t.Errorf("the default changed: %q %q", c.Active(MT), c.SceneActive("", MT))
	}
}

func TestSceneEngineOverride(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	m.ready["local-2b"] = true
	id := c.AddScene("FF7")
	c.SetSceneEngine(id, MT, "local-2b")
	c.SetSceneEngine(id, MT, "gone")
	if c.SceneEngine(id, MT) != "local-2b" || c.Active(MT) != "cloud" || c.SceneActive(id, MT) != "local-2b" {
		t.Fatalf("scene %q, active %q", c.SceneEngine(id, MT), c.Active(MT))
	}
	c.Start()
	f.flush(t, 1)
	c.SetScene(id)
	f.flush(t, 1)
	ep, _ := f.sessions[1].cfg.Translator.(*translate.Endpoint)
	if len(f.sessions) != 2 || c.runEngines.MT != "local-2b" || ep == nil || ep.URL != "http://127.0.0.1:1234" {
		t.Fatalf("switched to the scene: %d sessions, %q, %+v", len(f.sessions), c.runEngines.MT, ep)
	}
	c.SetEngine("cloud-9b")
	if len(f.sessions) != 2 || c.Active(MT) != "local-2b" {
		t.Errorf("a new default restarted the scene's own: %d sessions, %q", len(f.sessions), c.Active(MT))
	}
	c.SetScene("")
	f.flush(t, 1)
	if len(f.sessions) != 3 || c.runEngines.MT != "cloud-9b" {
		t.Errorf("back to general: %d sessions, %q", len(f.sessions), c.runEngines.MT)
	}
	c.SetSceneEngine(id, MT, "")
	if c.SceneActive(id, MT) != "cloud-9b" || len(f.sessions) != 3 {
		t.Errorf("follows again: %q, %d sessions", c.SceneActive(id, MT), len(f.sessions))
	}
}

func TestSceneEngineFallsBack(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	m.ready["local-2b"], m.ready["sensevoice-small"] = true, true
	id := c.AddScene("FF7")
	c.SetScene(id)
	c.SetSceneEngine(id, MT, "local-2b")
	c.SetSceneEngine(id, ASR, "sensevoice-small")
	if !slices.Equal(c.SourceLangs(), c.spoken("sensevoice-small")) || !slices.Equal(c.SceneSourceLangs(id), c.SourceLangs()) {
		t.Errorf("languages of the scene's recognition: %v", c.SourceLangs())
	}
	c.Start()
	f.flush(t, 1)
	if c.runEngines.ASR != "sensevoice-small" || c.runEngines.MT != "local-2b" {
		t.Fatalf("ran %+v", c.runEngines)
	}
	c.DeleteModel("local-2b")
	f.flush(t, 1)
	if !c.Running || c.Err != nil || c.runEngines.MT != "cloud" || c.Prefs.MT != "cloud" || c.SceneEngine(id, MT) != "local-2b" {
		t.Errorf("model deleted: running %v, err %v, ran %+v, scene %q", c.Running, c.Err, c.runEngines, c.SceneEngine(id, MT))
	}
	c.DeleteModel("sensevoice-small")
	f.flush(t, 1)
	if !c.Running || c.Err != nil || c.runEngines.ASR != c.Prefs.ASR {
		t.Errorf("recognition deleted: running %v, err %v, ran %+v", c.Running, c.Err, c.runEngines)
	}
	m.ready["local-2b"] = true
	if c.Active(MT) != "local-2b" {
		t.Errorf("downloaded again: %q", c.Active(MT))
	}
}

func TestSceneEndpointDeleted(t *testing.T) {
	c, f, _ := sceneEngineCore(t)
	ep := prefs.EndpointEngine(c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: "http://127.0.0.1:9/v1", Model: "m"}, "sk-1"))
	id := c.AddScene("FF7")
	c.SetScene(id)
	c.SetSceneEngine(id, MT, ep)
	c.Start()
	f.flush(t, 1)
	eid, _ := prefs.EndpointOf(ep)
	undo := c.DeleteEndpoint(eid)
	f.flush(t, 1)
	if !c.Running || c.Err != nil || c.runEngines.MT != "cloud" || len(f.sessions) != 2 {
		t.Fatalf("endpoint deleted: running %v, err %v, ran %q, %d sessions", c.Running, c.Err, c.runEngines.MT, len(f.sessions))
	}
	undo()
	f.flush(t, 1)
	if c.runEngines.MT != ep || len(f.sessions) != 3 {
		t.Errorf("undone: ran %q, %d sessions", c.runEngines.MT, len(f.sessions))
	}
}

func TestUsableEngines(t *testing.T) {
	c, _, m := sceneEngineCore(t)
	m.ready["local-2b"] = true
	has := func(k Kind, id string) bool {
		return slices.ContainsFunc(c.UsableEngines(k), func(e Engine) bool { return e.ID == id })
	}
	if !has(MT, "local-2b") || has(MT, "local-9b") || !has(MT, "cloud") || has(MT, HyMTCloud) || has(ASR, QwenCloud) || has(ASR, "sensevoice-small") {
		t.Errorf("usable %v / %v", c.UsableEngines(MT), c.UsableEngines(ASR))
	}
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-1")
	if !has(ASR, QwenCloud) {
		t.Error("a cloud service with its key")
	}
}
