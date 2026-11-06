package settings

import (
	"errors"
	"log"
	"net/url"
	"time"

	"github.com/egoist/mygo"
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

// engines is the page of recognition and translation (docs/design-system.md): a group of each, on
// this computer or in the cloud, each side a list of its engines; translating pictures, apart; then
// translating ahead and the models' folder, a setting each.
func (s *Settings) engines(c *ui.Context) {
	k := s.core
	p := kit.Colors(c)
	ui.Text(c, locales.TabEngines()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
	kit.Groups(c, func() {
		s.kind(c, core.ASR)
		s.kind(c, core.MT)
	})
	s.pictures(c)
	kit.RowsCard(c).Children(func() {
		kit.SettingRow(c, 0, locales.DraftLabel(), locales.DraftDesc(), func() {
			draft := k.Preferences().Draft
			if kit.Switch(c, &draft, locales.DraftLabel()).Changed() {
				k.UpdatePrefs(func(p *prefs.Prefs) { p.Draft = draft })
			}
		})
		if dir := k.ModelsDir(); dir != "" {
			s.modelsFolder(c, dir)
		}
	})
	s.endpointDialog(c)
	shared.ASRCloudDialog(c, s.core, &s.cloud)
	s.watchModels(c)
}

// pictures is the group of translating pictures: its engine, of those that read pictures, the
// cloud's and those on this computer, then the user's APIs; and its shortcut. Under it, with an
// engine in the cloud, what the service receives.
func (s *Settings) pictures(c *ui.Context) {
	k := s.core
	id := k.Preferences().ImageMT
	chosen, ok := k.Engine(core.MT, id)
	if !ok {
		chosen.ID = id
	}
	ui.Column(c).Gap(kit.GroupGap).Children(func() {
		kit.Group(c, locales.ImageTitle(), nil, kit.RowsCard, func() {
			kit.SettingRow(c, 0, locales.ImageEngine(), "", func() {
				kit.Control(c, func() {
					kit.Picker(c, shared.EngineName(chosen), func(m *ui.Menu) {
						all := k.ImageEngines()
						for i, e := range all {
							if e.Endpoint != nil && i > 0 && all[i-1].Endpoint == nil {
								m.Separator()
							}
							if m.Item(shared.EngineName(e)).Checked(e.ID == id).Chosen() {
								k.SetImageEngine(e.ID)
							}
						}
					}).Grow(1).Shrink(1).MinWidth(0).Label(locales.ImageEngine())
				})
			})
			kit.SettingRow(c, 1, locales.ImageShortcutRow(), pictureShortcutDesc(k), func() {
				kit.Control(c, func() {
					shared.ShortcutField(c, k, s.nav, core.PictureShortcut, locales.ImageShortcut(), &s.imageKeys)
				})
			})
		})
		if chosen.Cloud {
			kit.Caption(c, locales.CloudSendsPicture()).Padding(0, kit.Space(1))
		}
	})
}

// modelsFolder is the folder the local models are in, a setting's row: shown, or another chosen,
// which they move to, the move followed under the path until it ends.
func (s *Settings) modelsFolder(c *ui.Context, dir string) {
	k := s.core
	mv := k.ModelsMoving()
	if s.modelsMoving && !mv.Active && mv.Err == nil {
		c.Toast(locales.ModelsMoved())
	}
	s.modelsMoving = mv.Active
	var under func()
	switch {
	case mv.Active:
		c.After(250 * time.Millisecond) // the progress moves
		under = func() {
			kit.OptionProgress(c, float64(mv.Done)/float64(max(mv.Total, 1)), locales.ModelsMoving(shared.Size(mv.Done), shared.Size(mv.Total)))
		}
	case mv.Err != nil:
		var space models.ErrNoSpace
		why := locales.ModelsMoveFailed(mv.Err.Error())
		if errors.As(mv.Err, &space) {
			why = locales.NoSpace(shared.Size(space.Need))
		}
		under = func() { kit.Warning(c, why) }
	}
	kit.SettingRowUnder(c, 1, locales.ModelsFolder(), dir, func() {
		if mv.Active {
			return
		}
		ui.Row(c).Gap(kit.Space(2)).Shrink(0).Children(func() {
			if kit.Button(c, kit.Plain, kit.Small, nil, shared.OnWindows(locales.ShowInFinder(), locales.ShowInExplorer())).Clicked() {
				mygo.Shell.ShowItemInFolder(dir)
			}
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.ModelsFolderChange()).Clicked() {
				// The dialog waits for the user: not on the main thread, which draws.
				go func() {
					dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{DefaultPath: dir, Directory: true, CreateDirectories: true})
					if err != nil {
						log.Println("models folder:", err)
					} else if len(dirs) > 0 {
						mygo.RunOnMain(func() { k.MoveModels(dirs[0]) })
					}
				}()
			}
		})
	}, under)
}

