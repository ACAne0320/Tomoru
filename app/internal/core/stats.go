package core

import (
	"log"
	"strings"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/prefs"
	"tomoru/internal/usage"
)

// Today is how much translating has done on a day: the home page's numbers while it is stopped.
// It keeps numbers only, never what was said.
type Today struct {
	Day string // the local date, 2006-01-02
	usage.Counts
	Translated int // the lines translated, of Lines: not those of subtitles of the original only
}

// loadStats reads the numbers kept, of every day.
func (c *Core) loadStats() {
	if c.deps.StatsPath == "" {
		return
	}
	l, err := usage.Load(c.deps.StatsPath)
	if err != nil {
		log.Println("stats:", err)
	}
	c.ledger = l
}

// RunFor is the elapsed session time, including silence; 0 while stopped.
func (c *Core) RunFor() time.Duration {
	if !c.Running || c.runSince.IsZero() {
		return 0
	}
	return max(0, c.reports.endAt(c.now()).Sub(c.runSince))
}

// TodayStats includes only usage that occurred on today's local date.
func (c *Core) TodayStats() Today {
	now := c.now()
	day := usage.DayOf(now)
	l := c.usageAt(now)
	t := Today{Day: day, Counts: l.Day(day)}
	for _, d := range l.Days {
		if d.Day == day {
			t.Translated = d.TotalOf(func(u usage.Use) bool { return !u.Untranslated }).Lines
		}
	}
	return t
}

// Usage is a detached copy of the saved days and the current session's daily usage.
func (c *Core) Usage() usage.Ledger { return c.usageAt(c.now()) }

func (c *Core) usageAt(now time.Time) usage.Ledger {
	l := c.ledger.Clone()
	if c.Running {
		c.addRunUsage(&l, now)
	}
	return l
}

// addRunUsage merges event-time counts and splits elapsed time at local midnights.
// Reading the running totals never consumes them or writes the saved ledger.
func (c *Core) addRunUsage(l *usage.Ledger, now time.Time) {
	if c.runSince.IsZero() {
		return
	}
	days, end := c.reports.usageSnapshot(now)
	for _, d := range days.Days {
		l.Add(d.Day, c.runEngines, d.Total())
	}
	l.AddRun(c.runSince, end, c.runEngines)
}

// engines are the engines of p, as the numbers keep them: with an endpoint's name and model, and
// a cloud service's model, as they are now; with the original alone, no translator.
func engines(p prefs.Prefs) usage.Engines {
	e := usage.Engines{ASR: p.ASR, MT: p.MT}
	if s, ok := asr.ByID(p.ASR); ok && s.Cloud {
		if a, ok := p.API(p.ASR); ok {
			e.ASRModel = a.Model
		}
		if e.ASRModel == "" && len(s.Models) > 0 {
			e.ASRModel = s.Models[0]
		}
	}
	if id, ok := strings.CutPrefix(p.MT, prefs.CustomPrefix); ok {
		if ep, ok := p.Endpoint(id); ok {
			e.MTName, e.MTModel = ep.Name, ep.Model
		}
	}
	if s, ok := MTServiceByID(p.MT); ok {
		a, _ := p.API(p.MT)
		if a == nil {
			a = &prefs.Service{}
		}
		e.MTModel = s.endpoint(*a).Model
	}
	if !p.Translates() {
		e = usage.Engines{ASR: e.ASR, ASRModel: e.ASRModel, Untranslated: true}
	}
	return e
}

// endRun persists the session's already dated counts exactly once. It never assigns
// cumulative totals to the stopping date.
func (c *Core) endRun() {
	if c.runSince.IsZero() {
		return
	}
	defer func() { c.runSince = time.Time{} }()
	if !c.Prefs.Onboarded { // trying it in the guide is not translating yet
		return
	}
	c.addRunUsage(&c.ledger, c.now())
	c.saveStats()
}

// imageTranslated counts a picture translated, on its day, under the engine via that translated
// it. As translating, trying it in the guide is not counted.
func (c *Core) imageTranslated(via string) {
	if !c.Prefs.Onboarded {
		return
	}
	c.ledger.Add(usage.DayOf(c.now()), usage.Engines{MT: via}, usage.Counts{Images: 1})
	c.saveStats()
}

// saveStats writes the numbers kept.
func (c *Core) saveStats() {
	if path := c.deps.StatsPath; path != "" {
		if err := c.ledger.Save(path); err != nil {
			log.Println("stats:", err)
		}
	}
}

// now is the time, as Deps.Now says in tests.
func (c *Core) now() time.Time {
	if c.deps.Now != nil {
		return c.deps.Now()
	}
	return time.Now()
}
