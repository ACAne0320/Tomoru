package guide

import (
	"cmp"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// recognition is the step choosing how what is heard is recognized, on this computer: the
// system's, or a model to download. Without the system's, none is chosen for the user. The
// cloud's are set up in the settings.
func (g *Guide) recognition(c *ui.Context) {
	g.part(c, 0, func() {
		title(c, locales.EnginesAsr())
		body(c, locales.GuideEnginesLater())
	})
	g.part(c, 1, func() { g.recognitionChoices(c) })
}

// recognitionChoices are the recognition step's choices, as the settings list them: the system's,
// then the models, Qwen3-ASR recommended where the GPU runs it.
func (g *Guide) recognitionChoices(c *ui.Context) {
	k := g.core
	ui.Column(c).Gap(kit.Space(2)).Role(ui.RoleRadioGroup).Label(locales.GuideAsrChoose()).Children(func() {
		section(c, locales.TagLocal())
		var rows []kit.Option
		var takes []func()
		if prefs.SystemASR() != "" {
			o, take := g.systemOption(c)
			rows, takes = append(rows, o), append(takes, take)
		}
		for _, e := range k.Engines(core.ASR) {
			if e.Model == nil {
				continue
			}
			tag, body := "", ""
			if k.Recommended(e) {
				tag = locales.TagRecommended()
			}
			if e.ID == "sensevoice-small" {
				body = locales.EngineSenseVoiceDescription()
			}
			o, take := g.modelOption(c, core.ASR, *e.Model, tag, body)
			rows, takes = append(rows, o), append(takes, take)
		}
		choices(c, rows, takes)
	})
}

// translation is the step choosing how it is translated: in the cloud, chosen to begin with and
// recommended, so that the guide goes on at once; or on this computer, with a model downloading
// meanwhile, used once downloaded, the cloud translating until then, the guide not waiting for it.
func (g *Guide) translation(c *ui.Context) {
	g.part(c, 0, func() {
		title(c, locales.EnginesMt())
		body(c, locales.GuideMtLater())
	})
	g.part(c, 1, func() { g.translationChoices(c) })
}

// translationChoices are the translation step's choices: this computer's models, then the cloud.
func (g *Guide) translationChoices(c *ui.Context) {
	k := g.core
	ui.Column(c).Gap(kit.Space(2)).Role(ui.RoleRadioGroup).Children(func() {
		section(c, locales.TagLocal())
		var rows []kit.Option
		var takes []func()
		for _, m := range models.All {
			if !m.ASR {
				o, take := g.modelOption(c, core.MT, m, "", "")
				rows, takes = append(rows, o), append(takes, take)
			}
		}
		choices(c, rows, takes)
		section(c, locales.TagCloud())
		cloud := k.Chosen(core.MT) == "cloud"
		kit.OptionsCard(c).Children(func() {
			o := kit.Option{Key: "cloud", On: cloud, Title: shared.CloudName, Tag: locales.TagRecommended(), Accent: true, Meta: locales.EngineCloudThanks()}
			if kit.Options(c, []kit.Option{o}) == 0 && !cloud {
				k.Choose(core.MT, "cloud")
			}
			kit.Caption(c, shared.CloudSends(core.MT)).Margin(kit.Space(2), kit.Space(3), kit.Space(3))
		})
	})
}

// choices lists rows of a side in their card; choosing one that isn't chosen takes it, as its
// take of takes does.
func choices(c *ui.Context, rows []kit.Option, takes []func()) {
	kit.OptionsCard(c).Children(func() {
		if i := kit.Options(c, rows); i >= 0 && !rows[i].On {
			takes[i]()
		}
	})
}

// section heads a side of a step's choices: this computer's, or the cloud's.
func section(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).InkMuted).Margin(kit.Space(1), 0, 0, kit.Space(1))
}

// systemOption is the system's recognition as a choice, ready with nothing to download: its state
// for the language spoken at its end while chosen; and taking it.
func (g *Guide) systemOption(c *ui.Context) (kit.Option, func()) {
	k := g.core
	on := k.Chosen(core.ASR) == "apple"
	asr := k.ASR[k.Preferences().Src]
	desc := locales.EngineAppleDescription() // as the settings say it; its tag says it is built in
	switch asr {
	case "ready":
	case "error":
		desc = locales.AsrDownloadFailedNote()
	default:
		desc = locales.AsrFirstDownload()
	}
	o := kit.Option{Key: "apple", On: on, Title: locales.EngineApple(), Tag: locales.TagBuiltIn(), Meta: desc}
	if on {
		o.Action = func() { mark(c, 20, asr == "ready", asr == "preparing" || asr == "", asr == "error") }
	}
	return o, func() { k.Choose(core.ASR, "apple") }
}

