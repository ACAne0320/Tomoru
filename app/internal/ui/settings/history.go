package settings

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/history"
	"tomoru/internal/live"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// The history's page (docs/design-system.md): whether to keep the subtitles, the sessions kept by
// day, of a scene or all, and beside them the one chosen, its lines with the names the glossary
// corrected.

// historyEdit is the page's state: the scene whose sessions show, "*" for all, once the page chose
// it, and the session shown, by path.
type historyEdit struct {
	scene  string
	chosen bool
	record string
}

// allScenes is the history's filter of no scene.
const allScenes = "*"

func (s *Settings) history(c *ui.Context) {
	k, e := s.core, &s.historyEdit
	p := kit.Colors(c)
	if !e.chosen {
		e.scene, e.chosen = allScenes, true
	}
	if _, ok := k.SceneByID(e.scene); !ok {
		e.scene = allScenes
	}
	if k.HistoryStale() {
		k.LoadHistory()
	}
	var shown []history.Record
	for _, r := range k.History.Records {
		if e.scene == allScenes || r.Head.Scene == e.scene {
			shown = append(shown, r)
		}
	}
	ui.Text(c, locales.TabHistory()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink).MaxLines(1)
	kit.RowsCard(c).Children(func() {
		kit.SettingRow(c, 0, locales.KeepHistory(), "", func() {
			on := k.Preferences().History
			if kit.Switch(c, &on, locales.KeepHistory()).Changed() {
				k.UpdatePrefs(func(p *prefs.Prefs) { p.History = on })
			}
		})
	})
	if len(k.History.Records) > 0 {
		// The scene whose sessions show, and deleting those that show.
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Gap(kit.Space(2)).Wrap().Role(ui.RoleGroup).Label(locales.Scene()).Children(func() {
				if kit.Chip(c.Key(allScenes), locales.AllScenes(), e.scene == allScenes, 0).Clicked() {
					e.scene = allScenes
				}
				for _, sc := range k.SceneList() {
					if kit.Chip(c.Key(sc.ID), shared.SceneName(sc), e.scene == sc.ID, 0).Clicked() {
						e.scene = sc.ID
					}
				}
			})
			if kit.Off(c, kit.DeleteButton(c, kit.Small, locales.DeleteAll()), len(shown) == 0).Clicked() {
				paths := make([]string, len(shown))
				for i, r := range shown {
					paths[i] = r.Path
				}
				c.ToastAction(locales.DeletedSessions(len(paths)), locales.Undo(), k.DeleteHistory(paths))
			}
		})
	}
	if len(shown) == 0 {
		switch {
		case !k.History.Loaded:
		case len(k.History.Records) > 0:
			kit.SandCard(c).Children(func() { kit.Caption(c, locales.HistorySceneEmpty()) })
		case !k.Preferences().History:
			kit.Empty(c, kit.ArtHistoryOff, locales.HistoryOff(), nil) // the switch above turns it on
		default:
			kit.Empty(c, kit.ArtHistory, locales.NoHistory(), nil)
		}
		e.record = ""
	} else {
		at := 0
		for i, r := range shown {
			if r.Path == e.record {
				at = i
			}
		}
		e.record = shown[at].Path
		if k.History.Shown != e.record {
			k.ShowHistory(e.record)
		}
		ui.Row(c).Gap(kit.CardGap).AlignItems(ui.Stretch).Children(func() {
			s.records(c, shown)
			kit.Card(c).Grow(3).Basis(0).MinWidth(0).Children(func() {
				s.record(c, shown[at])
			})
		})
	}
	s.learnDialog(c)
	s.endpointDialog(c)
}

// records lists the sessions shown by day, the latest first, as History.Records has them.
func (s *Settings) records(c *ui.Context, shown []history.Record) {
	e := &s.historyEdit
	items := make([]kit.ListItem, len(shown))
	last := ""
	for i, r := range shown {
		items[i] = s.recordItem(c, r, r.Path == e.record, r.Head.Start.Format("15:04"))
		if d := day(r.Head.Start); d != last {
			items[i].Head, last = d, d
		}
	}
	kit.OptionsCard(c).Grow(2).Basis(0).MinWidth(0).MaxWidth(320).Children(func() {
		if i := kit.List(c, items); i >= 0 {
			e.record = shown[i].Path
		}
	})
}

