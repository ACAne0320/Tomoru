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

// The languages to choose: to speak, those the recognition in use hears; to translate into, all.

// SourceLangs are the languages the recognition in use hears, in the order of lang.All: the
// system's as it says, a model's, the cloud's. Any may be spoken while the system has not said,
// and with no recognition chosen.
func (c *Core) SourceLangs() []string { return c.spoken(c.Prefs.ASR) }

func (c *Core) spoken(id string) []string {
	if hears := c.Hears(id); hears != nil {
		return hears
	}
	return lang.Spoken()
}

// Hears are the languages recognition id hears that can be spoken, in the order of lang.All; nil
// where that is not known: the system's before it says, or no recognition.
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

// SpokenWith is the language spoken once recognition e is used: the one spoken now, if e hears it,
// else the one fitLangs makes it.
func (c *Core) SpokenWith(e Engine) string { return fitSrc(c.spoken(e.ID), c.Prefs.Src, c.Prefs.Tgt) }

// TargetLangs are the languages to translate into: any of lang.All, as every translation engine
// writes them.
func (c *Core) TargetLangs() []string { return slices.Clone(lang.All) }

// CanSwap tells whether the languages can be swapped: the recognition hears the target.
func (c *Core) CanSwap() bool { return slices.Contains(c.SourceLangs(), c.Prefs.Tgt) }

// fitLangs makes the spoken language one the recognition hears, if it is not, and tells whether it
// changed it.
func (c *Core) fitLangs() bool {
	src := fitSrc(c.SourceLangs(), c.Prefs.Src, c.Prefs.Tgt)
	if src == c.Prefs.Src {
		return false
	}
	c.Prefs.Src = src
	return true
}

// fitSrc is src if srcs has it, else English, else Japanese, else the first of srcs, but tgt; src
// where none is left.
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

// ListLangs asks the system which languages its recognition hears, once; the spoken language
// becomes one of them if it is not.
func (c *Core) ListLangs() {
	if prefs.SystemASR() == "" || c.sysLangs != nil {
		return
	}
	c.ask(10*time.Second, func(evs []engine.Event) {
		var hears []string
		for _, ev := range evs {
			if ev.Type == "locale" {
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

// sysLang is the language of a locale the system recognizes: its own, but Cantonese for zh-HK,
// as Apple names it.
func sysLang(locale string) string {
	if locale == "zh-HK" {
		return "yue"
	}
	l, _, _ := strings.Cut(locale, "-")
	return l
}