// modelOption is a model to download as a choice of kind, as the settings list it: its name and
// tag over shared.ModelNote of body; under them why its download failed, or its download's
// progress, under way or stopped; and at its end its download's button, or cancelling it. Chosen,
// it is downloaded, and used once it is; the step says so once for all. It returns the row and
// taking it.
func (g *Guide) modelOption(c *ui.Context, kind core.Kind, m models.Model, tag, body string) (kit.Option, func()) {
	k := g.core
	st := k.ModelStatus(m.ID)
	if st.State == models.Downloading {
		c.After(250 * time.Millisecond)
	}
	take := func() {
		k.Choose(kind, m.ID)
		if st.State == models.Missing {
			k.Download(m.ID)
		}
	}
	o := kit.Option{Key: m.ID, On: k.Chosen(kind) == m.ID, Title: shared.EngineName(core.Engine{ID: m.ID}), Tag: tag, Accent: tag != "",
		Meta: shared.ModelNote(m, st, body)}
	switch why := shared.DownloadError(st.Err); {
	case why != "" && st.State == models.Missing:
		o.Below = func() { kit.Warning(c, why).MaxLines(1).Tooltip(why) }
	case st.State == models.Downloading || st.State == models.Missing && st.Done > 0:
		total := cmp.Or(st.Total, m.Size)
		end := float32(0) // a Plain download's edge shows; a cancel's glyph is in from its own
		if st.State == models.Downloading {
			end = kit.QuietInset
		}
		o.Below = func() {
			kit.OptionProgress(c, float64(st.Done)/float64(max(total, 1)), locales.Downloading(shared.Size(st.Done), shared.Size(total))).Padding(0, end, 0, 0)
		}
	case st.State == models.Verifying:
		o.Below = func() { kit.Caption(c, locales.Verifying()).MaxLines(1) }
	}
	// Download, even to go on with one stopped: its progress under its name says so. The guide's
	// column is narrow: the button is its icon, named in its tooltip, so that the name and what the
	// model needs keep their lines in any language.
	label := locales.DownloadModel()
	if st.Err != nil && st.Done == 0 {
		label = locales.Retry()
	}
	switch st.State {
	case models.Missing:
		o.Action = func() {
			if kit.IconButton(c, kit.Plain, kit.Small, assets.Download, label).Clicked() {
				take()
			}
		}
	case models.Downloading:
		o.Action = func() {
			if kit.CancelButton(c, locales.Cancel()).Clicked() {
				k.CancelDownload(m.ID)
				k.Choose(kind, k.InUse(kind)) // what is in use again, not the model once resumed
			}
		}
	}
	return o, take
}

// The way of a sentence on the engine steps' stage, in time: a light goes down from what is heard
// to recognition, the sentence comes in there, a light goes on to translation, and its translation
// comes in there; then they stay, until the next. What a sentence replaces fades first (leave), so
// that the two never show over each other.
const (
	pipeCycle = 3200 * time.Millisecond // a sentence's turn
	toASR     = 0                       // the light from what is heard, until srcAt
	srcAt     = 420 * time.Millisecond  // the sentence comes in
	toMT      = 720 * time.Millisecond  // the light on to translation, until tgtAt
	tgtAt     = 1140 * time.Millisecond // its translation comes in
	leave     = 120 * time.Millisecond  // kit.FadeQuick
)

// pipeline is where the stage's sentences are at now: the number of the sentence recognized and
// of the one translated, each the one before until its turn comes, and t, how far into its turn.
func pipeline(now time.Time) (src, tgt int64, t time.Duration) {
	ms := now.UnixMilli()
	n := ms / pipeCycle.Milliseconds()
	t = time.Duration(ms%pipeCycle.Milliseconds()) * time.Millisecond
	src, tgt = n, n
	if t < srcAt {
		src--
	}
	if t < tgtAt {
		tgt--
	}
	return src, tgt, t
}

