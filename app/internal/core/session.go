package core

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/live"
	"tomoru/internal/models"
)

// Toggle starts translating, or stops it.
func (c *Core) Toggle() {
	if c.Running {
		c.Stop()
	} else {
		c.Start()
	}
}

// Start starts translating with the settings as they are. The translator gets ready first: a
// local model loads in a few seconds, in the background. With the original alone there is none to
// get ready, and no translation engine is asked for, nor needs setting up.
func (c *Core) Start() {
	if c.closed || c.work.ctx.Err() != nil {
		return
	}
	c.stop()
	c.runSince, c.runEngines = c.now(), engines(c.Prefs)
	bin, err := c.deps.EnginePath()
	if err != nil {
		c.SessionController.start(sessionRequest{Err: err})
		return
	}
	p := c.Preferences()
	recognize, err := c.localRecognizer(p.ASR)
	switch {
	case p.ASR == "":
		err = ErrNoASR
	case !hasKey(p.ASR, p.ASRAPIs):
		err = ErrASRCloudKey
	}
	if err != nil {
		c.SessionController.start(sessionRequest{Err: err})
		return
	}
	ready, local := c.EngineService.noTranslator(c.ImageService.usesServer()), false
	if p.Translates() {
		ready, local, err = c.translator(c.Prefs.MT)
	}
	c.syncHistoryPolicy()
	record := c.HistoryService.begin()
	cfg := live.Config{Now: c.now, Engine: bin, Recognize: recognize, Src: p.Src, Tgt: p.Tgt, App: p.Source, Draft: p.Draft && p.Translates(), Terms: c.applyTerms}
	if spec, ok := asr.ByID(p.ASR); ok {
		cfg.VolatileSentences, cfg.FinalsEnd = spec.VolatileSentences, spec.FinalsEnd
		if spec.Hint != nil {
			cfg.Hint = spec.Hint(p.Src)
		}
	}
	if m, local := models.ByID(c.Prefs.MT); !local || !m.NoAbout {
		cfg.About = c.sceneAbout
	}
	cfg.LineMetadata = func() any { return c.HistoryService.policy.Load() }
	cfg.OnLine = func(ev live.LineEvent) {
		policy, ok := ev.Metadata.(*historyPolicy)
		if !ok {
			policy = c.HistoryService.policy.Load()
		} // adapters without metadata
		record.accept(ev, policy)
		c.work.Post(func() {
			if c.HistoryService.recording == record {
				c.HistoryService.drain()
			}
		})
	}
	request := sessionRequest{Config: cfg, Ready: ready, Local: local, Err: err}
	spec, _ := asr.ByID(p.ASR)
	served := spec.Model != nil && spec.Model.Served()
	if !served && c.deps.ASRServer != nil {
		c.deps.ASRServer.Stop() // its memory back, the model not in use
	}
	switch {
	case spec.Cloud:
		providers, api, hints := c.EngineService, c.API(p.ASR), c.hints()
		request.Prepare = func(ctx context.Context, cfg *live.Config) error {
			// Credential access may block; resolve it with the translator's background
			// preparation, before the application thread starts the runtime.
			cloud := providers.cloudConfig(api, "")
			if err := ctx.Err(); err != nil {
				return err
			}
			var err error
			cfg.Recognize, err = spec.New(asr.Env{Service: cloud, Scene: hints})
			return err
		}
	case served:
		env := asr.Env{Server: c.deps.ASRServer, ModelPath: c.deps.Models.Path(p.ASR), Scene: c.hints()}
		request.Prepare = func(ctx context.Context, cfg *live.Config) error {
			// Loading the model takes seconds, warming it on a GPU the first time up to a minute.
			env.Context = ctx
			var err error
			if cfg.Recognize, err = spec.New(env); err != nil {
				return LocalError{err}
			}
			return nil
		}
	}
	c.SessionController.start(request)
}

