package batch

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"tomoru/internal/engine"
)

var caps = Caps{Volatile: .4, Gap: .25, Longest: 12}

// near tells whether time v is t, within a window of the voice detector.
func near(v float64, t float64) bool {
	return math.Abs(v-t) <= float64(window)/rate
}

// fakeDec decodes a character every 0.1 s of what it is given, and a full stop at the end; with
// stop, a full stop after the character at that time too, as the end of a sentence, until a
// shorter stretch shows the runner went on after it.
type fakeDec struct {
	lens  []float64
	fail  error
	stop  float64
	delay time.Duration // each decode takes
	done  chan struct{}
}

func newFake() *fakeDec { return &fakeDec{done: make(chan struct{})} }

func (d *fakeDec) Decode(_ context.Context, pcm []byte) ([]Piece, error) {
	time.Sleep(d.delay)
	if d.fail != nil {
		return nil, d.fail
	}
	s := float64(len(pcm)/2) / rate
	if n := len(d.lens); n > 0 && s < d.lens[n-1] {
		d.stop = 0 // the stretch starts after the sentence now: it is said once
	}
	d.lens = append(d.lens, s)
	var out []Piece
	for i := 0; float64(i)*.1 < s; i++ {
		at := float64(i) * .1
		out = append(out, Piece{Text: "あ", At: at})
		if d.stop > 0 && math.Abs(at-d.stop) < .01 {
			out = append(out, Piece{Text: "。", At: at + .05})
		}
	}
	return append(out, Piece{Text: "。", At: s}), nil
}

func (d *fakeDec) Stop() {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
}
func (d *fakeDec) Done() <-chan struct{} { return d.done }
func (d *fakeDec) Err() error            { return nil }

// feed gives r seconds of audio, in the engine's 0.1 s events, with its voice detection's events
// for spans of voice: on 64 ms after voice starts, off 96 ms after it stops, each at the start of
// those (docs/engine-protocol.md). Without spans, no voice event at all.
func feed(r *Runner, spans [][2]float64, seconds float64) error {
	type vev struct {
		at float64 // when the engine says it
		ev engine.Event
	}
	var vs []vev
	if spans != nil {
		vs = append(vs, vev{0, engine.Event{Type: "voice", On: false, T: 0}})
	}
	for _, s := range spans {
		vs = append(vs, vev{s[0] + .064, engine.Event{Type: "voice", On: true, T: s[0]}}, vev{s[1] + .096, engine.Event{Type: "voice", On: false, T: s[1]}})
	}
	chunk := make([]byte, 2*rate/10)
	for i := 0; i < int(seconds*10+.5); i++ {
		t := float64(i) / 10
		for len(vs) > 0 && vs[0].at <= t {
			if err := r.take(input{voice: &vs[0].ev}); err != nil {
				return err
			}
			vs = vs[1:]
		}
		if err := r.take(input{pcm: chunk}); err != nil {
			return err
		}
	}
	return nil
}

// run runs a runner's logic on the test's goroutine: no loop, so nothing is skipped.
func run(t *testing.T, spans [][2]float64, seconds float64) ([]engine.Event, *fakeDec, *Runner) {
	t.Helper()
	var evs []engine.Event
	dec := newFake()
	r := &Runner{dec: dec, caps: caps, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, spans, seconds); err != nil {
		t.Fatal(err)
	}
	return evs, dec, r
}

