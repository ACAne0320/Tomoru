package settings

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/scene"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// The translation scenes' page (docs/design-system.md): the scenes, and the one chosen beside them,
// its name, languages and description, and its glossary, a table edited in place.

// sceneEdit is the page's state: the scene shown, the name being added to its glossary, and the
// ways a name was misheard as being typed.
type sceneEdit struct {
	id       bool // the page chose a scene to show
	scene    string
	adding   bool // the row adding a name shows
	src, tgt string
	heard    string // separated by commas
	// heardAt is the name whose misheard ways are typed, heardText what is typed: as the user
	// types a separator, the name has no way more yet.
	heardAt, heardText string
}

func (s *Settings) scenes(c *ui.Context) {
	k, e := s.core, &s.sceneEdit
	// What adds or deletes a scene or a name waits for the page to be built: the fields bound to
	// them would be left on a list that moved.
	var then func()
	defer func() {
		if then != nil {
			then()
		}
	}()
	if _, ok := k.SceneByID(e.scene); !ok || !e.id {
		e.scene, e.id = k.Preferences().Scene, true
	}
	p := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.TabScenes()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink).Grow(1).MaxLines(1)
		if kit.Button(c, kit.Plain, kit.Medium, assets.Plus, locales.NewScene()).Clicked() {
			then = func() { e.scene, e.adding = k.AddScene(""), false }
		}
	})
	ui.Row(c).Gap(kit.CardGap).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).MaxWidth(280).Gap(kit.CardGap).Children(func() {
			list := k.SceneList()
			items := make([]kit.ListItem, len(list))
			for i, sc := range list {
				src, tgt := sc.Src, sc.Tgt
				inUse := sc.ID == k.Preferences().Scene
				if inUse {
					src, tgt = k.Preferences().Src, k.Preferences().Tgt
				}
				items[i] = kit.ListItem{Key: sc.ID, On: sc.ID == e.scene, Title: shared.SceneName(sc),
					Meta: []string{shared.LangName(src) + " → " + shared.LangName(tgt), locales.TermCount(len(sc.Terms))}}
				if inUse {
					items[i].Trail = func() { kit.InUse(c).Label(locales.SceneInUse()) }
				}
			}
			// As tall as the scene's card beside it, unless the empty state under it takes the room.
			grow := float32(1)
			if len(list) == 1 {
				grow = 0
			}
			kit.OptionsCard(c).Grow(grow).Children(func() {
				if i := kit.List(c, items); i >= 0 && list[i].ID != e.scene {
					e.scene, e.adding, e.heardAt = list[i].ID, false, ""
				}
			})
			if len(list) == 1 {
				kit.Empty(c, kit.ArtScenes, locales.NoScenes(), nil)
			}
		})
		sc, _ := k.SceneByID(e.scene)
		kit.Card(c).Grow(2).Basis(0).MinWidth(0).Gap(kit.Space(4)).Children(func() {
			if f := s.sceneDetail(c, sc); f != nil {
				then = f
			}
		})
	})
	s.learnDialog(c)
	s.endpointDialog(c)
}

