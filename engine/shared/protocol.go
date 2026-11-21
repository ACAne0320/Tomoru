package shared

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

const version = "0.1.0"

type Event struct {
	Type, Engine, Version, State, Code, Message, ID, Name, PCM string
	Protocol, X, Y, Width, Height                              int
	On, Playing                                                bool
	T                                                          float64
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
	case "state", "access":
		return json.Marshal(struct {
			Type  string `json:"type"`
			State string `json:"state"`
		}{e.Type, e.State})
	case "voice":
		return json.Marshal(struct {
			Type string  `json:"type"`
			On   bool    `json:"on"`
			T    float64 `json:"t"`
		}{e.Type, e.On, e.T})
	case "audio":
		return json.Marshal(struct {
			Type string  `json:"type"`
			T    float64 `json:"t"`
			PCM  string  `json:"pcm"`
		}{e.Type, e.T, e.PCM})
	case "app":
		return json.Marshal(struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Name    string `json:"name"`
			Playing bool   `json:"playing"`
		}{e.Type, e.ID, e.Name, e.Playing})
	case "area":
		return json.Marshal(struct {
			Type   string `json:"type"`
			X      int    `json:"x"`
			Y      int    `json:"y"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		}{e.Type, e.X, e.Y, e.Width, e.Height})
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

var out = struct {
	sync.Mutex
	w io.Writer
}{w: os.Stdout}

func Send(e Event) {
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("protocol encoding: %v", err)
		os.Exit(1)
	}
	out.Lock()
	defer out.Unlock()
	if _, err := out.w.Write(append(b, '\n')); err != nil {
		os.Exit(1)
	}
}

func Fail(code, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	Send(Event{Type: "error", Code: code, Message: msg})
	log.Printf("error %s: %s", code, msg)
	os.Exit(1)
}

type Command struct {
	Type    string  `json:"type"`
	Through float64 `json:"through"`
}

func readCommands(r io.Reader, apply func(Command)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var c Command
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		apply(c)
	}
	return sc.Err()
}
