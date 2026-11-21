package guide

import (
	"errors"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

type try struct {
	started                       bool
	firstLineAfter                int
	sayWhenListening              bool
	waitingForModel               bool
	heard, recognized, translated bool
	recognizedAt, translatedAt    arrival
	src, tgt                      arrival
}

type arrival struct {
	key any
	at  time.Time
}

func (a arrival) arrived(now time.Time) bool { return !a.at.IsZero() && !now.Before(a.at.Add(travel)) }

const travel = srcAt - toASR

func (t *try) works() bool { return t.heard && t.recognized && t.translated }

func (t *try) shown(now time.Time) (recognized, translated bool) {
	return t.recognized && t.recognizedAt.arrived(now), t.translated && t.translatedAt.arrived(now)
}

func (g *Guide) tryStep(c *ui.Context, showing bool) {
	k := g.core
	switch {
	case showing && !g.try.started:
		wait := g.pending() != nil
		g.try = try{started: true, firstLineAfter: lastID(k.State.Lines), waitingForModel: wait}
		if !wait && !k.Running {
			k.Start()
		}
	case !showing && g.try.started:
		g.try.started = false
		k.StopSpeaking()
		if g.Step < stepTry && k.Running {
			k.Stop()
		}
	}
	if !g.try.started {
		return
	}
	if g.try.waitingForModel {
		if wait := g.pending() != nil; wait {
			return
		}
		g.try.waitingForModel = false
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
	if !k.Preferences().Translates() {
		g.try.translated = g.try.recognized
	}
	g.flowing(now, subtitle.Visible(st, max(k.Since, g.try.firstLineAfter+1), k.Preferences()))
	if g.try.recognized && !recognized {
		g.try.recognizedAt = arrival{at: now}
	}
	if g.try.translated && !translated {
		g.try.translatedAt = arrival{at: later(now, g.try.recognizedAt.at.Add(travel))}
	}
	for _, a := range []arrival{g.try.recognizedAt, g.try.translatedAt, g.try.src, g.try.tgt} {
		at := a.at
		if d := at.Add(travel).Sub(now); !at.IsZero() && d > 0 {
			c.After(d)
		}
	}
	if g.try.sayWhenListening && k.Running && st.Phase == live.PhaseListening {
		g.try.sayWhenListening = false
		k.Speak()
	}
}

func (g *Guide) flowing(now time.Time, entries []subtitle.Entry) {
	if len(entries) == 0 {
		return
	}
	if e := entries[len(entries)-1]; e.Key != g.try.src.key {
		g.try.src = arrival{e.Key, now}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if e := entries[i]; e.Tgt != "" && !e.DimTgt {
			if e.Key != g.try.tgt.key {
				g.try.tgt = arrival{e.Key, later(now, g.try.src.at.Add(travel))}
			}
			return
		}
	}
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

type pendingModel struct {
	model  models.Model
	status models.Status
}

func (g *Guide) pending() *pendingModel {
	k := g.core
	id := k.Chosen(core.ASR)
	m, ok := models.ByID(id)
	if !ok || id == k.InUse(core.ASR) {
		return nil
	}
	return &pendingModel{m, k.ModelStatus(id)}
}

func (g *Guide) lines(st live.State) []live.Line {
	for i, l := range st.Lines {
		if l.ID > g.try.firstLineAfter {
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

func (g *Guide) trying(c *ui.Context) {
	k := g.core
	src := spokenName(k.Preferences().Src)
	saying := k.Speaking || g.try.sayWhenListening
	g.part(c, 0, func() { g.tryIntro(c, src, saying) })
	g.part(c, 1, func() { g.tryChecks(c, saying) })
}

func spokenName(code string) string {
	if code == lang.Auto {
		return locales.AnyLanguage()
	}
	return shared.LangName(code)
}

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
				g.try.sayWhenListening = false
				k.StopSpeaking()
			} else {
				g.try.sayWhenListening = true
				if !k.Running && !g.try.waitingForModel {
					k.Start()
				}
			}
		}
	}
	if err := k.SpeakErr; err != nil {
		why := shared.ErrorText(err)
		var e *engine.Error
		if errors.As(err, &e) && e.Code == "no-voice" {
			why = shared.MacOr(locales.SampleNoVoice(src), locales.SampleNoVoiceWin(src))
		}
		kit.Warning(c, why)
	}
}

func (g *Guide) tryChecks(c *ui.Context, saying bool) {
	k := g.core
	pal := kit.Colors(c)
	g.checks(c, saying)

	pm := g.pending()
	switch {
	case pm != nil && pm.status.Err != nil:
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			kit.Warning(c, shared.DownloadError(pm.status.Err)).Grow(1)
			if kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retry()).Clicked() {
				k.Download(pm.model.ID)
			}
		})
	case pm == nil && k.Err != nil:
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			kit.Warning(c, shared.ErrorText(k.Err)).Grow(1)
			if kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retry()).Clicked() {
				k.Start()
			}
		})
	case g.try.heard && g.try.recognized && g.try.translated && g.try.translatedAt.arrived(c.Now()):
		ui.Row(c.Key("works")).Gap(kit.Space(3)).Padding(kit.Space(3), kit.Space(3)).Radius(kit.RadiusInner).Background(pal.OkTint).AlignItems(ui.Center).
			Transition(ui.ElementTransition{Animation: &kit.Spring, Enter: &ui.Motion{Y: kit.RiseBy, Scale: .96}}).Children(func() {
			ui.Icon(c, assets.Check).FontSize(kit.TextBody).TextColor(pal.OkText)
			ui.Text(c, locales.TrySuccess()).FontSize(kit.TextControl).TextColor(pal.OkText).Grow(1)
		})
	}
}

