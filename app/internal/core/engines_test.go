package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"tomoru/internal/scene"

	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

type fakeModels struct{ ready map[string]bool }

func (m *fakeModels) Status(id string) models.Status {
	if m.ready[id] {
		return models.Status{State: models.Ready}
	}
	return models.Status{State: models.Missing}
}
func (m *fakeModels) Path(id string) string  { return "/models/" + id + ".gguf" }
func (m *fakeModels) Download(id string)     { m.ready[id] = true }
func (m *fakeModels) Cancel(string)          {}
func (m *fakeModels) Delete(id string) error { delete(m.ready, id); return nil }

type fakeServer struct {
	mu      sync.Mutex
	ensured []string
	stops   int
	url     string // where the model answers; "" for nowhere
}

func (s *fakeServer) Ensure(_ context.Context, path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensured = append(s.ensured, path)
	return cmp.Or(s.url, "http://127.0.0.1:1234"), nil
}

func (s *fakeServer) EnsureVision(ctx context.Context, path string) (string, error) {
	return s.Ensure(ctx, path)
}

func (s *fakeServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stops++
}

type fakeSecrets map[string][]byte

func (s fakeSecrets) Get(k string) ([]byte, error) {
	if v, ok := s[k]; ok {
		return v, nil
	}
	return nil, errors.New("not found")
}
func (s fakeSecrets) Set(k string, v []byte) error { s[k] = v; return nil }
func (s fakeSecrets) Delete(k string) error        { delete(s, k); return nil }

// engineCore is a Core whose translators are the real choice among engines.
func engineCore(t *testing.T) (*Core, *fake, *fakeModels, *fakeServer, fakeSecrets) {
	c, f := newCore(t)
	m, srv, sec := &fakeModels{ready: map[string]bool{}}, &fakeServer{}, fakeSecrets{}
	c.deps.Translator, c.deps.Models, c.deps.Server, c.deps.Secrets = nil, m, srv, sec
	c.EngineService = newEngineService(c.work, m, srv, nil, sec, c.changed)
	return c, f, m, srv, sec
}

func TestLocalModel(t *testing.T) {
	c, f, m, srv, _ := engineCore(t)
	c.SetEngine("local-2b")
	c.Start()
	if !errors.Is(c.Err, ErrModelMissing) || c.Running {
		t.Fatalf("not downloaded: running %v, err %v", c.Running, c.Err)
	}

	c.Download("local-2b")
	c.Start()
	if c.State.Phase != "loading" || !c.Running {
		t.Errorf("loading: phase %q", c.State.Phase)
	}
	f.flush(t, 1)
	ep, ok := f.sessions[0].cfg.Translator.(*translate.Endpoint)
	if !ok || ep.URL != "http://127.0.0.1:1234" || ep.Style != translate.Index || len(srv.ensured) != 1 || srv.ensured[0] != m.Path("local-2b") {
		t.Errorf("translator %+v, ensured %q", ep, srv.ensured)
	}

	// Back to the cloud: the server goes.
	c.SetEngine("cloud")
	f.flush(t, 1)
	srv.mu.Lock()
	stops := srv.stops
	srv.mu.Unlock()
	if stops == 0 {
		t.Error("the server still runs")
	}

	// Deleting the model in use goes back to the cloud.
	c.SetEngine("local-2b")
	f.flush(t, 1)
	c.DeleteModel("local-2b")
	f.flush(t, 1)
	if c.Prefs.MT != "cloud" || m.ready["local-2b"] {
		t.Errorf("after delete: engine %q", c.Prefs.MT)
	}
}

