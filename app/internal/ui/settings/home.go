package settings

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/usage"
	"tomoru/locales"
)

// home is the window's first page (docs/design-system.md): whether it translates, and the button
// to start or stop; the scenes, and under them what it hears and the scene's languages, in a
// sentence whose values are menus; its numbers; its lines day by day; the records. The subtitles
// show over the screen, not here.
func (s *Settings) home(c *ui.Context) {
	k := s.core
	p := kit.Colors(c)
	if k.Running {
		c.After(time.Second) // the numbers count on
	}
	ui.Column(c).Gap(kit.Space(4)).Children(func() {
		ui.Row(c).Gap(kit.Space(5)).AlignItems(ui.Start).Children(func() {
			ui.Text(c, shared.Title(k.Preferences(), k.Running)).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink).Grow(1).MaxLines(2)
			ui.Column(c).AlignItems(ui.End).Gap(kit.Space(2)).Children(func() {
				if k.Running {
					if kit.Button(c, kit.Destructive(kit.Strong), kit.Large, assets.Stop, shared.StopText(k.Preferences())).Clicked() {
						k.Stop()
					}
				} else if kit.Button(c, kit.Primary, kit.Large, assets.Play, shared.StartText(k.Preferences())).Clicked() {
					k.Start()
				}
				ui.Text(c, locales.ShortcutHint(kit.ShortcutText(k.Preferences().Shortcut))).FontSize(kit.TextCaption).TextColor(p.InkMuted)
			})
		})
		homeLine(c, s)
		if why := homeState(s); why != "" {
			ui.Text(c, why).FontSize(kit.TextControl).TextColor(p.InkMuted).MaxLines(3)
		}
		// Why it stopped, and the one action that resolves it.
		if err := shared.ErrorText(k.Err); err != "" {
			ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, err).FontSize(kit.TextControl).TextColor(p.Danger).MaxLines(3).Grow(1)
				shared.FixButton(c, k, k.Err, func() { s.Tab = "engines" })
			})
		}
	})
	homeStats(c, s)
	homeActivity(c, s)
	homeRecent(c, s)
}

// homeState says what translating waits for, when it does.
func homeState(s *Settings) string {
	k := s.core
	if !k.Running {
		return ""
	}
	switch k.State.Phase {
	case "downloading":
		return locales.StatusDownloading()
	case "starting":
		return locales.StatusPreparing()
	case "loading":
		return locales.StatusLoadingModel()
	}
	return ""
}

// homeStats are the numbers: this run's while translating, today's while not.
func homeStats(c *ui.Context, s *Settings) {
	k := s.core
	run, st := k.RunFor(), k.State.Stats
	label, lines, linesLabel := locales.StatRunning(), st.Lines, locales.StatLines()
	if k.Running && !k.Preferences().Translates() {
		linesLabel = locales.StatHeard() // the original only: nothing translated
	}
	if !k.Running {
		today := k.TodayStats()
		run, st, label, lines = time.Duration(today.Run*float64(time.Second)), today.Counts, locales.StatToday(), today.Translated
	}
	kit.StatStrip(c, locales.HomeStats(), []kit.Stat{
		{Label: label, Value: kit.Clock(run)},
		{Label: linesLabel, Value: strconv.Itoa(lines), Unit: locales.UnitLines()},
		{Label: locales.StatSpeech(), Value: fmt.Sprintf("%.1f", st.Speech/60), Unit: locales.UnitMinutes()},
		{Label: locales.StatChars(), Value: strconv.Itoa(st.Chars), Unit: locales.UnitChars()},
	})
}

// homeActivity is the lines of each day of the last weeks, as many as the card holds up to a year,
// a square a day, as the user's region writes dates: a week a column from Monday, a month's name
// over the week it starts in. A day clicked shows in the statistics.
func homeActivity(c *ui.Context, s *Settings) {
	now := time.Now()
	w, _ := c.Size()
	from, lines := activityDays(s.core.Usage(), min(max(kit.HeatWeeks(w-s.navWidth-kit.Space(27)), 8), 53), now)
	weeks := len(lines) / 7
	locale := i18n.Locale()
	months := i18n.Months(locale, i18n.Abbreviated, true)
	names := i18n.Weekdays(locale, i18n.Abbreviated, true)
	weekdays := [7]string{0: names[time.Monday], 2: names[time.Wednesday], 4: names[time.Friday]}
	heads := make([]string, weeks)
	for w := range heads {
		if t := from.AddDate(0, 0, 7*w); w > 0 && t.Month() != t.AddDate(0, 0, -7).Month() {
			heads[w] = months[t.Month()-1]
		}
	}
	if heads[1] == "" && heads[2] == "" { // room for the first week's month before the next
		heads[0] = months[from.Month()-1]
	}
	days := make([]kit.HeatDay, len(lines))
	total := 0
	for i, n := range lines {
		days[i].Value = n
		if !math.IsNaN(n) {
			total += int(n)
			days[i].Tip = i18n.FormatDate(locale, from.AddDate(0, 0, i), i18n.DateOptions{Skeleton: "yMMMEd"}) + " · " + locales.HistoryLines(int(n))
		}
	}
	kit.Group(c, locales.StatsActivity(), func() {
		kit.Caption(c, locales.HistoryLines(total)).SingleLine()
		if kit.IconButton(c, kit.Plain, kit.Small, assets.Arrow, locales.HomeOpenStats()).Clicked() {
			s.Tab = "stats"
		}
	}, kit.Card, func() {
		kit.Heatmap(c, locales.StatsActivity(), days, heads, weekdays, locales.StatsLess(), locales.StatsMore(), func(i int) {
			s.statsDay(from.AddDate(0, 0, i), now)
		})
	})
}

