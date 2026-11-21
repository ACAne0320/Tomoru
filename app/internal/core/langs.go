package core

import (
	"slices"
	"strings"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/prefs"
)

func (c *Core) SourceLangs() []string { return c.spoken(c.Prefs.ASR) }

func (c *Core) spoken(id string) []string {
	hears := c.Hears(id)
	if hears == nil {
		return lang.Spoken()
	}
	if s, _ := asr.ByID(id); s.Auto {
		return append([]string{lang.Auto}, hears...)
	}
	return hears
}

func (c *Core) Hears(id string) []string {
	s, ok := asr.ByID(id)
	if !ok {
		return nil
	}
	hears := s.Langs
	if hears == nil {
		hears = c.sysLangs
	}
	if hears == nil {
		return nil
	}
	return slices.DeleteFunc(lang.Spoken(), func(l string) bool { return !slices.Contains(hears, l) })
}

func (c *Core) SpokenWith(e Engine) string { return fitSrc(c.spoken(e.ID), c.Prefs.Src, c.Prefs.Tgt) }

func (c *Core) TargetLangs() []string { return slices.Clone(lang.All) }

func (c *Core) CanSwap() bool {
	return c.Prefs.Src != lang.Auto && slices.Contains(c.SourceLangs(), c.Prefs.Tgt)
}

func (c *Core) fitLangs() bool {
	src := fitSrc(c.SourceLangs(), c.Prefs.Src, c.Prefs.Tgt)
	if src == c.Prefs.Src {
		return false
	}
	c.Prefs.Src = src
	return true
}

func fitSrc(srcs []string, src, tgt string) string {
	if slices.Contains(srcs, src) {
		return src
	}
	for _, l := range append([]string{"en", "ja"}, srcs...) {
		if l != tgt && slices.Contains(srcs, l) {
			return l
		}
	}
	return src
}

func (c *Core) ListLangs() {
	if prefs.SystemASR() == "" || c.sysLangs != nil {
		return
	}
	c.ask(10*time.Second, func(evs []engine.Event) {
		var hears []string
		for _, ev := range evs {
			if ev.Type == engine.EventLocale {
				hears = append(hears, sysLang(ev.ID))
			}
		}
		if len(hears) == 0 {
			return
		}
		c.sysLangs = hears
		if c.fitLangs() {
			c.RestartIfRunning()
			c.settingsChanged()
		}
	}, "locales")
}

func sysLang(locale string) string {
	if locale == "zh-HK" {
		return "yue"
	}
	l, _, _ := strings.Cut(locale, "-")
	return l
}