// recordItem is a session as lists show it: when it started, at, its scene, and its languages, its
// lines and the app it heard, which the session's card says too, the first to go where the room
// runs out.
func (s *Settings) recordItem(c *ui.Context, r history.Record, on bool, at string) kit.ListItem {
	p := kit.Colors(c)
	langs := shared.Langs(r.Head.Src, r.Head.Tgt)
	if r.Head.Untranslated {
		langs = shared.LangName(r.Head.Src)
	}
	meta := []string{langs, locales.HistoryLines(r.Lines)}
	if r.Head.App != "" {
		meta = append(meta, r.Head.App)
	}
	return kit.ListItem{Key: r.Path, On: on, Title: s.recordScene(r), Meta: meta, Lead: func() {
		ui.Text(c, at).FontSize(kit.TextControl).FontWeight(kit.WeightBold).FontFeatures(kit.Tabular).TextColor(p.InkWarm).Width(44).Shrink(0)
	}}
}

// record is a session's card: what it was and when, learning names from it, showing its file in
// Finder, deleting it, and its lines, as heard and translated.
func (s *Settings) record(c *ui.Context, r history.Record) {
	k, e := s.core, &s.historyEdit
	p := kit.Colors(c)
	lines := k.History.Lines
	var terms []scene.Term
	if sc, ok := k.SceneByID(r.Head.Scene); ok {
		terms = sc.Terms
	}
	// Its name and when it was, across the card, and under them what is done with it, the deletion
	// apart at the end.
	ui.Column(c).Gap(kit.Space(1)).Children(func() {
		ui.Text(c, s.recordScene(r)).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(p.Ink).MaxLines(2)
		span := day(r.Head.Start) + " " + r.Head.Start.Format("15:04")
		if n := len(lines); n > 0 && !lines[n-1].At.IsZero() {
			span += " – " + lines[n-1].At.Format("15:04")
		}
		if r.Head.App != "" {
			span += " · " + locales.RecordSource(r.Head.App)
		}
		kit.Caption(c, span)
	})
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
		if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, assets.Sparkles, locales.LearnNames()), r.Lines == 0).Clicked() {
			s.learnFrom([]string{r.Path}, r.Head.Scene)
		}
		if kit.Button(c, kit.Plain, kit.Small, nil, shared.OnWindows(locales.ShowInFinder(), locales.ShowInExplorer())).Clicked() {
			mygo.Shell.ShowItemInFolder(r.Path)
		}
		ui.Spacer(c)
		if kit.DeleteButton(c, kit.Small, locales.DeleteRecord()).Clicked() {
			c.ToastAction(locales.DeletedItem(when(r)), locales.Undo(), k.DeleteHistory([]string{r.Path}))
			e.record = ""
		}
	})
	ui.Column(c).Children(func() {
		for i, l := range lines {
			ui.Row(c.Key(i)).Gap(kit.Space(3)).AlignItems(ui.Start).Padding(kit.Space(3), 0).BorderWidth(1, 0, 0).BorderColor(p.LineSoft).Children(func() {
				ui.Text(c, l.At.Format("15:04:05")).FontSize(kit.TextCaption).FontFeatures(kit.Tabular).TextColor(p.InkWarm).Width(56).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() { line(c, l, terms, r.Head.Untranslated) })
			})
		}
	})
}

// line is a line kept: as heard, the names the glossary corrected marked, as terms, the scene's,
// tell them; its translation; and what recognition heard of those names. Of a session untranslated,
// the line as heard is all there is, and shows as a translation would.
func line(c *ui.Context, l history.Entry, terms []scene.Term, untranslated bool) {
	p := kit.Colors(c)
	src := ui.Span{Size: kit.TextControl, Color: p.InkMuted}
	if untranslated {
		src = ui.Span{Size: kit.TextBody, Weight: kit.WeightMedium, Color: p.Ink}
	}
	var was []string
	if l.Heard != "" && l.Heard != l.Src {
		var marks []mark
		marks, was = corrections(l.Src, l.Heard, terms)
		spans := make([]ui.Span, len(marks))
		for i, m := range marks {
			spans[i] = src
			spans[i].Text = m.text
			if m.fixed {
				spans[i].Weight, spans[i].Color, spans[i].Background, spans[i].Radius = kit.WeightBold, p.LanternText, p.LanternTint, kit.RadiusMark
			}
		}
		ui.RichText(c, spans...)
	} else {
		t := ui.Text(c, l.Src).FontSize(src.Size).TextColor(src.Color)
		if untranslated {
			t.FontWeight(kit.WeightMedium)
		}
	}
	switch {
	case untranslated:
	case l.Tgt != "":
		ui.Text(c, l.Tgt).FontSize(kit.TextBody).FontWeight(kit.WeightMedium).TextColor(p.Ink)
	case l.Err == live.ErrBehind.Error():
		kit.Warning(c, locales.TranslationSkipped()).MaxLines(2)
	case l.Err != "":
		kit.Warning(c, l.Err).MaxLines(2)
	default:
		ui.Text(c, "—").FontSize(kit.TextBody).TextColor(p.InkMuted)
	}
	if len(was) > 0 {
		ui.Row(c).Gap(kit.Space(1)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, assets.Sparkles).FontSize(kit.TextMicro).TextColor(p.Lantern)
			ui.Text(c, locales.TermFixed(strings.Join(was, locales.TermHeardSep()))).FontSize(kit.TextCaption).TextColor(p.LanternText)
		})
	}
}

