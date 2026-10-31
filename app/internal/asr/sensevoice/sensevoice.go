// Package sensevoice decodes stretches of speech with tomoru-asr, which runs SenseVoice through
// sherpa-onnx (docs/local-asr.md).
package sensevoice

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"sync"

	"tomoru/internal/asr/batch"
	"tomoru/internal/engine"
)

// Caps are how SenseVoice is run: a volatile decode every 0.4 s, and a stretch made final at 12 s,
// as tomoru-asr ran it before decoding moved to the app; a stretch ended by 0.5 s without voice.
// tomoru-asr ended it after 0.25 s of its own detector's silence (0.35 s joined TTS sentences),
// but the engine's detector on macOS (Silero v6, Core ML) hears pauses under music that tomoru-asr's
// (silero_vad.onnx) didn't: at 0.25 s a variety show was cut in 28 stretches of 2 s where it had
// been 5, and words were lost at the cuts (Korean 18.5 → 26.0 % errors; the benchmark's quick set,
// 2026-10-31). 0.5 s gave back English and Chinese, and most of Korean (22.4 %); 0.8 s did no
// better. SenseVoice ends every decode with a full stop, wherever the stretch ends: it is no
// sentence's end.
var Caps = batch.Caps{Volatile: .4, Gap: .5, Longest: 12, Stops: true}

// Decoder is tomoru-asr, serving a model.
type Decoder struct {
	p *engine.Process

	mu      sync.Mutex
	n       int
	waiting map[string]chan []engine.Token
}

// Start starts tomoru-asr at bin decoding lang with the model in dir.
func Start(bin, dir, lang string) (*Decoder, error) {
	d := &Decoder{waiting: map[string]chan []engine.Token{}}
	p, err := engine.Serve(bin, dir, lang, d.event)
	if err != nil {
		return nil, err
	}
	d.p = p
	return d, nil
}

func (d *Decoder) event(ev engine.Event) {
	if ev.Type != "decoded" {
		return
	}
	d.mu.Lock()
	ch := d.waiting[ev.ID]
	delete(d.waiting, ev.ID)
	d.mu.Unlock()
	if ch != nil {
		ch <- ev.Tokens
	}
}

// Decode decodes a stretch: its tokens, each at its time from the stretch's start.
func (d *Decoder) Decode(ctx context.Context, pcm []byte) ([]batch.Piece, error) {
	ch := make(chan []engine.Token, 1)
	d.mu.Lock()
	d.n++
	id := strconv.Itoa(d.n)
	d.waiting[id] = ch
	d.mu.Unlock()
	forget := func() {
		d.mu.Lock()
		delete(d.waiting, id)
		d.mu.Unlock()
	}
	if err := d.p.Decode(id, base64.StdEncoding.EncodeToString(pcm)); err != nil {
		forget()
		return nil, err
	}
	select {
	case tokens := <-ch:
		pieces := make([]batch.Piece, 0, len(tokens))
		for _, t := range tokens {
			p := batch.Piece{Text: t.Text}
			if t.Start != nil {
				p.At = *t.Start
			}
			pieces = append(pieces, p)
		}
		return pieces, nil
	case <-ctx.Done():
		forget()
		return nil, ctx.Err()
	case <-d.p.Done():
		forget()
		if err := d.p.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("sensevoice: tomoru-asr ended")
	}
}

// Stop stops tomoru-asr.
func (d *Decoder) Stop() { d.p.Stop() }

// Done is closed when tomoru-asr has ended.
func (d *Decoder) Done() <-chan struct{} { return d.p.Done() }

// Err is why tomoru-asr ended, after Done: nil when stopped.
func (d *Decoder) Err() error { return d.p.Err() }
