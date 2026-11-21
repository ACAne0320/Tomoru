package main

import (
	"context"
	"fmt"
	"testing"

	"tomoru/internal/live"
)

func TestRecorderRetainsBacklogAndPartialCompletions(t *testing.T) {
	var result Result
	lines := map[int]live.Line{}
	n := live.MaxLines + 2
	for id := 1; id <= n; id++ {
		recordLine(&result, lines, 0, live.Line{ID: id, Src: fmt.Sprint(id)})
	}
	for id := 1; id <= n; id++ {
		line := lines[id]
		line.Tgt = "partial"
		recordLine(&result, lines, 1, line)
		line.Outcome = live.Translated
		if id == n {
			line.Outcome, line.Err = live.Canceled, context.Canceled
		}
		recordLine(&result, lines, 2, line)
		recordLine(&result, lines, 2, line)
	}
	commits, finals := 0, 0
	for _, event := range result.Events {
		if event.Src != "" {
			commits++
		}
		if event.Done {
			finals++
			if event.Tgt == nil || *event.Tgt != "partial" {
				t.Fatal("partial translation lost")
			}
		}
	}
	if commits != n || finals != n {
		t.Fatalf("got %d commits and %d completions", commits, finals)
	}
}
