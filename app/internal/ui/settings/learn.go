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

// The dialog that learns names from subtitles kept: the API to ask, what it found, for the user
// to take into a scene or not, and translating them again with what was taken.

// learnEdit is the dialog's state while it shows.
type learnEdit struct {
	open     bool
	paths    []string // the sessions
	scene    string   // theirs
	endpoint string
	taken    []bool // of the suggestions, by index
	target   string // the scene they go to; newTarget for a new one
	took     int    // how many were taken, once they were
	tookTo   string // the scene they went to
	done     bool   // they were taken
	// asking asks, in place of the dialog's buttons, before the names found are thrown away.
	asking bool
}

// unsaved tells whether names were found, as l has them, and not taken yet.
func (d *learnEdit) unsaved(l core.Learning) bool {
	return !d.done && l.Finished && l.Err != core.ErrNoChatEndpoint && len(l.Suggestions) > 0
}

// newTarget is the menu's choice of a new scene for the names learned.
const newTarget = "+new"

// learnFrom opens the dialog for the sessions at paths, of scene id.
func (s *Settings) learnFrom(paths []string, id string) {
	k := s.core
	k.ForgetLearning()
	k.ForgetRetranslating()
	if _, ok := k.SceneByID(id); !ok {
		id = "" // its scene was deleted: the names go to the general one, or where the user says
	}
	e := learnEdit{open: true, paths: slices.Clone(paths), scene: id, target: id}
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
	if !d.open || s.edit.open { // adding an API shows its own dialog first
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
	unsaved := d.unsaved(k.Learning) // before stopping, which keeps what was found
	// Escape or a click outside while it works stops it, as its Cancel does, and the dialog stays.
	if dialog.Dismissed() {
		switch {
		case k.Learning.Running:
			d.open = true
			k.CancelLearning()
		case k.Retranslating.Running:
			d.open = true
			k.CancelRetranslating()
		}
	}
	kit.Guard(dialog, &d.open, &d.asking, unsaved)
	if !d.open {
		k.ForgetLearning()
		k.ForgetRetranslating()
	}
}

// learnSetup chooses the API to ask, or says to add one.
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
				s.editEndpoint(prefs.Endpoint{})
			}
		})
		return
	}
	if !slices.ContainsFunc(eps, func(e prefs.Endpoint) bool { return e.ID == d.endpoint }) {
		d.endpoint = eps[len(eps)-1].ID // one just added
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
		kit.Picker(c, name(d.endpoint), kit.Choices(d.endpoint, ids, name, func(id string) { d.endpoint = id })).Label(locales.LearnEndpoint())
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

// learnFound lists what was found, each taken at first, and the scene to add them to.
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
	for len(d.taken) < len(l.Suggestions) {
		d.taken = append(d.taken, true)
	}
	ui.Scroll(c).MaxHeight(320).Children(func() {
		ui.Column(c).Radius(kit.RadiusField).Border(1, p.Line).Clip().Children(func() {
			for i, sg := range l.Suggestions {
				col := ui.Column(c.Key(sg.Src)).Gap(kit.Space(1)).Padding(kit.Space(3), kit.Space(3.5))
				if i > 0 {
					col = col.BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
				}
				col.Children(func() {
					ui.Checkbox(c, &d.taken[i], sg.Src+" → "+sg.Tgt).FontWeight(kit.WeightBold)
					ui.Column(c).Padding(0, 0, 0, kit.Space(6)).Gap(kit.Space(0.5)).Children(func() {
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
	ids = append(ids, newTarget)
	name := func(id string) string {
		if id == newTarget {
			return locales.NewScene()
		}
		sc, _ := k.SceneByID(id)
		return shared.SceneName(*sc)
	}
	if _, ok := k.SceneByID(d.target); !ok && d.target != newTarget {
		d.target = ""
	}
	kit.SettingRow(c, 0, locales.LearnAddTo(), "", func() {
		kit.Picker(c, name(d.target), kit.Choices(d.target, ids, name, func(id string) { d.target = id })).Label(locales.LearnAddTo())
	})
	if d.asking {
		kit.Discard(c, &d.open, &d.asking)
		return
	}
	some := slices.Contains(d.taken[:len(l.Suggestions)], true)
	kit.DialogActions(c, func() {
		ui.Spacer(c)
		if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			d.open = false
		}
		if kit.Off(kit.Button(c, kit.Primary, kit.Small, nil, locales.AddTerm()), !some).Clicked() {
			id := d.target
			if id == newTarget {
				id = k.AddScene("")
			}
			n, undo := k.TakeLearned(id, d.taken)
			sc, _ := k.SceneByID(id)
			d.took, d.tookTo, d.done = n, id, true
			c.ToastAction(locales.LearnTook(n, shared.SceneName(*sc)), locales.Undo(), undo)
		}
	})
}

// learnTaken says what was taken, and offers to translate the sessions again with it.
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
		if !r.Running && !r.Finished && d.took > 0 && kit.Button(c, kit.Soft, kit.Small, assets.Reset, locales.Retranslate()).Clicked() {
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
