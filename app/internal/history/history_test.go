package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 10, 19, 15, 30, 0, 0, time.Local)
	w := NewWriter(dir, Head{Scene: "ff7", Name: "FF7", Src: "ja", Tgt: "zh", Start: start})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if rs, _ := List(dir); len(rs) != 0 {
		t.Fatalf("a session with no lines kept: %+v", rs)
	}
	w.Add(Entry{At: start.Add(time.Second), Src: "メテオの脅威", Heard: "メデオの脅威", Tgt: "陨石的威胁"})
	w.Add(Entry{At: start.Add(2 * time.Second), Src: "クラウド", Err: "behind"})
	w.Close()
	// Another in the same second, in the general scene.
	g := NewWriter(dir, Head{Src: "en", Tgt: "zh", Start: start})
	g.Add(Entry{Src: "Hi", Tgt: "嗨"})
	g.Close()
	g2 := NewWriter(dir, Head{Src: "en", Tgt: "zh", Start: start.Add(time.Minute)})
	g2.Add(Entry{Src: "Bye", Tgt: "拜"})
	g2.Close()
	os.WriteFile(filepath.Join(dir, "general", "notes.jsonl"), []byte("hello\n"), 0o644)
	// The app quit in the middle of a line: it is not one.
	if f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND, 0); err == nil {
		f.WriteString(`{"at":"2026-10-19T15:30:03+08:00","src":"ティ`)
		f.Close()
	}

	rs, err := List(dir)
	if err != nil || len(rs) != 3 || rs[0].Head.Src != "en" || rs[0].Lines != 1 {
		t.Fatalf("%+v %v", rs, err)
	}
	var ff7 Record
	for _, r := range rs {
		if r.Head.Scene == "ff7" {
			ff7 = r
		}
	}
	if ff7.Lines != 2 || ff7.Head.Name != "FF7" || filepath.Base(filepath.Dir(ff7.Path)) != "ff7" {
		t.Errorf("ff7 %+v", ff7)
	}
	es, err := Read(ff7.Path)
	if err != nil || len(es) != 2 || es[0].Heard != "メデオの脅威" || es[1].Err != "behind" {
		t.Errorf("%+v %v", es, err)
	}

	if _, err := Delete([]string{ff7.Path, rs[0].Path}); err != nil {
		t.Fatal(err)
	}
	if rs, _ := List(dir); len(rs) != 1 || rs[0].Head.Start != start {
		t.Fatalf("deleted: %+v", rs)
	}
}

func TestUndo(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir, Head{Src: "en", Tgt: "zh", Start: time.Now()})
	w.Add(Entry{Src: "Hi", Tgt: "嗨"})
	w.Close()
	rs, _ := List(dir)
	undo, err := Delete([]string{rs[0].Path})
	if err != nil {
		t.Fatal(err)
	}
	undo()
	if rs, _ := List(dir); len(rs) != 1 {
		t.Fatalf("undone: %+v", rs)
	}
	Delete([]string{rs[0].Path})
	if rs, _ := List(dir); len(rs) != 0 {
		t.Fatalf("deleted: %+v", rs)
	}
	Purge(dir)
	if ms, _ := filepath.Glob(filepath.Join(dir, "*", "*")); len(ms) != 0 {
		t.Errorf("not purged: %v", ms)
	}
}

func TestRewrite(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	w := NewWriter(dir, Head{Scene: "ff7", Src: "ja", Tgt: "zh", Start: start})
	w.Add(Entry{Src: "メデオ", Tgt: "梅朵"})
	w.Close()
	rs, _ := List(dir)
	if err := Rewrite(rs[0].Path, []Entry{{Src: "メテオ", Heard: "メデオ", Tgt: "陨石"}}); err != nil {
		t.Fatal(err)
	}
	head, err := ReadHead(rs[0].Path)
	es, _ := Read(rs[0].Path)
	if err != nil || head.Scene != "ff7" || len(es) != 1 || es[0].Tgt != "陨石" {
		t.Errorf("%+v %+v %v", head, es, err)
	}
	if ms, _ := filepath.Glob(filepath.Join(dir, "*", "*.tmp")); len(ms) != 0 {
		t.Errorf("left %v", ms)
	}
}

// TestRewriteWhileRecording learns a session still recording: its lines rewritten, the one added
// since kept after them, and the lines after going on into the same file. On Windows the file
// could not be replaced while the writer held it open.
func TestRewriteWhileRecording(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir, Head{Src: "ja", Tgt: "zh", Start: time.Now()})
	w.Add(Entry{Src: "メデオ", Tgt: "梅德奥"})
	w.Add(Entry{Src: "クラウド", Tgt: "克劳德"})
	rs, _ := List(dir)
	path := rs[0].Path
	es, _ := Read(path)
	w.Add(Entry{Src: "ティファ", Tgt: "蒂法"}) // while the names are learned
	es[0].Heard, es[0].Src = es[0].Src, "メテオ"
	if err := Rewrite(path, es); err != nil {
		t.Fatal(err)
	}
	if err := w.Add(Entry{Src: "エアリス", Tgt: "爱丽丝"}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || len(got) != 4 || got[0].Src != "メテオ" || got[0].Heard != "メデオ" || got[2].Src != "ティファ" || got[3].Src != "エアリス" {
		t.Errorf("%+v, %v", got, err)
	}
	if rs, _ := List(dir); len(rs) != 1 {
		t.Errorf("sessions %+v", rs)
	}
}

// TestDeleteWhileRecording deletes a session still recording: the lines after go to a session of
// their own, which putting the deleted one back leaves alone.
func TestDeleteWhileRecording(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir, Head{Src: "en", Tgt: "zh", Start: time.Now()})
	w.Add(Entry{Src: "One", Tgt: "一"})
	rs, _ := List(dir)
	undo, err := Delete([]string{rs[0].Path})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Add(Entry{Src: "Two", Tgt: "二"}); err != nil {
		t.Fatal(err)
	}
	undo()
	rs, _ = List(dir)
	if len(rs) != 2 || rs[0].Lines != 1 || rs[1].Lines != 1 || rs[0].Path == rs[1].Path {
		t.Fatalf("sessions %+v", rs)
	}
}

// TestRemovedWhileRecording has a session's file removed outside the app, as from Finder, while it
// records: the lines after go to a new file, which takes the same name.
func TestRemovedWhileRecording(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(dir, Head{Src: "en", Tgt: "zh", Start: time.Now()})
	w.Add(Entry{Src: "One", Tgt: "一"})
	rs, _ := List(dir)
	if err := os.Remove(rs[0].Path); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- w.Add(Entry{Src: "Two", Tgt: "二"}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("adding a line hangs")
	}
	if rs, _ := List(dir); len(rs) != 1 || rs[0].Lines != 1 {
		t.Fatalf("sessions %+v", rs)
	}
}