func (g *Guide) checks(c *ui.Context, saying bool) {
	k := g.core
	src, tgt := spokenName(k.Preferences().Src), shared.LangName(k.Preferences().Tgt)
	recognized, translated := g.try.shown(c.Now())
	ui.Column(c).Gap(kit.Space(3)).Margin(6, 0, 0, 0).Children(func() {
		running := k.Running && k.Err == nil
		listening := running && k.State.Phase == live.PhaseListening
		detail := ""
		if !g.try.heard {
			detail = locales.TryHearingWait()
		}
		check(c, g.try.heard, listening && saying, false, shared.MacOr(locales.TryHearing(), locales.TryHearingWin()), detail)

		pm := g.pending()
		wait := pm != nil
		var failedDownload bool
		if wait {
			failedDownload = pm.status.Err != nil
			if pm.status.State == models.Downloading {
				c.After(250 * time.Millisecond)
			}
		}
		detail = ""
		switch {
		case g.try.recognized:
		case failedDownload:
		case wait && pm.status.State == models.Verifying:
			detail = locales.Verifying()
		case wait:
			detail = locales.TryModelDownloading(shared.Size(pm.status.Done), shared.Size(pm.model.Size))
		case running && k.State.Phase == live.PhaseDownloading:
			detail = locales.StatusDownloading()
		case running && k.State.Phase != live.PhaseListening:
			detail = locales.StatusPreparing()
		default:
			detail = locales.TryRecognizingWait(src)
		}
		check(c, recognized, running && !listening || wait && !failedDownload, failedDownload, locales.TryRecognizing(src), detail)

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
				detail = locales.TranslationFailedCloud()
			}
		}
		check(c, translated, recognized && !g.try.translated && !failed, failed && !g.try.translated, locales.TryTranslating(tgt), detail)
	})
}

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

func (g *Guide) tryStage(c *ui.Context) {
	k := g.core
	p := k.Preferences()
	now := c.Now()
	entries := subtitle.Visible(k.State, max(k.Since, g.try.firstLineAfter+1), p)
	recognized, translated := g.try.shown(now)
	status := ""
	wait := g.pending() != nil
	switch {
	case g.try.heard:
	case wait:
		status = locales.TryModelThen()
	case k.Err != nil:
		status = shared.ErrorText(k.Err)
	case k.State.Phase == live.PhaseDownloading:
		status = locales.StatusDownloading()
	case k.State.Phase == live.PhaseLoading:
		status = locales.StatusLoadingModel()
	case k.State.Phase != live.PhaseListening:
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
		node(c, "heard", g.try.heard, 0, assets.Waves, shared.MacOr(locales.SystemAudioOnly(), locales.SourceAll()), "", func(lit bool) {
			kit.Levels(c, k.State.Voice, 14, pal.Starlight.Alpha(.8)).Size(120, 18)
			if status != "" {
				ui.Text(c, status).FontSize(kit.TextControl).TextColor(pal.Starlight.Alpha(.6)).MaxLines(2).ContentTransition(ui.ContentTransition{})
			}
		})
		flow(c, "to-asr", leaving(g.try.src.at))
		node(c, "asr", recognized, 0, assets.Captions, shared.EngineName(asr), "", func(bool) {
			if len(entries) > 0 {
				e := entries[len(entries)-1]
				sentence(c, e.Key, e.Src, kit.TextBody, pal.Starlight.Alpha(.94), false, g.try.src.at.Add(travel).Sub(now))
			}
		})
		flow(c, "to-mt", leaving(g.try.tgt.at))
		node(c, "mt", translated, 0, icon, shared.EngineName(mt), "", func(bool) {
			for i := len(entries) - 1; i >= 0; i-- {
				if e := entries[i]; e.Tgt != "" && !e.DimTgt {
					sentence(c, e.Key, e.Tgt, kit.TextSection, pal.SubTgt, false, g.try.tgt.at.Add(travel).Sub(now)).FontWeight(kit.WeightBold)
					return
				}
			}
		})
	})
}