// pipeNext is how long from t until what the stage shows changes: a sentence going, or coming.
func pipeNext(t time.Duration) time.Duration {
	for _, at := range []time.Duration{srcAt - leave, srcAt, tgtAt - leave, tgtAt} {
		if at > t {
			return at - t
		}
	}
	return pipeCycle - t
}

// going tells that what shows until at fades now, before what comes at at.
func going(t, at time.Duration) bool { return t >= at-leave && t < at }

// enginesStage is the way of a sentence: heard, recognized, translated, a light going down from
// each to the next as the sentence does; the engines of focus, the step's kind, lit.
func (g *Guide) enginesStage(c *ui.Context, focus core.Kind) {
	k := g.core
	p := k.Preferences()
	src, tgt, t := pipeline(c.Now())
	c.After(pipeNext(t))
	mt := core.Engine{ID: k.Chosen(core.MT)}
	icon := assets.Cloud
	if mt.ID != "cloud" {
		icon = assets.Captions
	}
	asr := core.Engine{ID: k.Chosen(core.ASR)}
	where := locales.TagLocal()
	switch asr.ID {
	case core.QwenCloud, core.OpenAI:
		where = locales.TagCloud()
	case "":
		asr.ID = "sensevoice-small" // the stage shows the way of a sentence, chosen or not
	}
	pal := kit.Colors(c)
	ui.Column(c).Fill().Justify(ui.Center).Padding(kit.Space(6)).Children(func() {
		node(c, "heard", focus == core.ASR, 0, assets.Waves, shared.OnWindows(locales.SystemAudioOnly(), locales.SourceAll()), "", func(lit bool) {
			ink := pal.Starlight.Alpha(.8)
			if !lit {
				ink = pal.Starlight.Alpha(.4)
			}
			kit.Levels(c, true, 14, ink).Size(120, 18)
		})
		flow(c, "to-asr", inTurn(toASR, srcAt))
		node(c, "asr", focus == core.ASR, g.flash(c, core.ASR), assets.Captions, shared.EngineName(asr), where, func(lit bool) {
			ink := pal.Starlight.Alpha(.94)
			if !lit {
				ink = pal.Starlight.Alpha(.5)
			}
			sentence(c, src, subtitle.Sample(p, src).Src, kit.TextBody, ink, going(t, srcAt), kit.Settle)
		})
		flow(c, "to-mt", inTurn(toMT, tgtAt))
		node(c, "mt", focus == core.MT, g.flash(c, core.MT), icon, shared.EngineName(mt), "", func(lit bool) {
			ink := pal.SubTgt
			if !lit {
				ink = pal.SubTgt.Alpha(.5)
			}
			sentence(c, tgt, subtitle.Sample(p, tgt).Tgt, kit.TextSection, ink, going(t, tgtAt), kit.Settle).FontWeight(kit.WeightBold)
		})
	})
}

// flashFor is how long a pane flashes as its engine is chosen.
const flashFor = 700 * time.Millisecond

// pick is the engine of a kind the stage last showed, and when it was chosen.
type pick struct {
	id string
	at time.Time
}

// flash is how bright the pane of kind's engine flashes, from 1 as another is chosen down to 0:
// the stage answers the choice on the left. Not as the step shows: only for a choice made.
func (g *Guide) flash(c *ui.Context, kind core.Kind) float32 {
	if g.picks == nil {
		g.picks = map[core.Kind]pick{}
	}
	id, now := g.core.Chosen(kind), c.Now()
	p, seen := g.picks[kind]
	if p.id != id {
		p = pick{id: id}
		if seen {
			p.at = now
		}
		g.picks[kind] = p
	}
	k := 1 - float32(now.Sub(p.at))/float32(flashFor)
	if p.at.IsZero() || k <= 0 {
		return 0
	}
	c.AnimationFrame()
	return k * k // fading fast, then slowly
}