func TestEndpoints(t *testing.T) {
	var auth, model string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, model = r.Header.Get("Authorization"), r.URL.Path
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家晚上好。\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer api.Close()
	c, f, _, _, sec := engineCore(t)

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL + "/v1/", Model: "m"}, "sk-1")
	e, ok := c.Prefs.Endpoint(id)
	if !ok || e.URL != api.URL || string(sec[keyName(id)]) != "sk-1" {
		t.Fatalf("saved %+v, key %q", e, sec[keyName(id)])
	}

	// A test with the key kept.
	c.TestEndpoint(id, *e, "")
	if !c.Tests[id].Running {
		t.Error("not running")
	}
	f.flush(t, 1)
	if r := c.Tests[id]; r.Err != nil || r.Text != "大家晚上好。" || auth != "Bearer sk-1" || model != "/v1/chat/completions" {
		t.Errorf("test %+v, auth %q, path %q", r, auth, model)
	}

	// Translating with it.
	c.SetEngine(prefs.CustomPrefix + id)
	c.Start()
	f.flush(t, 1)
	if ep := f.sessions[0].cfg.Translator.(*translate.Endpoint); ep.APIKey != "sk-1" || ep.Style != translate.Chat || ep.Llama {
		t.Errorf("translator %+v", ep)
	}

	// Deleting it stops translating, rather than going on with the cloud, and forgets the key.
	undo := c.DeleteEndpoint(id)
	if c.Prefs.MT != prefs.CustomPrefix+id || len(c.Prefs.Endpoints) != 0 || sec[keyName(id)] != nil {
		t.Errorf("after delete: engine %q, endpoints %v, key %q", c.Prefs.MT, c.Prefs.Endpoints, sec[keyName(id)])
	}
	if c.Running || !errors.Is(c.Err, ErrNoEndpoint) || len(f.sessions) != 1 {
		t.Errorf("after delete: running %v, err %v, %d sessions", c.Running, c.Err, len(f.sessions))
	}

	// Undoing it puts it back, its key, and translating with it.
	undo()
	f.flush(t, 1)
	if _, ok := c.Prefs.Endpoint(id); !ok || c.Prefs.MT != prefs.CustomPrefix+id || string(sec[keyName(id)]) != "sk-1" {
		t.Errorf("after undo: engine %q, endpoints %v, key %q", c.Prefs.MT, c.Prefs.Endpoints, sec[keyName(id)])
	}
	if !c.Running || len(f.sessions) != 2 {
		t.Fatalf("after undo: running %v, err %v, %d sessions", c.Running, c.Err, len(f.sessions))
	}
	if ep, ok := f.sessions[1].cfg.Translator.(*translate.Endpoint); !ok || ep.APIKey != "sk-1" {
		t.Errorf("after undo: translator %+v", f.sessions[1].cfg.Translator)
	}
}

// TestChoosePending chooses a model not downloaded: the cloud translates until it comes in, then
// it does.
func TestChoosePending(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	c.Choose(MT, "local-9b")
	if c.Prefs.MT != "cloud" || c.Chosen(MT) != "local-9b" {
		t.Fatalf("in use %q, chosen %q", c.Prefs.MT, c.Chosen(MT))
	}
	m.ready["local-9b"] = true
	c.Changed()
	if c.Prefs.MT != "local-9b" || c.Chosen(MT) != "local-9b" {
		t.Errorf("came in: in use %q, chosen %q", c.Prefs.MT, c.Chosen(MT))
	}

	// Choosing one in use, or setting one, forgets the one waiting.
	c.Choose(MT, "local-2b")
	c.Choose(MT, "local-9b")
	m.ready["local-2b"] = true
	c.Changed()
	if c.Prefs.MT != "local-9b" {
		t.Errorf("chose again the one in use: in use %q", c.Prefs.MT)
	}
	c.Choose(MT, "cloud")
	delete(m.ready, "local-2b")
	c.Choose(MT, "local-2b")
	c.SetEngine("cloud")
	m.ready["local-2b"] = true
	c.Changed()
	if c.Prefs.MT != "cloud" {
		t.Errorf("set the cloud: in use %q", c.Prefs.MT)
	}
}

// TestRecognitionModel recognizes with a downloaded model: chosen before it is downloaded, it is
// used once it is, with tomoru-asr; deleted, recognition goes back to the system's, where there is
// one.
func TestRecognitionModel(t *testing.T) {
	c, f, m, _, _ := engineCore(t)
	c.Prefs.ASR = prefs.DefaultASR() // as the system has it at first
	c.deps.ASRPath = func() (string, error) { return "/asr", nil }
	c.Choose(ASR, "sensevoice-small")
	if c.Prefs.ASR != prefs.DefaultASR() || c.Chosen(ASR) != "sensevoice-small" {
		t.Fatalf("in use %q, chosen %q", c.Prefs.ASR, c.Chosen(ASR))
	}
	c.Download("sensevoice-small")
	c.Changed()
	if c.Prefs.ASR != "sensevoice-small" {
		t.Fatalf("came in: in use %q", c.Prefs.ASR)
	}
	c.Start()
	f.flush(t, 1)
	if f.sessions[0].cfg.Recognize == nil {
		t.Error("the session recognizes with the engine")
	}

	c.DeleteModel("sensevoice-small")
	if prefs.SystemASR() != "" {
		f.flush(t, 1) // restarted with the system's; without one, it stops for the user to choose
	} else if c.Running || !errors.Is(c.Err, ErrNoASR) {
		t.Errorf("after delete, no recognition: running %v, err %v", c.Running, c.Err)
	}
	if c.Prefs.ASR != prefs.DefaultASR() || m.ready["sensevoice-small"] {
		t.Errorf("after delete: in use %q", c.Prefs.ASR)
	}
	if prefs.SystemASR() != "" && f.sessions[len(f.sessions)-1].cfg.Recognize != nil {
		t.Error("restarted with a recognizer")
	}

	// Chosen in the settings file and not downloaded, it says so.
	c.Stop()
	c.Prefs.ASR = "sensevoice-small"
	c.Start()
	if !errors.Is(c.Err, ErrASRMissing) || c.Running {
		t.Errorf("not downloaded: running %v, err %v", c.Running, c.Err)
	}
}

