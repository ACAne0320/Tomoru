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

func near(v float64, t float64) bool {
	return math.Abs(v-t) <= float64(window)/rate
}

type fakeDec struct {
	lens  []float64
	fail  error
	stop  float64
	delay time.Duration
	lang  string
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
		d.stop = 0
	}
	d.lens = append(d.lens, s)
	var out []Piece
	for i := 0; float64(i)*.1 < s; i++ {
		at := float64(i) * .1
		out = append(out, Piece{Text: "あ", At: at, Lang: d.lang})
		if d.stop > 0 && math.Abs(at-d.stop) < .01 {
			out = append(out, Piece{Text: "。", At: at + .05, Lang: d.lang})
		}
	}
	return append(out, Piece{Text: "。", At: s, Lang: d.lang}), nil
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

func feed(r *Runner, spans [][2]float64, seconds float64) error {
	type vev struct {
		at float64
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
	if !f.Final || !near(f.Start, 1-preroll) || !near(f.End, 3+caps.Gap) || toks[0].Text != "あ" ||
		*toks[0].Start != f.Start || !near(*toks[1].Start, 1-preroll+.1) {
		t.Errorf("final %v", f)
	}
	if clocks := of(evs, "clock"); len(clocks) != 50 || clocks[49].T != 5.0 {
		t.Errorf("%d clocks, last %v", len(clocks), clocks[len(clocks)-1])
	}
	if len(dec.lens) != vol+2 {
		t.Errorf("decoded %d times", len(dec.lens))
	}
}

func TestEarly(t *testing.T) {
	var evs []engine.Event
	dec := newFake()
	dec.stop = .5
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
	if n := Sentences(pieces("あ", "。")); n != 0 {
		t.Errorf("%d pieces of a sentence with nothing after it", n)
	}
	if n := Sentences(pieces("あ", "。", "い", "！", "う")); n != 4 {
		t.Errorf("%d pieces, want through the last end", n)
	}
}

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

func TestLang(t *testing.T) {
	var evs []engine.Event
	dec := newFake()
	dec.lang = "ja"
	r := &Runner{dec: dec, caps: Caps{Volatile: .4, Gap: .25, Longest: 12}, on: func(e engine.Event) { evs = append(evs, e) }}
	if err := feed(r, [][2]float64{{1, 3}}, 5); err != nil {
		t.Fatal(err)
	}
	res := of(evs, "result")
	for _, e := range res {
		if e.Lang != "ja" {
			t.Fatalf("result %+v", e)
		}
	}
	if len(res) < 2 || !res[len(res)-1].Final {
		t.Fatalf("results %d", len(res))
	}
}

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
			sawEnd = true
		}
	}
	if !sawEnd {
		t.Error("no sentence end in the volatile text")
	}

	f = finals(run([][2]float64{{1, 4}, {4.1, 6}}, 7, .5))
	if len(f) != 2 || math.Abs(f[0].End-4) > .15 {
		t.Errorf("after a sentence, at a short pause: %v", f)
	}
	if f = finals(run([][2]float64{{1, 4}, {4.1, 6}}, 7, 0)); len(f) != 1 {
		t.Errorf("no sentence, a short pause: %v", f)
	}

	f = finals(run([][2]float64{{1, 14}, {14.2, 20}}, 21, 0))
	if len(f) != 2 || math.Abs(f[0].End-14) > .15 {
		t.Errorf("past Longest, at a short pause: %v", f)
	}
	f = finals(run([][2]float64{{1, 30}}, 31, 0))
	if len(f) != 2 || !near(f[0].End, 1-preroll+2*untimed.Longest) || f[1].Start != f[0].End {
		t.Errorf("no pause: %v", f)
	}
}

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