// node is a step of a sentence's way on the stage, a pane: an icon, its name and tag, and under
// them what fn shows. Lit, while it is the step's, it is brighter, in an amber hairline; else only
// what is in it dims. Flashing (1 to 0), it glows amber, its engine just chosen; its name, changing,
// gives way to the next, rising in. It grows and shrinks with what is in it, on the spring, clipping it on
// the way.
func node(c *ui.Context, key string, lit bool, flash float32, icon *ui.SVG, name, tag string, fn func(lit bool)) {
	pal := kit.Colors(c)
	row := ui.Row(c.Key(key)).Gap(kit.Space(3)).Padding(kit.Space(3), kit.Space(4)).AlignItems(ui.Start).
		Radius(kit.RadiusField).Clip().Transition(kit.Grow())
	bg, edge, label, mark := pal.Glass, pal.GlassLine, pal.Starlight.Alpha(.4), pal.Lantern.Alpha(.5)
	if lit {
		bg, edge, label, mark = pal.Starlight.Alpha(.11), pal.Lantern.Alpha(.5), pal.Starlight.Alpha(.66), pal.Lantern
	}
	edge = row.AnimateColor("edge", edge, kit.Fade)
	if flash > 0 {
		edge = pal.Lantern.Alpha(.5 + .5*flash)
		row.Shadow(0, 0, 18*flash, 0, pal.Lantern.Alpha(.45*flash))
	}
	row.Background(row.AnimateColor("bg", bg, kit.Fade)).Border(1, edge)
	label, mark = row.AnimateColor("label", label, kit.Fade), row.AnimateColor("mark", mark, kit.Fade)
	row.Children(func() {
		tint(c, 32, func() { ui.Icon(c, icon).FontSize(kit.IconSizeNav).TextColor(mark) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() {
			ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
				ui.Text(c.Key(name), name).NoWrap().FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(label).Transition(kit.Rise(0)) // another rises in its place
				if tag != "" {
					ui.Text(c, tag).NoWrap().FontSize(kit.TextMicro).TextColor(label).Background(pal.Glass).Radius(kit.Pill(16)).Padding(0, kit.Space(2))
				}
			})
			fn(lit)
		})
	})
}

// sentence is a sentence on the stage, s, the one of key, in ink: it fades as it goes; the next,
// in its place, rises in after a while, once its pane has taken its size for it (at least
// kit.Settle), never meeting the pane's edge.
func sentence(c *ui.Context, key any, s string, size float32, ink ui.Color, goes bool, after time.Duration) ui.Element {
	e := ui.Text(c.Key(key), s).FontSize(size).MaxLines(2).Transition(kit.RiseAfter(max(after, kit.Settle)))
	o := float32(1)
	if goes {
		o = 0
	}
	return e.TextColor(e.AnimateColor("ink", ink, kit.Fade)).Opacity(e.AnimateTo("going", o, kit.FadeQuick))
}

// light says where a flow's light is now: how far down, from 0 to 1, or below 0 while there is
// none, with how long until there is one, if it is known.
type light func(now time.Time) (k float32, wait time.Duration)

// inTurn is the light of the sample's turns, going from from to to into each.
func inTurn(from, to time.Duration) light {
	return func(now time.Time) (float32, time.Duration) {
		_, _, t := pipeline(now)
		switch {
		case t < from:
			return -1, from - t
		case t >= to:
			return -1, pipeCycle - t + from
		}
		return float32(t-from) / float32(to-from), 0
	}
}

// leaving is the light that left at at, for travel; none before it leaves and after it gets there.
func leaving(at time.Time) light {
	return func(now time.Time) (float32, time.Duration) {
		d := now.Sub(at)
		switch {
		case at.IsZero() || d >= travel:
			return -1, 0
		case d < 0:
			return -1, -d
		}
		return float32(d) / float32(travel), 0
	}
}

// flow joins two nodes, under the middle of their icons: a hairline, down which a light goes as
// the sentence does, arriving as what it brings comes in.
func flow(c *ui.Context, key string, at light) {
	still := c.Preferences().ReduceMotion
	pal := kit.Colors(c)
	x := kit.Space(4) + 16 // the pane's border and padding, half its icon's ground, less half the line, as wide as the border
	ui.Box(c.Key(key)).Height(26).Width(2).Margin(0, 0, 0, x).AlignSelf(ui.Start).Background(pal.Starlight.Alpha(.16)).Transition(kit.Grow()).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if still {
				return
			}
			k, wait := at(p.Now())
			if k < 0 {
				if wait > 0 {
					p.After(wait)
				}
				return
			}
			k = k * k * (3 - 2*k) // easing in and out
			const d = 6
			head := r.Y + (r.H-d)*k
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: head - r.Y + d/2}, pal.Lantern.Alpha(.45*(1-k)), 1) // what it passed, fading
			p.Fill(ui.Rect{X: r.X + r.W/2 - d/2, Y: head, W: d, H: d}, pal.Lantern, d/2)
			p.After(16 * time.Millisecond)
		})
}
