package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"slices"

	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

var ErrModelMissing = errors.New("the on-device model is not downloaded")

var ErrModelsMoving = errors.New("the on-device models are being moved")

var ErrNoASR = errors.New("no speech recognition is chosen")

var ErrASRMissing = errors.New("the recognition model is not downloaded")

var ErrNoEndpoint = errors.New("the translation API is gone")

var ErrNoVision = errors.New("the API chosen for pictures reads none")

var ErrModelBusy = errors.New("another on-device model is translating the subtitles")

type LocalError struct{ Err error }

func (e LocalError) Error() string { return "on-device model: " + e.Err.Error() }
func (e LocalError) Unwrap() error { return e.Err }

func (c *Core) SetEngine(id string) {
	c.EngineService.cancelPending(MT)
	if id == c.Prefs.MT {
		return
	}
	c.Prefs.MT = id
	if !c.Prefs.Onboarded {
		c.Prefs.ImageMT = imageEngineFrom(id, c.Prefs.Endpoints)
	}
	c.RestartIfRunning()
	c.settingsChanged()
}

const CloudRepo = "https://github.com/bilibili/Index-Translate"

func (c *Core) translator(id string) (ready func(context.Context) (translate.Translator, error), local bool, err error) {
	if tr := c.deps.Translator; tr != nil {
		return func(context.Context) (translate.Translator, error) { return tr, nil }, false, nil
	}
	return c.EngineService.translator(id, c.Prefs.Endpoints, c.Prefs.MTAPIs, c.ImageService.usesServer())
}

func keyName(id string) string { return "endpoint-key:" + id }

func (c *Core) testTgt() string {
	if c.Prefs.Tgt == "en" {
		return "zh"
	}
	return c.Prefs.Tgt
}

func Successor(k Kind) string {
	if k == ASR {
		return prefs.SystemASR()
	}
	return "cloud"
}

func (c *Core) retireModels() {
	m, retired := models.RetiredByID(c.Prefs.MT)
	if retired {
		c.Prefs.MT = m.Successor
	}
	if _, read := c.savedPrefs(); !read {
		c.saved.MT = c.Prefs.MT
		return
	}
	if retired && c.SavePrefs() != nil {
		return
	}
	saved, read := c.savedPrefs()
	if _, still := models.RetiredByID(saved.MT); !read || still {
		return
	}
	if d := c.deps.Models; d != nil && !d.Moving().Active {
		if err := models.RemoveRetired(d.Dir()); err != nil {
			log.Println("models:", err)
		}
	}
}

func (c *Core) savedPrefs() (prefs.Prefs, bool) {
	if _, err := os.Stat(c.deps.PrefsPath); err != nil {
		return prefs.Prefs{}, false
	}
	p, err := prefs.Load(c.deps.PrefsPath, "")
	return p, err == nil
}

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
	if c.Prefs.MT == prefs.EndpointEngine(e.ID) {
		c.RestartIfRunning()
	}
	c.settingsChanged()
	return e.ID
}

func (c *Core) DeleteEndpoint(id string) (undo func()) {
	i := slices.IndexFunc(c.Prefs.Endpoints, func(e prefs.Endpoint) bool { return e.ID == id })
	if i < 0 {
		return func() {}
	}
	e, mt := c.Prefs.Endpoints[i], prefs.EndpointEngine(id)
	key := c.EngineService.readKey(keyName(id))
	c.Prefs.Endpoints = slices.Delete(c.Prefs.Endpoints, i, i+1)
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
		if c.Prefs.MT == mt && !c.Running && errors.Is(c.Err, ErrNoEndpoint) {
			c.Start()
		}
		c.settingsChanged()
	}
}

func (c *Core) TestEndpoint(test string, e prefs.Endpoint, key string) {
	c.EngineService.testEndpoint(test, e, key, c.testTgt(), func(v prefs.Vision) {
		saved, ok := c.Prefs.Endpoint(e.ID)
		if !ok || apiOf(*saved) != apiOf(e) || saved.Vision == v {
			return
		}
		saved.Vision = v
		c.settingsChanged()
	})
}

func (c *Core) CancelChoice(k Kind, id string) {
	c.CancelDownload(id)
	c.Choose(k, c.InUse(k))
}
