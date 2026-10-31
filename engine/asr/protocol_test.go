package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// fakeRec decodes a character every 0.1 s of what it is given, with spaces between, as SenseVoice
// puts in Japanese, and a full stop at the end.
type fakeRec struct{ lens []int }

func (r *fakeRec) Decode(s []float32) []Piece {
	r.lens = append(r.lens, len(s))
	d := float64(len(s)) / rate
	var out []Piece
	for i := 0; float64(i)*.1 < d; i++ {
		out = append(out, Piece{Text: "あ", At: float64(i) * .1}, Piece{Text: " ", At: float64(i)*.1 + .05})
	}
	return append(out, Piece{Text: "。", At: d})
}

func TestSharedProtocolFixtures(t *testing.T) {
	b, err := os.ReadFile("../../protocol/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Events   []json.RawMessage
		Commands []json.RawMessage
		Samples  int
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	zero, half := 0.0, .48
	produced := map[int]Event{
		1:  {Type: "hello", Protocol: 1, Engine: "asr", Version: version},
		9:  {Type: "error", Code: "no-permission", Message: "off"},
		15: {Type: "decoded", ID: "7", Tokens: []Token{{Text: "Hello", Start: &zero}, {Text: ".", Start: &half}}},
		16: {Type: "decoded", ID: "8"},
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
	for _, command := range fixture.Commands {
		line, err := json.Marshal(command) // one line each
		if err != nil {
			t.Fatal(err)
		}
		input = append(input, line...)
		input = append(input, '\n')
	}
	rec := &fakeRec{}
	var answers []Event
	err = readCommands(bytes.NewReader(input), func(c Command) error {
		if c.Type != "decode" {
			return nil // audio, finalize and unknown commands are not tomoru-asr's
		}
		ev, err := decode(rec, "en", c)
		answers = append(answers, ev)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(answers) != 1 || answers[0].ID != "7" || len(rec.lens) != 1 || rec.lens[0] != fixture.Samples {
		t.Fatalf("decoded %v of %d samples; want one of %d", answers, rec.lens, fixture.Samples)
	}
	// EOF is successful, including empty stdin; malformed lines and half samples are not.
	if err := readCommands(strings.NewReader(""), func(Command) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := readCommands(strings.NewReader("{broken}\n"), func(Command) error { return nil }); err == nil {
		t.Fatal("malformed input accepted")
	}
	if _, err := decode(rec, "en", Command{Type: "decode", PCM: "AA=="}); err == nil {
		t.Fatal("half a PCM sample accepted")
	}
}

// TestDecode answers a stretch with its tokens timed from its start, without the spaces SenseVoice
// puts between Japanese words; and nothing at all for no audio.
func TestDecode(t *testing.T) {
	pcm := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/5)) // 0.2 s of silence
	ev, err := decode(&fakeRec{}, "ja", Command{Type: "decode", ID: "3", PCM: pcm})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, tok := range ev.Tokens {
		text.WriteString(tok.Text)
	}
	if ev.Type != "decoded" || ev.ID != "3" || text.String() != "ああ。" || *ev.Tokens[1].Start != .1 || *ev.Tokens[2].Start != .2 {
		t.Fatalf("decoded %+v", ev)
	}
	empty := &fakeRec{}
	if ev, err := decode(empty, "ja", Command{Type: "decode", ID: "4"}); err != nil || len(ev.Tokens) != 0 || len(empty.lens) != 0 {
		t.Fatalf("no audio: %+v %v, decoded %d times", ev, err, len(empty.lens))
	}
}
