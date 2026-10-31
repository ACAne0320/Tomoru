// Package batch makes a recognizer of a decoder that decodes whole stretches of speech, as
// SenseVoice does (docs/recognition.md): the engine's voice detection cuts what it hears into
// stretches; while a stretch goes on, it is decoded again every so often, as a volatile result,
// and once more when it ends, as a final one (docs/local-asr.md).
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

const (
	window = 512 // samples a step, the voice detector's window
	// minSpeech is the voice that starts a stretch: shorter is a click, or a cough.
	minSpeech = 0.25
	// preroll is the audio before the voice a stretch starts with: detection is late for it.
	preroll = 0.4
	// undetected is the audio, in seconds, after which an engine that sent no voice event detects
	// none: then speech is one stretch, cut every Longest.
	undetected = 1.0
	// behind is how many inputs waiting say decoding lags what is heard: volatile decodes are
	// skipped until it catches up, and final ones never are.
	behind = 3
	// finishing is how long the last stretch's final decode may take, as the session stops.
	finishing = 5 * time.Second
)

// forced is how long Stop waits for the runner to end before it stops the decoder, ending the
// decode going on: one that never returns (a llama-server that hangs) would hold the session,
// and the app stopping it, forever.
var forced = finishing + time.Second

// Piece is a token a decoder decoded, at At seconds into the stretch.
type Piece struct {
	Text string
	At   float64
}

// Decoder decodes a stretch of speech, 16 kHz mono 16-bit little-endian PCM, as a process does
// that ends with Stop.
type Decoder interface {
	Decode(ctx context.Context, pcm []byte) ([]Piece, error)
	Stop()
	Done() <-chan struct{}
	Err() error
}

// Caps are how a decoder is run, in seconds.
type Caps struct {
	Volatile float64 // between volatile decodes of a stretch going on; 0 for none, as a decode costs
	Gap      float64 // without voice that end a stretch
	Longest  float64 // a stretch is at most: longer, it is made final
	// Stops tells the decoder ends every decode with a full stop, where the stretch ends, which
	// is no sentence's end of the speaker's: it is left out of final results as of volatile ones.
	Stops bool
	// Untimed tells the decoder times no piece: the runner spreads the stretch over them by their
	// letters. Then the sentences it ends while a stretch goes on are not made final here, for
	// want of a time to cut the audio at, but committed from its volatile text (asr.Spec's
	// VolatileSentences); and the stretch ends at the speaker's next pause, however short, once the
	// decoder has ended a sentence in it, as a timed one goes on after the sentence, or once past
	// Longest; past twice Longest wherever it is.
	Untimed bool
	// Steady tells the decoder reads a stretch alike as it grows: each volatile decode shows as it
	// is, not only what two decodes in a row agree on.
	Steady bool
	// Remote tells the decoder decodes over the network: the runner sends how long each decode
	// took (Took), for package asr to tell whether the network is slow.
	Remote bool
}

// ErrStopped is audio or voice given a runner that stopped.
var ErrStopped = errors.New("batch: stopped")

// Runner is a recognizer of the engine's audio and voice events, with a decoder. It reports what
// it recognizes as the engine protocol's result, voice and clock events.
type Runner struct {
	dec  Decoder
	caps Caps
	on   func(engine.Event)

	in       chan input
	quit     chan struct{}
	stopOnce sync.Once
	forcing  atomic.Bool // Stop stopped the decoder, as the runner had not ended
	done     chan struct{}
	err      error // why it ended, set before done closes

	// The loop's, as it goes through the inputs.
	audio   []byte // 16-bit samples since base
	base    int64
	total   int64 // samples heard
	stepped int64 // where the last step was
	clocked int64 // where the clock was last sent
	detect  int   // the engine's voice detection: 0 unknown yet, 1 on, -1 none
	voiceOn bool  // the engine hears voice
	onAt    int64 // where its voice started
	offAt   int64 // where its voice stopped
	open    bool  // in a stretch
	ended   bool  // the decoder ended a sentence in it: untimed, it ends at the next pause
	start   int64 // the stretch's first sample, preroll included
	decoded int64 // where the last volatile result was decoded through
	last    []Piece
	agreed  []Piece // what the stretch's last two volatile decodes agree on, from its start: what shows
}

type input struct {
	pcm   []byte
	voice *engine.Event
}

// Start runs dec with caps, calling on with each event, in order, from a goroutine of its own.
func Start(dec Decoder, caps Caps, on func(engine.Event)) *Runner {
	r := &Runner{dec: dec, caps: caps, on: on, in: make(chan input, 256), quit: make(chan struct{}), done: make(chan struct{})}
	go r.loop()
	return r
}

// Audio takes an audio event of the engine.
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

