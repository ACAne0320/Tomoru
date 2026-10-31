package main

import (
	"encoding/base64"
	"encoding/binary"
	"sync"
	"time"
)

// Hearing (`listen --asr none`): what is heard sent as protocol `audio`, for tomoru-asr or the
// cloud to recognize. No recognition and no clock: they come from what is recognized. Voice
// detection runs on the audio sent, for the cloud's session to hang up while nobody speaks.

const (
	rate  = 16000 // samples a second of what is sent: mono, 16-bit
	chunk = 1600  // samples an `audio` object carries: 100 ms
)

// Source is what the engine hears: the system, or a recording. It gives what it hears to feed, at
// 16 kHz mono, from a goroutine of its own, until the process exits.
type Source interface {
	Start(feed func([]int16)) error
}

// Hearer sends what a Source hears in chunks, and silence while it hears nothing, so that time
// goes on as the audio does.
type Hearer struct {
	send  func(Event)
	voice func([]float32) // what was sent, to voice detection; nil without
	now   func() time.Time

	// What is heard and not sent yet; under mu, as the source adds to it and the ticker fills
	// silence.
	mu       sync.Mutex
	pending  []int16
	sent     int64 // samples sent
	started  time.Time
	lastFeed time.Time
}

func NewHearer(send func(Event), voice func([]float32)) *Hearer {
	return &Hearer{send: send, voice: voice, now: time.Now}
}

// Start starts hearing src, and says so.
func (h *Hearer) Start(src Source) error {
	h.mu.Lock()
	h.started = h.now()
	h.mu.Unlock()
	if err := src.Start(h.Feed); err != nil {
		return err
	}
	h.send(Event{Type: "state", State: "listening"})
	go func() {
		for range time.Tick(100 * time.Millisecond) {
			h.FillSilence()
		}
	}()
	return nil
}

// Feed takes samples heard, after those before, and sends the whole chunks.
func (h *Hearer) Feed(samples []int16) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastFeed = h.now()
	h.pending = append(h.pending, samples...)
	h.flush()
}

// FillSilence sends silence for the time the source gave nothing in, once it has given nothing
// for a while. Only then: a source delivering is the timeline, though its clock drifts from the
// wall's.
func (h *Hearer) FillSilence() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	due := int64(now.Sub(h.started).Seconds() * rate)
	have := h.sent + int64(len(h.pending))
	if now.Sub(h.lastFeed) > 300*time.Millisecond && due > have {
		missing := due - have
		// A suspended process must not send the entire sleep as PCM. Keep enough
		// silence to finalize speech, then resume the clock at the samples sent.
		if missing > 2*rate {
			missing = 2 * rate
			h.started = now.Add(-time.Duration(have+missing) * (time.Second / rate))
		}
		h.pending = append(h.pending, make([]int16, missing)...)
		h.flush()
	}
}

// flush sends the whole chunks of pending, under mu.
func (h *Hearer) flush() {
	for len(h.pending) >= chunk {
		c := h.pending[:chunk]
		b := make([]byte, 2*chunk)
		for i, s := range c {
			binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
		}
		h.send(Event{Type: "audio", T: float64(h.sent) / rate, PCM: base64.StdEncoding.EncodeToString(b)})
		if h.voice != nil {
			x := make([]float32, chunk)
			for i, s := range c {
				x[i] = float32(s) / 32768
			}
			h.voice(x)
		}
		h.sent += chunk
		h.pending = h.pending[chunk:]
	}
}
