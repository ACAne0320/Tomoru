package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// The translation engines: the cloud, the local models, and the endpoints the user adds; and the
// local models of both kinds.

// ErrModelMissing is a local model chosen and not downloaded.
var ErrModelMissing = errors.New("the on-device model is not downloaded")

// ErrNoASR is no recognition chosen, where the system has none.
var ErrNoASR = errors.New("no speech recognition is chosen")

// ErrASRMissing is a recognition model chosen and not downloaded.
var ErrASRMissing = errors.New("the recognition model is not downloaded")

// ErrNoEndpoint is an endpoint chosen that is gone.
var ErrNoEndpoint = errors.New("the translation API is gone")

// LocalError is a local model that could not be run.
type LocalError struct{ Err error }

func (e LocalError) Error() string { return "on-device model: " + e.Err.Error() }
func (e LocalError) Unwrap() error { return e.Err }

// SetEngine translates with engine id from now on, rather than with one chosen to use once
// downloaded.
func (c *Core) SetEngine(id string) {
	c.EngineService.cancelPending(MT)
	if id == c.Prefs.MT {
		return
	}
	c.Prefs.MT = id
	c.RestartIfRunning()
	c.settingsChanged()
}

// CloudRepo is Index-Translate's repository, where the app asks those who use its cloud to star
// it.
const CloudRepo = "https://github.com/bilibili/Index-Translate"

// translator says how to get the translator of engine id ready: ready runs in the background, and
// local tells it loads a model. The checks that need no waiting are made now.
func (c *Core) translator(id string) (ready func(context.Context) (translate.Translator, error), local bool, err error) {
	if tr := c.deps.Translator; tr != nil {
		return func(context.Context) (translate.Translator, error) { return tr, nil }, false, nil
	}
	return c.EngineService.translator(id, c.Prefs.Endpoints)
}

func keyName(id string) string { return "endpoint-key:" + id }

// Successor is the engine of kind k that takes over from a local model in use as it is deleted:
// the cloud's translation, or the system's recognition, "" where there is none.
func Successor(k Kind) string {
	if k == ASR {
		return prefs.DefaultASR()
	}
	return "cloud"
}

// DeleteModel deletes a local model. Its Successor takes over from it if it was in use; with
// none, the user chooses.
func (c *Core) DeleteModel(id string) {
	if c.EngineService.models == nil {
		return
	}
	if c.Prefs.MT == id {
		c.SetEngine(Successor(MT))
	}
	if c.Prefs.ASR == id {
		c.Prefs.ASR = Successor(ASR)
		c.fitLangs()
		c.RestartIfRunning()
	}
	c.EngineService.deleteModel(id)
	c.settingsChanged()
}

// Endpoints.

// SaveEndpoint adds e, or changes the endpoint of its ID, keeping key as its API key unless "".
// A new endpoint gets an ID; SaveEndpoint returns it.
func (c *Core) SaveEndpoint(e prefs.Endpoint, key string) string {
	if e.ID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		e.ID = hex.EncodeToString(b)
	}
	e.URL = strings.TrimRight(strings.TrimSpace(e.URL), "/")
	e.URL = strings.TrimSuffix(e.URL, "/v1")
	if old, ok := c.Prefs.Endpoint(e.ID); ok {
		*old = e
	} else {
		c.Prefs.Endpoints = append(c.Prefs.Endpoints, e)
	}
	if key != "" {
		c.EngineService.saveKey(keyName(e.ID), []byte(key))
	}
	if c.Prefs.MT == prefs.CustomPrefix+e.ID {
		c.RestartIfRunning()
	}
	c.settingsChanged()
	return e.ID
}

// DeleteEndpoint deletes an endpoint and its key. Translating with it goes on with the cloud. undo
// puts them back, and translating with it if it did.
func (c *Core) DeleteEndpoint(id string) (undo func()) {
	i := slices.IndexFunc(c.Prefs.Endpoints, func(e prefs.Endpoint) bool { return e.ID == id })
	if i < 0 {
		return func() {}
	}
	e, inUse := c.Prefs.Endpoints[i], c.Prefs.MT == prefs.CustomPrefix+id
	key := c.EngineService.readKey(keyName(id))
	if inUse {
		c.SetEngine("cloud")
	}
	c.Prefs.Endpoints = slices.Delete(c.Prefs.Endpoints, i, i+1)
	c.EngineService.deleteKey(keyName(id))
	c.EngineService.forgetTest(id)
	c.settingsChanged()
	return func() {
		if _, ok := c.Prefs.Endpoint(id); ok {
			return
		}
		c.Prefs.Endpoints = slices.Insert(c.Prefs.Endpoints, min(i, len(c.Prefs.Endpoints)), e)
		if key != nil {
			c.EngineService.saveKey(keyName(id), key)
		}
		if inUse {
			c.SetEngine(prefs.CustomPrefix + id)
		}
		c.settingsChanged()
	}
}

// TestEndpoint translates the sample with e as it is being edited, with key, or the one kept for it
// when "". Its result goes to Tests under key test.
func (c *Core) TestEndpoint(test string, e prefs.Endpoint, key string) {
	tgt := c.Prefs.Tgt
	if tgt == "en" {
		tgt = "zh"
	}
	c.EngineService.testEndpoint(test, e, key, tgt)
}
