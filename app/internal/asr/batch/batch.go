package batch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"tomoru/internal/engine"
)

const rate = 16000

const BytesPerSecond = 2 * rate

const (
	window     = 512
	minSpeech  = 0.25
	preroll    = 0.4
	undetected = 1.0
	behind     = 3
	finishing  = 5 * time.Second
)

var forced = finishing + time.Second

type Piece struct {
	Text string
	At   float64
	Lang string
}

type Decoder interface {
	Decode(ctx context.Context, pcm []byte) ([]Piece, error)
	Stop()
	Done() <-chan struct{}
	Err() error
}

type Caps struct {
	Volatile         float64
	Gap              float64
	Longest          float64
	TrailingFullStop bool
	Untimed          bool
	Remote           bool
}

var ErrStopped = errors.New("batch: stopped")

var ErrBehind = errors.New("batch: decoding fell behind")

type Runner struct {
	dec  Decoder
	caps Caps
	on   func(engine.Event)

	in       chan input
	quit     chan struct{}
	stopOnce sync.Once
	cause    atomic.Pointer[error]
	done     chan struct{}
	err      error

	audio         []byte
	base          int64
	total         int64
	stepped       int64
	clocked       int64
	detect        Detection
	voiceOn       bool
	onAt          int64
	offAt         int64
	open          bool
	sentenceEnded bool
	start         int64
	decoded       int64
	last          []Piece
	agreed        []Piece
}

type Detection int

const (
	DetectionUnknown Detection = iota
	DetectionOn
	DetectionNone
)

var errForced = errors.New("batch: stopped by force")

type input struct {
	pcm   []byte
	voice *engine.Event
}

func Start(dec Decoder, caps Caps, on func(engine.Event)) *Runner {
	r := &Runner{dec: dec, caps: caps, on: on, in: make(chan input, 256), quit: make(chan struct{}), done: make(chan struct{})}
	go r.loop()
	return r
}

func (r *Runner) Audio(ev engine.Event) error {
	b, err := base64.StdEncoding.DecodeString(ev.PCM)
	if err != nil {
		return fmt.Errorf("batch: audio: %w", err)
	}
	if len(b)%2 != 0 {
		return errors.New("batch: audio: incomplete PCM sample")
	}
	return r.put(input{pcm: b})
}

func (r *Runner) Voice(ev engine.Event) { r.put(input{voice: &ev}) }

func (r *Runner) put(x input) error {
	select {
	case <-r.quit:
		return ErrStopped
	case <-r.done:
		return ErrStopped
	default:
	}
	select {
	case r.in <- x:
		return nil
	default:
	}
	r.stopFor(ErrBehind, r.dec.Stop)
	if errors.Is(r.stopCause(), ErrBehind) {
		return ErrBehind
	}
	return ErrStopped
}

func (r *Runner) stopFor(cause error, stop func()) {
	r.stopOnce.Do(func() {
		r.cause.Store(&cause)
		stop()
	})
}

func (r *Runner) stopCause() error {
	if c := r.cause.Load(); c != nil {
		return *c
	}
	return nil
}

func (r *Runner) Stop() {
	r.stopFor(ErrStopped, func() {
		close(r.quit)
		time.AfterFunc(forced, func() {
			select {
			case <-r.done:
			default:
				r.cause.Store(&errForced)
				r.dec.Stop()
			}
		})
	})
}

func (r *Runner) Done() <-chan struct{} { return r.done }

func (r *Runner) Err() error { return r.err }

func (r *Runner) loop() {
	defer close(r.done)
	err := r.run()
	r.dec.Stop()
	<-r.dec.Done()
	switch cause := r.stopCause(); {
	case errors.Is(cause, ErrBehind):
		err = ErrBehind
	case errors.Is(cause, errForced):
		err = nil
	}
	r.err = err
}

func (r *Runner) run() error {
	for {
		select {
		case x := <-r.in:
			if err := r.take(x); err != nil {
				return err
			}
		case <-r.quit:
			for {
				select {
				case x := <-r.in:
					if err := r.take(x); err != nil {
						return err
					}
					continue
				default:
				}
				return r.finish()
			}
		case <-r.dec.Done():
			return ended(r.dec)
		}
	}
}

func ended(dec Decoder) error {
	if err := dec.Err(); err != nil {
		return err
	}
	return errors.New("batch: the decoder ended")
}

func (r *Runner) take(x input) error {
	if x.voice != nil {
		r.voice(*x.voice)
		return nil
	}
	r.audio = append(r.audio, x.pcm...)
	r.total += int64(len(x.pcm) / 2)
	for r.total-r.stepped >= window {
		r.stepped += window
		if err := r.step(); err != nil {
			return err
		}
	}
	if r.total-r.clocked >= rate/10 {
		r.clocked = r.total
		r.send(engine.Event{Type: engine.EventClock, T: sec(r.total)})
	}
	return nil
}

