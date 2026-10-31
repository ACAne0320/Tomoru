package openai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tomoru/internal/engine"
)

// fakeRealtime is the service: it takes a session's configuration, and answers each commit as
// script says, with the turn's item ID.
type fakeRealtime struct {
	t       *testing.T
	refuse  string // an error to answer the configuration with
	mu      sync.Mutex
	config  map[string]any
	sent    int // bytes of audio appended
	commits int
	// script answers a commit (its number, from 0, and item ID) with events.
	script func(n int, item string) []map[string]any
	conn   *websocket.Conn
}

func (f *fakeRealtime) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "Bearer bad" {
		http.Error(w, "no", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/v1/realtime" || r.URL.Query().Get("intent") != "transcription" {
		f.t.Errorf("connected to %s", r.URL)
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 22) // a turn starts with its preroll
	f.mu.Lock()
	f.conn = c
	f.mu.Unlock()
	ctx := context.Background()
	send := func(ev map[string]any) {
		b, _ := json.Marshal(ev)
		c.Write(ctx, websocket.MessageText, b)
	}
	send(map[string]any{"type": "session.created"})
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		var ev map[string]any
		json.Unmarshal(b, &ev)
		switch ev["type"] {
		case "session.update":
			f.mu.Lock()
			f.config = ev
			f.mu.Unlock()
			if f.refuse != "" {
				send(map[string]any{"type": "error", "error": map[string]any{"type": "invalid_request_error", "message": f.refuse}})
			} else {
				send(map[string]any{"type": "session.updated"})
			}
		case "input_audio_buffer.append":
			pcm, _ := base64.StdEncoding.DecodeString(ev["audio"].(string))
			f.mu.Lock()
			f.sent += len(pcm)
			f.mu.Unlock()
		case "input_audio_buffer.commit":
			f.mu.Lock()
			n := f.commits
			f.commits++
			f.mu.Unlock()
			item := "item_" + string(rune('a'+n))
			send(map[string]any{"type": "input_audio_buffer.committed", "item_id": item})
			for _, e := range f.script(n, item) {
				send(e)
			}
		}
	}
}

func delta(item, text string) map[string]any {
	return map[string]any{"type": "conversation.item.input_audio_transcription.delta", "item_id": item, "delta": text}
}

func completed(item, text string) map[string]any {
	return map[string]any{"type": "conversation.item.input_audio_transcription.completed", "item_id": item, "transcript": text}
}

// feedRT gives r seconds of audio, in 0.1 s events, with voice for the spans, said 64 ms late.
func feedRT(r *Realtime, spans [][2]float64, seconds float64) {
	chunk := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/10))
	r.Voice(engine.Event{Type: "voice", On: false, T: 0}) // the engine detects voice
	next := 0
	on := false
	for i := 0; i < int(seconds*10+.5); i++ {
		t := float64(i) / 10
		if next < len(spans) {
			s := spans[next]
			if !on && t >= s[0]+.064 {
				r.Voice(engine.Event{Type: "voice", On: true, T: s[0]})
				on = true
			}
			if on && t >= s[1]+.064 {
				r.Voice(engine.Event{Type: "voice", On: false, T: s[1]})
				on = false
				next++
			}
		}
		r.Audio(engine.Event{Type: "audio", PCM: chunk})
	}
}

