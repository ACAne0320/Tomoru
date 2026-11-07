package settings

import (
	"cmp"
	"errors"
	"log"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// engines is the page of recognition and translation (docs/design-system.md): a card of each, its
// families on device and in the cloud listed on the left, the one shown on the right, its sizes
// or its service; translating pictures, apart; then translating ahead and the models' folder, a
// setting each. Only what is downloaded or set up is used, by its radio; a download only
// downloads.
func (s *Settings) engines(c *ui.Context) {
	k := s.core
	p := kit.Colors(c)
	ui.Text(c, locales.TabEngines()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
	s.kind(c, core.ASR)
	s.kind(c, core.MT)
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
	s.watchModels(c)
}

// enginesEdit is the page's state: the family shown of each kind, and the forms open in the
// detail.
type enginesEdit struct {
	shown     map[core.Kind]string // by core.Engine.Family; else the one in use's
	addingAPI bool                 // translation's detail adds an API
	api       endpointEdit         // the API added or edited there
	svc       [2]serviceEdit       // by core.Kind, the cloud service set up there
}

// show shows family of kind, closing the forms of the one shown before.
func (s *Settings) show(kind core.Kind, family string) {
	s.shown[kind] = family
	if kind == core.MT {
		s.addingAPI = false
		s.closeAPI()
	}
	if d := &s.svc[kind]; d.open {
		s.core.ForgetTest(d.test)
		*d = serviceEdit{}
	}
}

// pictures is the group of translating pictures: its engine, of those that read pictures, the
// cloud's and those on this computer, then the user's APIs; and its shortcut.
func (s *Settings) pictures(c *ui.Context) {
	k := s.core
	id := k.Preferences().ImageMT
	chosen, ok := k.Engine(core.MT, id)
	if !ok {
		chosen.ID = id
	}
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
		kit.SettingRow(c, 1, locales.ImageShortcutRow(), "", func() {
			kit.Control(c, func() {
				shared.ShortcutField(c, k, s.nav, core.PictureShortcut, locales.ImageShortcut(), &s.imageKeys)
			})
		})
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

// kind is the card of the engines of a kind: its families on device, then in the cloud, the one in
// use's radio filled, and beside them the one shown. Translation's list ends with adding an API.
func (s *Settings) kind(c *ui.Context, kind core.Kind) {
	k := s.core
	title := locales.EnginesAsr()
	if kind == core.MT {
		title = locales.EnginesMt()
	}
	fams := k.Families(kind)
	shown := slices.IndexFunc(fams, func(f []core.Engine) bool { return f[0].Family() == s.shown[kind] })
	if shown < 0 {
		shown = max(0, slices.IndexFunc(fams, func(f []core.Engine) bool { return s.inUse(f) }))
	}
	adding := kind == core.MT && s.addingAPI
	rows := make([]kit.Choice, len(fams))
	for i, f := range fams {
		rows[i] = s.choice(c, f, i == shown && !adding)
		if i == 0 || f[0].Cloud != fams[i-1][0].Cloud {
			rows[i].Head = locales.TagLocal()
			if f[0].Cloud {
				rows[i].Head = locales.TagCloud()
			}
		}
	}
	kit.Group(c.Key(kind), title, nil, kit.SplitCard, func() {
		kit.SplitList(c, func() {
			if online, _ := k.Network(); kind == core.MT && !online {
				kit.Warning(c, locales.OfflineNote()).Margin(kit.Space(2), kit.Space(3))
			}
			show, use := kit.ChoiceList(c, title, rows)
			if show >= 0 {
				s.show(kind, fams[show][0].Family())
			}
			if use >= 0 {
				if e, ok := k.Pick(fams[use]); ok {
					k.Use(kind, e.ID)
				}
			}
			if kind == core.MT && kit.AddRow(c, locales.AddApi()).Clicked() {
				s.show(kind, "")
				s.addingAPI = true
				s.editAPI(&s.api, prefs.Endpoint{})
			}
		})
		kit.SplitDetail(c, func() {
			switch {
			case adding:
				s.apiPane(c)
			case len(fams) > 0:
				s.detail(c, kind, fams[shown])
			}
		})
	})
}

// inUse tells whether a size of family f is in use.
func (s *Settings) inUse(f []core.Engine) bool {
	return slices.ContainsFunc(f, func(e core.Engine) bool { return e.ID == s.core.InUse(e.Kind) })
}

// choice is family f as a row of its list: its name and tag, and what it is at: in use, what keeps
// it from use, its download, or what of it is there.
func (s *Settings) choice(c *ui.Context, f []core.Engine, shown bool) kit.Choice {
	k := s.core
	name := shared.FamilyName(f)
	r := kit.Choice{Key: f[0].Family(), Shown: shown, Title: name, Use: locales.UseEngine(name), Progress: -1}
	_, usable := k.Pick(f)
	r.On, r.Off = s.inUse(f), !usable
	switch {
	case f[0].ID == "apple":
		r.Tag = locales.TagBuiltIn()
	case slices.ContainsFunc(f, k.Recommended):
		r.Tag, r.Accent = locales.TagRecommended(), true
	}
	sizes := len(f) > 1 && (f[0].Model != nil || f[0].Family() == "cloud")
	var loading *core.Engine
	var st models.Status
	for _, e := range f {
		if e.Model != nil && !k.ModelsMoving().Active {
			if st = k.ModelStatus(e.ID); st.State == models.Downloading {
				loading = &e
				break
			}
		}
	}
	switch {
	case r.On && sizes: // its radio says it is in use: the size, and another's download
		for _, e := range f {
			if e.ID == k.InUse(e.Kind) {
				r.Meta = sizeLabel(e)
			}
		}
		if loading != nil {
			r.Meta += " · " + locales.SizeDownloading(loading.Model.Label)
		}
	case !r.On && f[0].Kind == core.ASR && k.SpokenWith(f[0]) != k.Preferences().Src:
		r.Meta, r.Warn = locales.DoesntHear(shared.LangName(k.Preferences().Src)), true
	case loading != nil:
		r.Meta = locales.Downloading(shared.Size(st.Done), shared.Size(st.Total))
		if sizes {
			r.Meta = locales.SizeDownloading(loading.Model.Label) + " · " + r.Meta
		}
	case f[0].Cloud && !usable:
		r.Meta = locales.NeedsKey()
	case f[0].Model != nil && !usable:
		r.Meta = locales.NotDownloaded(shared.Size(f[0].Model.Size))
		if sizes {
			r.Meta = locales.SizesNotDownloaded(len(f))
		}
	case sizes && f[0].Model != nil:
		var have []string
		for _, e := range f {
			if k.ModelStatus(e.ID).State == models.Ready {
				have = append(have, e.Model.Label)
			}
		}
		r.Meta = locales.SizesDownloaded(len(f), strings.Join(have, locales.ListSeparator()))
	case f[0].Model != nil: // its one size, downloaded
		r.Meta = locales.Downloaded()
	default:
		r.Meta = cmp.Or(summary(k, f[0]), shared.FamilyNote(f))
	}
	if loading != nil && !shown {
		c.After(250 * time.Millisecond) // the progress moves
		r.Progress = float64(st.Done) / float64(max(st.Total, 1))
	}
	return r
}

// sizeLabel is the name of e's size: its model's, or its size of Index-Translate's cloud.
func sizeLabel(e core.Engine) string {
	if e.Model != nil {
		return e.Model.Label
	}
	size, _ := core.CloudSizeByID(e.ID)
	return size.Label
}

// head titles a part of a family's detail: as a section's title, but without the height a
// section's buttons need, close above what it titles.
func head(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).Ink).MaxLines(1).Margin(kit.Space(2), 0, 0)
}

// summary is a cloud service as set up, in a line: an API's model and host; a service's of the
// user's key, and that its key is kept. "" for one not set up, or of no settings.
func summary(k *core.Core, e core.Engine) string {
	switch {
	case e.Endpoint != nil:
		return e.Endpoint.Model + " · " + host(e.Endpoint.URL)
	case e.Cloud && k.API(e.ID).Key:
		a := k.API(e.ID)
		url, models := core.ServiceOf(e.ID)
		u, model := cmp.Or(a.URL, url), a.Model
		if model == "" && len(models) > 0 {
			model = models[0]
		}
		return model + " · " + host(u) + " · " + locales.KeySaved()
	}
	return ""
}

// detail is family f shown: its name, with where it is from or, an API of the user's, Delete, and
// what is worth knowing of it; then its sizes, with the languages a recognizer hears, or its
// service, set up in place.
func (s *Settings) detail(c *ui.Context, kind core.Kind, f []core.Engine) {
	k := s.core
	p := kit.Colors(c)
	inUse := s.inUse(f)
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Start).Children(func() { // a long name wraps beside it
		ui.Text(c, shared.FamilyName(f)).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(p.Ink).MaxLines(2).Grow(1)
		if ep := f[0].Endpoint; ep != nil {
			if kit.Button(c, kit.Destructive(kit.Quiet), kit.Small, assets.Trash, locales.Delete()).Clicked() {
				s.deleteAPI(c, *ep)
			}
		} else if label, url := home(f[0].Family()); url != "" {
			if kit.Button(c, kit.Quiet, kit.Small, assets.External, label).Clicked() {
				c.OpenURL(url)
			}
		}
	})
	if note := shared.FamilyNote(f); note != "" {
		kit.Caption(c, note).Margin(-kit.Space(2), 0, 0)
	}
	if src := k.Preferences().Src; kind == core.ASR && !inUse {
		if then := k.SpokenWith(f[0]); then != src {
			kit.Warning(c, locales.DoesntHearSwitch(shared.LangName(src), shared.LangName(then)))
		}
	}
	switch e := f[0]; {
	case e.Model != nil:
		label := locales.ModelSizes()
		if len(f) == 1 {
			label = locales.EndpointModel()
		}
		head(c, label)
		s.sizes(c, kind, f)
	case e.Family() == "cloud":
		head(c, locales.ModelSizes())
		s.cloudSizes(c, kind, f)
	case e.Cloud:
		s.service(c, e)
	default: // the system's: a row of its own, its radio as the others' say it is in use
		head(c, locales.EndpointModel())
		ui.Column(c).Children(func() { // in the pane's card: no card of its own
			if kit.Options(c, []kit.Option{{Key: e.ID, On: inUse, Title: locales.TagBuiltIn()}}) == 0 && !inUse {
				k.Use(kind, e.ID)
			}
		})
	}
	if kind == core.ASR {
		if hears := k.Hears(f[0].ID); len(hears) > 0 {
			head(c, locales.SupportedLanguages())
			names := make([]string, len(hears))
			for i, l := range hears {
				names[i] = shared.LangName(l)
			}
			kit.Caption(c, strings.Join(names, locales.ListSeparator()))
		}
	}
}

