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

var Caps = batch.Caps{Volatile: .4, Gap: .5, Longest: 12, Stops: true}

type Decoder struct {
	p *engine.Process

	mu      sync.Mutex
	n       int
	waiting map[string]chan engine.Event
}

func Start(bin, dir, lang string) (*Decoder, error) {
	d := &Decoder{waiting: map[string]chan engine.Event{}}
	p, err := engine.Serve(bin, dir, lang, d.event)
	if err != nil {
		return nil, err
	}
	d.p = p
	return d, nil
}

func (d *Decoder) event(ev engine.Event) {
	if ev.Type != engine.EventDecoded {
		return
	}
	d.mu.Lock()
	ch := d.waiting[ev.ID]
	delete(d.waiting, ev.ID)
	d.mu.Unlock()
	if ch != nil {
		ch <- ev
	}
}

func (d *Decoder) Decode(ctx context.Context, pcm []byte) ([]batch.Piece, error) {
	ch := make(chan engine.Event, 1)
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
	case ev := <-ch:
		pieces := make([]batch.Piece, 0, len(ev.Tokens))
		for _, t := range ev.Tokens {
			p := batch.Piece{Text: t.Text, Lang: ev.Lang}
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

func (d *Decoder) Stop() { d.p.Stop() }

func (d *Decoder) Done() <-chan struct{} { return d.p.Done() }

func (d *Decoder) Err() error { return d.p.Err() }
