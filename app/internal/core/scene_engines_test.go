package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"tomoru/internal/lang"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
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

func TestSceneRecognitionGoneFitsLanguage(t *testing.T) {
	c, f, _ := sceneEngineCore(t)
	id := c.AddScene("VTuber")
	c.SetSceneEngine(id, ASR, "sensevoice-small")
	c.UpdateScene(id, func(sc *scene.Scene) { sc.ASR, sc.Src, sc.Tgt = "sensevoice-small", lang.Auto, "zh" })
	if err := c.SavePrefs(); err != nil {
		t.Fatal(err)
	}
	c.Prefs.Scene, c.Prefs.Src, c.Prefs.Tgt = id, lang.Auto, "zh"
	c.loadScenes()
	if c.Active(ASR) != c.Prefs.ASR {
		t.Fatalf("the scene's model is gone: %q", c.Active(ASR))
	}
	c.Start()
	f.flush(t, 1)
	if src := f.sessions[0].cfg.Src; src == lang.Auto || !slices.Contains(c.spoken(c.Prefs.ASR), src) {
		t.Errorf("the default heard %q", src)
	}
}

func TestSceneModelDownloadedAgain(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	a, b := c.AddScene("A"), c.AddScene("B")
	m.ready["local-2b"] = true
	c.SetSceneEngine(a, MT, "local-2b")
	c.SetSceneEngine(b, MT, "local-2b")
	delete(m.ready, "local-2b")
	c.SetScene(a)
	c.Start()
	f.flush(t, 1)
	if c.runEngines.MT != "cloud" || c.serving() != "" {
		t.Fatalf("missing: ran %q, serving %q", c.runEngines.MT, c.serving())
	}
	m.ready["local-2b"] = true
	c.SetScene(b)
	f.flush(t, 1)
	if len(f.sessions) != 2 || c.runEngines.MT != "local-2b" || c.serving() != "local-2b" {
		t.Fatalf("switched after downloading: %d sessions, ran %q, serving %q", len(f.sessions), c.runEngines.MT, c.serving())
	}
	delete(m.ready, "local-2b")
	c.Changed()
	f.flush(t, 1)
	m.ready["local-2b"] = true
	c.Changed()
	f.flush(t, 1)
	if len(f.sessions) != 4 || c.runEngines.MT != "local-2b" {
		t.Errorf("downloaded while running: %d sessions, ran %q", len(f.sessions), c.runEngines.MT)
	}
}

func TestRetranslatingKeepsLiveServer(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	srv := c.deps.Server.(*fakeServer)
	m.ready["local-2b"], m.ready["local-9b"] = true, true
	a, b, d := c.AddScene("A"), c.AddScene("B"), c.AddScene("D")
	c.SetSceneEngine(a, MT, "local-2b")
	c.SetSceneEngine(b, MT, prefs.EndpointEngine(c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: "http://127.0.0.1:9/v1", Model: "m"}, "sk-1")))
	c.SetSceneEngine(d, MT, "local-9b")
	acquire := func(scene string) error {
		t.Helper()
		ready, err := c.retranslator(c.SceneActive(scene, MT))
		if err != nil {
			return err
		}
		done := make(chan error, 1)
		go func() { _, err := ready(context.Background()); done <- err }()
		return waitFor(t, f, done)
	}
	cloud, err := c.retranslator(c.SceneActive(b, MT))
	if err != nil {
		t.Fatal(err)
	}
	c.SetScene(a)
	c.Start()
	f.flush(t, 1)
	stops := srv.stops
	if _, err := cloud(context.Background()); err != nil || srv.stops != stops {
		t.Errorf("an API made before the subtitles started stopped their server: %v, %d stops", err, srv.stops-stops)
	}
	if err := acquire(b); err != nil || srv.stops != stops {
		t.Errorf("an API stopped the live server: %v, %d stops", err, srv.stops-stops)
	}
	if err := acquire(d); !errors.Is(err, ErrModelBusy) {
		t.Errorf("another on-device model while subtitles run: %v", err)
	}
	if err := acquire(a); err != nil {
		t.Errorf("the model the subtitles use: %v", err)
	}
	c.Stop()
	c.Image = ImageView{PNG: []byte{1}, Via: "local-9b", Running: true}
	if err := acquire(a); !errors.Is(err, ErrModelBusy) {
		t.Errorf("another on-device model while a picture translates: %v", err)
	}
}

