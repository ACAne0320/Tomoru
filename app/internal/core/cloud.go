package core

import (
	"errors"
	"slices"
	"strings"

	"tomoru/internal/asr"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// The cloud services that take the user's API key, as the user set them up: of recognition, those
// of asr.Specs, Qwen's and OpenAI's; of translation, those of MTServices.

// The cloud recognition services' IDs.
const (
	QwenCloud = asr.QwenCloud
	OpenAI    = asr.OpenAI
)

// CloudSize is a size of Index-Translate that the bilibili team serves free: its engine's ID, its
// size's name, and the model it asks for.
type CloudSize struct{ ID, Label, Model string }

// CloudSizes are Index-Translate's cloud, the smallest first, as a model's sizes are: 35B-A3B,
// "cloud", the default; and 9B and 2B, which, measured, translated closer to the human subtitles
// and sooner (bench/README.md). Each reads pictures too, though the service's model list says
// only 35B does (2026-11-10).
var CloudSizes = []CloudSize{
	{ID: "cloud-2b", Label: "2B", Model: "Index-Translate-2B"},
	{ID: "cloud-9b", Label: "9B", Model: "Index-Translate-9B"},
	{ID: "cloud", Label: "35B", Model: translate.CloudModel},
}

// CloudSizeByID finds a size of Index-Translate's cloud.
func CloudSizeByID(id string) (CloudSize, bool) {
	i := slices.IndexFunc(CloudSizes, func(s CloudSize) bool { return s.ID == id })
	if i < 0 {
		return CloudSize{}, false
	}
	return CloudSizes[i], true
}

// HyMTCloud is Hy-MT2 on Tencent Cloud's TokenHub.
const HyMTCloud = "hy-mt-cloud"

// MTService is a translation service in the cloud: its address, as an Endpoint's, its models, the
// first its default, and how they are prompted.
type MTService struct {
	ID, URL string
	Models  []string
	Format  *translate.Format
}

// MTServices are the cloud translation services to set up with an API key. TokenHub serves Hy-MT2
// in three sizes: Plus 7B, the default, as good as Pro 30B-A3B, quicker and seldom refused; Pro,
// which takes 60 requests a minute; and Lite 1.8B (bench/README.md).
var MTServices = []MTService{
	{ID: HyMTCloud, URL: "https://tokenhub.tencentmaas.com", Models: []string{"hy-mt2-plus", "hy-mt2-pro", "hy-mt2-lite"}, Format: translate.HyMT},
}

// MTServiceByID finds a cloud translation service.
func MTServiceByID(id string) (MTService, bool) {
	i := slices.IndexFunc(MTServices, func(s MTService) bool { return s.ID == id })
	if i < 0 {
		return MTService{}, false
	}
	return MTServices[i], true
}

// endpoint is the service as a set up, as an API to translate with.
func (s MTService) endpoint(a prefs.Service) prefs.Endpoint {
	e := prefs.Endpoint{ID: s.ID, URL: a.URL, Model: a.Model, Format: s.Format.ID}
	if e.URL == "" {
		e.URL = s.URL
	}
	if e.Model == "" {
		e.Model = s.Models[0]
	}
	return e
}

// ErrASRCloudKey is a cloud recognition service chosen without an API key.
var ErrASRCloudKey = errors.New("cloud recognition has no API key")

// ErrMTCloudKey is a cloud translation service chosen without an API key.
var ErrMTCloudKey = errors.New("cloud translation has no API key")

// apiKey names a service's API key in the credential store: a recognition service's as Qwen's was
// kept before there were others.
func apiKey(id string) string {
	if _, ok := MTServiceByID(id); ok {
		return "mt-key:" + id
	}
	return "asr-key:" + id
}

// ServiceOf is the cloud service id's own address and models, the first its default.
func ServiceOf(id string) (url string, models []string) {
	if s, ok := MTServiceByID(id); ok {
		return s.URL, s.Models
	}
	s, _ := asr.ByID(id)
	return s.URL, s.Models
}

// API is the cloud service id as the user set it up; not set up, its ID alone.
func (c *Core) API(id string) prefs.Service {
	if a, ok := c.Prefs.API(id); ok {
		return *a
	}
	return prefs.Service{ID: id}
}

// SaveAPI sets up the cloud service a.ID, keeping key as its API key unless "". Saving uses
// nothing: Use does.
func (c *Core) SaveAPI(a prefs.Service, key string) {
	a.Key = c.API(a.ID).Key
	if key != "" && c.EngineService.saveKey(apiKey(a.ID), []byte(key)) {
		a.Key = true
	}
	a.URL, a.Model = strings.TrimSpace(a.URL), strings.TrimSpace(a.Model)
	_, mt := MTServiceByID(a.ID)
	switch old, ok := c.Prefs.API(a.ID); {
	case ok:
		*old = a
	case mt:
		c.Prefs.MTAPIs = append(c.Prefs.MTAPIs, a)
	default:
		c.Prefs.ASRAPIs = append(c.Prefs.ASRAPIs, a)
	}
	if c.Prefs.ASR == a.ID || c.Prefs.MT == a.ID {
		c.RestartIfRunning()
	}
	c.settingsChanged()
}

// TestAPI connects to the service a as it is being edited, with key, or the one kept when "".
// Its result goes to Tests under key test: whether it worked and how long it took, and a
// translation service's translation of TestSample.
func (c *Core) TestAPI(test string, a prefs.Service, key string) {
	if s, ok := MTServiceByID(a.ID); ok {
		c.EngineService.testEndpoint(test, s.endpoint(a), key, c.testTgt(), func(prefs.Vision) bool { return false })
		return
	}
	c.EngineService.testAPI(test, a, key)
}

// hasKey tells whether engine id can be used for its key: it takes none, or one is kept, in apis,
// the services the user set up of its kind.
func hasKey(id string, apis []prefs.Service) bool {
	s, ok := asr.ByID(id)
	if _, mt := MTServiceByID(id); !mt && (!ok || !s.Key) {
		return true
	}
	for _, a := range apis {
		if a.ID == id {
			return a.Key
		}
	}
	return false
}
