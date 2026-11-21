package settings

import (
	"log"
	"path/filepath"
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

type historyEdit struct {
	scene     string
	chosen    bool
	record    string
	exports   int
	exportDir string
}

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
	if n := k.History.Exports; n != e.exports {
		e.exports = n
		if err := k.History.ExportErr; err != nil {
			c.Toast(locales.ExportFailed(err.Error()))
		} else {
			c.Toast(locales.Exported())
		}
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
			kit.Empty(c, kit.ArtHistoryOff, locales.HistoryOff(), nil)
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

func (s *Settings) recordItem(c *ui.Context, r history.Record, on bool, at string) kit.ListItem {
	p := kit.Colors(c)
	meta := []string{recordLangs(r), locales.HistoryLines(r.Lines)}
	if r.Head.App != "" {
		meta = append(meta, r.Head.App)
	}
	return kit.ListItem{Key: r.Path, On: on, Title: s.recordScene(r), Meta: meta, Lead: func() {
		ui.Text(c, at).FontSize(kit.TextControl).FontWeight(kit.WeightBold).FontFeatures(kit.Tabular).TextColor(p.InkWarm).Width(44).Shrink(0)
	}}
}

func (s *Settings) record(c *ui.Context, r history.Record) {
	k, e := s.core, &s.historyEdit
	p := kit.Colors(c)
	lines := k.History.Lines
	var terms []scene.Term
	if sc, ok := k.SceneByID(r.Head.Scene); ok {
		terms = sc.Terms
	}
	hours := r.Head.Start.Format("15:04")
	if n := len(lines); n > 0 && !lines[n-1].At.IsZero() {
		hours += " – " + lines[n-1].At.Format("15:04")
	}
	from := ""
	if r.Head.App != "" {
		from = " · " + locales.RecordSource(r.Head.App)
	}
	span := day(r.Head.Start) + " " + hours + from
	ui.Column(c).Gap(kit.Space(1)).Children(func() {
		ui.Text(c, s.recordScene(r)).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(p.Ink).MaxLines(2)
		kit.Caption(c, span)
	})
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
		if kit.Off(c, kit.Button(c, kit.Soft, kit.Small, assets.Sparkles, locales.LearnNames()), r.Lines == 0).Clicked() {
			s.learnFrom([]string{r.Path}, r.Head.Scene)
		}
		ui.Spacer(c)
		kit.Off(c, kit.Button(c, kit.Plain, kit.Small, assets.Download, locales.ExportRecord()), len(lines) == 0).Menu(func(m *ui.Menu) {
			text := func() string {
				date := i18n.FormatDate(i18n.Locale(), r.Head.Start, i18n.DateOptions{Date: i18n.MediumStyle})
				return history.Text(s.recordScene(r)+" · "+date+" "+hours+from+" · "+recordLangs(r), r.Head, lines)
			}
			name := fileName(s.recordScene(r) + " " + r.Head.Start.Format("2006-01-02 15-04"))
			if m.Item(locales.ExportText()).Chosen() {
				s.export(name, mygo.FileFilter{Name: locales.ExportTextType(), Extensions: []string{"txt"}}, text())
			}
			if m.Item(locales.ExportSubtitles()).Chosen() {
				s.export(name, mygo.FileFilter{Name: locales.ExportSubtitlesType(), Extensions: []string{"srt"}}, history.SRT(r.Head, lines))
			}
			m.Separator()
			if m.Item(locales.CopyAll()).Chosen() {
				c.Services().WriteClipboard(text())
				c.Toast(locales.Copied())
			}
		})
		if kit.IconButton(c, kit.Quiet, kit.Small, assets.Folder, shared.OnEach(locales.ShowInFinder(), locales.ShowInExplorer(), locales.ShowInFolder())).Clicked() {
			mygo.Shell.ShowItemInFolder(r.Path)
		}
		if kit.DeleteButton(c, kit.Small, locales.DeleteRecord()).Clicked() {
			c.ToastAction(locales.DeletedItem(when(r)), locales.Undo(), k.DeleteHistory([]string{r.Path}))
			e.record = ""
		}
	})
	ui.Column(c).Children(func() {
		for i, l := range lines {
			said := l.Start
			if said.IsZero() {
				said = l.At
			}
			ui.Row(c.Key(i)).Gap(kit.Space(3)).AlignItems(ui.Start).Padding(kit.Space(3), 0).BorderWidth(1, 0, 0).BorderColor(p.LineSoft).Children(func() {
				ui.Text(c, said.Format("15:04:05")).FontSize(kit.TextCaption).FontFeatures(kit.Tabular).TextColor(p.InkWarm).Width(56).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() { line(c, l, terms, r.Head.Untranslated) })
			})
		}
	})
}

func (s *Settings) export(name string, kind mygo.FileFilter, data string) {
	k, e := s.core, &s.historyEdit
	dir := e.exportDir
	if dir == "" {
		dir, _ = mygo.App.Path(mygo.PathDownloads)
	}
	opts := mygo.SaveDialogOptions{Parent: mygo.FocusedWindow(), DefaultPath: filepath.Join(dir, name+"."+kind.Extensions[0]),
		Filters: []mygo.FileFilter{kind}, CreateDirectories: true}
	go func() {
		path, err := mygo.Dialog.Save(opts)
		if err != nil {
			log.Println("export:", err)
		}
		if path != "" {
			mygo.RunOnMain(func() {
				e.exportDir = filepath.Dir(path)
				k.ExportHistory(path, data)
			})
		}
	}()
}

func fileName(name string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return '-'
		}
		return r
	}, name)
}

func recordLangs(r history.Record) string {
	if r.Head.Untranslated {
		return shared.LangName(r.Head.Src)
	}
	return shared.Langs(r.Head.Src, r.Head.Tgt)
}

func line(c *ui.Context, l history.Entry, terms []scene.Term, untranslated bool) {
	untranslated = untranslated || l.Said
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

type mark struct {
	text  string
	fixed bool
}

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
	slices.SortFunc(names, func(a, b string) int { return len(b) - len(a) })
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

func day(t time.Time) string {
	switch now := time.Now(); {
	case sameDay(t, now):
		return locales.DayToday()
	case sameDay(t, now.AddDate(0, 0, -1)):
		return locales.DayYesterday()
	}
	return i18n.FormatDate(i18n.Locale(), t, i18n.DateOptions{Date: i18n.MediumStyle})
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func when(r history.Record) string {
	return i18n.FormatDate(i18n.Locale(), r.Head.Start, i18n.DateOptions{Date: i18n.MediumStyle, Time: i18n.ShortStyle})
}

func (s *Settings) recordScene(r history.Record) string {
	if sc, ok := s.core.SceneByID(r.Head.Scene); ok {
		return shared.SceneName(*sc)
	}
	if r.Head.Name != "" {
		return r.Head.Name + " (" + locales.DeletedScene() + ")"
	}
	return locales.DeletedScene()
}