func of(evs []engine.Event, typ string) []engine.Event {
	var out []engine.Event
	for _, e := range evs {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func TestStretch(t *testing.T) {
	evs, dec, _ := run(t, [][2]float64{{1, 3}}, 5)
	voice := of(evs, "voice")
	if len(voice) != 2 || voice[0].T != 1 || !voice[0].On || voice[1].T != 3 || voice[1].On {
		t.Errorf("voice %v", voice)
	}
	res := of(evs, "result")
	var vol int
	for _, r := range res[:len(res)-1] {
		if r.Final || !near(r.Start, 1-preroll) {
			t.Errorf("volatile %v", r)
		}
		if text := r.Tokens[0].Text; strings.HasSuffix(text, "。") {
			t.Errorf("volatile ends a sentence: %q", text)
		}
		vol++
	}
	if vol < 3 {
		t.Errorf("%d volatile results", vol)
	}
	f := res[len(res)-1]
	toks := f.Tokens
	// From the preroll to the end of the gap that ended it.
	if !f.Final || !near(f.Start, 1-preroll) || !near(f.End, 3+caps.Gap) || toks[0].Text != "あ" ||
		*toks[0].Start != f.Start || !near(*toks[1].Start, 1-preroll+.1) {
		t.Errorf("final %v", f)
	}
	if clocks := of(evs, "clock"); len(clocks) != 50 || clocks[49].T != 5.0 {
		t.Errorf("%d clocks, last %v", len(clocks), clocks[len(clocks)-1])
	}
	// The first volatile decode shows nothing, having none to agree with; the final decodes too.
	if len(dec.lens) != vol+2 {
		t.Errorf("decoded %d times", len(dec.lens))
	}
}

// TestEarly makes a sentence the decoder ended final while the stretch goes on, once two decodes
// agree on it and on speech after it; the stretch goes on from after it, decoded without it.
func TestEarly(t *testing.T) {
	var evs []engine.Event
	dec := newFake()
	dec.stop = .5 // 0.5 s into the stretch, 1.1 s of audio: it starts at 0.6 with its preroll
	r := &Runner{dec: dec, caps: caps, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, [][2]float64{{1, 6}}, 7); err != nil {
		t.Fatal(err)
	}
	var finals []engine.Event
	for _, e := range of(evs, "result") {
		if e.Final {
			finals = append(finals, e)
		}
	}
	if len(finals) != 2 {
		t.Fatalf("finals %+v", finals)
	}
	first, rest := finals[0], finals[1]
	if text := first.Text(); text != "ああああああ。" || !near(first.Start, 1-preroll) || math.Abs(first.End-(1-preroll+(.55+.6)/2)) > .001 {
		t.Errorf("early %q %v-%v", text, first.Start, first.End)
	}
	if rest.Start != first.End || !near(rest.End, 6+caps.Gap) || strings.Contains(strings.TrimSuffix(rest.Text(), "。"), "。") {
		t.Errorf("the rest %q %v-%v", rest.Text(), rest.Start, rest.End)
	}
	// A sentence the decoder ended last, with nothing after it yet, waits.
	if n := Sentences(pieces("あ", "。")); n != 0 {
		t.Errorf("%d pieces of a sentence with nothing after it", n)
	}
	if n := Sentences(pieces("あ", "。", "い", "！", "う")); n != 4 {
		t.Errorf("%d pieces, want through the last end", n)
	}
}

// TestStops leaves the full stop a decoder ends every decode with out of final results, where the
// stretch ends: no sentence ends there.
func TestStops(t *testing.T) {
	var evs []engine.Event
	r := &Runner{dec: newFake(), caps: Caps{Volatile: .4, Gap: .25, Longest: 12, Stops: true}, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, [][2]float64{{1, 3}}, 5); err != nil {
		t.Fatal(err)
	}
	res := of(evs, "result")
	if f := res[len(res)-1]; !f.Final || strings.HasSuffix(f.Text(), "。") || !strings.HasSuffix(f.Text(), "あ") {
		t.Errorf("final %q", f.Text())
	}
}

// TestUntimed spreads an untimed decoder's stretch over its pieces, makes no sentence final while
// the stretch goes on (the segmenter commits it from the volatile text), and ends a stretch past
// Longest at the next pause, however short, else past twice Longest where it is.
func TestUntimed(t *testing.T) {
	untimed := Caps{Volatile: .4, Gap: .25, Longest: 12, Untimed: true}
	run := func(spans [][2]float64, seconds float64, stop float64) []engine.Event {
		var evs []engine.Event
		dec := newFake()
		dec.stop = stop
		r := &Runner{dec: dec, caps: untimed, on: func(e engine.Event) { evs = append(evs, e) }}
		if err := feed(r, spans, seconds); err != nil {
			t.Fatal(err)
		}
		return evs
	}
	finals := func(evs []engine.Event) (out []engine.Event) {
		for _, e := range of(evs, "result") {
			if e.Final {
				out = append(out, e)
			}
		}
		return out
	}

	evs := run([][2]float64{{1, 6}}, 7, .5)
	f := finals(evs)
	if len(f) != 1 || !strings.Contains(strings.TrimSuffix(f[0].Text(), "。"), "。") {
		t.Fatalf("finals %v", f)
	}
	toks := f[0].Tokens
	for i := 1; i < len(toks); i++ {
		if *toks[i].Start < *toks[i-1].Start || *toks[i].End > f[0].End+.001 {
			t.Fatalf("token %d at %v-%v after %v", i, *toks[i].Start, *toks[i].End, *toks[i-1].Start)
		}
	}
	sawEnd := false
	for _, e := range of(evs, "result") {
		if !e.Final && strings.Contains(e.Text(), "。") {
			sawEnd = true // a sentence end in the volatile text, for the segmenter
		}
	}
	if !sawEnd {
		t.Error("no sentence end in the volatile text")
	}

	// A sentence ended, and a pause of 0.1 s after it, shorter than the gap: the stretch ends there.
	f = finals(run([][2]float64{{1, 4}, {4.1, 6}}, 7, .5))
	if len(f) != 2 || math.Abs(f[0].End-4) > .15 {
		t.Errorf("after a sentence, at a short pause: %v", f)
	}
	// No sentence ended: a short pause goes on.
	if f = finals(run([][2]float64{{1, 4}, {4.1, 6}}, 7, 0)); len(f) != 1 {
		t.Errorf("no sentence, a short pause: %v", f)
	}

	// A pause of 0.2 s, shorter than the gap, past Longest: the stretch ends there.
	f = finals(run([][2]float64{{1, 14}, {14.2, 20}}, 21, 0))
	if len(f) != 2 || math.Abs(f[0].End-14) > .15 { // the detector says so 96 ms late, at a window
		t.Errorf("past Longest, at a short pause: %v", f)
	}
	// No pause at all: final past twice Longest, and the rest when the voice ends.
	f = finals(run([][2]float64{{1, 30}}, 31, 0))
	if len(f) != 2 || !near(f[0].End, 1-preroll+2*untimed.Longest) || f[1].Start != f[0].End {
		t.Errorf("no pause: %v", f)
	}
}

// TestNoVolatile decodes each stretch once, as it ends, for a decoder whose decodes cost.
func TestNoVolatile(t *testing.T) {
	var evs []engine.Event
	dec := newFake()
	r := &Runner{dec: dec, caps: Caps{Gap: .5, Longest: 8, Untimed: true}, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, [][2]float64{{1, 3}, {5, 6}}, 7); err != nil {
		t.Fatal(err)
	}
	res := of(evs, "result")
	if len(dec.lens) != 2 || len(res) != 2 || !res[0].Final || !res[1].Final {
		t.Fatalf("%d decodes, results %v", len(dec.lens), res)
	}
}

// TestPauses keeps a stretch through pauses shorter than the gap, and ignores voice too short to
// be speech.
func TestPauses(t *testing.T) {
	evs, _, _ := run(t, [][2]float64{{1, 2}, {2.1, 3}, {4, 4.1}}, 6)
	if voice := of(evs, "voice"); len(voice) != 2 || voice[0].T != 1 || voice[1].T != 3 {
		t.Errorf("voice %v", voice)
	}
	if finals := of(evs, "result"); !finals[len(finals)-1].Final || len(of(evs, "voice")) != 2 {
		t.Errorf("results %v", finals)
	}
}

// TestLong makes a long stretch final every 12 s but for its last token, which starts the next
// stretch, and never decodes more.
func TestLong(t *testing.T) {
	evs, dec, _ := run(t, [][2]float64{{1, 30}}, 32)
	var finals []string
	var ends []float64
	for _, r := range of(evs, "result") {
		if r.Final {
			finals = append(finals, fmt.Sprint(r.Start, "-", r.End))
			if len(ends) > 0 && r.Start != ends[len(ends)-1] {
				t.Errorf("a gap between finals: %v", finals)
			}
			ends = append(ends, r.End)
		}
	}
	if len(finals) != 3 {
		t.Errorf("finals %v", finals)
	}
	// The cut is at the last character's start, within 0.1 s of the end of what was decoded.
	if len(ends) == 0 || ends[0] > 1-preroll+caps.Longest+float64(window)/rate || ends[0] < 1-preroll+caps.Longest-.1 {
		t.Errorf("first final ends at %v", ends)
	}
	for _, d := range dec.lens {
		if d > caps.Longest+.1 {
			t.Errorf("decoded %.1f s", d)
		}
	}
}

// TestUndetected takes an engine that sends no voice event for one that detects none: all it hears
// is a stretch, made final every Longest.
func TestUndetected(t *testing.T) {
	evs, _, _ := run(t, nil, 30)
	finals := 0
	for _, r := range of(evs, "result") {
		if r.Final {
			finals++
		}
	}
	if voice := of(evs, "voice"); len(voice) != 1 || !voice[0].On || !near(voice[0].T, undetected) || finals != 2 {
		t.Errorf("voice %v, %d finals", voice, finals)
	}
}

// TestSilence keeps little audio while nobody speaks.
func TestSilence(t *testing.T) {
	_, dec, r := run(t, [][2]float64{}, 60)
	if len(dec.lens) != 0 || len(r.audio) > 2*2*rate {
		t.Errorf("decoded %d times, kept %d bytes", len(dec.lens), len(r.audio))
	}
}

// TestFinish makes the stretch going on final as the runner stops, once.
func TestFinish(t *testing.T) {
	evs, _, r := run(t, [][2]float64{{0, 10}}, 1)
	before := len(evs)
	r.on = func(e engine.Event) { evs = append(evs, e) }
	if err := r.finish(); err != nil {
		t.Fatal(err)
	}
	if len(evs) != before+2 || evs[before].Type != "voice" || evs[before].On || !evs[before+1].Final || evs[before+1].End != 1 {
		t.Fatalf("finish did not end the voice: %+v", evs[before:])
	}
	r.finish()
	if len(evs) != before+2 {
		t.Fatal("finish sent the final result again")
	}
}

func pieces(words ...string) []Piece {
	var out []Piece
	for i, w := range words {
		out = append(out, Piece{Text: w, At: float64(i) * .3})
	}
	return out
}

// TestSettle shows what two decodes in a row agree on, as a reading goes back and forth, and a
// new reading once two in a row agree on it.
func TestSettle(t *testing.T) {
	r := &Runner{}
	for i, c := range []struct {
		cur  []Piece
		want string
	}{
		{pieces("공산", " 과학", " 소설"), ""},
		{pieces("공산", " 과학", " 소설", " 속", " 기술"), "공산 과학 소설"},
		// Agreed on "공산 과학 소설"; the next reads it otherwise, then back.
		{pieces("공산", " 과학", " 소", " 속", " 기술", " 이"), "공산 과학 소설"},
		{pieces("공산", " 과학", " 소설", " 속", " 기술", " 이", " 있다"), "공산 과학 소설"},
		{pieces("공산", " 과학", " 소설", " 속", " 기술", " 이", " 있다", "고"), "공산 과학 소설 속 기술 이 있다"},
		// Two in a row read it otherwise: so it is.
		{pieces("공상", " 과학", " 소설", " 속", " 기술", " 이", " 있다", "고", " 해도"), "공산 과학 소설 속 기술 이 있다"},
		{pieces("공상", " 과학", " 소설", " 속", " 기술", " 이", " 있다", "고", " 해도", " 우리"), "공상 과학 소설 속 기술 이 있다고 해도"},
	} {
		if got := join(r.settle(c.cur)); got != c.want {
			t.Errorf("%d: %q, want %q", i, got, c.want)
		}
	}
}

func wait(t *testing.T, r *Runner) {
	t.Helper()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the runner did not end")
	}
}

// TestStop ends the runner and its decoder, the stretch going on made final, and refuses what
// comes after.
func TestStop(t *testing.T) {
	dec := newFake()
	results := make(chan engine.Event, 1000)
	r := Start(dec, caps, func(e engine.Event) { results <- e })
	chunk := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/10))
	r.Voice(engine.Event{Type: "voice", On: true, T: 0})
	for range 10 {
		if err := r.Audio(engine.Event{Type: "audio", PCM: chunk}); err != nil {
			t.Fatal(err)
		}
	}
	r.Stop()
	wait(t, r)
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	select {
	case <-dec.Done():
	default:
		t.Fatal("the decoder still runs")
	}
	close(results)
	var last engine.Event
	for e := range results {
		if e.Type == "result" {
			last = e
		}
	}
	if !last.Final || last.End != 1 {
		t.Fatalf("last result %+v", last)
	}
	if err := r.Audio(engine.Event{Type: "audio", PCM: chunk}); !errors.Is(err, ErrStopped) {
		t.Fatalf("audio after stopping: %v", err)
	}
	if err := r.Audio(engine.Event{Type: "audio", PCM: "AA=="}); err == nil {
		t.Fatal("half a PCM sample accepted")
	}
}