func (r *Runner) voice(ev engine.Event) {
	r.detect = DetectionOn
	t := int64(ev.T*rate + .5)
	switch {
	case ev.On && !r.voiceOn:
		r.voiceOn, r.onAt = true, t
	case !ev.On:
		r.voiceOn, r.offAt = false, t
	}
}

func (r *Runner) speaking() bool {
	if r.detect == DetectionUnknown && r.stepped >= int64(undetected*rate) {
		r.detect = DetectionNone
		log.Printf("batch: the engine detects no voice: one stretch, made final every %g s", r.caps.Longest)
	}
	switch {
	case r.detect == DetectionUnknown:
		return false
	case r.detect == DetectionNone:
		return true
	case r.voiceOn:
		return r.open || r.stepped >= r.onAt+int64(minSpeech*rate)
	default:
		gap := r.caps.Gap
		if r.caps.Untimed && (r.sentenceEnded || r.stepped-r.start >= int64(r.caps.Longest*rate)) {
			gap = 0
		}
		return r.open && r.stepped < r.offAt+int64(gap*rate)
	}
}

func (r *Runner) step() error {
	speaking := r.speaking()
	switch {
	case speaking && !r.open:
		on := r.onAt
		if r.detect == DetectionNone {
			on = r.stepped
		}
		r.open, r.sentenceEnded = true, false
		r.start = max(r.base, on-int64(preroll*rate))
		r.decoded = r.stepped
		r.last, r.agreed = nil, nil
		r.send(engine.Event{Type: engine.EventVoice, On: true, T: sec(on)})
	case speaking && !r.caps.Untimed && r.stepped-r.start >= int64(r.caps.Longest*rate):
		return r.long()
	case speaking && r.caps.Untimed && r.stepped-r.start >= int64(2*r.caps.Longest*rate):
		if err := r.final(context.Background(), r.stepped); err != nil {
			return err
		}
		r.start, r.decoded, r.sentenceEnded = r.stepped, r.stepped, false
	case speaking && r.caps.Volatile > 0 && r.stepped-r.decoded >= int64(r.caps.Volatile*rate):
		r.decoded = r.stepped
		if len(r.in) >= behind {
			break
		}
		pieces, err := r.decode(context.Background(), r.start, r.stepped)
		if err != nil {
			return err
		}
		agreed := r.settle(pieces)
		if n := Sentences(agreed); n > 0 {
			if !r.caps.Untimed {
				r.early(agreed, n)
				break
			}
			r.sentenceEnded = true
		}
		if ev := result(false, r.start, r.stepped, agreed); ev.Type != "" {
			r.send(ev)
		}
	case !speaking && r.open:
		r.open = false
		r.send(engine.Event{Type: engine.EventVoice, On: false, T: sec(r.offAt)})
		return r.final(context.Background(), r.stepped)
	}
	if !r.open {
		if keep := int64((preroll+minSpeech+.2)*rate) + window; r.stepped-r.base > 2*keep {
			r.cut(r.stepped - keep)
		}
	}
	return nil
}

func (r *Runner) finish() error {
	if !r.open {
		return nil
	}
	r.open = false
	r.send(engine.Event{Type: engine.EventVoice, On: false, T: sec(r.total)})
	ctx, cancel := context.WithTimeout(context.Background(), finishing)
	defer cancel()
	return r.final(ctx, r.total)
}

func (r *Runner) final(ctx context.Context, end int64) error {
	pieces, err := r.decode(ctx, r.start, end)
	if err != nil {
		return err
	}
	if r.caps.TrailingFullStop {
		pieces = unstopped(pieces)
	}
	r.send(result(true, r.start, end, pieces))
	r.cut(end)
	r.last, r.agreed = nil, nil
	return nil
}

func (r *Runner) long() error {
	pieces, err := r.decode(context.Background(), r.start, r.stepped)
	if err != nil {
		return err
	}
	pieces = unstopped(pieces)
	end := r.stepped
	if n := len(pieces); n > 1 {
		end = r.start + int64(pieces[n-1].At*rate)
		pieces = pieces[:n-1]
	}
	r.send(result(true, r.start, end, pieces))
	r.cut(end)
	r.start, r.decoded = end, r.stepped
	r.last, r.agreed = nil, nil
	return nil
}

func (r *Runner) early(agreed []Piece, n int) {
	end := r.start + int64((agreed[n-1].At+agreed[n].At)/2*rate)
	r.send(result(true, r.start, end, agreed[:n]))
	r.cut(end)
	r.start = end
	r.last, r.agreed = nil, nil
}

