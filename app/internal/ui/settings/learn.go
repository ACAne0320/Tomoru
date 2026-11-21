package settings

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

type learnEdit struct {
	open     bool
	paths    []string
	scene    string
	endpoint string
	skipped  map[string]bool
	target   learnTarget
	took     int
	tookTo   string
	done     bool
	discard  kit.DiscardGuard
}

func (d *learnEdit) unsaved(l core.Learning) bool {
	return !d.done && l.Finished && l.Err != core.ErrNoChatEndpoint && len(l.Suggestions) > 0
}

type learnTarget struct {
	scene    string
	newScene bool
}

const newSceneChoice = "+new"

func (t learnTarget) choice() string {
	if t.newScene {
		return newSceneChoice
	}
	return t.scene
}

func targetOf(choice string) learnTarget {
	if choice == newSceneChoice {
		return learnTarget{newScene: true}
	}
	return learnTarget{scene: choice}
}

func (s *Settings) learnFrom(paths []string, id string) {
	k := s.core
	k.ForgetLearning()
	k.ForgetRetranslating()
	if _, ok := k.SceneByID(id); !ok {
		id = ""
	}
	e := learnEdit{open: true, paths: slices.Clone(paths), scene: id, target: learnTarget{scene: id, newScene: id == ""}, skipped: map[string]bool{}}
	eps := k.ChatEndpoints()
	if i := slices.IndexFunc(eps, func(e prefs.Endpoint) bool { return e.ID == k.Preferences().Learn }); i >= 0 {
		e.endpoint = eps[i].ID
	} else if len(eps) > 0 {
		e.endpoint = eps[0].ID
	}
	s.learn = e
}

func (s *Settings) learnDialog(c *ui.Context) {
	k, d := s.core, &s.learn
	if !d.open || s.learnAPI.open {
		return
	}
	dialog := kit.Dialog(c, &d.open, locales.LearnNames(), "", func() {
		l := k.Learning
		switch {
		case d.done:
			s.learnTaken(c)
		case l.Running:
			kit.Caption(c, locales.LearnProgress(l.Done, l.Total))
			kit.Progress(c, float64(l.Done)/float64(max(1, l.Total)))
			kit.DialogActions(c, func() {
				ui.Spacer(c)
				if kit.Button(c, kit.Destructive(kit.Plain), kit.Small, nil, locales.Cancel()).Clicked() {
					k.CancelLearning()
				}
			})
		case l.Finished && l.Err != core.ErrNoChatEndpoint:
			s.learnFound(c)
		default:
			s.learnSetup(c)
		}
	})
	d.discard.Guard(dialog, &d.open, d.unsaved(k.Learning))
	switch {
	case kit.StayOpen(dialog, &d.open, k.Learning.Running):
		k.CancelLearning()
	case kit.StayOpen(dialog, &d.open, k.Retranslating.Running):
		k.CancelRetranslating()
	}
	if !d.open {
		k.ForgetLearning()
		k.ForgetRetranslating()
	}
}

func (s *Settings) learnSetup(c *ui.Context) {
	k, d := s.core, &s.learn
	eps := k.ChatEndpoints()
	if len(eps) == 0 {
		kit.Caption(c, locales.LearnNoEndpoint())
		kit.DialogActions(c, func() {
			ui.Spacer(c)
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
				d.open = false
			}
			if kit.Button(c, kit.Primary, kit.Small, assets.Plus, locales.AddEndpoint()).Clicked() {
				s.editAPI(&s.learnAPI, prefs.Endpoint{})
				s.learnAPI.saved = func(id string) { d.endpoint = id }
			}
		})
		return
	}
	if !slices.ContainsFunc(eps, func(e prefs.Endpoint) bool { return e.ID == d.endpoint }) {
		d.endpoint = eps[0].ID
	}
	ids := make([]string, len(eps))
	for i, e := range eps {
		ids[i] = e.ID
	}
	name := func(id string) string {
		preferences := k.Preferences()
		e, _ := preferences.Endpoint(id)
		return e.Name
	}
	kit.SettingRow(c, 0, locales.LearnEndpoint(), "", func() {
		kit.Picker(c, name(d.endpoint), kit.Choices(d.endpoint, [][]string{ids}, name, func(id string) { d.endpoint = id })).Label(locales.LearnEndpoint())
	})
	kit.Caption(c, locales.LearnSends(name(d.endpoint)))
	kit.DialogActions(c, func() {
		ui.Spacer(c)
		if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			d.open = false
		}
		if kit.Button(c, kit.Primary, kit.Small, assets.Sparkles, locales.LearnStart()).Clicked() {
			k.LearnFrom(d.paths, d.scene, d.endpoint)
		}
	})
}