// sceneDetail is a scene's card: its name, whether it is in use, its languages and description,
// and its glossary. It gives what adds or deletes a scene or a name, for after the page is built.
func (s *Settings) sceneDetail(c *ui.Context, sc *scene.Scene) (then func()) {
	k, e := s.core, &s.sceneEdit
	p := kit.Colors(c)
	id, inUse := sc.ID, sc.ID == k.Preferences().Scene
	before := scene.Clone([]scene.Scene{*sc})[0]
	defer func() {
		// Only bound text fields are copied back. Language menus and add/delete actions
		// commit independently, so this draft cannot overwrite their newer state.
		if sc.Name == before.Name && sc.About == before.About && reflect.DeepEqual(sc.Terms, before.Terms) {
			return
		}
		k.UpdateScene(id, func(current *scene.Scene) {
			if sc.Name != before.Name {
				current.Name = sc.Name
			}
			if sc.About != before.About {
				current.About = sc.About
			}
			if !reflect.DeepEqual(sc.Terms, before.Terms) {
				current.Terms = scene.Clone([]scene.Scene{*sc})[0].Terms
			}
		})
	}()
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
		if id == "" {
			ui.Text(c, shared.SceneName(*sc)).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(p.Ink).Grow(1).MaxLines(1)
		} else {
			kit.CellInput(c.Key("name-"+id), &sc.Name).Placeholder(locales.NewScene()).Label(locales.SceneName()).
				FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).Height(kit.Space(11)).Grow(1).Margin(0, 0, 0, -kit.Space(2))
		}
		if inUse {
			kit.InUseLabel(c, locales.SceneInUse())
		} else if kit.Button(c, kit.Primary, kit.Small, nil, locales.UseScene()).Clicked() {
			k.SetScene(id)
		}
		if id != "" {
			kit.IconButton(c, kit.Plain, kit.Small, assets.More, locales.MoreActions()).Menu(func(m *ui.Menu) {
				if m.Item(locales.DeleteScene()).Chosen() {
					name := shared.SceneName(*sc)
					then = func() {
						c.ToastAction(locales.DeletedItem(name), locales.Undo(), k.DeleteScene(id))
						e.scene, e.adding = k.Preferences().Scene, false
					}
				}
			})
		}
	})
	// The scene in use's languages are translating's; another's are kept for when it is used.
	section(c, locales.SceneLangs(), nil, func() {
		src, tgt := sc.Src, sc.Tgt
		setSrc := func(l string) {
			k.UpdateScene(id, func(current *scene.Scene) { setLang(&current.Src, &current.Tgt, l) })
		}
		setTgt := func(l string) {
			k.UpdateScene(id, func(current *scene.Scene) { setLang(&current.Tgt, &current.Src, l) })
		}
		if inUse {
			current := k.Preferences()
			src, tgt, setSrc, setTgt = current.Src, current.Tgt, k.SetSrc, k.SetTgt
		}
		kit.Picker(c, shared.LangName(src), kit.Choices(src, shared.LangChoices(k.SourceLangs()), shared.LangName, setSrc)).Label(locales.SpokenLanguage()).Shrink(1)
		ui.Icon(c, assets.Arrow).FontSize(kit.TextBody).TextColor(p.InkWarm)
		kit.Picker(c, shared.LangName(tgt), kit.Choices(tgt, shared.LangChoices(k.TargetLangs()), shared.LangName, setTgt)).Label(locales.TranslateInto()).Shrink(1)
	})
	if id != "" {
		ui.Column(c).Gap(kit.Space(2)).Children(func() {
			section(c, locales.SceneAbout(), nil, nil)
			kit.TextBox(c.Key("about-"+id), &sc.About).Placeholder(locales.SceneAboutPlaceholder()).Label(locales.SceneAbout()).Height(68)
			kit.Caption(c, locales.SceneAboutHint())
		})
	}
	if f := s.glossary(c, sc); f != nil {
		then = f
	}
	return then
}

// section starts a part of a scene's card: its name, what after builds beside it, and what end
// builds at the row's end, its controls, the row as tall as they are.
func section(c *ui.Context, label string, after, end func()) {
	ui.Row(c).Height(kit.Space(9)).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(kit.Colors(c).Ink).MaxLines(1).Shrink(0)
		if after != nil {
			after()
		}
		ui.Spacer(c)
		if end != nil {
			end()
		}
	})
}

// setLang sets a language of a scene not in use, swapping them when it is the other's.
func setLang(side, other *string, l string) {
	if l == *other {
		*other = *side
	}
	*side = l
}

