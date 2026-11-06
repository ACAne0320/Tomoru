package guide

import (
	"errors"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// try is the try step's: translating, started for it, and what it has seen work since. It checks
// that all works as it will for the user's videos: the permission, recognition of the language,
// translation, with what the user plays, or sample sentences the system's voice says.
type try struct {
	on                            bool // the step shows: translating was started for it
	after                         int  // the last line's ID as it started: those after are the step's
	sample                        bool // the sample is to be said once translating listens
	wait                          bool // translating waits for the recognition model to download
	heard, recognized, translated bool
	// When recognizing and translating first worked, and the stage's sentence and translation, by
	// their sentence's key, and when each came: the stage's light goes on to their pane then, and
	// they come in, and their checks tick, as it gets there, travel later.
	recognizedAt, translatedAt time.Time
	srcKey, tgtKey             any
	srcAt, tgtAt               time.Time
}

// travel is how long the stage's light takes from a pane to the next.
const travel = srcAt - toASR

// works tells whether all three worked.
func (t *try) works() bool { return t.heard && t.recognized && t.translated }

// arrived tells whether the light that left at at has got there by now.
func arrived(at, now time.Time) bool { return !at.IsZero() && !now.Before(at.Add(travel)) }

// shown tells whether recognizing and translating show as worked: their lights got there.
func (t *try) shown(now time.Time) (recognized, translated bool) {
	return t.recognized && arrived(t.recognizedAt, now), t.translated && arrived(t.translatedAt, now)
}

// tryStep starts translating as the step shows, and stops it, and the sample, as the guide goes
// back from it. Going on, translating goes on: it is what the guide ends with.
func (g *Guide) tryStep(c *ui.Context, showing bool) {
	k := g.core
	switch {
	case showing && !g.try.on:
		_, _, wait := g.pending()
		g.try = try{on: true, after: lastID(k.State.Lines), wait: wait}
		if !wait && !k.Running {
			k.Start()
		}
	case !showing && g.try.on:
		g.try.on = false
		k.StopSpeaking()
		if g.Step < stepTry && k.Running {
			k.Stop()
		}
	}
	if !g.try.on {
		return
	}
	if g.try.wait {
		if _, _, wait := g.pending(); wait {
			return
		}
		// Downloaded, the model is in use: translating starts with it.
		g.try.wait = false
		if !k.Running {
			k.Start()
		}
	}
	st := k.State
	lines := g.lines(st)
	now := c.Now()
	recognized, translated := g.try.recognized, g.try.translated
	g.try.recognized = g.try.recognized || st.Pending != "" || len(lines) > 0
	g.try.heard = g.try.heard || st.Voice || g.try.recognized
	for _, l := range lines {
		g.try.translated = g.try.translated || l.Tgt != "" && l.Err == nil || l.Draft != ""
	}
	g.try.translated = g.try.translated || st.Draft != ""
	g.flowing(now, subtitle.Visible(st, max(k.Since, g.try.after+1), k.Preferences()))
	if g.try.recognized && !recognized {
		g.try.recognizedAt = now
	}
	if g.try.translated && !translated {
		g.try.translatedAt = later(now, g.try.recognizedAt.Add(travel)) // after recognizing's light
	}
	// Again as the lights get there.
	for _, at := range []time.Time{g.try.recognizedAt, g.try.translatedAt, g.try.srcAt, g.try.tgtAt} {
		if d := at.Add(travel).Sub(now); !at.IsZero() && d > 0 {
			c.After(d)
		}
	}
	if g.try.sample && k.Running && st.Phase == "listening" {
		g.try.sample = false
		k.Speak()
	}
}

// flowing notes the stage's sentence and translation, the last of entries, as they come: a sentence
// as it does, and a translation once the light bringing its sentence got there.
func (g *Guide) flowing(now time.Time, entries []subtitle.Entry) {
	if len(entries) == 0 {
		return
	}
	if e := entries[len(entries)-1]; e.Key != g.try.srcKey {
		g.try.srcKey, g.try.srcAt = e.Key, now
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if e := entries[i]; e.Tgt != "" && !e.DimTgt {
			if e.Key != g.try.tgtKey {
				g.try.tgtKey, g.try.tgtAt = e.Key, later(now, g.try.srcAt.Add(travel))
			}
			return
		}
	}
}

// later is the later of a and b.
func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// pending is the recognition model chosen on the engines step and not in use yet, as it is till
// it is downloaded, and how its download goes.
func (g *Guide) pending() (models.Model, models.Status, bool) {
	k := g.core
	id := k.Chosen(core.ASR)
	m, ok := models.ByID(id)
	if !ok || id == k.InUse(core.ASR) {
		return models.Model{}, models.Status{}, false
	}
	return m, k.ModelStatus(id), true
}

// lines are the session's lines said since the step showed.
func (g *Guide) lines(st live.State) []live.Line {
	for i, l := range st.Lines {
		if l.ID > g.try.after {
			return st.Lines[i:]
		}
	}
	return nil
}

func lastID(lines []live.Line) int {
	if len(lines) == 0 {
		return 0
	}
	return lines[len(lines)-1].ID
}

// trying is the try step's words: what to play, the sample's button where there is one, and the
// three checks.
func (g *Guide) trying(c *ui.Context) {
	k := g.core
	src := shared.LangName(k.Preferences().Src)
	saying := k.Speaking || g.try.sample
	g.part(c, 0, func() { g.tryIntro(c, src, saying) })
	g.part(c, 1, func() { g.tryChecks(c, saying) })
}

// tryIntro is the try step's heading, what to play, and the sample's button where there is one.
func (g *Guide) tryIntro(c *ui.Context, src string, saying bool) {
	k := g.core
	title(c, locales.GuideTryTitle())
	if !k.HasSample() && !saying {
		body(c, locales.GuideTryVideo(src))
	} else {
		body(c, locales.GuideTryBody(src))
		label, icon := locales.PlaySample(), assets.Play
		if saying {
			label, icon = locales.StopSample(), assets.Stop
		}
		if kit.Button(c, kit.Plain, kit.Small, icon, label).AlignSelf(ui.Start).Clicked() {
			if saying {
				g.try.sample = false
				k.StopSpeaking()
			} else {
				g.try.sample = true
				if !k.Running && !g.try.wait {
					k.Start()
				}
			}
		}
	}
	if err := k.SpeakErr; err != nil {
		why := shared.ErrorText(err)
		var e *engine.Error
		if errors.As(err, &e) && e.Code == "no-voice" {
			why = shared.OnWindows(locales.SampleNoVoice(src), locales.SampleNoVoiceWin(src))
		}
		kit.Warning(c, why)
	}
}

// tryChecks are the three checks, and under them what went wrong, or that all works.
func (g *Guide) tryChecks(c *ui.Context, saying bool) {
	k := g.core
	pal := kit.Colors(c)
	g.checks(c, saying)

	m, st, wait := g.pending()
	switch {
	case wait && st.Err != nil:
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			kit.Warning(c, shared.DownloadError(st.Err)).Grow(1)
			if kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retry()).Clicked() {
				k.Download(m.ID)
			}
		})
	case !wait && k.Err != nil:
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			kit.Warning(c, shared.ErrorText(k.Err)).Grow(1)
			if kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retry()).Clicked() {
				k.Start()
			}
		})
	case g.try.heard && g.try.recognized && g.try.translated && arrived(g.try.translatedAt, c.Now()):
		ui.Row(c.Key("works")).Gap(kit.Space(3)).Padding(kit.Space(3), kit.Space(3)).Radius(kit.RadiusInner).Background(pal.OkTint).AlignItems(ui.Center).
			Transition(ui.ElementTransition{Animation: &kit.Spring, Enter: &ui.Motion{Y: kit.RiseBy, Scale: .96}}).Children(func() {
			ui.Icon(c, assets.Check).FontSize(kit.TextBody).TextColor(pal.OkText)
			ui.Text(c, locales.TrySuccess()).FontSize(kit.TextControl).TextColor(pal.OkText).Grow(1)
		})
	}
}

