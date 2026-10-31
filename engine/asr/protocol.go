package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Event and Token are protocol v1 records. MarshalJSON includes required zero-valued fields
// (voice off, time zero and empty final tokens) without adding fields of other event types.
type Event struct {
	Type, Engine, Version, Code, Message, ID string
	Protocol                                 int
	Tokens                                   []Token
}

type Token struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
	Conf  *float64 `json:"conf,omitempty"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	switch e.Type {
	case "hello":
		return json.Marshal(struct {
			Type     string `json:"type"`
			Protocol int    `json:"protocol"`
			Engine   string `json:"engine"`
			Version  string `json:"version"`
		}{e.Type, e.Protocol, e.Engine, e.Version})
	case "decoded":
		tokens := e.Tokens
		if tokens == nil {
			tokens = []Token{}
		}
		return json.Marshal(struct {
			Type   string  `json:"type"`
			ID     string  `json:"id"`
			Tokens []Token `json:"tokens"`
		}{e.Type, e.ID, tokens})
	case "error":
		return json.Marshal(struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}{e.Type, e.Code, e.Message})
	default:
		return nil, fmt.Errorf("unsupported output event %q", e.Type)
	}
}

// Command decodes stdin. Unknown commands and fields are forward-compatible.
type Command struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	PCM  string `json:"pcm"`
}

// readCommands preserves input ordering and propagates malformed lines/oversized input. EOF
// returns normally, which lets the entry point exit.
func readCommands(r io.Reader, apply func(Command) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var command Command
		if err := json.Unmarshal(sc.Bytes(), &command); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		if err := apply(command); err != nil {
			return err
		}
	}
	return sc.Err()
}