// sizes are the sizes of a model's family, each a row its radio uses once it is downloaded and
// this computer runs it; each with its download, cancelling it, or deleting it at its end.
func (s *Settings) sizes(c *ui.Context, kind core.Kind, f []core.Engine) {
	k := s.core
	_, metered := k.Network()
	moving := k.ModelsMoving().Active
	rows := make([]kit.Option, len(f))
	for i, e := range f {
		m := *e.Model
		st := k.ModelStatus(e.ID)
		o := kit.Option{Key: e.ID, On: e.ID == k.InUse(kind), Off: !k.Usable(e), Title: m.Label, Meta: shared.ModelNote(m, st, k.Runs(m))}
		if note := shared.SizeNote(m); note != "" {
			o.Meta = note + " · " + o.Meta
		}
		if k.Recommended(e) {
			o.Tag, o.Accent = locales.TagRecommended(), true
		}
		if st.State == models.Ready {
			o.Done = locales.Downloaded()
		}
		short := k.Shortfall(m)
		if short != (core.Shortfall{}) {
			o.Warn = true
			o.Meta = locales.TooLarge(int(short.Need), int(short.Has))
			if short.GPU {
				o.Meta = locales.TooLargeGpu(int(short.Need), int(short.Has))
			}
		}
		o.Action = func() { s.download(c, e, st, short == core.Shortfall{}) }
		var below []func()
		if st.State == models.Downloading && !moving {
			c.After(250 * time.Millisecond) // the progress moves
			below = append(below, func() {
				kit.OptionProgress(c, float64(st.Done)/float64(max(st.Total, 1)), locales.Downloading(shared.Size(st.Done), shared.Size(st.Total))).Padding(0, kit.QuietInset, 0, 0)
			})
		}
		if why := shared.DownloadError(st.Err); why != "" && st.State == models.Missing {
			below = append(below, func() { kit.Warning(c, why) })
		}
		if st.State == models.Missing && metered && short == (core.Shortfall{}) {
			below = append(below, func() { kit.Caption(c, locales.MeteredNote(shared.Size(m.Size-st.Done))) })
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
	ui.Column(c).Children(func() {
		if i := kit.Options(c, rows); i >= 0 && !rows[i].On {
			k.Use(kind, f[i].ID)
		}
	})
}

// cloudSizes are the sizes of Index-Translate's cloud, each a row its radio uses.
func (s *Settings) cloudSizes(c *ui.Context, kind core.Kind, f []core.Engine) {
	k := s.core
	rows := make([]kit.Option, len(f))
	for i, e := range f {
		rows[i] = kit.Option{Key: e.ID, On: e.ID == k.InUse(kind), Title: sizeLabel(e)}
		if k.Recommended(e) {
			rows[i].Tag, rows[i].Accent = locales.TagRecommended(), true
		}
	}
	ui.Column(c).Children(func() { // in the pane's card: no card of its own
		if i := kit.Options(c, rows); i >= 0 && !rows[i].On {
			k.Use(kind, f[i].ID)
		}
	})
}

// home is where a family of engines is from, as its button names it and the page it opens: an
// open model's repository, or a service's page of its models. "" for the system's and the user's.
func home(family string) (label, url string) {
	switch family {
	case "SenseVoice":
		return "GitHub", "https://github.com/QwenAudio/SenseVoice"
	case "Qwen3-ASR":
		return "GitHub", "https://github.com/QwenLM/Qwen3-ASR"
	case "Index-Translate", "cloud":
		return "GitHub", core.CloudRepo
	case "Hy-MT2":
		return "GitHub", "https://github.com/Tencent-Hunyuan/Hy-MT2"
	case core.QwenCloud:
		return locales.LinkModelStudio(), "https://help.aliyun.com/zh/model-studio/real-time-speech-recognition-user-guide"
	case core.OpenAI:
		return "OpenAI", "https://developers.openai.com/api/docs/guides/speech-to-text"
	case core.HyMTCloud:
		return locales.LinkTokenHub(), "https://console.cloud.tencent.com/tokenhub/models/detail?modelId=hy-mt2-plus&regionId=1"
	}
	return "", ""
}

// download is what is done with a model: download it where this computer runs it, follow and
// cancel its download, or delete it once it is in.
func (s *Settings) download(c *ui.Context, e core.Engine, st models.Status, runs bool) {
	k := s.core
	if k.ModelsMoving().Active {
		return
	}
	switch st.State {
	case models.Missing:
		if !runs {
			return
		}
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

// service is a cloud service: a service of the user's API key or an API of the user's, as set up,
// with Use while another is in use, and Edit; edited, its form in its place, a service's from the
// start until it is set up.
func (s *Settings) service(c *ui.Context, e core.Engine) {
	k := s.core
	usable := k.Usable(e)
	var editing bool
	var edit func()
	switch {
	case e.Endpoint != nil:
		editing = s.api.open && s.api.e.ID == e.Endpoint.ID
		ep := *e.Endpoint
		edit = func() { s.editAPI(&s.api, ep) }
	default: // a service of the user's API key
		d := &s.svc[e.Kind]
		if !usable && !(d.open && d.a.ID == e.ID) {
			d.Open(k, e.ID, s.next())
		}
		editing = d.open && d.a.ID == e.ID
		edit = func() { d.Open(k, e.ID, s.next()) }
	}
	switch {
	case !usable: // its key is all it needs: the form, under nothing but the note above
		s.serviceForm(c, &s.svc[e.Kind], false)
	case !editing:
		head(c, locales.Service())
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, summary(k, e)).FontSize(kit.TextBody).TextColor(kit.Colors(c).Ink).MaxLines(2).Grow(1)
			if e.ID != k.InUse(e.Kind) && kit.Button(c, kit.Plain, kit.Small, nil, locales.Use()).Clicked() {
				k.Use(e.Kind, e.ID)
			}
			if kit.Button(c, kit.Quiet, kit.Small, assets.Pencil, locales.Edit()).Clicked() {
				edit()
			}
		})
	case e.Endpoint != nil:
		head(c, locales.Service())
		s.apiForm(c, &s.api, func() { s.closeAPI() })
	default:
		head(c, locales.Service())
		s.serviceForm(c, &s.svc[e.Kind], true)
	}
}

// watchModels says when a local model has come in, whether it is shown or not: with a button that
// uses it, unless it is in use already, as the guide's choice is once it comes in.
func (s *Settings) watchModels(c *ui.Context) {
	k := s.core
	for _, e := range append(k.Engines(core.ASR), k.Engines(core.MT)...) {
		if e.Model == nil {
			continue
		}
		st := k.ModelStatus(e.ID).State
		if was, ok := s.models[e.ID]; ok && was != st && st == models.Ready && (was == models.Downloading || was == models.Verifying) {
			name, kind, id := shared.EngineName(e), e.Kind, e.ID
			switch {
			case k.InUse(kind) == id:
				c.Toast(locales.ModelReadyInUse(name))
			case k.Usable(e):
				c.ToastAction(locales.ModelDownloaded(name), locales.Use(), func() { k.Use(kind, id) })
			default:
				c.Toast(locales.ModelDownloaded(name))
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