func Sentences(ps []Piece) int {
	n := 0
	for i := 1; i+1 < len(ps); i++ {
		if endsSentence(ps[i].Text) && hasSpeech(join(ps[:i])) && hasSpeech(join(ps[i+1:])) {
			n = i + 1
		}
	}
	return n
}

func endsSentence(text string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRightFunc(text, unicode.IsSpace))
	return strings.ContainsRune("。．.！!？?", r)
}

func hasSpeech(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0
}

func unstopped(pieces []Piece) []Piece {
	for len(pieces) > 0 && strings.TrimFunc(pieces[len(pieces)-1].Text, ending) == "" {
		pieces = pieces[:len(pieces)-1]
	}
	return pieces
}

func (r *Runner) settle(cur []Piece) []Piece {
	if len(cur) > 0 && len(r.agreed) > 0 && cur[0].Lang != r.agreed[0].Lang {
		r.last, r.agreed = nil, nil
	}
	if r.last != nil {
		if n := common(r.last, cur); n >= len(r.agreed) {
			r.agreed = slices.Clone(cur[:n])
		}
	}
	r.last = cur
	return r.agreed
}

func common(a, b []Piece) int {
	n := 0
	for n < len(a) && n < len(b) && a[n].Text == b[n].Text {
		n++
	}
	return n
}

func (r *Runner) cut(to int64) {
	r.audio = append(r.audio[:0], r.audio[2*(to-r.base):]...)
	r.base = to
}

func (r *Runner) decode(ctx context.Context, from, to int64) ([]Piece, error) {
	start := time.Now()
	pieces, err := r.dec.Decode(ctx, r.audio[2*(from-r.base):2*(to-r.base)])
	if r.caps.Remote && err == nil {
		r.send(Took(time.Since(start)))
	}
	if err == nil && r.caps.Untimed {
		Spread(pieces, float64(to-from)/rate)
	}
	return Spaced(pieces), err
}

func Spaced(pieces []Piece) []Piece {
	if len(pieces) == 0 {
		return pieces
	}
	r, _ := utf8.DecodeRuneInString(pieces[0].Text)
	if isSpeechRune(r) && !unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar) {
		pieces[0].Text = " " + pieces[0].Text
	}
	return pieces
}

func isSpeechRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func Spread(pieces []Piece, seconds float64) {
	total, before := 0, 0
	for _, p := range pieces {
		total += letters(p.Text)
	}
	for i := range pieces {
		if total > 0 {
			pieces[i].At = seconds * float64(before) / float64(total)
		}
		before += letters(pieces[i].Text)
	}
}

func letters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			n++
		}
	}
	return n
}

func (r *Runner) send(ev engine.Event) { r.on(ev) }

func result(final bool, from, to int64, pieces []Piece) engine.Event {
	t0, t1 := float64(from)/rate, float64(to)/rate
	var tokens []engine.Token
	if final {
		for i, p := range pieces {
			end := t1
			if i+1 < len(pieces) {
				end = t0 + pieces[i+1].At
			}
			start, until := Round(t0+p.At), Round(min(end, t1))
			tokens = append(tokens, engine.Token{Text: p.Text, Start: &start, End: &until})
		}
	} else if text := strings.TrimRightFunc(join(pieces), ending); text != "" {
		tokens = []engine.Token{{Text: text}}
	}
	if tokens == nil {
		if !final {
			return engine.Event{}
		}
		tokens = []engine.Token{}
	}
	ev := engine.Event{Type: engine.EventResult, Final: final, Start: Round(t0), End: Round(t1), Tokens: tokens}
	for _, p := range pieces {
		if p.Lang != "" {
			ev.Lang = p.Lang
			break
		}
	}
	return ev
}

func ending(r rune) bool {
	return strings.ContainsRune("。．.！!？?、，,", r) || unicode.IsSpace(r)
}

func join(pieces []Piece) string {
	var b strings.Builder
	for _, p := range pieces {
		b.WriteString(p.Text)
	}
	return b.String()
}

func sec(n int64) float64 { return Round(float64(n) / rate) }

func Round(x float64) float64 {
	return float64(int64(x*1000+.5)) / 1000
}

func WAV(pcm []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(2 * rate), uint16(2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}

func Pieces(text string) []Piece {
	var out []Piece
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			out = append(out, Piece{Text: word.String()})
			word.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) || unicode.IsPunct(r):
			flush()
			out = append(out, Piece{Text: string(r)})
		case unicode.IsSpace(r):
			flush()
			word.WriteRune(r)
		default:
			word.WriteRune(r)
		}
	}
	flush()
	return out
}