// checks is the try step's three checks, hearing, recognizing and translating, each ticked as
// soon as it passes; saying is whether the sample is being said.
func (g *Guide) checks(c *ui.Context, saying bool) {
	k := g.core
	src, tgt := shared.LangName(k.Preferences().Src), shared.LangName(k.Preferences().Tgt)
	// Ticked as the stage's light gets to the pane of each.
	recognized, translated := g.try.shown(c.Now())
	ui.Column(c).Gap(kit.Space(3)).Margin(6, 0, 0, 0).Children(func() {
		running := k.Running && k.Err == nil
		listening := running && k.State.Phase == "listening"
		detail := ""
		if !g.try.heard {
			detail = locales.TryHearingWait()
		}
		check(c, g.try.heard, listening && saying, false, shared.OnWindows(locales.TryHearing(), locales.TryHearingWin()), detail)

		m, st, wait := g.pending()
		if wait && st.State == models.Downloading {
			c.After(250 * time.Millisecond)
		}
		detail = ""
		switch {
		case g.try.recognized:
		case wait && st.Err != nil: // said under the checks, with its retry
		case wait && st.State == models.Verifying:
			detail = locales.Verifying()
		case wait:
			detail = locales.TryModelDownloading(shared.Size(st.Done), shared.Size(m.Size))
		case running && k.State.Phase == "downloading":
			detail = locales.StatusDownloading()
		case running && k.State.Phase != "listening":
			detail = locales.StatusPreparing()
		default:
			detail = locales.TryRecognizingWait(src)
		}
		check(c, recognized, running && !listening || wait && st.Err == nil, wait && st.Err != nil, locales.TryRecognizing(src), detail)

		failed := false
		for _, l := range g.lines(k.State) {
			failed = failed || l.Err != nil && !errors.Is(l.Err, live.ErrBehind)
		}
		mt, ok := k.Engine(core.MT, k.Preferences().MT)
		if !ok {
			mt.ID = k.Preferences().MT
		}
		detail = locales.TryTranslatingWith(shared.EngineName(mt))
		if failed && !g.try.translated {
			detail = locales.TranslationFailed()
			if k.Preferences().MT == "cloud" {
				detail = locales.TranslationFailedCloud() // the free service is down at times
			}
		}
		check(c, translated, recognized && !g.try.translated && !failed, failed && !g.try.translated, locales.TryTranslating(tgt), detail) // busy once recognizing shows done
	})
}