func TestPauses(t *testing.T) {
	evs, _, _ := run(t, [][2]float64{{1, 2}, {2.1, 3}, {4, 4.1}}, 6)
	if voice := of(evs, "voice"); len(voice) != 2 || voice[0].T != 1 || voice[1].T != 3 {
		t.Errorf("voice %v", voice)
	}
	if finals := of(evs, "result"); !finals[len(finals)-1].Final || len(of(evs, "voice")) != 2 {
		t.Errorf("results %v", finals)
	}
}

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
	if len(ends) == 0 || ends[0] > 1-preroll+caps.Longest+float64(window)/rate || ends[0] < 1-preroll+caps.Longest-.1 {
		t.Errorf("first final ends at %v", ends)
	}
	for _, d := range dec.lens {
		if d > caps.Longest+.1 {
			t.Errorf("decoded %.1f s", d)
		}
	}
}

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

func TestSilence(t *testing.T) {
	_, dec, r := run(t, [][2]float64{}, 60)
	if len(dec.lens) != 0 || len(r.audio) > 2*2*rate {
		t.Errorf("decoded %d times, kept %d bytes", len(dec.lens), len(r.audio))
	}
}

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

func TestSettle(t *testing.T) {
	r := &Runner{}
	for i, c := range []struct {
		cur  []Piece
		want string
	}{
		{pieces("공산", " 과학", " 소설"), ""},
		{pieces("공산", " 과학", " 소설", " 속", " 기술"), "공산 과학 소설"},
		{pieces("공산", " 과학", " 소", " 속", " 기술", " 이"), "공산 과학 소설"},
		{pieces("공산", " 과학", " 소설", " 속", " 기술", " 이", " 있다"), "공산 과학 소설"},
		{pieces("공산", " 과학", " 소설", " 속", " 기술", " 이", " 있다", "고"), "공산 과학 소설 속 기술 이 있다"},
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
	for range 300 {
		if err := r.Audio(engine.Event{Type: "audio", PCM: chunk}); !errors.Is(err, ErrStopped) {
			t.Fatalf("audio after stopping: %v", err)
		}
	}
	wait(t, r)
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
}

func TestBehind(t *testing.T) {
	dec := &stuckDec{done: make(chan struct{})}
	r := Start(dec, caps, func(engine.Event) {})
	chunk := base64.StdEncoding.EncodeToString(make([]byte, 2*rate/10))
	r.Voice(engine.Event{Type: "voice", On: true, T: 0})
	var err error
	for i := 0; err == nil && i < 1000; i++ {
		err = r.Audio(engine.Event{Type: "audio", PCM: chunk})
	}
	if !errors.Is(err, ErrBehind) {
		t.Fatalf("audio as the decode hangs: %v", err)
	}
	wait(t, r)
	if !errors.Is(r.Err(), ErrBehind) {
		t.Fatalf("error %v", r.Err())
	}
	select {
	case <-dec.Done():
	default:
		t.Fatal("the decoder still runs")
	}
	r.Stop()
	if !errors.Is(r.Err(), ErrBehind) {
		t.Fatalf("error after Stop %v", r.Err())
	}
}

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
	r.Stop()
	wait(t, r)
	if r.Err() == nil || r.Err().Error() != "broken" {
		t.Fatalf("error %v", r.Err())
	}
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

func TestSpaced(t *testing.T) {
	for in, want := range map[string]string{"us": " us", " us": " us", "우리": " 우리", "その": "その", "我们": "我们", "。": "。"} {
		if got := Spaced([]Piece{{Text: in}}); got[0].Text != want {
			t.Errorf("Spaced(%q) = %q, want %q", in, got[0].Text, want)
		}
	}
}

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

func TestSettleLang(t *testing.T) {
	r := &Runner{}
	en := []Piece{{Text: "Congratulations", Lang: "en"}}
	zh := []Piece{{Text: "恭喜", Lang: "zh"}}
	r.settle(en)
	if got := r.settle(en); len(got) != 1 || got[0].Lang != "en" {
		t.Fatalf("agreed %+v", got)
	}
	if got := r.settle(zh); len(got) != 0 {
		t.Fatalf("another language kept %+v", got)
	}
	if got := r.settle(zh); len(got) != 1 || got[0].Text != "恭喜" {
		t.Fatalf("agreed anew %+v", got)
	}
}
