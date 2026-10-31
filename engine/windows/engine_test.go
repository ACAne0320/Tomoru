package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSharedProtocolFixtures(t *testing.T) {
	b, err := os.ReadFile("../../protocol/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Events   []json.RawMessage
		Commands []json.RawMessage
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	produced := map[int]Event{
		2:  {Type: "state", State: "listening"},
		7:  {Type: "voice", On: false, T: 0},
		8:  {Type: "audio", T: 0, PCM: "AAD//wEA/38="},
		9:  {Type: "error", Code: "no-permission", Message: "off"},
		10: {Type: "access", State: "granted"},
		11: {Type: "app", ID: "example.app", Name: "Example"},
		12: {Type: "hello", Protocol: 1, Engine: "windows", Version: version},
		13: {Type: "area", X: -1920, Y: 40, Width: 300, Height: 120},
	}
	for index, event := range produced {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		json.Unmarshal(encoded, &got)
		json.Unmarshal(fixture.Events[index], &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("event %d: got %s want %s", index, encoded, fixture.Events[index])
		}
	}
	var input []byte
	for _, c := range fixture.Commands {
		var line bytes.Buffer
		if err := json.Compact(&line, c); err != nil {
			t.Fatal(err)
		}
		input = append(append(input, line.Bytes()...), '\n')
	}
	var got []Command
	if err := readCommands(bytes.NewReader(input), func(c Command) { got = append(got, c) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[1].Through != 1.234 || got[2].Type != "future-command" || got[3].Type != "decode" {
		t.Errorf("commands %+v", got)
	}
	if err := readCommands(strings.NewReader("{broken}\n"), func(Command) {}); err == nil {
		t.Error("malformed input accepted")
	}
}

// TestHearer sends what is heard in chunks of 100 ms, and silence for what was not, once nothing
// came for a while: time goes on as the audio does.
func TestHearer(t *testing.T) {
	var events []Event
	var voiced int
	h := NewHearer(func(e Event) { events = append(events, e) }, func(x []float32) { voiced += len(x) })
	now := time.Unix(0, 0)
	h.now = func() time.Time { return now }
	h.started = now

	h.Feed(make([]int16, chunk+100)) // a chunk and a little
	if len(events) != 1 || events[0].Type != "audio" || events[0].T != 0 || voiced != chunk {
		t.Fatalf("one chunk: %+v, voiced %d", events, voiced)
	}
	if b, _ := base64.StdEncoding.DecodeString(events[0].PCM); len(b) != 2*chunk {
		t.Errorf("a chunk of %d bytes", len(b))
	}

	now = now.Add(200 * time.Millisecond)
	h.FillSilence() // the source is not quiet yet
	if len(events) != 1 {
		t.Fatalf("filled too soon: %d", len(events))
	}
	now = now.Add(time.Second)
	h.FillSilence() // quiet for 1.2 s: silence up to 1.2 s of audio
	if len(events) != 12 || events[11].T != 1.1 {
		t.Fatalf("filled to %d chunks, the last at %v", len(events), events[len(events)-1].T)
	}
}

func TestHearerResumesWithoutSilenceBacklog(t *testing.T) {
	var events []Event
	h := NewHearer(func(e Event) { events = append(events, e) }, nil)
	now := time.Unix(0, 0)
	h.now = func() time.Time { return now }
	h.started = now
	h.Feed(make([]int16, chunk+100))

	// A suspended process must not replay the whole gap as audio. Three seconds is
	// enough to exercise the bound without allocating an hour of PCM on failure.
	now = now.Add(3 * time.Second)
	h.FillSilence()
	if h.sent > chunk+100+2*rate {
		t.Fatalf("resumed with %d samples of silence", h.sent-chunk-100)
	}
	before := h.sent
	now = now.Add(100 * time.Millisecond)
	h.FillSilence()
	if h.sent-before != chunk {
		t.Fatalf("the next tick still caught up: %d samples", h.sent-before)
	}
	for i, e := range events {
		if e.T != float64(i*chunk)/rate {
			t.Fatalf("audio clock jumped at chunk %d: %v", i, e.T)
		}
	}
}

// wav is a WAV file of PCM, 16-bit, of channels interleaved at r Hz.
func wav(r, channels int, samples []int16) []byte {
	var b bytes.Buffer
	data := 2 * len(samples)
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+data))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(channels), uint32(r), uint32(r * channels * 2), uint16(channels * 2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(data))
	binary.Write(&b, binary.LittleEndian, samples)
	return b.Bytes()
}

func TestDecodeWAV(t *testing.T) {
	// 0.1 s of 48 kHz stereo, a 400 Hz tone on the left only: half as loud, mono.
	in := make([]int16, 2*4800)
	for i := range 4800 {
		in[2*i] = int16(16000 * math.Sin(2*math.Pi*400*float64(i)/48000))
	}
	got, err := decodeWAV(wav(48000, 2, in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1600 {
		t.Fatalf("%d samples", len(got))
	}
	var peak int16
	for _, s := range got {
		peak = max(peak, s)
	}
	if peak < 7500 || peak > 8100 {
		t.Errorf("peak %d, want about 8000", peak)
	}
	same, err := decodeWAV(wav(16000, 1, []int16{1, -2, 3}))
	if err != nil || !reflect.DeepEqual(same, []int16{1, -2, 3}) {
		t.Errorf("16 kHz mono: %v, %v", same, err)
	}
	if _, err := decodeWAV([]byte("RIFF....WAVE")); err == nil {
		t.Error("no chunks accepted")
	}
}

// script is a VAD that speaks in the windows it is told to.
type script struct {
	speaking []bool
	i        int
}

func (s *script) Accept([]float32) { s.i++ }
func (s *script) Speaking() bool   { return s.i <= len(s.speaking) && s.speaking[s.i-1] }

// TestDetector places voice's start and stop at the start of the runs that decided them.
func TestDetector(t *testing.T) {
	// Voice decided at the 5th window, after 2 over; stopped at the 12th, after 3 under.
	s := &script{speaking: []bool{false, false, false, false, true, true, true, true, true, true, true, false}}
	var got []string
	d := NewDetector(s, func(on bool, at float64) {
		b, _ := json.Marshal(Event{Type: "voice", On: on, T: at})
		got = append(got, string(b))
	})
	d.Feed(make([]float32, 12*window+100))
	want := []string{`{"type":"voice","on":true,"t":0.096}`, `{"type":"voice","on":false,"t":0.288}`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
