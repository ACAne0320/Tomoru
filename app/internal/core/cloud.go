package core

import (
	"errors"
	"slices"
	"strings"

	"tomoru/internal/asr"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

const (
	QwenCloud = asr.QwenCloud
	OpenAI    = asr.OpenAI
)

type CloudSize struct{ ID, Label, Model string }

var CloudSizes = []CloudSize{
	{ID: "cloud-2b", Label: "2B", Model: "Index-Translate-2B"},
	{ID: "cloud-9b", Label: "9B", Model: "Index-Translate-9B"},
	{ID: "cloud", Label: "35B", Model: translate.CloudModel},
}

func CloudSizeByID(id string) (CloudSize, bool) {
	i := slices.IndexFunc(CloudSizes, func(s CloudSize) bool { return s.ID == id })
	if i < 0 {
		return CloudSize{}, false
	}
	return CloudSizes[i], true
}

const HyMTCloud = "hy-mt-cloud"

type MTService struct {
	ID, URL string
	Models  []string
	Format  *translate.Format
}

var MTServices = []MTService{
	{ID: HyMTCloud, URL: "https://tokenhub.tencentmaas.com", Models: []string{"hy-mt2-plus", "hy-mt2-pro", "hy-mt2-lite"}, Format: translate.HyMT},
}

func MTServiceByID(id string) (MTService, bool) {
	i := slices.IndexFunc(MTServices, func(s MTService) bool { return s.ID == id })
	if i < 0 {
		return MTService{}, false
	}
	return MTServices[i], true
}

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

var ErrASRCloudKey = errors.New("cloud recognition has no API key")

var ErrMTCloudKey = errors.New("cloud translation has no API key")

func apiKey(id string) string {
	if _, ok := MTServiceByID(id); ok {
		return "mt-key:" + id
	}
	return "asr-key:" + id
}

type ServiceInfo struct {
	URL    string
	Models []ServiceModel
}

type ServiceModel struct{ ID, Label string }

func (s ServiceInfo) Default() string { return s.Models[0].ID }

func (s ServiceInfo) Labeled() bool { return s.Models[0].Label != "" }

var modelLabels = map[string][]string{
	QwenCloud: {"3.1", "3.0"},
	HyMTCloud: {"Plus", "Pro", "Lite"},
}

func ServiceOf(id string) ServiceInfo {
	url, ids := "", []string(nil)
	if s, ok := MTServiceByID(id); ok {
		url, ids = s.URL, s.Models
	} else {
		s, _ := asr.ByID(id)
		url, ids = s.URL, s.Models
	}
	info := ServiceInfo{URL: url}
	labels := modelLabels[id]
	for i, m := range ids {
		sm := ServiceModel{ID: m}
		if i < len(labels) {
			sm.Label = labels[i]
		}
		info.Models = append(info.Models, sm)
	}
	return info
}

func (c *Core) API(id string) prefs.Service {
	if a, ok := c.Prefs.API(id); ok {
		return *a
	}
	return prefs.Service{ID: id}
}

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

func (c *Core) TestAPI(test string, a prefs.Service, key string) {
	if s, ok := MTServiceByID(a.ID); ok {
		c.EngineService.testEndpoint(test, s.endpoint(a), key, c.testTgt(), func(prefs.Vision) {})
		return
	}
	c.EngineService.testAPI(test, a, key)
}

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