func (s *Settings) learnFound(c *ui.Context) {
	k, d := s.core, &s.learn
	p := kit.Colors(c)
	l := k.Learning
	if l.Failed > 0 && l.Err != nil {
		kit.Warning(c, locales.LearnFailed(l.Failed, l.Err.Error())).MaxLines(3)
	} else if l.Err != nil {
		kit.Warning(c, l.Err.Error()).MaxLines(3)
	}
	if len(l.Suggestions) == 0 {
		kit.Caption(c, locales.LearnNone())
		kit.DialogActions(c, func() {
			ui.Spacer(c)
			if kit.Button(c, kit.Primary, kit.Small, nil, locales.Done()).Clicked() {
				d.open = false
			}
		})
		return
	}
	ui.Scroll(c).MaxHeight(320).Children(func() {
		ui.Column(c).Radius(kit.RadiusField).Border(1, p.Line).Clip().Children(func() {
			for i, sg := range l.Suggestions {
				col := ui.Column(c.Key(sg.Src)).Gap(kit.Space(1)).Padding(kit.Space(3), kit.Space(4))
				if i > 0 {
					col = col.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
				}
				col.Children(func() {
					take := !d.skipped[sg.Src]
					if ui.Checkbox(c, &take, sg.Src+" → "+sg.Tgt).FontWeight(kit.WeightBold).Changed() {
						d.skipped[sg.Src] = !take
					}
					ui.Column(c).Padding(0, 0, 0, kit.Space(6)).Gap(kit.Space(1)).Children(func() {
						var more []string
						if len(sg.Heard) > 0 {
							more = append(more, locales.TermHeard()+": "+strings.Join(sg.Heard, ", "))
						}
						more = append(more, locales.HistoryLines(sg.Count))
						kit.Caption(c, strings.Join(more, " · "))
						kit.Caption(c, "「"+sg.Example+"」").MaxLines(2)
					})
				})
			}
		})
	})
	ids := []string{}
	for _, sc := range k.SceneList() {
		ids = append(ids, sc.ID)
	}
	ids = append(ids, newSceneChoice)
	name := func(id string) string {
		if id == newSceneChoice {
			return locales.NewScene()
		}
		sc, _ := k.SceneByID(id)
		return shared.SceneName(*sc)
	}
	if _, ok := k.SceneByID(d.target.scene); !ok && !d.target.newScene {
		d.target = learnTarget{}
	}
	kit.SettingRow(c, 0, locales.LearnAddTo(), "", func() {
		kit.Picker(c, name(d.target.choice()), kit.Choices(d.target.choice(), [][]string{ids}, name, func(id string) { d.target = targetOf(id) })).Label(locales.LearnAddTo())
	})
	if d.discard.Confirming {
		d.discard.Actions(c, &d.open)
		return
	}
	taken := make([]bool, len(l.Suggestions))
	for i, sg := range l.Suggestions {
		taken[i] = !d.skipped[sg.Src]
	}
	some := slices.Contains(taken, true)
	kit.DialogActions(c, func() {
		ui.Spacer(c)
		if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			d.open = false
		}
		if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.AddTerm()), !some).Clicked() {
			id := d.target.scene
			if d.target.newScene {
				id = k.AddScene("")
			}
			n, undo := k.TakeLearned(id, taken)
			sc, _ := k.SceneByID(id)
			d.took, d.tookTo, d.done = n, id, true
			c.ToastAction(locales.LearnTook(n, shared.SceneName(*sc)), locales.Undo(), undo)
		}
	})
}

func (s *Settings) learnTaken(c *ui.Context) {
	k, d := s.core, &s.learn
	p := kit.Colors(c)
	name := locales.SceneGeneral()
	if sc, ok := k.SceneByID(d.tookTo); ok {
		name = shared.SceneName(*sc)
	}
	ui.Text(c, locales.LearnTook(d.took, name)).FontSize(kit.TextBody).TextColor(p.Ink)
	r := k.Retranslating
	switch {
	case r.Running:
		kit.Caption(c, locales.Retranslating(r.Done, r.Total))
		kit.Progress(c, float64(r.Done)/float64(max(1, r.Total)))
	case r.Finished && r.Err != nil:
		kit.Warning(c, r.Err.Error()).MaxLines(3)
	case r.Finished:
		kit.Result(c, true, locales.Retranslated(r.Done), "")
	}
	kit.DialogActions(c, func() {
		ui.Spacer(c)
		if !r.Running && !r.Finished && d.took > 0 && kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.Retranslate()).Clicked() {
			k.Retranslate(d.paths, d.tookTo)
		}
		if r.Running && kit.Button(c, kit.Destructive(kit.Plain), kit.Small, nil, locales.Cancel()).Clicked() {
			k.CancelRetranslating()
		}
		if kit.Button(c, kit.Primary, kit.Small, nil, locales.Done()).Clicked() {
			d.open = false
		}
	})
}
