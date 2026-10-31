package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSharedProtocolFixtures(t *testing.T) {
	b, err := os.ReadFile("../../../protocol/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Events  []Event `json:"events"`
		Streams []struct {
			Name, Input, Error string
			Received           int
		} `json:"streams"`
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, s := range fixture.Streams {
		t.Run(s.Name, func(t *testing.T) {
			var events []Event
			err := read(strings.NewReader(s.Input), func(e Event) { events = append(events, e) })
			if s.Error == "" && err != nil || s.Error != "" && (err == nil || !strings.Contains(err.Error(), s.Error)) {
				t.Fatalf("error %v; want %q", err, s.Error)
			}
			if len(events) != s.Received {
				t.Fatalf("received %d; want %d", len(events), s.Received)
			}
		})
	}
	volatile, final := fixture.Events[3], fixture.Events[4]
	if volatile.Tokens[0].Start != nil || final.Tokens[0].Start == nil || *final.Tokens[0].Start != 0 || *final.Tokens[0].Conf != .9 {
		t.Fatal("optional token times changed")
	}
	if fixture.Events[6].T != 1.234 || fixture.Events[7].On || fixture.Events[8].PCM != "AAD//wEA/38=" {
		t.Fatal("audio clock/voice/PCM changed")
	}
}

// The test binary is its own child process, avoiding shell/platform dependencies.
func TestEngineChild(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "engine-child-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "engine-child-")
	if mode == "bad" {
		fmt.Println("not json")
	} else {
		fmt.Println(`{"type":"hello","protocol":1}`)
	}
	if mode == "eof" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func childArgs(mode string) []string {
	return []string{"-test.run=^TestEngineChild$", "--", "engine-child-" + mode}
}

func TestChildShutdown(t *testing.T) {
	for _, mode := range []string{"eof", "hang", "bad"} {
		t.Run(mode, func(t *testing.T) {
			p, err := start(os.Args[0], childArgs(mode), func(Event) {})
			if err != nil {
				t.Fatal(err)
			}
			if mode != "bad" {
				p.Stop()
			}
			select {
			case <-p.Done():
			case <-time.After(3 * time.Second):
				_ = p.cmd.Process.Kill()
				t.Fatal("child did not end within shutdown deadline")
			}
			if mode == "bad" && p.Err() == nil {
				t.Fatal("protocol error lost")
			}
			if mode != "bad" && p.Err() != nil {
				t.Fatal(p.Err())
			}
		})
	}
}

func TestStopUnblocksAudio(t *testing.T) {
	p, err := start(os.Args[0], childArgs("hang"), func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	wrote := make(chan error, 1)
	go func() { wrote <- p.Audio(Event{PCM: strings.Repeat("A", 2<<20)}) }()
	time.Sleep(25 * time.Millisecond)
	p.Stop()
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked writer survived Stop")
	}
	select {
	case <-p.Done():
	case <-time.After(3 * time.Second):
		_ = p.cmd.Process.Kill()
		t.Fatal("child survived Stop")
	}
}

func TestRunProtocolFailureAndCancellation(t *testing.T) {
	// No caller deadline: the invalid protocol itself must terminate the child.
	done := make(chan error, 1)
	go func() { _, err := Run(context.Background(), os.Args[0], childArgs("bad")...); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "invalid character") {
			t.Fatalf("protocol error lost: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("protocol failure did not terminate Run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Run(ctx, os.Args[0], childArgs("hang")...)
	if err != context.DeadlineExceeded {
		t.Fatalf("cancellation error lost: %v", err)
	}
}