// Voice takes a voice event of the engine.
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
	case <-r.quit:
		return ErrStopped
	case <-r.done:
		return ErrStopped
	}
}

// Stop ends the runner: the stretch going on is made final, and the decoder stops. It returns at
// once; Done tells when it has ended, within forced.
func (r *Runner) Stop() {
	r.stopOnce.Do(func() {
		close(r.quit)
		time.AfterFunc(forced, func() {
			select {
			case <-r.done:
			default:
				r.forcing.Store(true)
				r.dec.Stop()
			}
		})
	})
}

// Done is closed when the runner has ended.
func (r *Runner) Done() <-chan struct{} { return r.done }

// Err is why the runner ended, after Done: nil when stopped.
func (r *Runner) Err() error { return r.err }

func (r *Runner) loop() {
	defer close(r.done)
	err := r.run()
	r.dec.Stop()
	<-r.dec.Done()
	if r.forcing.Load() {
		err = nil // the decode Stop ended, not the decoder's failure
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

// ended is why the decoder ended when it was not asked to.
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
		r.send(engine.Event{Type: "clock", T: sec(r.total)})
	}
	return nil
}

// voice follows the engine's voice detection.
func (r *Runner) voice(ev engine.Event) {
	r.detect = 1
	t := int64(ev.T*rate + .5)
	switch {
	case ev.On && !r.voiceOn:
		r.voiceOn, r.onAt = true, t
	case !ev.On:
		r.voiceOn, r.offAt = false, t
	}
}

// speaking tells whether a stretch goes on through the samples stepped: voice for minSpeech
// starts one, and the Gap without voice after it ends it.
func (r *Runner) speaking() bool {
	if r.detect == 0 && r.stepped >= int64(undetected*rate) {
		r.detect = -1
		log.Printf("batch: the engine detects no voice: one stretch, made final every %g s", r.caps.Longest)
	}
	switch {
	case r.detect == 0:
		return false
	case r.detect < 0:
		return true
	case r.voiceOn:
		return r.open || r.stepped >= r.onAt+int64(minSpeech*rate)
	default:
		gap := r.caps.Gap
		if r.caps.Untimed && (r.ended || r.stepped-r.start >= int64(r.caps.Longest*rate)) {
			gap = 0 // past a sentence's end or Longest, the first pause ends it
		}
		return r.open && r.stepped < r.offAt+int64(gap*rate)
	}
}

// step follows the voice after a window.
func (r *Runner) step() error {
	speaking := r.speaking()
	switch {
	case speaking && !r.open:
		on := r.onAt
		if r.detect < 0 {
			on = r.stepped
		}
		r.open, r.ended = true, false
		r.start = max(r.base, on-int64(preroll*rate))
		r.decoded = r.stepped
		r.last, r.agreed = nil, nil
		r.send(engine.Event{Type: "voice", On: true, T: sec(on)})
	case speaking && !r.caps.Untimed && r.stepped-r.start >= int64(r.caps.Longest*rate):
		return r.long()
	case speaking && r.caps.Untimed && r.stepped-r.start >= int64(2*r.caps.Longest*rate):
		// No pause came: final where it is, and the stretch goes on.
		if err := r.final(context.Background(), r.stepped); err != nil {
			return err
		}
		r.start, r.decoded, r.ended = r.stepped, r.stepped, false
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
			r.ended = true
		}
		if ev := result(false, r.start, r.stepped, agreed); ev.Type != "" {
			r.send(ev)
		}
	case !speaking && r.open:
		r.open = false
		r.send(engine.Event{Type: "voice", On: false, T: sec(r.offAt)})
		return r.final(context.Background(), r.stepped)
	}
	if !r.open {
		// Out of voice, keep only the preroll of the next stretch, and the voice before it starts.
		if keep := int64((preroll+minSpeech+.2)*rate) + window; r.stepped-r.base > 2*keep {
			r.cut(r.stepped - keep)
		}
	}
	return nil
}

// finish makes the stretch going on final, as the runner stops.
func (r *Runner) finish() error {
	if !r.open {
		return nil
	}
	r.open = false
	r.send(engine.Event{Type: "voice", On: false, T: sec(r.total)})
	ctx, cancel := context.WithTimeout(context.Background(), finishing)
	defer cancel()
	return r.final(ctx, r.total)
}

// final sends the stretch through end as a final result, and forgets its audio.
func (r *Runner) final(ctx context.Context, end int64) error {
	pieces, err := r.decode(ctx, r.start, end)
	if err != nil {
		return err
	}
	if r.caps.Stops {
		pieces = unstopped(pieces)
	}
	r.send(result(true, r.start, end, pieces))
	r.cut(end)
	r.last, r.agreed = nil, nil
	return nil
}

// long makes a stretch past Longest final but for its last token, which the cut may have halved:
// the next stretch starts with it, to be decoded again with what follows.
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