// monday is the start of the Monday of t's week.
func monday(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}

// activityDays are the lines of each day of the last weeks weeks to now, from the Monday of the
// first: NaN for the days after today.
func activityDays(l usage.Ledger, weeks int, now time.Time) (from time.Time, lines []float64) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from = monday(now).AddDate(0, 0, -7*(weeks-1))
	lines = make([]float64, 7*weeks)
	for i := range lines {
		if from.AddDate(0, 0, i).After(today) {
			lines[i] = math.NaN()
		}
	}
	for _, d := range l.Days {
		t, err := time.ParseInLocation(time.DateOnly, d.Day, now.Location())
		if err != nil || t.Before(from) || t.After(today) {
			continue
		}
		lines[(t.Sub(from)+12*time.Hour)/(24*time.Hour)] += float64(d.Total().Lines) // days of 23 or 25 hours too
	}
	return from, lines
}

// homeLine is what is heard, then the scene in use and its languages, from one to the other, each a
// menu, and a button between the two that swaps them: the sound is what is translated, and the scene decides how, keeping its languages. The two
// go to the next line whole, in a narrow window, and in a narrower one, what is in them.
func homeLine(c *ui.Context, s *Settings) {
	k, p := s.core, kit.Colors(c)
	label := func(t string) { ui.Text(c, t).SingleLine().Shrink(0) }
	ui.Row(c).GapX(kit.Space(6)).GapY(kit.Space(3)).AlignItems(ui.Center).Wrap().FontSize(kit.TextBody).TextColor(p.InkMuted).Children(func() {
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
			label(locales.Source()) // as the panel names it
			k.ListApps()
			c.After(core.ListEvery) // the apps playing change
			kit.Picker(c, shared.SourceName(k), s.source.Build(k)).Label(locales.Source())
		})
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
			label(locales.Scene())
			kit.Picker(c, shared.CurrentSceneName(k), shared.SceneMenu(k, false, func() { s.Open("scenes") })).Label(locales.Scene())
			kit.Picker(c, shared.LangName(k.Preferences().Src), kit.Choices(k.Preferences().Src, shared.LangChoices(k.SourceLangs()), shared.LangName, k.SetSrc)).Label(locales.SpokenLanguage())
			// From one to the other, which swaps them, as in the panel.
			off := !k.Preferences().Translates() // the original only: nothing is translated into it
			if kit.Off(c, kit.IconButton(c, kit.Quiet, kit.Small, assets.Swap, locales.SwapLanguages()), !k.CanSwap() || off).Clicked() {
				k.Swap()
			}
			kit.Off(c, kit.Picker(c, shared.LangName(k.Preferences().Tgt), kit.Choices(k.Preferences().Tgt, shared.LangChoices(k.TargetLangs()), shared.LangName, k.SetTgt)).Label(locales.TranslateInto()), off)
		})
	})
}

// homeRecent is the records kept last, as the history lists them, once there are any: one
// clicked shows in the history.
func homeRecent(c *ui.Context, s *Settings) {
	k := s.core
	if !k.HistoryOn() {
		return
	}
	if k.HistoryStale() {
		k.LoadHistory()
	}
	recs := k.History.Records[:min(len(k.History.Records), 2)]
	if len(recs) == 0 {
		return
	}
	kit.Group(c, locales.HomeRecent(), func() {
		if kit.IconButton(c, kit.Plain, kit.Small, assets.Arrow, locales.AllRecords()).Clicked() {
			s.Tab = "history"
		}
	}, kit.OptionsCard, func() {
		items := make([]kit.ListItem, len(recs))
		for i, r := range recs {
			at := r.Head.Start.Format("15:04")
			if !sameDay(r.Head.Start, time.Now()) {
				at = i18n.FormatDate(i18n.Locale(), r.Head.Start, i18n.DateOptions{Skeleton: "Md"})
			}
			items[i] = s.recordItem(c, r, false, at)
			items[i].Trail = func() {
				if kit.Button(c, kit.Quiet, kit.Small, assets.Sparkles, locales.LearnNames()).Clicked() {
					s.learnFrom([]string{r.Path}, r.Head.Scene)
				}
			}
		}
		if i := kit.List(c, items); i >= 0 {
			e := &s.historyEdit
			e.scene, e.chosen, e.record = allScenes, true, recs[i].Path
			s.Tab = "history"
		}
	})
}
