package core

import (
	"log"
	"os"

	"tomoru/internal/history"
)

type HistoryView struct {
	Loaded    bool
	Loading   bool
	Records   []history.Record
	Err       error
	Shown     string
	Lines     []history.Entry
	Exports   int
	ExportErr error
}

func (c *Core) HistoryOn() bool { return c.deps.HistoryDir != "" }

func (c *Core) syncHistoryPolicy() {
	sc := c.Scene()
	head := history.Head{Scene: sc.ID, Name: sc.Name, Src: c.Prefs.Src, Tgt: c.Prefs.Tgt, Untranslated: !c.Prefs.Translates()}
	if c.Prefs.Source != "" {
		head.App = c.Prefs.SourceName
	}
	c.HistoryService.configure(c.deps.HistoryDir, c.Prefs.History && c.Prefs.Onboarded, head)
}

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
			c.listed = gen == c.histGen
			c.changed()
		})
	})
}

func (c *HistoryService) HistoryStale() bool { return !c.listed && !c.History.Loading }

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

func (c *HistoryService) histChanged() {
	c.histGen++
	c.listed = false
}

func (c *HistoryService) ExportHistory(path, data string) {
	c.work.Go(func() {
		err := os.WriteFile(path, []byte(data), 0o644)
		if err != nil {
			log.Println("history:", err)
		}
		c.work.Post(func() {
			c.History.Exports, c.History.ExportErr = c.History.Exports+1, err
			c.changed()
		})
	})
}
