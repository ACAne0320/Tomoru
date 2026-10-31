package core

import (
	"os"
	"path/filepath"
	"testing"

	"tomoru/internal/history"
	"tomoru/internal/live"
)

func TestHistoryRetriesFailedWritesAcrossSessionRestart(t *testing.T) {
	base := t.TempDir()
	blocked := filepath.Join(base, "history")
	if err := os.WriteFile(blocked, []byte("blocks directory creation"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &HistoryService{}
	s.configure(blocked, true, history.Head{Src: "en", Tgt: "zh"})
	first := s.begin()
	policy := s.policy.Load()
	for id, text := range []string{"first", "second"} {
		first.accept(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: id + 1, Src: text, Tgt: "translated", Done: true}}, policy)
	}
	s.drain()
	if s.History.Err == nil || first.next != 1 || len(first.lines) != 2 {
		t.Fatal("failed write was acknowledged")
	}
	s.drain()
	if first.next != 1 || len(first.lines) != 2 {
		t.Fatal("repeated failure lost pending lines")
	}

	// Starting another session must retain the failed run and preserve the session boundary.
	second := s.begin()
	second.accept(live.LineEvent{Kind: live.LineCompleted, Line: live.Line{ID: 1, Src: "new session", Done: true}}, policy)
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	s.drain()
	s.closeHistory()
	if s.History.Err != nil || len(s.retryRuns) != 0 || len(first.lines) != 0 || len(second.lines) != 0 {
		t.Fatalf("retry incomplete: %v", s.History.Err)
	}
	records, err := history.List(blocked)
	if err != nil || len(records) != 2 {
		t.Fatalf("session boundary lost: %+v %v", records, err)
	}
	counts := map[string]int{}
	for _, record := range records {
		entries, err := history.Read(record.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			counts[entry.Src]++
		}
		if len(entries) == 2 && (entries[0].Src != "first" || entries[1].Src != "second") {
			t.Fatalf("retry reordered lines: %+v", entries)
		}
	}
	for _, text := range []string{"first", "second", "new session"} {
		if counts[text] != 1 {
			t.Fatalf("%q written %d times", text, counts[text])
		}
	}
	// Draining again cannot repeat already acknowledged sentences.
	s.drain()
	if len(first.lines) != 0 || len(second.lines) != 0 {
		t.Fatal("acknowledged lines returned")
	}
}

func TestHistoryRetryStillHonorsRevokedConsent(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := &HistoryService{}
	s.configure(blocked, true, history.Head{})
	r := s.begin()
	r.accept(live.LineEvent{Line: live.Line{ID: 1, Src: "revoked", Done: true}}, s.policy.Load())
	s.drain()
	s.configure(blocked, false, history.Head{})
	s.configure(blocked, true, history.Head{})
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	s.drain()
	if len(r.lines) != 0 {
		t.Fatal("revoked sentence remained pending")
	}
	if records, _ := history.List(blocked); len(records) != 0 {
		t.Fatal("retry wrote revoked speech")
	}
}
