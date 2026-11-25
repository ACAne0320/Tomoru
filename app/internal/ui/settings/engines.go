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
	"tomoru/internal/lang"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

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
}

type enginesEdit struct {
	shown     map[core.Kind]string
	addingAPI bool
	api       endpointEdit
	svc       map[core.Kind]*serviceEdit
}

func (s *Settings) serviceEdit(kind core.Kind) *serviceEdit {
	if s.svc == nil {
		s.svc = map[core.Kind]*serviceEdit{}
	}
	if s.svc[kind] == nil {
		s.svc[kind] = &serviceEdit{}
	}
	return s.svc[kind]
}

func (s *Settings) shownFamily(kind core.Kind, fams [][]core.Engine) int {
	if i := slices.IndexFunc(fams, func(f []core.Engine) bool { return f[0].Family() == s.shown[kind] }); i >= 0 {
		return i
	}
	return max(0, slices.IndexFunc(fams, func(f []core.Engine) bool { return s.inUse(f) }))
}

const progressRedraw = 250 * time.Millisecond

func (s *Settings) show(kind core.Kind, family string) {
	s.shown[kind] = family
	if kind == core.MT {
		s.addingAPI = false
		s.closeAPI()
	}
	if d := s.serviceEdit(kind); d.open {
		s.core.ForgetTest(d.test)
		*d = serviceEdit{}
	}
}

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
					for _, cloud := range []bool{true, false} {
						head := locales.TagLocal()
						if cloud {
							head = locales.TagCloud()
						}
						for _, e := range all {
							if e.Cloud != cloud {
								continue
							}
							if head != "" {
								if !cloud {
									m.Separator()
								}
								m.Item(head).Disabled(true)
								head = ""
							}
							name := shared.EngineName(e)
							if size, ok := core.CloudSizeByID(e.ID); ok {
								name = "Index-Translate " + size.Label
							}
							if m.Item(name).Checked(e.ID == id).Chosen() {
								k.SetImageEngine(e.ID)
							}
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

func (s *Settings) modelsFolder(c *ui.Context, dir string) {
	k := s.core
	mv := k.ModelsMoving()
	var under func()
	switch {
	case mv.Active:
		c.After(progressRedraw)
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
			if kit.Button(c, kit.Plain, kit.Small, nil, shared.OnEach(locales.ShowInFinder(), locales.ShowInExplorer(), locales.ShowInFolder())).Clicked() {
				mygo.Shell.ShowItemInFolder(dir)
			}
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.ModelsFolderChange()).Clicked() {
				opts := mygo.OpenDialogOptions{Parent: mygo.FocusedWindow(), DefaultPath: dir, Directory: true, CreateDirectories: true}
				go func() {
					dirs, err := mygo.Dialog.Open(opts)
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

func (s *Settings) kind(c *ui.Context, kind core.Kind) {
	k := s.core
	title := locales.EnginesAsr()
	if kind == core.MT {
		title = locales.EnginesMt()
	}
	fams := k.Families(kind)
	shown := s.shownFamily(kind, fams)
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

func (s *Settings) inUse(f []core.Engine) bool {
	return slices.ContainsFunc(f, func(e core.Engine) bool { return e.ID == s.core.InUse(e.Kind) })
}

func (s *Settings) choice(c *ui.Context, f []core.Engine, shown bool) kit.Choice {
	k := s.core
	name := shared.FamilyName(f)
	r := kit.Choice{Key: f[0].Family(), Shown: shown, Title: name, Use: locales.UseEngine(name)}
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
	case r.On && sizes:
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
		if k.Preferences().Src == lang.Auto {
			r.Meta = locales.CantDetect()
		}
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
	case f[0].Model != nil:
		r.Meta = locales.Downloaded()
	default:
		r.Meta = cmp.Or(summary(k, f[0]), shared.FamilyNote(f))
	}
	if loading != nil && !shown {
		c.After(progressRedraw)
		r.Downloading, r.Progress = true, float64(st.Done)/float64(max(st.Total, 1))
	}
	return r
}

func sizeLabel(e core.Engine) string {
	if e.Model != nil {
		return e.Model.Label
	}
	size, _ := core.CloudSizeByID(e.ID)
	return size.Label
}

func head(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).Ink).MaxLines(1).Margin(kit.Space(2), 0, 0)
}

func summary(k *core.Core, e core.Engine) string {
	switch {
	case e.Endpoint != nil:
		return e.Endpoint.Model + " · " + host(e.Endpoint.URL)
	case e.Cloud && k.API(e.ID).Key:
		a := k.API(e.ID)
		info := core.ServiceOf(e.ID)
		u, model := cmp.Or(a.URL, info.URL), cmp.Or(a.Model, info.Default())
		return model + " · " + host(u) + " · " + locales.KeySaved()
	}
	return ""
}

func (s *Settings) detail(c *ui.Context, kind core.Kind, f []core.Engine) {
	k := s.core
	p := kit.Colors(c)
	inUse := s.inUse(f)
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Start).Children(func() {
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
		switch then := k.SpokenWith(f[0]); {
		case then == src:
		case src == lang.Auto:
			kit.Warning(c, locales.CantDetectSwitch(shared.LangName(then)))
		default:
			kit.Warning(c, locales.DoesntHearSwitch(shared.LangName(src), shared.LangName(then)))
		}
	}
	switch e := f[0]; e.Source() {
	case core.SourceLocal:
		label := locales.ModelSizes()
		if len(f) == 1 {
			label = locales.EndpointModel()
		}
		head(c, label)
		s.sizes(c, kind, f)
	case core.SourceIndexCloud:
		head(c, locales.ModelSizes())
		s.cloudSizes(c, kind, f)
	case core.SourceService, core.SourceUserAPI:
		s.service(c, e)
	case core.SourceSystem:
		head(c, locales.EndpointModel())
		ui.Column(c).Children(func() {
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
			o.Meta = locales.TooLarge(int(short.NeedGB), int(short.HasGB))
			if short.GPU {
				o.Meta = locales.TooLargeGpu(int(short.NeedGB), int(short.HasGB))
			}
		}
		o.Action = func() { s.download(c, e, st, short == core.Shortfall{}) }
		var below []func()
		if st.State == models.Downloading && !moving {
			c.After(progressRedraw)
			below = append(below, func() {
				kit.OptionProgress(c, float64(st.Done)/float64(max(st.Total, 1)), locales.Downloading(shared.Size(st.Done), shared.Size(st.Total))).Padding(0, kit.ProgressEnd(true), 0, 0)
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

func (s *Settings) cloudSizes(c *ui.Context, kind core.Kind, f []core.Engine) {
	k := s.core
	rows := make([]kit.Option, len(f))
	for i, e := range f {
		rows[i] = kit.Option{Key: e.ID, On: e.ID == k.InUse(kind), Title: sizeLabel(e)}
		if k.Recommended(e) {
			rows[i].Tag, rows[i].Accent = locales.TagRecommended(), true
		}
	}
	ui.Column(c).Children(func() {
		if i := kit.Options(c, rows); i >= 0 && !rows[i].On {
			k.Use(kind, f[i].ID)
		}
	})
}

func home(family string) (label, url string) {
	switch family {
	case "SenseVoice":
		return "GitHub", "https://github.com/QwenAudio/SenseVoice"
	case "Qwen3-ASR":
		return "GitHub", "https://github.com/QwenLM/Qwen3-ASR"
	case "Index-Translate", "cloud":
		return "GitHub", core.CloudRepo
	case "Hy-MT3":
		return "GitHub", "https://github.com/Tencent-Hunyuan/Hy-MT3"
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
		del := kit.DeleteButton(c, kit.Small, locales.Delete())
		question, detail := locales.AskDeleteModel(shared.EngineName(e)), locales.AskDeleteModelDetail(shared.Size(e.Model.Size))
		if e.Model.Legacy {
			detail = locales.AskDeleteLegacyModel()
		}
		remove := func(c *ui.Context) {
			k.DeleteModel(e.ID)
			c.Toast(locales.ModelDeleted(shared.EngineName(e)))
		}
		if del.Clicked() {
			if k.InUse(e.Kind) == e.ID {
				s.ask(question, s.takingOver(e.Kind, *e.Model), remove)
			} else {
				s.confirmingDeleteModel = e.ID
			}
		}
		open := s.confirmingDeleteModel == e.ID
		if kit.Popconfirm(c, del, &open, question, detail, locales.Delete()) {
			remove(c)
		}
		if !open && s.confirmingDeleteModel == e.ID {
			s.confirmingDeleteModel = ""
		}
	}
}

func (s *Settings) takingOver(k core.Kind, m models.Model) string {
	size := shared.Size(m.Size)
	next, ok := s.core.Engine(k, core.Successor(k))
	switch {
	case !ok || next.ID == "":
		return locales.AskDeleteModelLast(size)
	case m.Legacy:
		return locales.AskDeleteLegacyModelInUse(shared.EngineName(next))
	}
	return locales.AskDeleteModelInUse(shared.EngineName(next), size)
}

func (s *Settings) service(c *ui.Context, e core.Engine) {
	k := s.core
	usable := k.Usable(e)
	var editing bool
	var edit func()
	d := s.serviceEdit(e.Kind)
	switch e.Source() {
	case core.SourceUserAPI:
		editing = s.api.open && s.api.e.ID == e.Endpoint.ID
		ep := *e.Endpoint
		edit = func() { s.editAPI(&s.api, ep) }
	case core.SourceService:
		if !usable && !(d.open && d.service.ID == e.ID) {
			d.Open(k, e.ID)
		}
		editing = d.open && d.service.ID == e.ID
		edit = func() { d.Open(k, e.ID) }
	}
	switch {
	case !usable:
		s.serviceForm(c, d, false)
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
	case e.Source() == core.SourceUserAPI:
		head(c, locales.Service())
		s.apiForm(c, &s.api, func() { s.closeAPI() })
	default:
		head(c, locales.Service())
		s.serviceForm(c, d, true)
	}
}

func host(u string) string {
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return p.Host
	}
	return u
}
