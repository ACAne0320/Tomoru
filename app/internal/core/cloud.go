package core

import (
	"errors"
	"strings"

	"tomoru/internal/asr"
	"tomoru/internal/prefs"
)

// Recognition in the cloud: the services of asr.Specs that take an API key, Qwen's and OpenAI's,
// as the user set them up.

// The cloud recognition services' IDs.
const (
	QwenCloud = asr.QwenCloud
	OpenAI    = asr.OpenAI
)

// ErrASRCloudKey is a cloud recognition service chosen without an API key.
var ErrASRCloudKey = errors.New("cloud recognition has no API key")

// asrKey names a service's API key in the credential store: Qwen's was kept so before there were
// others.
func asrKey(id string) string { return "asr-key:" + id }

// ASRAPI is the cloud recognition service id as the user set it up; not set up, its ID alone.
func (c *Core) ASRAPI(id string) prefs.ASRAPI {
	if a, ok := c.Prefs.ASRAPI(id); ok {
		return *a
	}
	return prefs.ASRAPI{ID: id}
}

// SaveASRAPI sets up the cloud recognition service a.ID, keeping key as its API key unless "".
// The service chosen to use once set up is used from now on.
func (c *Core) SaveASRAPI(a prefs.ASRAPI, key string) {
	a.Key = c.ASRAPI(a.ID).Key
	if key != "" && c.EngineService.saveKey(asrKey(a.ID), []byte(key)) {
		a.Key = true
	}
	a.URL, a.Model = strings.TrimSpace(a.URL), strings.TrimSpace(a.Model)
	if old, ok := c.Prefs.ASRAPI(a.ID); ok {
		*old = a
	} else {
		c.Prefs.ASRAPIs = append(c.Prefs.ASRAPIs, a)
	}
	if c.Prefs.ASR == a.ID {
		c.RestartIfRunning()
	}
	c.usePending()
	c.settingsChanged()
}

// TestASRAPI connects to the service a as it is being edited, with key, or the one kept when "".
// Its result goes to Tests under key test: no text, only whether it worked and how long it took.
func (c *Core) TestASRAPI(test string, a prefs.ASRAPI, key string) {
	c.EngineService.testASRAPI(test, a, key)
}

// hasKey tells whether the recognition id can be used for its key: it takes none, or one is kept.
func hasKey(id string, apis []prefs.ASRAPI) bool {
	s, ok := asr.ByID(id)
	if !ok || !s.Key {
		return true
	}
	for _, a := range apis {
		if a.ID == id {
			return a.Key
		}
	}
	return false
}