// check is a check of the try step: its mark, its name, and under it what it waits for, if
// anything.
func check(c *ui.Context, done, busy, failed bool, name, detail string) {
	pal := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Start).Children(func() {
		mark(c, 18, done, busy, failed).Margin(1, 0, 0, 0)
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, name).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(pal.Ink)
			if detail != "" {
				kit.Caption(c, detail).ContentTransition(ui.ContentTransition{})
			}
		})
	})
}

// tryStage is the way of a sentence, as on the engine steps, for real: what is heard, its levels,
// and until then what it waits for; what is recognized; its translation. Each pane lights as it
// works, a light going down to it from the one before as a sentence and its translation come, and
// they come in as it gets there.
func (g *Guide) tryStage(c *ui.Context) {
	k := g.core
	p := k.Preferences()
	now := c.Now()
	entries := subtitle.Visible(k.State, max(k.Since, g.try.after+1), p)
	recognized, translated := g.try.shown(now)
	status := ""
	_, _, wait := g.pending()
	switch {
	case g.try.heard:
	case wait: // the download's progress, or why it failed, is under the checks
		status = locales.TryModelThen()
	case k.Err != nil:
		status = shared.ErrorText(k.Err)
	case k.State.Phase == "downloading":
		status = locales.StatusDownloading()
	case k.State.Phase == "loading":
		status = locales.StatusLoadingModel()
	case k.State.Phase != "listening":
		status = locales.StatusPreparing()
	default:
		status = locales.TryListening()
	}
	asr := core.Engine{ID: k.Chosen(core.ASR)}
	mt := core.Engine{ID: p.MT}
	icon := assets.Cloud
	if mt.ID != "cloud" {
		icon = assets.Captions
	}
	pal := kit.Colors(c)
	ui.Column(c).Fill().Justify(ui.Center).Padding(kit.Space(6)).Children(func() {
		node(c, "heard", g.try.heard, 0, assets.Waves, shared.OnWindows(locales.SystemAudioOnly(), locales.SourceAll()), "", func(lit bool) {
			kit.Levels(c, k.State.Voice, 14, pal.Starlight.Alpha(.8)).Size(120, 18)
			if status != "" {
				ui.Text(c, status).FontSize(kit.TextControl).TextColor(pal.Starlight.Alpha(.6)).MaxLines(2).ContentTransition(ui.ContentTransition{})
			}
		})
		flow(c, "to-asr", leaving(g.try.srcAt))
		node(c, "asr", recognized, 0, assets.Captions, shared.EngineName(asr), "", func(bool) {
			if len(entries) > 0 {
				e := entries[len(entries)-1]
				sentence(c, e.Key, e.Src, kit.TextBody, pal.Starlight.Alpha(.94), false, g.try.srcAt.Add(travel).Sub(now))
			}
		})
		flow(c, "to-mt", leaving(g.try.tgtAt))
		node(c, "mt", translated, 0, icon, shared.EngineName(mt), "", func(bool) {
			for i := len(entries) - 1; i >= 0; i-- {
				if e := entries[i]; e.Tgt != "" && !e.DimTgt {
					sentence(c, e.Key, e.Tgt, kit.TextSection, pal.SubTgt, false, g.try.tgtAt.Add(travel).Sub(now)).FontWeight(kit.WeightBold)
					return
				}
			}
		})
	})
}
