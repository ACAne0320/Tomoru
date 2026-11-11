package core

import (
	"log"

	"tomoru/internal/history"
)

// The subtitles kept, when the user turned it on (Prefs.History): on this computer only, a file a
// session, under its scene. A line is kept once translated, or as far as it came when translating
// stops.

// HistoryView is the sessions kept, as the settings show them.
type HistoryView struct {
	Loaded  bool
	Loading bool
	Records []history.Record
	Err     error
	// Shown is the session being read, by path, and Lines its lines once read.
	Shown string
	Lines []history.Entry
}

// HistoryOn tells whether history has a storage location.
func (c *Core) HistoryOn() bool { return c.deps.HistoryDir != "" }

func (c *Core) syncHistoryPolicy() {
	sc := c.Scene()
	head := history.Head{Scene: sc.ID, Name: sc.Name, Src: c.Prefs.Src, Tgt: c.Prefs.Tgt, Untranslated: !c.Prefs.Translates()}
	if c.Prefs.Source != "" {
		head.App = c.Prefs.SourceName
	}
	c.HistoryService.configure(c.deps.HistoryDir, c.Prefs.History && c.Prefs.Onboarded, head)
}

// LoadHistory lists the sessions kept, in the background. The first time a launch, it purges
// those deleted before.
func (c *HistoryService) LoadHistory() {
	if c.dir == "" || c.History.Loading {
		return
	}
	c.History.Loading = true
	dir, purge, gen := c.dir, !c.purged, c.histGen
	c.purged = true
	c.work.Go(func() {
		if purge {
			history.Purge(dir)
		}
		rs, err := history.List(dir)
		c.work.Post(func() {
			c.History.Loading, c.History.Loaded, c.History.Records, c.History.Err = false, true, rs, err
			c.listed = gen == c.histGen // else what is kept changed meanwhile
			c.changed()
		})
	})
}

// HistoryStale tells whether the sessions shown are not those kept: a session was kept since, or
// they were never listed. The settings list them again then.
func (c *HistoryService) HistoryStale() bool { return !c.listed && !c.History.Loading }

// ShowHistory reads the session at path, in the background; "" shows none.
func (c *HistoryService) ShowHistory(path string) {
	c.History.Shown, c.History.Lines = path, nil
	c.changed()
	if path == "" {
		return
	}
	c.work.Go(func() {
		es, err := history.Read(path)
		if err != nil {
			log.Println("history:", err)
		}
		c.work.Post(func() {
			if c.History.Shown == path {
				c.History.Lines = es
				c.changed()
			}
		})
	})
}

// DeleteHistory deletes the sessions at paths. undo puts them back.
func (c *HistoryService) DeleteHistory(paths []string) (undo func()) {
	u, err := history.Delete(paths)
	if err != nil {
		log.Println("history:", err)
	}
	c.histChanged()
	c.LoadHistory()
	return func() {
		u()
		c.histChanged()
		c.LoadHistory()
	}
}

// histChanged tells that what is kept changed: the sessions listed, or being listed, are stale.
func (c *HistoryService) histChanged() {
	c.histGen++
	c.listed = false
}