// glossary is a scene's names, a table: each as written, its translation and the ways it was
// misheard, edited in place; the row that adds one, at its top. It gives what adds or deletes one,
// for after the page is built.
func (s *Settings) glossary(c *ui.Context, sc *scene.Scene) (then func()) {
	k, e := s.core, &s.sceneEdit
	p := kit.Colors(c)
	id := sc.ID
	records := s.sceneRecords(id)
	section(c, locales.Glossary(), func() {
		if len(sc.Terms) > 0 {
			kit.CountBadge(c, len(sc.Terms))
		}
	}, func() {
		if len(records) > 0 && kit.Button(c, kit.Plain, kit.Small, assets.Sparkles, locales.LearnFromRecords()).Clicked() {
			s.learnFrom(records, id)
		}
		if kit.IconButton(c, kit.Plain, kit.Small, assets.Plus, locales.NewTerm()).Clicked() {
			e.adding = true
		}
	})
	// No names yet: the way to some, learning them from the records the scene has, else adding one.
	if len(sc.Terms) == 0 && !e.adding {
		art, text := kit.ArtTerms, locales.NoTermsYet()
		if len(records) > 0 {
			art, text = kit.ArtTermsLearn, locales.NoTermsLearn()
		}
		if len(k.SceneList()) == 1 {
			// The scenes' list has its drawing already: one to a screen.
			kit.SandCard(c).Children(func() { kit.Caption(c, text) })
		} else {
			kit.Empty(c, art, text, nil)
		}
		return then
	}
	// The columns: as written, the translation, the ways misheard, and the row's button.
	cells := func(src, tgt, heard func(), action func()) {
		ui.Box(c).Grid().ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1.3), ui.Fixed(kit.Space(8))).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
			src()
			tgt()
			heard()
			action()
		})
	}
	ui.Column(c).Radius(kit.RadiusField).Border(1, p.Line).Clip().Children(func() {
		ui.Box(c).Background(p.Paper).Padding(kit.Space(2), kit.Space(1)).Children(func() {
			head := func(s string) func() {
				return func() {
					ui.Text(c, s).FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(p.InkMuted).Padding(0, kit.Space(2)).MaxLines(1)
				}
			}
			cells(head(locales.TermSrc()), head(locales.TermTgt()), head(locales.TermHeard()), func() { ui.Box(c) })
		})
		row := func(key string, fn func()) {
			ui.Box(c.Key(key)).Padding(kit.Space(1)).BorderWidth(1, 0, 0).BorderColor(p.LineSoft).Children(fn)
		}
		if e.adding {
			row("new", func() {
				var submitted bool
				add := func(in ui.Element) { submitted = in.Submitted() || submitted }
				cells(func() {
					add(kit.CellInput(c.Key("new-src"), &e.src).Placeholder(locales.TermSrc()).Label(locales.TermSrc()).FontWeight(kit.WeightBold).AutoFocus())
				}, func() {
					add(kit.CellInput(c.Key("new-tgt"), &e.tgt).Placeholder(locales.TermTgt()).Label(locales.TermTgt()))
				}, func() {
					add(kit.CellInput(c.Key("new-heard"), &e.heard).Placeholder(locales.TermHeardHint()).Label(locales.TermHeard()).FontSize(kit.TextControl))
				}, func() {
					ok := strings.TrimSpace(e.src) != "" && strings.TrimSpace(e.tgt) != ""
					if (kit.Off(c, kit.IconButton(c, kit.Plain, kit.Small, assets.Check, locales.AddTerm()), !ok).Clicked() || submitted) && ok {
						t := scene.Term{Src: e.src, Tgt: e.tgt, Heard: splitHeard(e.heard)}
						then = func() {
							k.AddTerm(id, t)
							e.src, e.tgt, e.heard, e.adding = "", "", "", false
						}
					}
				})
			})
		}
		for i := range sc.Terms {
			term := &sc.Terms[i]
			key := fmt.Sprintf("%s-%d-", id, i)
			row(key, func() {
				cells(func() {
					ui.Row(c).Gap(kit.Space(1)).AlignItems(ui.Center).MinWidth(0).Children(func() {
						kit.CellInput(c.Key(key+"src"), &term.Src).Label(locales.TermSrc()).FontWeight(kit.WeightBold).Grow(1)
						if term.Auto {
							ui.Icon(c, assets.Sparkles).FontSize(kit.TextCaption).TextColor(p.Lantern).Tooltip(locales.TermLearned()).Label(locales.TermLearned())
						}
					})
				}, func() {
					kit.CellInput(c.Key(key+"tgt"), &term.Tgt).Label(locales.TermTgt())
				}, func() {
					s.heardCell(c, key, term)
				}, func() {
					if kit.DeleteButton(c.Key(key+"del"), kit.Small, locales.Delete()).Clicked() {
						name := term.Src
						then = func() {
							c.ToastAction(locales.DeletedItem(name), locales.Undo(), k.DeleteTerm(id, i))
							e.heardAt = ""
						}
					}
				})
			})
		}
	})
	return then
}

// heardCell edits the ways term was misheard, as text separated as the locale separates a list.
// While it is typed in, the text stays as typed: a separator at its end is the start of one more.
func (s *Settings) heardCell(c *ui.Context, key string, term *scene.Term) {
	e := &s.sceneEdit
	text := strings.Join(term.Heard, locales.TermHeardSep())
	v := &text
	if e.heardAt == key {
		v = &e.heardText
	}
	in := kit.CellInput(c.Key(key+"heard"), v).Label(locales.TermHeard()).FontSize(kit.TextControl).TextColor(kit.Colors(c).InkMuted)
	if in.Changed() {
		term.Heard = splitHeard(*v)
	}
	switch focused := in.Focused(); {
	case focused && e.heardAt != key:
		e.heardAt, e.heardText = key, *v
	case !focused && e.heardAt == key:
		e.heardAt = ""
	}
}

// splitHeard is the ways a name was misheard, as typed: separated by commas of any language.
func splitHeard(s string) []string {
	var heard []string
	for _, h := range strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(",，、", r) }) {
		if h = strings.TrimSpace(h); h != "" {
			heard = append(heard, h)
		}
	}
	return heard
}

// sceneRecords are the paths of the sessions kept in scene id.
func (s *Settings) sceneRecords(id string) []string {
	k := s.core
	if !k.HistoryOn() {
		return nil
	}
	if k.HistoryStale() {
		k.LoadHistory()
	}
	var paths []string
	for _, r := range k.History.Records {
		if r.Head.Scene == id {
			paths = append(paths, r.Path)
		}
	}
	return paths
}