// stuckDec never returns a decode, as a llama-server that hangs, until it is stopped.
type stuckDec struct {
	once sync.Once
	done chan struct{}
}

func (d *stuckDec) Decode(ctx context.Context, _ []byte) ([]Piece, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-d.done:
		return nil, errors.New("stopped")
	}
}
func (d *stuckDec) Stop()                 { d.once.Do(func() { close(d.done) }) }
func (d *stuckDec) Done() <-chan struct{} { return d.done }
func (d *stuckDec) Err() error            { return nil }

// TestStopStuck ends the runner, with no error, though a decode never returns.
func TestStopStuck(t *testing.T) {
	defer func(f time.Duration) { forced = f }(forced)
	forced = 200 * time.Millisecond
	r := Start(&stuckDec{done: make(chan struct{})}, caps, func(engine.Event) {})
	chunk := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/10))
	r.Voice(engine.Event{Type: "voice", On: true, T: 0})
	for range 10 {
		r.Audio(engine.Event{Type: "audio", PCM: chunk})
	}
	r.Stop()
	wait(t, r)
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
}

// TestDecoderFails ends the runner with the decoder's error, and the decoder with it.
func TestDecoderFails(t *testing.T) {
	dec := newFake()
	dec.fail = errors.New("broken")
	r := Start(dec, caps, func(engine.Event) {})
	chunk := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/10))
	r.Voice(engine.Event{Type: "voice", On: true, T: 0})
	for range 10 {
		if r.Audio(engine.Event{Type: "audio", PCM: chunk}) != nil {
			break
		}
	}
	// Volatile decodes may be skipped, the audio given at once; the final one never is.
	r.Stop()
	wait(t, r)
	if r.Err() == nil || r.Err().Error() != "broken" {
		t.Fatalf("error %v", r.Err())
	}
	// A decoder that ends by itself ends the runner too.
	dec = newFake()
	r = Start(dec, caps, func(engine.Event) {})
	dec.Stop()
	wait(t, r)
	if r.Err() == nil {
		t.Fatal("a decoder that ended unasked is no error")
	}
}

