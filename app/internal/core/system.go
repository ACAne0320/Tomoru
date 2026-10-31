package core

import (
	"context"
	"log"
	"runtime"
	"slices"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/prefs"
	"tomoru/internal/samples"
)

// ask runs an engine command in the background, and hands its events, or nil, to done on the
// main thread.
func (c *Core) ask(timeout time.Duration, done func([]engine.Event), args ...string) {
	c.work.Go(func() {
		var evs []engine.Event
		bin, err := c.deps.EnginePath()
		if err == nil {
			ctx, cancel := context.WithTimeout(c.work.ctx, timeout)
			evs, err = c.deps.RunEngine(ctx, bin, args...)
			cancel()
		}
		if err != nil {
			log.Println("engine", args, err)
			evs = nil
		}
		c.work.Post(func() {
			done(evs)
			c.changed()
		})
	})
}

// PrepareASR has recognition for a language got ready, downloading it if need be (13–21 s for a
// new one), so that the first start does not wait on it. It is done once: starting translating
// tries again after an error. Only the system's recognition gets ready so: a model's is downloaded.
func (c *Core) PrepareASR(code string) {
	if c.ASR[code] != "" || prefs.SystemASR() == "" || !slices.Contains(c.SourceLangs(), code) {
		return
	}
	c.ASR[code] = "preparing"
	c.ask(2*time.Minute, func(evs []engine.Event) {
		c.ASR[code] = "error"
		if len(evs) > 0 && evs[len(evs)-1].State == "ready" {
			c.ASR[code] = "ready"
		}
	}, "prepare", "--locale", lang.Locale(code))
}

// AsksAccess tells whether the system asks the user before the app may hear what it plays:
// macOS does, Windows does not.
func AsksAccess() bool { return runtime.GOOS == "darwin" }

// CheckAccess asks whether the engine may hear the system. Run from a terminal, the engine has the
// terminal's permissions; the packaged app has its own.
func (c *Core) CheckAccess() {
	if c.Checking || c.Asking {
		return
	}
	c.Checking = true
	c.ask(10*time.Second, func(evs []engine.Event) { c.Checking = false; c.setAccess(evs) }, "access")
}

// RequestAccess has the system ask the user, which it does the first time the engine records.
func (c *Core) RequestAccess() {
	if c.Asking {
		return
	}
	c.Asking = true
	c.changed()
	c.ask(2*time.Minute, func(evs []engine.Event) { c.Asking = false; c.setAccess(evs) }, "request-access")
}

func (c *Core) setAccess(evs []engine.Event) {
	c.Access, c.CheckedAt = "denied", time.Now()
	if len(evs) > 0 {
		c.Access = evs[len(evs)-1].State
	}
}

// ListApps lists the apps to hear anew, unless it did in the last few seconds: views that show
// them ask while they show, as apps start and stop playing.
func (c *Core) ListApps() {
	if time.Since(c.listedAt) < ListEvery {
		return
	}
	c.listedAt = time.Now()
	c.ask(10*time.Second, func(evs []engine.Event) {
		if evs == nil {
			return
		}
		var apps []App
		for _, ev := range evs {
			if ev.Type == "app" {
				apps = append(apps, App{Name: ev.Name, ID: ev.ID, Playing: ev.Playing})
			}
		}
		c.Apps = apps
	}, "apps")
}

// ListEvery is how often the apps are listed while a view shows them.
const ListEvery = 3 * time.Second

// SetOpenAtLogin opens the app at login, or not. It needs the packaged app.
func (c *Core) SetOpenAtLogin(on bool) {
	if c.deps.Login == nil {
		return
	}
	c.LoginErr = ""
	if err := c.deps.Login.SetOpenAtLogin(on); err != nil {
		c.LoginErr = err.Error()
	}
	c.AtLogin = c.deps.Login.OpenAtLogin()
	c.changed()
}

// HasSample tells whether there are sample sentences to say in the spoken language.
func (c *Core) HasSample() bool { return samples.Has(c.Prefs.Src) }

// Speak says the sample sentences in the spoken language aloud, with the system's voice for it,
// for translating to hear as it hears a video: the first-run guide tries translating with them.
// Speaking is true until they are said or StopSpeaking stops them; SpeakErr says why they could
// not be. It says nothing in a language with no samples (HasSample).
func (c *Core) Speak() {
	c.StopSpeaking()
	if !c.HasSample() {
		return
	}
	c.speakRun++
	run := c.speakRun
	ctx, cancel := context.WithTimeout(c.work.ctx, time.Minute)
	c.Speaking, c.SpeakErr, c.speak = true, nil, cancel
	args := append([]string{"speak", "--locale", lang.Locale(c.Prefs.Src), "--"}, samples.Spoken(c.Prefs.Src)...)
	c.work.Go(func() {
		bin, err := c.deps.EnginePath()
		if err == nil {
			_, err = c.deps.RunEngine(ctx, bin, args...)
		}
		stopped := ctx.Err() != nil
		cancel()
		c.work.Post(func() {
			if c.speakRun != run {
				return
			}
			c.Speaking, c.speak = false, nil
			if err != nil && !stopped {
				log.Println("speak:", err)
				c.SpeakErr = err
			}
			c.changed()
		})
	})
	c.changed()
}

// StopSpeaking stops saying the samples.
func (c *Core) StopSpeaking() {
	if c.speak == nil {
		return
	}
	c.speak()
	c.speakRun++
	c.Speaking, c.speak = false, nil
	c.changed()
}