func TestSceneServiceKeyRemovedWhileOpen(t *testing.T) {
	c, f, _ := sceneEngineCore(t)
	sec := c.deps.Secrets.(fakeSecrets)
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-1")
	id := c.AddScene("Stream")
	c.SetSceneEngine(id, ASR, QwenCloud)
	if c.SceneActive(id, ASR) != QwenCloud {
		t.Fatalf("key saved: %q", c.SceneActive(id, ASR))
	}
	clear(sec)
	c.SetScene(id)
	c.Start()
	f.flush(t, 1)
	if c.runEngines.ASR != c.Prefs.ASR || c.Active(ASR) != c.Prefs.ASR {
		t.Errorf("started without the key: ran %q", c.runEngines.ASR)
	}
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-1")
	f.flush(t, 1)
	if c.Active(ASR) != QwenCloud || c.runEngines.ASR != QwenCloud {
		t.Fatalf("key saved again: %q, ran %q", c.Active(ASR), c.runEngines.ASR)
	}
	clear(sec)
	c.EngineService.keysAt = c.EngineService.keysAt.Add(-RefreshEvery - time.Second)
	if slices.ContainsFunc(c.UsableEngines(ASR), func(e Engine) bool { return e.ID == QwenCloud }) {
		t.Error("the picker still offers a service whose key was removed")
	}
}

func TestDeletingDefaultKeepsSceneServer(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	srv := c.deps.Server.(*fakeServer)
	m.ready["local-2b"], m.ready["local-9b"] = true, true
	c.SetEngine("local-2b")
	id := c.AddScene("FF7")
	c.SetSceneEngine(id, MT, "local-9b")
	c.SetScene(id)
	c.Start()
	f.flush(t, 1)
	stops := srv.stops
	c.DeleteModel("local-2b")
	if srv.stops != stops || len(f.sessions) != 1 || !c.Running || c.Prefs.MT != "cloud" {
		t.Errorf("stopped %d times, %d sessions, running %v, default %q", srv.stops-stops, len(f.sessions), c.Running, c.Prefs.MT)
	}
	c.DeleteModel("local-9b")
	f.flush(t, 1)
	if srv.stops == stops || c.runEngines.MT != "cloud" {
		t.Errorf("the scene's own deleted: %d stops, ran %q", srv.stops-stops, c.runEngines.MT)
	}
}

func TestSceneServiceKeyGone(t *testing.T) {
	c, _, _ := sceneEngineCore(t)
	c.Prefs.ASRAPIs = []prefs.Service{{ID: QwenCloud, Key: true}}
	id := c.AddScene("Stream")
	c.UpdateScene(id, func(sc *scene.Scene) { sc.ASR = QwenCloud })
	if slices.ContainsFunc(c.UsableEngines(ASR), func(e Engine) bool { return e.ID == QwenCloud }) || c.SceneActive(id, ASR) != c.Prefs.ASR {
		t.Fatalf("no key in the keychain: %q", c.SceneActive(id, ASR))
	}
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-1")
	if c.SceneActive(id, ASR) != QwenCloud {
		t.Errorf("key saved: %q", c.SceneActive(id, ASR))
	}
}

type gatedServer struct {
	*fakeServer
	gated string
	gate  chan struct{}
}

func (s *gatedServer) Ensure(ctx context.Context, path string) (string, error) {
	if path == s.gated {
		<-s.gate
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.fakeServer.Ensure(ctx, path)
}

func waitFor(t *testing.T, f *fake, done <-chan error) error {
	t.Helper()
	var err error
	f.until(t, func() bool {
		select {
		case err = <-done:
			return true
		default:
			return false
		}
	})
	return err
}

func (s *gatedServer) EnsureProjector(ctx context.Context, path string) (string, error) {
	return s.Ensure(ctx, path)
}

func TestLiveStartedWhileRetranslatingWaits(t *testing.T) {
	c, f, m := sceneEngineCore(t)
	g := &gatedServer{fakeServer: &fakeServer{}, gated: m.Path("local-2b"), gate: make(chan struct{})}
	c.deps.Server = g
	c.EngineService = newEngineService(c.work, m, g, nil, c.deps.Secrets, c.changed)
	m.ready["local-2b"], m.ready["local-9b"] = true, true
	a, b := c.AddScene("A"), c.AddScene("B")
	c.SetSceneEngine(a, MT, "local-2b")
	c.SetSceneEngine(b, MT, "local-9b")
	ready, err := c.retranslator(c.SceneActive(a, MT))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := ready(context.Background()); done <- err }()
	f.flush(t, 1)
	c.SetScene(b)
	c.Start()
	f.flush(t, 1)
	close(g.gate)
	if err := waitFor(t, f, done); !errors.Is(err, ErrModelBusy) {
		t.Errorf("retranslating took the server from live subtitles: %v", err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !slices.Equal(g.ensured, []string{m.Path("local-9b")}) || !c.Running || c.serving() != "local-9b" {
		t.Errorf("loaded %q, running %v, serving %q", g.ensured, c.Running, c.serving())
	}
}
