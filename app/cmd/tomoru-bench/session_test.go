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
	for id := 1; id <= 12; id++ {
		recordLine(&result, lines, 0, live.Line{ID: id, Src: fmt.Sprint(id)})
	}
	// Display eviction is irrelevant: the old sentences still receive independent events.
	for id := 1; id <= 12; id++ {
		line := lines[id]
		line.Tgt = "partial"
		recordLine(&result, lines, 1, line)
		line.Done = true
		if id == 12 {
			line.Err = context.Canceled
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
	if commits != 12 || finals != 12 {
		t.Fatalf("got %d commits and %d completions", commits, finals)
	}
}