// kind is the group of the engines of a kind: its title, this computer or the cloud to look at,
// and the engines there, the one in use marked among them.
func (s *Settings) kind(c *ui.Context, kind core.Kind) {
	k := s.core
	title := locales.EnginesAsr()
	if kind == core.MT {
		title = locales.EnginesMt()
	}
	all := k.Engines(kind)
	chosen, _ := k.Engine(kind, k.Chosen(kind))
	cloud, ok := s.sides[kind]
	if !ok {
		cloud = chosen.Cloud
	}
	where := 0
	if cloud {
		where = 1
	}
	kit.Group(c.Key(kind), title, func() {
		if kit.Segmented(c, &where, locales.TagLocal(), locales.TagCloud()).Label(locales.EngineWhere()).Changed() {
			s.sides[kind] = where == 1
		}
	}, kit.OptionsCard, func() {
		if online, _ := k.Network(); kind == core.MT && !online {
			kit.Warning(c, locales.OfflineNote()).Margin(kit.Space(3), kit.Space(3), 0)
		}
		here := side(all, cloud)
		switch {
		case len(here) == 0: // only on a computer without the system's recognition and models
			kit.Caption(c, locales.AsrLocalNone()).Margin(kit.Space(3))
		default:
			s.options(c, kind, here, chosen)
		}
	}).Grow(1).Basis(320)
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
// its end and what goes under it: choosing one uses it, once a model is downloaded or a service
// set up, which choosing it starts. The cloud's translation ends with adding an API of one's own.
func (s *Settings) options(c *ui.Context, kind core.Kind, here []core.Engine, chosen core.Engine) {
	k := s.core
	_, metered := k.Network()
	moving := k.ModelsMoving().Active
	rows := make([]kit.Option, len(here))
	for i, e := range here {
		on := e.ID == chosen.ID
		var st models.Status
		o := kit.Option{Key: e.ID, On: on, Title: shared.EngineName(e), Meta: describe(e)}
		switch {
		case e.Model != nil:
			// A model's size leads its description, and what it needs to run ends it.
			st = k.ModelStatus(e.ID)
			o.Meta = shared.ModelNote(*e.Model, st, o.Meta)
			if e.ID == "local-9b" || k.Recommended(e) {
				o.Tag, o.Accent = locales.TagRecommended(), true
			}
		case e.ID == "apple":
			o.Tag = locales.TagBuiltIn()
		case e.ID == "cloud":
			o.Tag, o.Accent = locales.TagRecommended(), true
		case e.Endpoint != nil:
			o.Meta = e.Endpoint.Model + " · " + host(e.Endpoint.URL)
		case e.Kind == core.ASR && e.Cloud && k.ASRAPI(e.ID).Key:
			o.Meta = asrAPI(k, e.ID)
		}
		o.Action = func() { s.action(c, e, st) }
		// Under its name: a download's progress, then what to heed, what it costs, and what is
		// used until it is ready.
		var below []func()
		if m := e.Model; m != nil {
			if st.State == models.Downloading && !moving {
				c.After(250 * time.Millisecond) // the progress moves
				below = append(below, func() {
					kit.OptionProgress(c, float64(st.Done)/float64(max(st.Total, 1)), locales.Downloading(shared.Size(st.Done), shared.Size(st.Total))).Padding(0, kit.QuietInset, 0, 0)
				})
			}
			if mem := models.Memory(); m.Memory != 0 && mem != 0 && mem < m.Memory {
				below = append(below, func() {
					kit.Warning(c, shared.OnWindows(locales.MemoryWarning(int(mem>>30)), locales.MemoryWarningWin(int(mem>>30))))
				})
			}
			if why := shared.DownloadError(st.Err); why != "" && st.State == models.Missing {
				below = append(below, func() { kit.Warning(c, why) })
			}
			if st.State == models.Missing && metered {
				below = append(below, func() { kit.Caption(c, locales.MeteredNote(shared.Size(m.Size-st.Done))) })
			}
		}
		if on && e.ID != k.InUse(kind) {
			current, _ := k.Engine(kind, k.InUse(kind))
			waiting := locales.EngineWaiting(shared.EngineName(current))
			if e.Cloud {
				waiting = locales.EngineWaitingSetup(shared.EngineName(current))
			}
			below = append(below, func() { kit.Caption(c, waiting) })
		}
		if len(below) > 0 {
			o.Below = func() {
				for _, fn := range below {
					fn()
				}
			}
		}
		rows[i] = o
	}
	if i := kit.Options(c, rows); i >= 0 && !rows[i].On {
		e := here[i]
		k.Choose(kind, e.ID)
		switch {
		case e.Model != nil && k.ModelStatus(e.ID).State == models.Missing:
			k.Download(e.ID)
		case e.Cloud && !k.Usable(e):
			s.edits++
			s.cloud.Open(s.core, e.ID, s.edits) // a service to set up: set it up now
		}
	}
	if !here[0].Cloud {
		return
	}
	if kind == core.MT && kit.AddRow(c, locales.AddApi()).Clicked() {
		s.editEndpoint(prefs.Endpoint{})
	}
	kit.Caption(c, shared.CloudSends(kind)).Margin(kit.Space(2), kit.Space(3), kit.Space(3))
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
	if e.Model == nil || k.ModelsMoving().Active {
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
		if kit.Button(c, kit.Plain, kit.Small, assets.Download, label).Clicked() {
			k.Download(e.ID)
		}
	case models.Downloading:
		if kit.CancelButton(c, locales.Cancel()).Clicked() {
			k.CancelDownload(e.ID)
		}
	case models.Verifying:
		kit.Caption(c, locales.Verifying())
	case models.Ready:
		// It can't be undone: by its button, or in a dialog if it is in use, as another takes over.
		del := kit.DeleteButton(c, kit.Small, locales.Delete())
		question, size := locales.AskDeleteModel(shared.EngineName(e)), shared.Size(e.Model.Size)
		remove := func(c *ui.Context) {
			k.DeleteModel(e.ID)
			c.Toast(locales.ModelDeleted(shared.EngineName(e)))
		}
		if del.Clicked() {
			if k.InUse(e.Kind) == e.ID {
				s.ask(question, s.takingOver(e.Kind, size), remove)
			} else {
				s.deleting = e.ID
			}
		}
		open := s.deleting == e.ID
		if kit.Popconfirm(c, del, &open, question, locales.AskDeleteModelDetail(size), locales.Delete()) {
			remove(c)
		}
		if !open && s.deleting == e.ID {
			s.deleting = ""
		}
	}
}

// takingOver says, under the question before deleting a local model of kind k in use, what takes
// over from it, and how much using it again downloads.
func (s *Settings) takingOver(k core.Kind, size string) string {
	next, ok := s.core.Engine(k, core.Successor(k))
	if !ok || next.ID == "" {
		return locales.AskDeleteModelLast(size)
	}
	return locales.AskDeleteModelInUse(shared.EngineName(next), size)
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