// TestCloudRecognition chooses the cloud's recognition before it has a key: it is used once the
// key is saved, which goes to the credential store under the service's name.
func TestCloudRecognition(t *testing.T) {
	c, f, _, _, sec := engineCore(t)
	c.Prefs.ASR = prefs.DefaultASR() // as the system has it at first
	c.Choose(ASR, QwenCloud)
	if c.Prefs.ASR != prefs.DefaultASR() || c.Chosen(ASR) != QwenCloud {
		t.Fatalf("in use %q, chosen %q", c.Prefs.ASR, c.Chosen(ASR))
	}
	c.SaveASRAPI(prefs.ASRAPI{ID: QwenCloud, URL: " ws-1.cn-beijing.maas.aliyuncs.com "}, "sk-1")
	if a := c.ASRAPI(QwenCloud); c.Prefs.ASR != QwenCloud || !a.Key || string(sec["asr-key:qwen-cloud"]) != "sk-1" || a.URL != "ws-1.cn-beijing.maas.aliyuncs.com" {
		t.Fatalf("saved: in use %q, prefs %+v", c.Prefs.ASR, c.Prefs.ASRAPIs)
	}
	if got := c.EngineService.cloudConfig(c.ASRAPI(QwenCloud), ""); got.Key != "sk-1" {
		t.Errorf("config %+v", got)
	}
	// Saved again without a key, it keeps the one it has.
	c.SaveASRAPI(prefs.ASRAPI{ID: QwenCloud, Model: "qwen-audio-3.0-asr-flash-streaming"}, "")
	if a := c.ASRAPI(QwenCloud); !a.Key || len(c.Prefs.ASRAPIs) != 1 || string(sec["asr-key:qwen-cloud"]) != "sk-1" {
		t.Errorf("resaved: %+v", c.Prefs.ASRAPIs)
	}
	c.Start()
	f.flush(t, 1)
	if len(f.sessions) == 0 || f.sessions[0].cfg.Recognize == nil {
		t.Fatal("no recognizer")
	}

	c.Stop()
	c.Prefs.ASRAPIs[0].Key = false
	c.Start()
	if !errors.Is(c.Err, ErrASRCloudKey) {
		t.Errorf("without a key: %v", c.Err)
	}
}

// TestOpenAIRecognition sets up OpenAI's recognition beside Qwen's: each has its own key, and the
// one in use is the one started.
func TestOpenAIRecognition(t *testing.T) {
	c, f, _, _, sec := engineCore(t)
	c.SaveASRAPI(prefs.ASRAPI{ID: QwenCloud}, "sk-qwen")
	c.Choose(ASR, OpenAI)
	if c.Chosen(ASR) != OpenAI || c.Prefs.ASR == OpenAI {
		t.Fatalf("not set up, it waits: in use %q", c.Prefs.ASR)
	}
	c.SaveASRAPI(prefs.ASRAPI{ID: OpenAI, URL: "https://api.groq.com/openai", Model: "whisper-large-v3"}, "sk-oa")
	if c.Prefs.ASR != OpenAI || string(sec["asr-key:openai"]) != "sk-oa" || string(sec["asr-key:qwen-cloud"]) != "sk-qwen" {
		t.Fatalf("in use %q, keys %v", c.Prefs.ASR, sec)
	}
	if e, _ := c.Engine(ASR, OpenAI); !e.Cloud || !c.Usable(e) {
		t.Fatalf("%+v not usable", e)
	}
	c.Start()
	f.flush(t, 1)
	if len(f.sessions) == 0 || f.sessions[0].cfg.Recognize == nil || f.sessions[0].cfg.FinalsEnd {
		t.Fatal("no recognizer, or one whose finals end utterances")
	}
	if e := engines(c.Prefs); e.ASRModel != "whisper-large-v3" {
		t.Errorf("usage notes %+v", e)
	}
}