// early makes the first n pieces of the agreed decode final, the sentences the recognizer ended
// while the stretch goes on, so that they are translated without waiting for it to end; the
// stretch goes on from between their end mark and the next piece, decoded without them: a piece's
// time is about its middle, and cutting there lost the next word's start.
func (r *Runner) early(agreed []Piece, n int) {
	end := r.start + int64((agreed[n-1].At+agreed[n].At)/2*rate)
	r.send(result(true, r.start, end, agreed[:n]))
	r.cut(end)
	r.start = end
	r.last, r.agreed = nil, nil
}

// Sentences is how many of the pieces a decode agreed on finish sentences that the recognizer
// ended and went on after: through the last sentence end with speech before and after it; 0 for
// none.
func Sentences(ps []Piece) int {
	n := 0
	for i := 1; i+1 < len(ps); i++ {
		if endsSentence(ps[i].Text) && hasSpeech(join(ps[:i])) && hasSpeech(join(ps[i+1:])) {
			n = i + 1
		}
	}
	return n
}

// endsSentence tells whether a piece ends with a sentence's end mark.
func endsSentence(text string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRightFunc(text, unicode.IsSpace))
	return strings.ContainsRune("。．.！!？?", r)
}

func hasSpeech(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0
}

// unstopped is pieces without the punctuation they end with.
func unstopped(pieces []Piece) []Piece {
	for len(pieces) > 0 && strings.TrimFunc(pieces[len(pieces)-1].Text, ending) == "" {
		pieces = pieces[:len(pieces)-1]
	}
	return pieces
}

// settle steadies a volatile decode cur for the screen: of it, what two decodes in a row agree
// on, from the start. Decoding the stretch again with more of it reads it otherwise now and then,
// and back ("과학 소설 속", "과학 소 속", "과학 소설 속"): a reader sees words they read change.
// SenseVoice's volatile text changed 25 words in 100 that way, Apple's 5 (the benchmark,
// 2026-10-17). Showing what two decodes agreed on changed 3, and showed words 0.5 s later, a
// decode on; keeping it and showing the rest of the latest changed 20; leaving out the latest's
// last word, a word the decode cut, did nothing more. What shows shrinks not: two decodes that
// agree on less leave it, as a reading going back and forth. The final decode reads the whole
// stretch again, as before.
func (r *Runner) settle(cur []Piece) []Piece {
	if r.caps.Steady {
		return cur
	}
	if r.last != nil {
		if n := common(r.last, cur); n >= len(r.agreed) {
			r.agreed = slices.Clone(cur[:n])
		}
	}
	r.last = cur
	return r.agreed
}

// common is how many pieces a and b start with alike.
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

// decode decodes the samples from..to.
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

// Spaced starts a stretch's text with a space where its script spaces words: the stretch before
// may have ended mid-sentence, "…tell" before "us to do", which would run together.
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

// Spread times untimed pieces over seconds of speech, by their letters: each at the share of the
// letters before it.
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

// result is the result of the pieces decoded from..to: volatile, their text as one token, or none
// (no Type); or final, each token timed.
func result(final bool, from, to int64, pieces []Piece) engine.Event {
	t0, t1 := float64(from)/rate, float64(to)/rate
	var tokens []engine.Token
	if final {
		for i, p := range pieces {
			end := t1
			if i+1 < len(pieces) {
				end = t0 + pieces[i+1].At
			}
			start, until := round(t0+p.At), round(min(end, t1))
			tokens = append(tokens, engine.Token{Text: p.Text, Start: &start, End: &until})
		}
	} else if text := strings.TrimRightFunc(join(pieces), ending); text != "" {
		// The stretch is cut where the audio has come to, mid-sentence: the full stop the
		// recognizer ends it with is not the speaker's, and would have the app commit a sentence.
		tokens = []engine.Token{{Text: text}}
	}
	if tokens == nil {
		if !final {
			return engine.Event{}
		}
		tokens = []engine.Token{}
	}
	return engine.Event{Type: "result", Final: final, Start: round(t0), End: round(t1), Tokens: tokens}
}

// ending is a sentence's end, or a clause's, as a recognizer ends a stretch.
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

func sec(n int64) float64 { return round(float64(n) / rate) }

// round is seconds to the millisecond, as the protocol's times are.
func round(x float64) float64 {
	return float64(int64(x*1000+.5)) / 1000
}

// WAV is 16 kHz mono 16-bit PCM, as decoders get it, as a WAV file.
func WAV(pcm []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(2 * rate), uint16(2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v) // the format: PCM, mono, 16 kHz, 2 bytes a sample
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}

// Pieces cuts an untimed decoder's text into what decodes are compared by: a character of the scripts that don't space
// their words (Chinese, Japanese), else a word with the space before it, and each punctuation mark.
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