// localRecognizer resolves a recognizer on this computer: a model tomoru-asr runs, downloaded, or
// the system's, whose nil factory leaves recognition to the capture engine. A model llama-server
// runs, downloaded, is made in the worker, as the cloud's with its credentials: nil here.
func (c *Core) localRecognizer(id string) (asr.Factory, error) {
	s, ok := asr.ByID(id)
	if !ok || s.Cloud {
		return nil, nil
	}
	var env asr.Env
	if s.Model != nil {
		if c.deps.Models != nil && c.deps.Models.Moving().Active {
			return nil, ErrModelsMoving
		}
		if c.deps.Models == nil || c.deps.Models.Status(id).State != models.Ready {
			return nil, ErrASRMissing
		}
		if s.Model.Served() {
			if c.deps.ASRServer == nil {
				return nil, ErrASRMissing
			}
			return nil, nil
		}
		bin, err := c.deps.ASRPath()
		if err != nil {
			return nil, err
		}
		env.ASR, env.ModelDir = bin, filepath.Dir(c.deps.Models.Path(id))
	}
	return s.New(env)
}

// MoveModels moves the local models into a folder of their own in dir (models.Destination), and
// keeps them there. Translating stops while they
// move, and the models' servers, which hold their files open, then goes on if it was on.
func (c *Core) MoveModels(dir string) {
	if c.closed || c.deps.Models == nil || c.deps.Models.Moving().Active {
		return
	}
	dir = models.Destination(dir, c.deps.Models.Dir())
	running := c.Running
	c.stop()
	for _, s := range []Server{c.deps.Server, c.deps.ASRServer} {
		if s != nil {
			s.Stop()
		}
	}
	c.deps.Models.Move(dir, func(err error) {
		c.work.Post(func() {
			if err == nil {
				c.Prefs.ModelsDir = c.deps.Models.Dir()
				c.settingsChanged()
			} else {
				log.Println("models: moving:", err)
				c.changed()
			}
			if running {
				c.Start()
			}
		})
	})
	c.changed()
}

// Shutdown cancels every feature before waiting for application workers.
func (c *Core) Shutdown() {
	if c.closed {
		return
	}
	c.closed = true
	c.work.cancel() // invalidate queued callbacks before any feature begins shutting down
	c.stop()
	// A stopped session may still have retryable output after a storage failure.
	c.HistoryService.drain()
	c.closeHistory()
	c.StopSpeaking()
	c.CloseImage()
	c.CancelLearning()
	c.CancelRetranslating()
	c.EngineService.stopTests()
	if m, ok := c.deps.Models.(interface{ Close() }); ok {
		m.Close()
	}
	for _, s := range []Server{c.deps.Server, c.deps.ASRServer} {
		if s != nil {
			s.Stop()
		}
	}
	if !c.work.close(3 * time.Second) {
		log.Println("shutdown: some workers did not stop within 3 seconds")
	}
	c.SavePrefs()
}

// Stop stops translating.
func (c *Core) Stop() {
	if c.stop() {
		c.changed()
	}
}

// Pause stops translating from the subtitles, which then offer to go on.
func (c *Core) Pause() {
	c.stop()
	c.Paused = true
	c.changed()
}

func (c *Core) stop() bool { return c.SessionController.stop() }

// RestartIfRunning restarts translating, which takes its languages, source, drafting and whether
// it translates as it starts.
func (c *Core) RestartIfRunning() {
	if c.Running {
		c.Start()
	}
}

// SetSrc makes l the language spoken; choosing the target swaps them.
func (c *Core) SetSrc(l string) { c.setLang(&c.Prefs.Src, &c.Prefs.Tgt, l) }

// SetTgt makes l the language to translate into; choosing the source swaps them.
func (c *Core) SetTgt(l string) { c.setLang(&c.Prefs.Tgt, &c.Prefs.Src, l) }

func (c *Core) setLang(side, other *string, l string) {
	if l == *side {
		return
	}
	if l == *other {
		*other = *side
	}
	*side = l
	c.fitLangs()
	c.keepLangs()
	c.RestartIfRunning()
	c.settingsChanged()
}

// Swap swaps the languages, if the recognition hears the target (CanSwap).
func (c *Core) Swap() {
	if !c.CanSwap() {
		return
	}
	c.Prefs.Src, c.Prefs.Tgt = c.Prefs.Tgt, c.Prefs.Src
	c.keepLangs()
	c.RestartIfRunning()
	c.settingsChanged()
}

// SetSource hears the app id, named name, or with "" all the system's sound.
func (c *Core) SetSource(id, name string) {
	if id == c.Prefs.Source {
		return
	}
	c.Prefs.Source, c.Prefs.SourceName = id, name
	c.RestartIfRunning()
	c.settingsChanged()
}
