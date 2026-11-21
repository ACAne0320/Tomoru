package shared

import (
	"encoding/base64"
	"encoding/binary"
	"sync"
	"time"
)

const (
	Rate  = 16000
	chunk = 1600
)

type Source interface {
	Start(feed func([]int16)) error
}

type Hearer struct {
	send  func(Event)
	voice func([]float32)
	now   func() time.Time

	mu       sync.Mutex
	pending  []int16
	sent     int64
	started  time.Time
	lastFeed time.Time
}

func NewHearer(send func(Event), voice func([]float32)) *Hearer {
	return &Hearer{send: send, voice: voice, now: time.Now}
}

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

func (h *Hearer) Feed(samples []int16) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastFeed = h.now()
	h.pending = append(h.pending, samples...)
	h.flush()
}

func (h *Hearer) FillSilence() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	due := int64(now.Sub(h.started).Seconds() * Rate)
	have := h.sent + int64(len(h.pending))
	if now.Sub(h.lastFeed) > 300*time.Millisecond && due > have {
		missing := due - have
		if missing > 2*Rate {
			missing = 2 * Rate
			h.started = now.Add(-time.Duration(have+missing) * (time.Second / Rate))
		}
		h.pending = append(h.pending, make([]int16, missing)...)
		h.flush()
	}
}

func (h *Hearer) flush() {
	for len(h.pending) >= chunk {
		c := h.pending[:chunk]
		b := make([]byte, 2*chunk)
		for i, s := range c {
			binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
		}
		h.send(Event{Type: "audio", T: float64(h.sent) / Rate, PCM: base64.StdEncoding.EncodeToString(b)})
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
