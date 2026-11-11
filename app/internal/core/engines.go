package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"

	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// The translation engines: the cloud, the local models, and the endpoints the user adds; and the
// local models of both kinds.

// ErrModelMissing is a local model chosen and not downloaded.
var ErrModelMissing = errors.New("the on-device model is not downloaded")

// ErrModelsMoving is a local model chosen while the models move to another folder.
var ErrModelsMoving = errors.New("the on-device models are being moved")

// ErrNoASR is no recognition chosen, where the system has none.
var ErrNoASR = errors.New("no speech recognition is chosen")

// ErrASRMissing is a recognition model chosen and not downloaded.
var ErrASRMissing = errors.New("the recognition model is not downloaded")

// ErrNoEndpoint is an endpoint chosen that is gone.
var ErrNoEndpoint = errors.New("the translation API is gone")

// ErrModelBusy is a local model chosen for pictures while another translates the subtitles: one
// runs at a time.
var ErrModelBusy = errors.New("another on-device model is translating the subtitles")

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
	if !c.Prefs.Onboarded { // the guide's one choice of a translation engine, for pictures too
		c.Prefs.ImageMT = imageEngineFrom(id, c.Prefs.Endpoints)
	}
	c.RestartIfRunning()
	c.settingsChanged()
}

// CloudRepo is Index-Translate's repository, which the app links to where it names the model.
const CloudRepo = "https://github.com/bilibili/Index-Translate"

// translator says how to get the translator of engine id ready: ready runs in the background, and
// local tells it loads a model. The checks that need no waiting are made now.
func (c *Core) translator(id string) (ready func(context.Context) (translate.Translator, error), local bool, err error) {
	if tr := c.deps.Translator; tr != nil {
		return func(context.Context) (translate.Translator, error) { return tr, nil }, false, nil
	}
	return c.EngineService.translator(id, c.Prefs.Endpoints, c.Prefs.MTAPIs, c.ImageService.usesServer())
}

func keyName(id string) string { return "endpoint-key:" + id }

// testTgt is the language a test translates TestSample, English, into: the user's, else Chinese.
func (c *Core) testTgt() string {
	if c.Prefs.Tgt == "en" {
		return "zh"
	}
	return c.Prefs.Tgt
}

// Successor is the engine of kind k that takes over from a local model in use as it is deleted:
// the cloud's translation, or the system's recognition, "" where there is none.
func Successor(k Kind) string {
	if k == ASR {
		return prefs.DefaultASR()
	}
	return "cloud"
}

// DeleteModel deletes a local model. Its Successor takes over from it if it was in use, and the
// cloud for pictures; with none, the user chooses.
func (c *Core) DeleteModel(id string) {
	if c.EngineService.models == nil {
		return
	}
	if c.Prefs.MT == id {
		c.SetEngine(Successor(MT))
	}
	if c.Prefs.ImageMT == id {
		c.Prefs.ImageMT = "cloud"
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
// A new endpoint gets an ID; SaveEndpoint returns it. Whether it reads pictures is what a test of
// its API found last, not e.Vision: kept while the API stays the same, else not known.
func (c *Core) SaveEndpoint(e prefs.Endpoint, key string) string {
	if e.ID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		e.ID = hex.EncodeToString(b)
	}
	e.URL, e.Format = apiOf(e).url, apiOf(e).format
	e.Vision = prefs.VisionUnknown
	old, ok := c.Prefs.Endpoint(e.ID)
	if ok && apiOf(*old) == apiOf(e) {
		e.Vision = old.Vision
	}
	if v, found := c.EngineService.visions[apiOf(e)]; found {
		e.Vision = v
	}
	if ok {
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
	c.fitImageEngine()
	c.settingsChanged()
	return e.ID
}

// DeleteEndpoint deletes an endpoint and its key. Translating with it stops with ErrNoEndpoint
// rather than going on with another engine: the subtitles go only where the user chose. Pictures
// go to the cloud, chosen for them at first. undo puts all back, and translating with it where
// deleting stopped it.
func (c *Core) DeleteEndpoint(id string) (undo func()) {
	i := slices.IndexFunc(c.Prefs.Endpoints, func(e prefs.Endpoint) bool { return e.ID == id })
	if i < 0 {
		return func() {}
	}
	e, mt := c.Prefs.Endpoints[i], prefs.CustomPrefix+id
	key := c.EngineService.readKey(keyName(id))
	c.Prefs.Endpoints = slices.Delete(c.Prefs.Endpoints, i, i+1)
	pictures := c.Prefs.ImageMT == mt
	c.fitImageEngine()
	c.EngineService.deleteKey(keyName(id))
	c.EngineService.forgetTest(id)
	if c.Prefs.MT == mt {
		c.RestartIfRunning()
	}
	c.settingsChanged()
	return func() {
		if _, ok := c.Prefs.Endpoint(id); ok {
			return
		}
		c.Prefs.Endpoints = slices.Insert(c.Prefs.Endpoints, min(i, len(c.Prefs.Endpoints)), e)
		if key != nil {
			c.EngineService.saveKey(keyName(id), key)
		}
		if pictures && c.Prefs.ImageMT == "cloud" {
			c.Prefs.ImageMT = mt
		}
		if c.Prefs.MT == mt && !c.Running && errors.Is(c.Err, ErrNoEndpoint) {
			c.Start()
		}
		c.settingsChanged()
	}
}

// TestEndpoint translates the sample with e as it is being edited, with key, or the one kept for it
// when "". Its result goes to Tests under key test.
// It finds too whether e reads pictures, which e as saved keeps if it is the same API.
func (c *Core) TestEndpoint(test string, e prefs.Endpoint, key string) {
	c.EngineService.testEndpoint(test, e, key, c.testTgt(), func(v prefs.Vision) bool {
		saved, ok := c.Prefs.Endpoint(e.ID)
		if !ok || apiOf(*saved) != apiOf(e) || saved.Vision == v {
			return false
		}
		saved.Vision = v
		was := c.Prefs.ImageMT
		c.fitImageEngine()
		c.settingsChanged()
		return was != c.Prefs.ImageMT
	})
}