func TestPieces(t *testing.T) {
	var got []string
	for _, p := range Pieces("いや、試菜は後に。 Okay, see you.") {
		got = append(got, p.Text)
	}
	want := []string{"い", "や", "、", "試", "菜", "は", "後", "に", "。", " Okay", ",", " see", " you", "."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%q", got)
	}
}

// TestSpaced starts a stretch in a script that spaces its words with a space, for it not to run
// into the stretch before.
func TestSpaced(t *testing.T) {
	for in, want := range map[string]string{"us": " us", " us": " us", "우리": " 우리", "その": "その", "我们": "我们", "。": "。"} {
		if got := Spaced([]Piece{{Text: in}}); got[0].Text != want {
			t.Errorf("Spaced(%q) = %q, want %q", in, got[0].Text, want)
		}
	}
}

// TestRemote sends how long each decode of a decoder over the network took.
func TestRemote(t *testing.T) {
	var evs []engine.Event
	dec := newFake()
	dec.delay = 20 * time.Millisecond
	r := &Runner{dec: dec, caps: Caps{Gap: .3, Longest: 5, Untimed: true, Remote: true}, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, [][2]float64{{1, 1.5}, {2, 2.5}}, 3.5); err != nil {
		t.Fatal(err)
	}
	if lag := of(evs, "lag"); len(lag) != 2 || lag[0].T < .02 {
		t.Errorf("lag events %+v", lag)
	}
}
