package settings

import (
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/asr"
	"tomoru/internal/core"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// engines is the page of recognition and translation (docs/design-system.md): a card of each, on
// this computer or in the cloud, each side a list of its engines.
func (s *Settings) engines(c *ui.Context) {
	k := s.core
	p := kit.Colors(c)
	ui.Text(c, locales.TabEngines()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
	ui.Row(c).Gap(kit.Space(3.5)).AlignItems(ui.Start).Wrap().Children(func() {
		s.kind(c, core.ASR)
		s.kind(c, core.MT)
	})
	ui.Row(c).Gap(kit.Space(3.5)).AlignItems(ui.Center).Padding(kit.Space(4), kit.Space(5)).Radius(kit.RadiusCard).Background(p.Sand).Children(func() {
		icon(c, assets.Zap)
		ui.Text(c, locales.DraftLabel()).FontSize(kit.TextSection).FontWeight(kit.WeightBold).TextColor(p.Ink).Grow(1)
		draft := k.Preferences().Draft
		if kit.Switch(c, &draft, locales.DraftLabel()).Changed() {
			k.UpdatePrefs(func(p *prefs.Prefs) { p.Draft = draft })
		}
	})
	s.endpointDialog(c)
	shared.ASRCloudDialog(c, s.core, &s.cloud)
	s.watchModels(c)
}

// icon is a card's icon, on the lantern's tint.
func icon(c *ui.Context, svg *ui.SVG) {
	p := kit.Colors(c)
	const d = 32
	ui.Box(c).Size(d, d).Radius(kit.Pill(d)).Background(p.LanternTint).Center().Shrink(0).Children(func() {
		ui.Icon(c, svg).FontSize(kit.IconSizeButton + 1).TextColor(p.LanternText)
	})
}

// kind is the card of the engines of a kind: its title, this computer or the cloud to look at,
// and the engines there, the one in use marked among them.
func (s *Settings) kind(c *ui.Context, kind core.Kind) {
	k := s.core
	p := kit.Colors(c)
	title, svg := locales.EnginesAsr(), assets.Waves
	if kind == core.MT {
		title, svg = locales.EnginesMt(), assets.Languages
	}
	all := k.Engines(kind)
	chosen, _ := k.Engine(kind, k.Chosen(kind))
	cloud, ok := s.sides[kind]
	if !ok {
		cloud = chosen.Cloud
	}
	kit.Card(c.Key(kind)).Grow(1).Basis(380).Gap(kit.Space(3.5)).Children(func() {
		ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center).Children(func() {
			icon(c, svg)
			ui.Text(c, title).FontSize(kit.TextSection + 1).FontWeight(kit.WeightBold).TextColor(p.Ink).Grow(1)
			where := 0
			if cloud {
				where = 1
			}
			if kit.Segmented(c, &where, locales.TagLocal(), locales.TagCloud()).Label(locales.EngineWhere()).Changed() {
				s.sides[kind] = where == 1
			}
		})
		if online, _ := k.Network(); kind == core.MT && !online {
			kit.Warning(c, locales.OfflineNote())
		}
		here := side(all, cloud)
		switch {
		case len(here) == 0: // only on a computer without the system's recognition and models
			kit.Caption(c, locales.AsrLocalNone())
		default:
			s.options(c, kind, here, chosen)
		}
	})
}

// side are the engines of all in the cloud, or on this computer.
func side(all []core.Engine, cloud bool) []core.Engine {
	var out []core.Engine
	for _, e := range all {
		if e.Cloud == cloud {
			out = append(out, e)
		}
	}
	return out
}

// options lists the engines of a side, the one chosen marked, each with what is done with it at
// its end: choosing one uses it, once a model is downloaded or a service set up, which choosing it
// starts. The cloud's translation ends with adding an API of one's own.
func (s *Settings) options(c *ui.Context, kind core.Kind, here []core.Engine, chosen core.Engine) {
	k := s.core
	_, metered := k.Network()
	ui.Column(c).Gap(kit.Space(1)).Children(func() {
		for _, e := range here {
			on := e.ID == chosen.ID
			var st models.Status
			tag, accent, meta := "", false, describe(e)
			switch {
			case e.Model != nil:
				// A model's size leads its description, and what it needs to run ends it.
				st = k.ModelStatus(e.ID)
				meta = shared.ModelNote(*e.Model, st, meta)
				if e.ID == "local-9b" || k.Recommended(e) {
					tag, accent = locales.TagRecommended(), true
				}
			case e.ID == "apple":
				tag = locales.TagBuiltIn()
			case e.ID == "cloud":
				tag, accent = locales.TagRecommended(), true
			case e.Endpoint != nil:
				meta = e.Endpoint.Model + " · " + host(e.Endpoint.URL)
			case e.Kind == core.ASR && e.Cloud && k.ASRAPI(e.ID).Key:
				meta = asrAPI(k, e.ID)
			}
			ui.Column(c.Key(e.ID)).Gap(kit.Space(1)).Children(func() {
				if kit.OptionRow(c, on, shared.EngineName(e), tag, accent, meta, func() { s.action(c, e, st) }).Clicked() && !on {
					k.Choose(kind, e.ID)
					switch {
					case e.Model != nil && st.State == models.Missing:
						k.Download(e.ID)
					case e.Cloud && !k.Usable(e):
						s.edits++
						s.cloud.Open(s.core, e.ID, s.edits) // a service to set up: set it up now
					}
				}
				ui.Column(c).Gap(kit.Space(1)).Padding(0, kit.Space(3), 0, kit.Space(11.5)).Children(func() {
					if e.Model != nil {
						if mem := models.Memory(); e.Model.Memory != 0 && mem != 0 && mem < e.Model.Memory {
							kit.Warning(c, shared.OnWindows(locales.MemoryWarning(int(mem>>30)), locales.MemoryWarningWin(int(mem>>30))))
						}
						if why := shared.DownloadError(st.Err); why != "" && st.State == models.Missing {
							kit.Warning(c, why)
						}
						if st.State == models.Missing && metered {
							kit.Caption(c, locales.MeteredNote(shared.Size(e.Model.Size-st.Done)))
						}
					}
					if on && e.ID != k.InUse(kind) {
						current, _ := k.Engine(kind, k.InUse(kind))
						waiting := locales.EngineWaiting(shared.EngineName(current))
						if e.Cloud {
							waiting = locales.EngineWaitingSetup(shared.EngineName(current))
						}
						kit.Caption(c, waiting)
					}
				})
			})
		}
		if len(here) == 0 || !here[0].Cloud {
			return
		}
		if kind == core.MT && kit.AddRow(c, locales.AddApi()).Clicked() {
			s.editEndpoint(prefs.Endpoint{})
		}
		kit.Caption(c, shared.CloudSends(kind)).Margin(kit.Space(1), kit.Space(3), 0, kit.Space(11.5))
	})
}

// action is what is done with an engine at the end of its row: a model's download, a service's
// setup, Index-Translate's star.
func (s *Settings) action(c *ui.Context, e core.Engine, st models.Status) {
	k := s.core
	switch {
	case e.Model != nil:
		s.download(c, e, st)
	case e.ID == "cloud":
		if kit.Button(c, kit.Quiet, kit.Small, assets.Star, locales.StarOnGithub()).Clicked() {
			c.OpenURL(core.CloudRepo)
		}
	case e.Endpoint != nil:
		ep := *e.Endpoint
		if kit.IconButton(c, kit.Quiet, kit.Small, assets.Pencil, locales.Edit()).Clicked() {
			s.editEndpoint(ep)
		}
	case e.Kind == core.ASR && e.Cloud:
		// Set up, it is edited as an API is; else setting it up is what it waits for.
		var b ui.Element
		if k.ASRAPI(e.ID).Key {
			b = kit.IconButton(c, kit.Quiet, kit.Small, assets.Pencil, locales.Edit())
		} else {
			b = kit.Button(c, kit.Plain, kit.Small, assets.Pencil, locales.SetUp())
		}
		if b.Clicked() {
			s.edits++
			s.cloud.Open(s.core, e.ID, s.edits)
		}
	}
}

// asrAPI is a cloud recognition service as set up: its model and host.
func asrAPI(k *core.Core, id string) string {
	a := k.ASRAPI(id)
	spec, _ := asr.ByID(id)
	u, model := a.URL, a.Model
	if u == "" {
		u = spec.URL
	}
	if model == "" && len(spec.Models) > 0 {
		model = spec.Models[0]
	}
	return model + " · " + host(u)
}

// download is what is done with a model: download it, follow and cancel its download, or delete
// it once it is in.
func (s *Settings) download(c *ui.Context, e core.Engine, st models.Status) {
	k := s.core
	p := kit.Colors(c)
	if e.Model == nil {
		return
	}
	switch st.State {
	case models.Missing:
		label := locales.DownloadModel()
		switch {
		case st.Done > 0:
			label = locales.DownloadResume()
		case st.Err != nil:
			label = locales.Retry()
		}
		if kit.Button(c, kit.Strong, kit.Small, assets.Download, label).Clicked() {
			k.Download(e.ID)
		}
	case models.Downloading:
		c.After(250 * time.Millisecond) // the progress moves
		ui.Column(c).Width(kit.Space(30)).Gap(kit.Space(1)).Children(func() {
			kit.Progress(c, float64(st.Done)/float64(max(st.Total, 1)))
			ui.Text(c, locales.Downloading(shared.Size(st.Done), shared.Size(st.Total))).FontSize(kit.TextMicro).FontFeatures(kit.Tabular).TextColor(p.InkMuted).
				ContentTransition(ui.ContentTransition{Numeric: true})
		})
		if kit.IconButton(c, kit.Destructive(kit.Quiet), kit.Small, assets.Minus, locales.Cancel()).Clicked() {
			k.CancelDownload(e.ID)
		}
	case models.Verifying:
		kit.Caption(c, locales.Verifying())
	case models.Ready:
		if kit.IconButton(c, kit.Destructive(kit.Quiet), kit.Small, assets.Trash, locales.Delete()).Clicked() {
			s.ask(locales.AskDeleteModel(shared.EngineName(e)), locales.AskDeleteModelDetail(shared.Size(e.Model.Size)), func(c *ui.Context) {
				k.DeleteModel(e.ID)
				c.Toast(locales.ModelDeleted(shared.EngineName(e)))
			})
		}
	}
}

// describe says what an engine is, in a sentence.
func describe(e core.Engine) string {
	switch {
	case e.ID == "apple":
		return locales.EngineAppleDescription()
	case e.ID == "cloud":
		return locales.EngineCloudThanks()
	case e.Endpoint != nil:
		return locales.EndpointDescription(e.Endpoint.Model, host(e.Endpoint.URL))
	case e.ID == "sensevoice-small":
		return locales.EngineSenseVoiceDescription()
	case e.ID == core.QwenCloud:
		return locales.EngineQwenAsrDescription()
	case e.ID == core.OpenAI:
		return locales.EngineOpenAiDescription()
	}
	return ""
}

// watchModels says when a local model has come in, whether it is shown or not: in use now, if it
// was chosen.
func (s *Settings) watchModels(c *ui.Context) {
	k := s.core
	for _, e := range append(k.Engines(core.ASR), k.Engines(core.MT)...) {
		if e.Model == nil {
			continue
		}
		st := k.ModelStatus(e.ID).State
		if was, ok := s.models[e.ID]; ok && was != st && st == models.Ready && (was == models.Downloading || was == models.Verifying) {
			if k.InUse(e.Kind) == e.ID {
				c.Toast(locales.ModelReadyInUse(shared.EngineName(e)))
			} else {
				c.Toast(locales.ModelReady(shared.EngineName(e)))
			}
		}
		s.models[e.ID] = st
	}
}

func host(u string) string {
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return p.Host
	}
	return u
}
