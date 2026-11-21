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

func (c *Core) PrepareASR(code string) {
	if c.ASR[code] != ASRUnprepared || prefs.SystemASR() == "" || !slices.Contains(c.SourceLangs(), code) {
		return
	}
	c.ASR[code] = ASRPreparing
	c.ask(prepareWait, func(evs []engine.Event) {
		c.ASR[code] = ASRFailed
		if len(evs) > 0 && evs[len(evs)-1].State == engine.StateReady {
			c.ASR[code] = ASRReady
		}
	}, "prepare", "--locale", lang.Locale(code))
}

func AsksAccess() bool { return runtime.GOOS == "darwin" }

func (c *Core) CheckAccess() {
	if c.Checking || c.Asking {
		return
	}
	c.Checking = true
	c.ask(10*time.Second, func(evs []engine.Event) { c.Checking = false; c.setAccess(evs) }, "access")
}

func (c *Core) RequestAccess() {
	if c.Asking {
		return
	}
	c.Asking = true
	c.changed()
	c.ask(accessDialogWait, func(evs []engine.Event) { c.Asking = false; c.setAccess(evs) }, "request-access")
}

func (c *Core) setAccess(evs []engine.Event) {
	c.Access, c.CheckedAt = PermissionDenied, time.Now()
	if len(evs) > 0 {
		c.Access = Permission(evs[len(evs)-1].State)
	}
}

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
			if ev.Type == engine.EventApp {
				apps = append(apps, App{Name: ev.Name, ID: ev.ID, Playing: ev.Playing})
			}
		}
		c.Apps = apps
	}, "apps")
}

const ListEvery = 3 * time.Second

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

func (c *Core) HasSample() bool { return !c.deps.NoVoice && samples.Has(c.Prefs.Src) }

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

func (c *Core) StopSpeaking() {
	if c.speak == nil {
		return
	}
	c.speak()
	c.speakRun++
	c.Speaking, c.speak = false, nil
	c.changed()
}