// TestRealtimeSession sends a turn of speech and nothing else, commits it at the pause, shows the
// streamed text, and gives the final text in the order the turns were committed, though the
// service completes them out of it.
func TestRealtimeSession(t *testing.T) {
	f := &fakeRealtime{t: t}
	f.script = func(n int, item string) []map[string]any {
		if n == 0 {
			return []map[string]any{delta(item, " Hello"), delta(item, " there")} // completed after the next
		}
		return []map[string]any{delta(item, " How are"), completed(item, "How are you?"), completed("item_a", "Hello there.")}
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	defer srv.Close()

	var mu sync.Mutex
	var evs []engine.Event
	r := StartRealtime(Config{URL: srv.URL, Key: "k", Keywords: []string{"一心", "葦名"}, About: "只狼的对话"}, "ja", func(ev engine.Event) {
		mu.Lock()
		evs = append(evs, ev)
		mu.Unlock()
	})
	feedRT(r, [][2]float64{{1, 2}, {3, 4}}, 6)
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		finals := 0
		for _, e := range evs {
			if e.Type == "result" && e.Final {
				finals++
			}
		}
		mu.Unlock()
		if finals == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	r.Stop()
	<-r.Done()
	if r.Err() != nil {
		t.Fatal(r.Err())
	}

	f.mu.Lock()
	input := f.config["session"].(map[string]any)["audio"].(map[string]any)["input"].(map[string]any)
	format, tr := input["format"].(map[string]any), input["transcription"].(map[string]any)
	if format["rate"] != float64(rtRate) || tr["model"] != LiveModel || input["turn_detection"] != nil || tr["languages"].([]any)[0] != "ja" ||
		len(tr["keywords"].([]any)) != 2 || tr["prompt"] != "只狼的对话" {
		t.Errorf("configured %v", input)
	}
	// Two turns of a second each, with their preroll and the pause that ends them, at 24 kHz: not
	// the silence between them.
	if sec := float64(f.sent) / 2 / rtRate; sec < 3 || sec > 4.5 || f.commits != 2 {
		t.Errorf("sent %.2f s in %d turns", sec, f.commits)
	}
	f.mu.Unlock()
	if s := r.Sent(); s < 3 || s > 4.5 {
		t.Errorf("Sent %.2f", s)
	}

	var finals []string
	sawVolatile := false
	for _, e := range evs {
		switch {
		case e.Type == "result" && e.Final:
			finals = append(finals, e.Text())
			if e.Start >= e.End || *e.Tokens[0].Start != e.Start {
				t.Errorf("timed %+v", e)
			}
		case e.Type == "result" && e.Text() == " Hello there":
			sawVolatile = true
		}
	}
	if strings.Join(finals, "|") != " Hello there.| How are you?" || !sawVolatile {
		t.Errorf("finals %q, volatile seen %v", finals, sawVolatile)
	}
}

// TestRealtimeRefused ends a session the service refused: its key, or its configuration.
func TestRealtimeRefused(t *testing.T) {
	f := &fakeRealtime{t: t, refuse: "Invalid 'session.audio.input.format.rate'"}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	defer srv.Close()
	r := StartRealtime(Config{URL: srv.URL, Key: "bad"}, "en", func(engine.Event) {})
	<-r.Done()
	if !errors.Is(r.Err(), ErrAuth) {
		t.Errorf("a refused key: %v", r.Err())
	}
	var e *Error
	if _, err := TestRealtime(context.Background(), Config{URL: srv.URL, Key: "k"}); !errors.As(err, &e) || !strings.Contains(e.Message, "rate") {
		t.Errorf("a refused configuration: %v", err)
	}
}

// TestUpsample makes 16 kHz 24 kHz, through the chunks of a turn alike.
func TestUpsample(t *testing.T) {
	in := make([]byte, 0, 200)
	for i := range 100 {
		in = binary.LittleEndian.AppendUint16(in, uint16(int16(i*30)))
	}
	var whole, parts upsampler
	a := whole.convert(in)
	b := append(parts.convert(in[:62]), parts.convert(in[62:])...)
	if len(a) != len(b) || string(a) != string(b) || len(a)/2 < 148 || len(a)/2 > 150 {
		t.Fatalf("%d and %d samples", len(a)/2, len(b)/2)
	}
	// A ramp stays a ramp: each sample two thirds of a step after the one before.
	for i := 1; i < len(a)/2; i++ {
		d := int16(binary.LittleEndian.Uint16(a[2*i:])) - int16(binary.LittleEndian.Uint16(a[2*i-2:]))
		if d != 20 {
			t.Fatalf("sample %d steps %d", i, d)
		}
	}
}

// TestRealtimeOrder keeps a turn's text before the next's, when its text comes after the next's
// streams: the next turn's text waits for it.
func TestRealtimeOrder(t *testing.T) {
	f := &fakeRealtime{t: t}
	f.script = func(n int, item string) []map[string]any {
		if n == 0 {
			return nil // a short turn: nothing streamed yet
		}
		return []map[string]any{delta(item, " そなたからも何かあるのか？"), completed("item_a", "狼よ、"), completed(item, "そなたからも何かあるのか？")}
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	defer srv.Close()
	var mu sync.Mutex
	var evs []engine.Event
	r := StartRealtime(Config{URL: srv.URL, Key: "k"}, "ja", func(ev engine.Event) {
		mu.Lock()
		evs = append(evs, ev)
		mu.Unlock()
	})
	feedRT(r, [][2]float64{{1, 1.6}, {2.4, 4}}, 6)
	r.Stop()
	<-r.Done()
	var order []string
	for _, e := range evs {
		if e.Type == "result" && e.Text() != "" {
			order = append(order, fmt.Sprint(e.Final, ":", strings.TrimSpace(e.Text())))
		}
	}
	if len(order) == 0 || order[0] != "true:狼よ、" {
		t.Errorf("results %q", order)
	}
}