// mark is a stretch of a line kept: a name the glossary corrected, or the text around them.
type mark struct {
	text  string
	fixed bool
}

// corrections splits src, a line as the glossary corrected heard, at the names it corrected, and
// gives those names as heard. terms tell them: a name in src one of whose ways misheard is in
// heard. Where none does, as the scene's names changed since, src is split around the stretch it
// differs from heard in.
func corrections(src, heard string, terms []scene.Term) (marks []mark, was []string) {
	var names []string
	for _, t := range terms {
		if t.Src == "" || !strings.Contains(src, t.Src) || strings.Contains(heard, t.Src) {
			continue
		}
		for _, h := range t.Heard {
			if h != "" && strings.Contains(heard, h) {
				names, was = append(names, t.Src), append(was, h)
				break
			}
		}
	}
	if len(names) == 0 {
		before, fixed, after, w := corrected(src, heard)
		for _, m := range []mark{{before, false}, {fixed, true}, {after, false}} {
			if m.text != "" {
				marks = append(marks, m)
			}
		}
		return marks, []string{w}
	}
	slices.SortFunc(names, func(a, b string) int { return len(b) - len(a) }) // the longest first
	plain := 0
	for i := 0; i < len(src); {
		at := slices.IndexFunc(names, func(n string) bool { return strings.HasPrefix(src[i:], n) })
		if at < 0 {
			_, size := utf8.DecodeRuneInString(src[i:])
			i += size
			continue
		}
		if plain < i {
			marks = append(marks, mark{src[plain:i], false})
		}
		marks = append(marks, mark{names[at], true})
		i += len(names[at])
		plain = i
	}
	if plain < len(src) {
		marks = append(marks, mark{src[plain:], false})
	}
	return marks, was
}

// corrected splits src around the stretch it differs from heard in: the text before, the stretch
// in src, the text after, and the stretch in heard.
func corrected(src, heard string) (before, fixed, after, was string) {
	s, h := []rune(src), []rune(heard)
	i := 0
	for i < len(s) && i < len(h) && s[i] == h[i] {
		i++
	}
	j := 0
	for j < len(s)-i && j < len(h)-i && s[len(s)-1-j] == h[len(h)-1-j] {
		j++
	}
	return string(s[:i]), string(s[i : len(s)-j]), string(s[len(s)-j:]), string(h[i : len(h)-j])
}

// day is the day of t as the list heads it: today, yesterday, or its date.
func day(t time.Time) string {
	switch now := time.Now(); {
	case sameDay(t, now):
		return locales.DayToday()
	case sameDay(t, now.AddDate(0, 0, -1)):
		return locales.DayYesterday()
	}
	return i18n.FormatDate(i18n.Locale(), t, i18n.DateOptions{Date: i18n.MediumStyle})
}

// sameDay tells whether a and b are on the same day.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// when is when a session started, as the reader's locale writes it.
func when(r history.Record) string {
	return i18n.FormatDate(i18n.Locale(), r.Head.Start, i18n.DateOptions{Date: i18n.MediumStyle, Time: i18n.ShortStyle})
}

// recordScene names a session's scene: as it is named now, or was, if it was deleted since.
func (s *Settings) recordScene(r history.Record) string {
	if sc, ok := s.core.SceneByID(r.Head.Scene); ok {
		return shared.SceneName(*sc)
	}
	if r.Head.Name != "" {
		return r.Head.Name + " (" + locales.DeletedScene() + ")"
	}
	return locales.DeletedScene()
}
