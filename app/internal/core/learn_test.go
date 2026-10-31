package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"tomoru/internal/history"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

// until runs what is posted until done, or fails.
func (f *fake) until(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); !done(); time.Sleep(time.Millisecond) {
		f.mu.Lock()
		posted := f.posted
		f.posted = nil
		f.mu.Unlock()
		for _, fn := range posted {
			fn()
		}
		if time.Now().After(deadline) {
			t.Fatal("not done")
		}
	}
}

// keepSession keeps a session of scene id with lines a second apart, after a start.
func keepSession(t *testing.T, dir, id string, start time.Time, lines ...string) string {
	t.Helper()
	w := history.NewWriter(dir, history.Head{Scene: id, Src: "zh", Tgt: "en", Start: start})
	for i, l := range lines {
		w.Add(history.Entry{At: start.Add(time.Duration(i) * 70 * time.Second), Src: l, Tgt: "t"})
	}
	w.Close()
	rs, _ := history.List(dir)
	for _, r := range rs {
		if r.Head.Start.Equal(start) {
			return r.Path
		}
	}
	t.Fatal("not kept")
	return ""
}

// TestLearnFrom asks the user's chat model about a session two chunks long, and takes one of what
// it suggests, turning the other down.
func TestLearnFrom(t *testing.T) {
	c, f := scenesCore(t)
	c.deps.HistoryDir = t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reply := `{"terms":[{"heard":"刘北","src":"纽北","tgt":"Nürburgring","sure":true}]}`
		if strings.Contains(string(b), "舒七") {
			reply = `{"terms":[{"heard":"舒七","src":"SU7","tgt":"SU7","sure":true}]}`
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\ndata: [DONE]\n\n", reply)
	}))
	defer srv.Close()
	ep := c.SaveEndpoint(prefs.Endpoint{Name: "M", URL: srv.URL, Model: "m"}, "k")
	c.SaveEndpoint(prefs.Endpoint{Name: "Index", URL: srv.URL, Model: "i", Index: true}, "")
	if es := c.ChatEndpoints(); len(es) != 1 || es[0].ID != ep {
		t.Fatalf("chat endpoints %+v", es)
	}
	id := c.AddScene("雷军演讲")
	// 0 s and 70 s, then 140 s and 210 s: two chunks.
	path := keepSession(t, c.deps.HistoryDir, id, time.Now(), "舒七的研发", "到了关键的时候", "在刘北赛道上", "挑战刘北")

	c.LearnFrom([]string{path}, id, ep)
	f.until(t, func() bool { return c.Learning.Finished })
	l := c.Learning
	if l.Total != 2 || l.Done != 2 || l.Failed != 0 || len(l.Suggestions) != 2 || c.Prefs.Learn != ep {
		t.Fatalf("learning %+v", l)
	}
	if s := l.Suggestions[1]; s.Src != "纽北" || s.Count != 2 || !slices.Equal(s.Heard, []string{"刘北"}) {
		t.Errorf("suggestion %+v", s)
	}

	n, undo := c.TakeLearned(id, []bool{true, false})
	s, _ := c.SceneByID(id)
	if n != 1 || len(s.Terms) != 1 || s.Terms[0].Src != "SU7" || !s.Terms[0].Auto || !slices.Equal(s.Rejected, []string{"刘北"}) {
		t.Errorf("took %d: %+v", n, s)
	}
	undo()
	if s, _ := c.SceneByID(id); len(s.Terms) != 0 || len(s.Rejected) != 0 {
		t.Errorf("undone: %+v", s)
	}

	// Without a chat model's endpoint, it says so.
	c.LearnFrom([]string{path}, id, "nope")
	if c.Learning.Err != ErrNoChatEndpoint || c.Learning.Running {
		t.Errorf("learning %+v", c.Learning)
	}
}

type marked struct{}

func (marked) Translate(_ context.Context, r translate.Request, _ func(string)) (string, error) {
	return fmt.Sprintf("%s %v", r.Text, r.Terms), nil
}

// TestRetranslate translates again the lines the glossary concerns, with names heard wrong written
// right, and keeps them so; the others stay as they were.
func TestRetranslate(t *testing.T) {
	c, f := scenesCore(t)
	c.deps.HistoryDir = t.TempDir()
	c.deps.Translator = marked{}
	id := c.AddScene("雷军演讲")
	c.AddTerm(id, scene.Term{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}})
	path := keepSession(t, c.deps.HistoryDir, id, time.Now(), "在刘北赛道上", "你好")
	c.Retranslate([]string{path}, id)
	f.until(t, func() bool { return c.Retranslating.Finished })
	if r := c.Retranslating; r.Total != 1 || r.Done != 1 || r.Err != nil {
		t.Fatalf("retranslating %+v", r)
	}
	es, _ := history.Read(path)
	if es[0].Src != "在纽北赛道上" || es[0].Heard != "在刘北赛道上" || es[0].Tgt != "在纽北赛道上 [{纽北 Nürburgring}]" || es[1].Tgt != "t" {
		t.Errorf("lines %+v", es)
	}
}
