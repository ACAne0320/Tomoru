package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func familyIDs(c *Core) [][]string {
	var got [][]string
	for _, f := range c.Families(MT) {
		var ids []string
		for _, e := range f {
			if e.Model != nil {
				ids = append(ids, e.ID)
			}
		}
		if ids != nil {
			got = append(got, ids)
		}
	}
	return got
}

func TestHyMT2NotDownloaded(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	want := [][]string{{"local-2b", "local-9b"}, {"hy-mt3-1.8b", "hy-mt3-7b"}}
	if got := familyIDs(c); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("local families %q, want %q", got, want)
	}
	for _, id := range []string{"hy-mt2-1.8b", "hy-mt2-7b"} {
		if _, ok := c.Engine(MT, id); ok {
			t.Errorf("%s listed", id)
		}
		if c.Use(MT, id) {
			t.Errorf("%s used", id)
		}
		c.Choose(MT, id)
		if c.Chosen(MT) == id || m.ready[id] {
			t.Errorf("%s chosen or downloaded", id)
		}
	}
}

func TestHyMT2Downloaded(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	m.ready["hy-mt2-7b"] = true
	want := [][]string{{"local-2b", "local-9b"}, {"hy-mt3-1.8b", "hy-mt3-7b"}, {"hy-mt2-7b"}}
	if got := familyIDs(c); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("local families %q, want %q", got, want)
	}
	e, ok := c.Engine(MT, "hy-mt2-7b")
	if !ok || !c.Usable(e) || !c.Use(MT, "hy-mt2-7b") || c.Prefs.MT != "hy-mt2-7b" {
		t.Errorf("Hy-MT2 7B: listed %v, in use %q", ok, c.Prefs.MT)
	}
}

func TestHyMT2SettingStaysLocal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	p := prefs.Default("zh-Hans")
	p.ASR, p.MT = "apple", "hy-mt2-7b"
	if err := prefs.Save(path, p); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, srv := &fakeModels{ready: map[string]bool{"hy-mt2-7b": true}}, &fakeServer{}
	c, f := newCoreWith(t, p, func(d *Deps) {
		d.PrefsPath, d.Translator, d.Models, d.Server, d.Secrets = path, nil, m, srv, fakeSecrets{}
	})
	if after, _ := os.ReadFile(path); c.Prefs.MT != "hy-mt2-7b" || !bytes.Equal(before, after) {
		t.Fatalf("at start: in use %q, settings rewritten %v", c.Prefs.MT, !bytes.Equal(before, after))
	}
	c.Start()
	f.flush(t, 1)
	ep, ok := f.sessions[0].cfg.Translator.(*translate.Endpoint)
	if !ok || ep.URL == translate.CloudURL || ep.Format != translate.HyMT || !slices.Equal(srv.ensured, []string{m.Path("hy-mt2-7b")}) {
		t.Errorf("translator %+v, ensured %q", f.sessions[0].cfg.Translator, srv.ensured)
	}

	c.Stop()
	delete(m.ready, "hy-mt2-7b")
	c.Start()
	if !errors.Is(c.Err, ErrModelMissing) || c.Running || len(f.sessions) != 1 || c.Prefs.MT != "hy-mt2-7b" {
		t.Errorf("file gone: err %v, running %v, %d sessions, in use %q", c.Err, c.Running, len(f.sessions), c.Prefs.MT)
	}
}

func TestDeleteHyMT2(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	m.ready["hy-mt2-1.8b"], m.ready["hy-mt2-7b"] = true, true
	c.SetEngine("hy-mt2-7b")
	c.DeleteModel("hy-mt2-7b")
	want := [][]string{{"local-2b", "local-9b"}, {"hy-mt3-1.8b", "hy-mt3-7b"}, {"hy-mt2-1.8b"}}
	if got := familyIDs(c); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("local families %q, want %q", got, want)
	}
	if c.Prefs.MT != Successor(MT) || m.ready["hy-mt2-7b"] {
		t.Errorf("after delete: in use %q, downloaded again %v", c.Prefs.MT, m.ready["hy-mt2-7b"])
	}
	c.DeleteModel("hy-mt2-1.8b")
	if got := familyIDs(c); len(got) != 2 {
		t.Errorf("both deleted: local families %q", got)
	}
}