// TestNoRecognition starts with no recognition chosen, as on a computer without the system's:
// translating does not start, and says to choose one.
func TestNoRecognition(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	c.Prefs.ASR = ""
	c.Start()
	if !errors.Is(c.Err, ErrNoASR) || c.Running {
		t.Errorf("running %v, err %v", c.Running, c.Err)
	}
}

// TestServedModel: a recognition model llama-server runs is loaded on its own server, apart from
// translation's, in the background before the session starts; choosing another recognizer, or
// deleting the model, stops that server.
func TestServedModel(t *testing.T) {
	c, f, m, _, _ := engineCore(t)
	asrSrv := &fakeServer{}
	c.deps.ASRServer = asrSrv
	c.EngineService.asrServer = asrSrv
	m.ready["qwen3-asr-1.7b"] = true
	c.Choose(ASR, "qwen3-asr-1.7b")
	c.Start()
	f.flush(t, 1)
	if f.sessions[0].cfg.Recognize == nil || !f.sessions[0].cfg.VolatileSentences || f.sessions[0].cfg.FinalsEnd {
		t.Fatalf("the session: %+v", f.sessions[0].cfg)
	}
	if !slices.Equal(asrSrv.ensured, []string{"/models/qwen3-asr-1.7b.gguf"}) {
		t.Errorf("recognition's server ensured %v", asrSrv.ensured)
	}
	m.ready["sensevoice-small"] = true
	c.deps.ASRPath = func() (string, error) { return "/asr", nil }
	before := asrSrv.stops
	c.Choose(ASR, "sensevoice-small")
	if asrSrv.stops == before {
		t.Error("recognition's server runs on, the model no longer used")
	}
	before = asrSrv.stops
	c.DeleteModel("qwen3-asr-1.7b")
	if asrSrv.stops == before {
		t.Error("the model deleted while its server may hold it")
	}
}

// TestRecommended recommends Qwen3-ASR where a GPU has the memory it needs, and nothing else.
func TestRecommended(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	qwen3, _ := c.Engine(ASR, "qwen3-asr-1.7b")
	sense, _ := c.Engine(ASR, "sensevoice-small")
	if c.Recommended(qwen3) {
		t.Error("recommended with no GPU known")
	}
	gpu := uint64(8 << 30)
	c.deps.GPU = func() uint64 { return gpu }
	if !c.Recommended(qwen3) || c.Recommended(sense) {
		t.Error("an 8 GB GPU: Qwen3-ASR, not SenseVoice")
	}
	gpu = 2 << 30
	if c.Recommended(qwen3) {
		t.Error("recommended on a 2 GB GPU")
	}
}

// TestRecognitionHints tells recognition the active scene's names, as the session starts, and
// starts again for another scene's.
func TestRecognitionHints(t *testing.T) {
	c, f, _, _, _ := engineCore(t)
	c.SaveASRAPI(prefs.ASRAPI{ID: QwenCloud}, "sk-1")
	c.Prefs.ASR = QwenCloud
	id := c.AddScene("只狼")
	c.AddTerm(id, scene.Term{Src: "一心", Tgt: "一心"})
	c.AddTerm(id, scene.Term{Src: "葦名", Tgt: "苇名", Auto: true})
	c.SetScene(id)
	if h := c.hints(); !slices.Equal(h.Names, []string{"一心", "葦名"}) {
		t.Fatalf("hints %+v", h)
	}
	c.Start()
	f.flush(t, 1)
	if len(f.sessions) != 1 || f.sessions[0].cfg.Recognize == nil {
		t.Fatal("no recognizer")
	}
	// Another scene, of other names, in the same languages: a new session hears them.
	other := c.AddScene("葬送のフリーレン")
	c.AddTerm(other, scene.Term{Src: "フリーレン", Tgt: "芙莉莲"})
	c.SetScene(other)
	f.flush(t, 1)
	if len(f.sessions) != 2 {
		t.Fatalf("%d sessions after the scene changed", len(f.sessions))
	}
}
