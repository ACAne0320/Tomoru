package core

import (
	"log"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/prefs"
	"tomoru/internal/usage"
)

type Today struct {
	Day string
	usage.Counts
	Translated int
}

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

func (c *Core) RunFor() time.Duration {
	if !c.Running || c.runSince.IsZero() {
		return 0
	}
	return max(0, c.reports.endAt(c.now()).Sub(c.runSince))
}

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

func (c *Core) Usage() usage.Ledger { return c.usageAt(c.now()) }

func (c *Core) usageAt(now time.Time) usage.Ledger {
	l := c.ledger.Clone()
	if c.Running {
		c.addRunUsage(&l, now)
	}
	return l
}

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
	if id, ok := prefs.EndpointOf(p.MT); ok {
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

func (c *Core) endRun() {
	if c.runSince.IsZero() {
		return
	}
	defer func() { c.runSince = time.Time{} }()
	if !c.Prefs.Onboarded {
		return
	}
	c.addRunUsage(&c.ledger, c.now())
	c.saveStats()
}

func (c *Core) imageTranslated(via string) {
	if !c.Prefs.Onboarded {
		return
	}
	c.ledger.Add(usage.DayOf(c.now()), usage.Engines{MT: via}, usage.Counts{Images: 1})
	c.saveStats()
}

func (c *Core) saveStats() {
	if path := c.deps.StatsPath; path != "" {
		if err := c.ledger.Save(path); err != nil {
			log.Println("stats:", err)
		}
	}
}

func (c *Core) now() time.Time {
	if c.deps.Now != nil {
		return c.deps.Now()
	}
	return time.Now()
}
