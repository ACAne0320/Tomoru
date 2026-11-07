package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"tomoru/internal/scene"

	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

type fakeModels struct {
	ready  map[string]bool
	dir    string
	moving models.Move
	moved  func(error) // ends the move asked for
}

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
func (m *fakeModels) Dir() string            { return m.dir }
func (m *fakeModels) Moving() models.Move    { return m.moving }
func (m *fakeModels) Move(to string, done func(error)) {
	m.moving = models.Move{Active: true}
	m.moved = func(err error) {
		m.moving = models.Move{Err: err}
		if err == nil {
			m.dir = to
		}
		done(err)
	}
}

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
	if !ok || ep.URL != "http://127.0.0.1:1234" || ep.Format != translate.Index || len(srv.ensured) != 1 || srv.ensured[0] != m.Path("local-2b") {
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
		if b, _ := io.ReadAll(r.Body); !strings.Contains(string(b), "image_url") { // the test's picture goes alongside
			auth, model = r.Header.Get("Authorization"), r.URL.Path
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家晚上好。\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer api.Close()
	c, f, _, _, sec := engineCore(t)

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL + "/v1/", Model: "m"}, "sk-1")
	e, ok := c.Prefs.Endpoint(id)
	if !ok || e.URL != api.URL || e.Format != "chat" || string(sec[keyName(id)]) != "sk-1" {
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
	if ep := f.sessions[0].cfg.Translator.(*translate.Endpoint); ep.APIKey != "sk-1" || ep.Format != translate.Chat || ep.Llama {
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

	// Prompted as Index-Translate is, from the next session.
	e, _ = c.Prefs.Endpoint(id)
	changed := *e
	changed.Format = "index"
	c.SaveEndpoint(changed, "")
	f.flush(t, 1)
	if ep, ok := f.sessions[len(f.sessions)-1].cfg.Translator.(*translate.Endpoint); !ok || ep.Format != translate.Index {
		t.Errorf("as Index-Translate: translator %+v", f.sessions[len(f.sessions)-1].cfg.Translator)
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

// TestCloudRecognition can't use the cloud's recognition before it has a key; saving the key,
// which goes to the credential store under the service's name, uses nothing; then it can be used.
func TestCloudRecognition(t *testing.T) {
	c, f, _, _, sec := engineCore(t)
	c.Prefs.ASR = prefs.DefaultASR() // as the system has it at first
	if c.Use(ASR, QwenCloud) || c.Prefs.ASR != prefs.DefaultASR() {
		t.Fatalf("used without a key: in use %q", c.Prefs.ASR)
	}
	c.SaveAPI(prefs.Service{ID: QwenCloud, URL: " ws-1.cn-beijing.maas.aliyuncs.com "}, "sk-1")
	if a := c.API(QwenCloud); c.Prefs.ASR != prefs.DefaultASR() || !a.Key || string(sec["asr-key:qwen-cloud"]) != "sk-1" || a.URL != "ws-1.cn-beijing.maas.aliyuncs.com" {
		t.Fatalf("saved: in use %q, prefs %+v", c.Prefs.ASR, c.Prefs.ASRAPIs)
	}
	if !c.Use(ASR, QwenCloud) || c.Prefs.ASR != QwenCloud {
		t.Fatalf("set up: in use %q", c.Prefs.ASR)
	}
	if got := c.EngineService.cloudConfig(c.API(QwenCloud), ""); got.Key != "sk-1" {
		t.Errorf("config %+v", got)
	}
	// Saved again without a key, it keeps the one it has.
	c.SaveAPI(prefs.Service{ID: QwenCloud, Model: "qwen-audio-3.0-asr-flash-streaming"}, "")
	if a := c.API(QwenCloud); !a.Key || len(c.Prefs.ASRAPIs) != 1 || string(sec["asr-key:qwen-cloud"]) != "sk-1" {
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

// TestCloudSizes translates with Index-Translate's cloud in the size chosen; choosing the family
// uses 35B, the default, which alone of them reads pictures.
func TestCloudSizes(t *testing.T) {
	c, f, _, _, _ := engineCore(t)
	var family []Engine
	for _, fam := range c.Families(MT) {
		if fam[0].Family() == "cloud" {
			family = fam
		}
	}
	if e, ok := c.Pick(family); !ok || e.ID != "cloud" {
		t.Errorf("picked %q", e.ID)
	}
	c.SetEngine("cloud-9b")
	c.Start()
	f.flush(t, 1)
	if ep, ok := f.sessions[0].cfg.Translator.(*translate.Endpoint); !ok || ep.Model != "Index-Translate-9B" || ep.URL != translate.CloudURL {
		t.Errorf("translator %+v", f.sessions[0].cfg.Translator)
	}
	var pictures []string
	for _, e := range c.ImageEngines() {
		if e.Family() == "cloud" {
			pictures = append(pictures, e.ID)
		}
	}
	if !slices.Equal(pictures, []string{"cloud"}) {
		t.Errorf("for pictures %q", pictures)
	}
}

// TestCloudTranslation can't translate with Hy-MT2 on Tencent Cloud before it has a key; saved,
// with its key in the credential store under its own name, it translates with TokenHub's Hy-MT2 in
// Hy-MT2's format, Plus unless another model is chosen; and it is not offered for pictures, which
// Hy-MT2 doesn't read.
func TestCloudTranslation(t *testing.T) {
	c, f, _, _, sec := engineCore(t)
	if c.Use(MT, HyMTCloud) || c.Prefs.MT == HyMTCloud {
		t.Fatalf("used without a key: in use %q", c.Prefs.MT)
	}
	c.Prefs.MT = HyMTCloud // as settings kept it, its key gone
	c.Start()
	if !errors.Is(c.Err, ErrMTCloudKey) || c.Running {
		t.Fatalf("without a key: running %v, err %v", c.Running, c.Err)
	}
	c.Prefs.MT = "cloud"

	c.SaveAPI(prefs.Service{ID: HyMTCloud}, "sk-th")
	if !c.API(HyMTCloud).Key || string(sec["mt-key:hy-mt-cloud"]) != "sk-th" || len(c.Prefs.MTAPIs) != 1 || len(c.Prefs.ASRAPIs) != 0 || c.Prefs.MT != "cloud" {
		t.Fatalf("saved: in use %q, translation %+v, recognition %+v, keys %v", c.Prefs.MT, c.Prefs.MTAPIs, c.Prefs.ASRAPIs, sec)
	}
	if !c.Use(MT, HyMTCloud) || c.Prefs.MT != HyMTCloud {
		t.Fatalf("set up: in use %q", c.Prefs.MT)
	}
	translator := func() *translate.Endpoint {
		t.Helper()
		f.sessions = nil
		c.Start()
		f.flush(t, 1)
		c.Stop()
		ep, ok := f.sessions[0].cfg.Translator.(*translate.Endpoint)
		if !ok {
			t.Fatalf("translator %T", f.sessions[0].cfg.Translator)
		}
		return ep
	}
	if ep := translator(); ep.URL != "https://tokenhub.tencentmaas.com" || ep.Model != "hy-mt2-plus" || ep.APIKey != "sk-th" || ep.Format != translate.HyMT {
		t.Errorf("translator %+v", ep)
	}
	c.SaveAPI(prefs.Service{ID: HyMTCloud, Model: "hy-mt2-pro"}, "")
	if ep := translator(); ep.Model != "hy-mt2-pro" || ep.APIKey != "sk-th" {
		t.Errorf("with Pro: %+v", ep)
	}
	if slices.ContainsFunc(c.ImageEngines(), func(e Engine) bool { return e.ID == HyMTCloud }) {
		t.Error("offered for pictures")
	}
}

// TestOpenAIRecognition sets up OpenAI's recognition beside Qwen's: each has its own key, and the
// one in use is the one started.
func TestOpenAIRecognition(t *testing.T) {
	c, f, _, _, sec := engineCore(t)
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-qwen")
	c.SaveAPI(prefs.Service{ID: OpenAI, URL: "https://api.groq.com/openai", Model: "whisper-large-v3"}, "sk-oa")
	c.Use(ASR, OpenAI)
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

// TestRecommended recommends one engine of each kind: a family's size to recommend where the GPU
// has the memory it needs, Qwen3-ASR from 3 GB, Index-Translate 9B from 8 GB; else the cloud's
// translation, and no recognition.
func TestRecommended(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	recommended := func() (out []string) {
		for _, k := range []Kind{ASR, MT} {
			for _, e := range c.Engines(k) {
				if c.Recommended(e) {
					out = append(out, e.ID)
				}
			}
		}
		return out
	}
	if got := recommended(); !slices.Equal(got, []string{"cloud"}) {
		t.Errorf("no GPU known: %q", got)
	}
	gpu := uint64(0)
	c.deps.GPU = func() uint64 { return gpu }
	for _, tc := range []struct {
		gpu  uint64
		want []string
	}{
		{0, []string{"cloud"}},
		{2 << 30, []string{"cloud"}},
		{4 << 30, []string{"qwen3-asr-1.7b", "cloud"}},
		{7895 << 20, []string{"qwen3-asr-1.7b", "local-9b"}}, // a card of 8 GB, as its driver says
		{24 << 30, []string{"qwen3-asr-1.7b", "local-9b"}},
	} {
		gpu = tc.gpu
		if got := recommended(); !slices.Equal(got, tc.want) {
			t.Errorf("a GPU of %d MiB: %q, want %q", tc.gpu>>20, got, tc.want)
		}
	}
	// A Mac of 8 GB runs no 9B, whatever its GPU may take.
	c.deps.Memory = func() uint64 { return 8 << 30 }
	if got := recommended(); !slices.Equal(got, []string{"qwen3-asr-1.7b", "cloud"}) {
		t.Errorf("8 GB of memory: %q", got)
	}
}

// TestShortfall says what a computer lacks to run a model, its memory first, in whole GB; nothing
// where what it has is not known.
func TestShortfall(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	large, _ := models.ByID("local-9b")
	if got := c.Shortfall(large); got != (Shortfall{}) {
		t.Errorf("nothing known: %+v", got)
	}
	mem, gpu := uint64(8<<30), uint64(16<<30)
	c.deps.Memory, c.deps.GPU = func() uint64 { return mem }, func() uint64 { return gpu }
	if got := c.Shortfall(large); got != (Shortfall{Need: 16, Has: 8}) {
		t.Errorf("8 GB of memory: %+v", got)
	}
	mem, gpu = 32<<30, 6<<30
	if got := c.Shortfall(large); got != (Shortfall{Need: 8, Has: 6, GPU: true}) {
		t.Errorf("a GPU of 6 GB: %+v", got)
	}
	mem, gpu = 16<<30-200<<20, 7895<<20 // as Windows and a card of 8 GB tell them
	if got := c.Shortfall(large); got != (Shortfall{}) {
		t.Errorf("16 GB and a card of 8: %+v", got)
	}
}

// TestPick picks the size of a family to use on choosing it: the one in use, else the one to
// recommend, else the largest; only those downloaded that this computer runs.
func TestPick(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	index := c.Families(MT)[0]
	pick := func() string {
		e, ok := c.Pick(index)
		if !ok {
			return ""
		}
		return e.ID
	}
	if got := pick(); got != "" {
		t.Errorf("none downloaded: %q", got)
	}
	m.ready["local-2b"] = true
	if got := pick(); got != "local-2b" {
		t.Errorf("the 2B downloaded: %q", got)
	}
	m.ready["local-9b"] = true
	if got := pick(); got != "local-9b" {
		t.Errorf("both downloaded: %q, want the one to recommend", got)
	}
	c.SetEngine("local-2b")
	if got := pick(); got != "local-2b" {
		t.Errorf("the 2B in use: %q", got)
	}
	c.SetEngine("cloud")
	c.deps.Memory = func() uint64 { return 8 << 30 }
	if got := pick(); got != "local-2b" {
		t.Errorf("the 9B too large: %q", got)
	}
	if cloud := c.Families(MT)[2]; !slices.ContainsFunc(cloud, c.Usable) {
		t.Error("the cloud can't be used")
	}
}

// TestSuggested offers a family's size to recommend where this computer runs it, else the largest
// it runs, else the smallest.
func TestSuggested(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	index, hy := c.Families(MT)[0], c.Families(MT)[1]
	mem := uint64(16 << 30)
	c.deps.Memory = func() uint64 { return mem }
	for _, tc := range []struct {
		mem       uint64
		index, hy string
	}{
		{16 << 30, "local-9b", "hy-mt2-7b"},
		{8 << 30, "local-2b", "hy-mt2-1.8b"},
		{4 << 30, "local-2b", "hy-mt2-1.8b"},
	} {
		mem = tc.mem
		if got, got2 := c.Suggested(index).ID, c.Suggested(hy).ID; got != tc.index || got2 != tc.hy {
			t.Errorf("%d GB: %q and %q, want %q and %q", tc.mem>>30, got, got2, tc.index, tc.hy)
		}
	}
}

// TestUse uses only what can be used now: a model downloaded, never one to download, which it
// leaves waiting for nothing; and forgets what the guide chose to use once downloaded.
func TestUse(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	if c.Use(MT, "local-2b") || c.Prefs.MT != "cloud" || c.Chosen(MT) != "cloud" {
		t.Fatalf("not downloaded: in use %q, chosen %q", c.Prefs.MT, c.Chosen(MT))
	}
	m.ready["local-2b"] = true
	c.Changed()
	if c.Prefs.MT != "cloud" {
		t.Fatalf("came in, used unasked: %q", c.Prefs.MT)
	}
	c.Choose(MT, "local-9b") // as the guide does
	if !c.Use(MT, "local-2b") || c.Prefs.MT != "local-2b" {
		t.Fatalf("downloaded: in use %q", c.Prefs.MT)
	}
	m.ready["local-9b"] = true
	c.Changed()
	if c.Prefs.MT != "local-2b" {
		t.Errorf("the guide's choice came in after another was used: %q", c.Prefs.MT)
	}
}

// TestSpokenWith tells the language spoken once a recognizer is used: the one now, if it hears
// it, else English, else Japanese, but the one translated into.
func TestSpokenWith(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	sense, _ := c.Engine(ASR, "sensevoice-small")
	qwen3, _ := c.Engine(ASR, "qwen3-asr-1.7b")
	c.Prefs.Src, c.Prefs.Tgt = "fr", "zh"
	if got := c.SpokenWith(sense); got != "en" {
		t.Errorf("French with SenseVoice: %q", got)
	}
	if got := c.SpokenWith(qwen3); got != "fr" {
		t.Errorf("French with Qwen3-ASR: %q", got)
	}
	c.Prefs.Tgt = "en"
	if got := c.SpokenWith(sense); got != "ja" {
		t.Errorf("French into English with SenseVoice: %q", got)
	}
}

// TestFamilies lists a model's sizes together, in the catalog's order, local before cloud, and
// the cloud's and the user's APIs each on its own.
func TestFamilies(t *testing.T) {
	c, _, _, _, _ := engineCore(t)
	c.Prefs.Endpoints = []prefs.Endpoint{{ID: "a"}, {ID: "b"}}
	var got [][]string
	for _, f := range c.Families(MT) {
		var ids []string
		for _, e := range f {
			ids = append(ids, e.ID)
		}
		got = append(got, ids)
	}
	want := [][]string{{"local-2b", "local-9b"}, {"hy-mt2-1.8b", "hy-mt2-7b"}, {"cloud-2b", "cloud-9b", "cloud"}, {HyMTCloud}, {"custom:a"}, {"custom:b"}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("translation's families %q, want %q", got, want)
	}
}

// TestModelFormats prompts each translation model in a format translate knows, and a local model's
// translator in its model's.
func TestModelFormats(t *testing.T) {
	for _, m := range models.All {
		if _, ok := translate.FormatByID(m.Format); !m.ASR && !ok {
			t.Errorf("%s: no format %q", m.ID, m.Format)
		}
	}
	for id, want := range map[string]*translate.Format{"local-2b": translate.Index, "local-9b": translate.Index, "hy-mt2-1.8b": translate.HyMT, "hy-mt2-7b": translate.HyMT} {
		c, f, _, _, _ := engineCore(t)
		c.Download(id)
		c.SetEngine(id)
		c.Start()
		f.flush(t, 1)
		if ep, ok := f.sessions[0].cfg.Translator.(*translate.Endpoint); !ok || ep.Format != want {
			t.Errorf("%s: translator %+v, want format %s", id, f.sessions[0].cfg.Translator, want.ID)
		}
	}
}

// TestRecognitionHints tells recognition the active scene's names, as the session starts, and
// starts again for another scene's.
func TestRecognitionHints(t *testing.T) {
	c, f, _, _, _ := engineCore(t)
	c.SaveAPI(prefs.Service{ID: QwenCloud}, "sk-1")
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

// TestMoveModels stops translating and the model's server while the models move, starts nothing
// with them meanwhile, and goes on after, the folder kept on success only.
func TestMoveModels(t *testing.T) {
	for _, fails := range []bool{false, true} {
		t.Run(map[bool]string{false: "moved", true: "failed"}[fails], func(t *testing.T) {
			c, f, m, srv, _ := engineCore(t)
			c.deps.PrefsPath = filepath.Join(t.TempDir(), "settings.json")
			m.dir, m.ready["local-2b"] = "/models", true
			c.SetEngine("local-2b")
			c.Start()
			f.flush(t, 1)
			if !c.Running {
				t.Fatal("not running")
			}
			c.MoveModels("/elsewhere")
			srv.mu.Lock()
			stops := srv.stops
			srv.mu.Unlock()
			if c.Running || stops == 0 {
				t.Errorf("while moving: running %v, server stopped %d times", c.Running, stops)
			}
			c.Start()
			if !errors.Is(c.Err, ErrModelsMoving) || c.Running {
				t.Errorf("started while moving: running %v, err %v", c.Running, c.Err)
			}
			var err error
			if fails {
				err = errors.New("no room")
			}
			m.moved(err)
			f.flush(t, 1)
			saved, _ := prefs.Load(c.deps.PrefsPath, "en")
			want := filepath.Join("/elsewhere", models.FolderName)
			if fails {
				want = ""
			}
			if c.Prefs.ModelsDir != want || saved.ModelsDir != want || !c.Running {
				t.Errorf("after: folder %q, saved %q, running %v", c.Prefs.ModelsDir, saved.ModelsDir, c.Running)
			}
		})
	}
}
