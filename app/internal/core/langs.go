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
func (c *Core) SourceLangs() []string {
	var hears []string
	if s, ok := asr.ByID(c.Prefs.ASR); ok {
		if hears = s.Langs; hears == nil {
			hears = c.sysLangs
		}
	}
	spoken := lang.Spoken()
	if hears == nil {
		return spoken
	}
	return slices.DeleteFunc(spoken, func(l string) bool { return !slices.Contains(hears, l) })
}

// TargetLangs are the languages to translate into: any of lang.All, as every translation engine
// writes them.
func (c *Core) TargetLangs() []string { return slices.Clone(lang.All) }

// CanSwap tells whether the languages can be swapped: the recognition hears the target.
func (c *Core) CanSwap() bool { return slices.Contains(c.SourceLangs(), c.Prefs.Tgt) }

// fitLangs makes the spoken language one the recognition hears, if it is not, and tells whether it
// changed it: English, else Japanese, else the first it hears, but the target.
func (c *Core) fitLangs() bool {
	srcs := c.SourceLangs()
	if slices.Contains(srcs, c.Prefs.Src) {
		return false
	}
	for _, l := range append([]string{"en", "ja"}, srcs...) {
		if l != c.Prefs.Tgt && slices.Contains(srcs, l) {
			c.Prefs.Src = l
			return true
		}
	}
	return false
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
