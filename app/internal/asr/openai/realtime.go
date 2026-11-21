package openai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"tomoru/internal/asr/batch"
	"tomoru/internal/engine"
)

const LiveModel = "gpt-live-transcribe"

var RealtimeModels = []string{LiveModel, "gpt-realtime-whisper"}

func Realtimed(model string) bool { return model == "" || slices.Contains(RealtimeModels, model) }

const (
	engineRate  = 16000
	serviceRate = 24000
	rateUnit    = 8000
	minSpeech   = .25
	preroll     = .4
	gap         = .5
	longest     = 8.0
	undetected  = 1.0
	late        = 2 * time.Second
	finishing   = 5 * time.Second
)

type RealtimeConfig struct {
	Config
	About string
}

type Realtime struct {
	cfg  RealtimeConfig
	lang string
	on   func(engine.Event)

	in       chan rtInput
	srv      chan serverMsg
	quit     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	err      error
	ctx      context.Context
	cancel   context.CancelFunc

	mu   sync.Mutex
	sent float64

	loopOwned
}

type loopOwned struct {
	conn        *websocket.Conn
	gen         int
	total       samples
	clocked     samples
	detect      batch.Detection
	voiceOn     bool
	onAt        samples
	offAt       samples
	recent      []byte
	recentAt    samples
	inTurn      bool
	sentTo      samples
	up          upsampler
	turns       []*turn
	lateCommits map[itemID]time.Time
}

type samples int64

func samplesOf(sec float64) samples { return samples(sec*engineRate + .5) }

func (n samples) sec() float64 { return batch.Round(float64(n) / engineRate) }

func (n samples) bytes() int { return 2 * int(n) }

type itemID string

type turn struct {
	id         itemID
	acked      bool
	start, end samples
	committed  time.Time
	text       strings.Builder
	final      *string
}

type rtInput struct {
	pcm   []byte
	voice *engine.Event
}

type serverMsg struct {
	gen int
	err error
	ev  rtEvent
}

type rtEvent struct {
	Type       string `json:"type"`
	ItemID     itemID `json:"item_id"`
	Delta      string `json:"delta"`
	Transcript string `json:"transcript"`
	Error      struct {
		Type, Code, Message string
	} `json:"error"`
}

func StartRealtime(cfg RealtimeConfig, lang string, on func(engine.Event)) *Realtime {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Realtime{cfg: cfg, lang: iso639(lang), on: on, in: make(chan rtInput, 600), srv: make(chan serverMsg, 256),
		quit: make(chan struct{}), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	r.lateCommits = map[itemID]time.Time{}
	go r.loop()
	return r
}

func (r *Realtime) Audio(ev engine.Event) error {
	b, err := base64.StdEncoding.DecodeString(ev.PCM)
	if err != nil {
		return fmt.Errorf("openai: audio: %w", err)
	}
	return r.put(rtInput{pcm: b})
}

func (r *Realtime) Voice(ev engine.Event) { r.put(rtInput{voice: &ev}) }

func (r *Realtime) put(x rtInput) error {
	select {
	case r.in <- x:
		return nil
	case <-r.quit:
		return batch.ErrStopped
	case <-r.done:
		return batch.ErrStopped
	}
}

func (r *Realtime) Stop() { r.stopOnce.Do(func() { close(r.quit) }) }

func (r *Realtime) Done() <-chan struct{} { return r.done }

func (r *Realtime) Err() error { return r.err }

func (r *Realtime) Sent() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sent
}

func (r *Realtime) loop() {
	defer close(r.done)
	err := r.run()
	if r.conn != nil {
		r.conn.Close(websocket.StatusNormalClosure, "")
	}
	r.cancel()
	r.err = err
}

func (r *Realtime) run() error {
	if err := r.connect(); err != nil {
		return err
	}
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case x := <-r.in:
			if err := r.take(x); err != nil {
				return err
			}
		case m := <-r.srv:
			r.server(m)
		case <-tick.C:
			r.overdue()
		case <-r.quit:
			for len(r.in) > 0 {
				if err := r.take(<-r.in); err != nil {
					return err
				}
			}
			return r.finish()
		}
	}
}

func (r *Realtime) connect() error {
	c, err := dial(r.ctx, r.cfg, r.lang)
	if err != nil {
		return err
	}
	r.conn = c
	r.gen++
	go r.read(c, r.gen)
	return nil
}

func dial(ctx context.Context, cfg RealtimeConfig, lang string) (*websocket.Conn, error) {
	u, err := cfg.realtime()
	if err != nil {
		return nil, err
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(dctx, u, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + cfg.Key},
		"User-Agent":    {"Tomoru"},
	}})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuth
		}
		if resp != nil {
			return nil, &Error{resp.StatusCode, err.Error()}
		}
		return nil, fmt.Errorf("openai: %w", err)
	}
	c.SetReadLimit(1 << 20)
	model := cfg.Model
	if model == "" {
		model = RealtimeModels[0]
	}
	tr := map[string]any{"model": model}
	if lang != "" {
		tr["languages"] = []string{lang}
	}
	if k := cfg.keywords(); len(k) > 0 {
		tr["keywords"] = k
	}
	if about := []rune(strings.TrimSpace(cfg.About)); len(about) > 0 {
		tr["prompt"] = string(about[:min(len(about), maxAbout)])
	}
	b, _ := json.Marshal(map[string]any{"type": "session.update", "session": map[string]any{
		"type": "transcription",
		"audio": map[string]any{"input": map[string]any{
			"format":         map[string]any{"type": "audio/pcm", "rate": serviceRate},
			"transcription":  tr,
			"turn_detection": nil,
		}},
	}})
	if err := c.Write(dctx, websocket.MessageText, b); err != nil {
		c.CloseNow()
		return nil, fmt.Errorf("openai: %w", err)
	}
	for {
		var ev rtEvent
		if err := readEvent(dctx, c, &ev); err != nil {
			c.CloseNow()
			return nil, err
		}
		switch ev.Type {
		case "session.updated":
			return c, nil
		case "error":
			c.CloseNow()
			return nil, ev.failure()
		}
	}
}

func (c Config) realtime() (string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.URL), "/")
	if base == "" {
		base = DefaultURL
	}
	u, err := url.Parse(strings.TrimSuffix(base, "/v1") + "/v1/realtime")
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("openai: not an address: %q", c.URL)
	}
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.RawQuery = "intent=transcription"
	return u.String(), nil
}

func (ev rtEvent) failure() error {
	if ev.Error.Code == "invalid_api_key" {
		return ErrAuth
	}
	return &Error{Message: ev.Error.Message}
}

func readEvent(ctx context.Context, c *websocket.Conn, ev *rtEvent) error {
	typ, b, err := c.Read(ctx)
	if err != nil {
		var ce websocket.CloseError
		if errors.As(err, &ce) {
			return &Error{Message: fmt.Sprintf("closed (%d): %s", ce.Code, ce.Reason)}
		}
		return fmt.Errorf("openai: %w", err)
	}
	if typ != websocket.MessageText {
		return nil
	}
	return json.Unmarshal(b, ev)
}

func (r *Realtime) read(c *websocket.Conn, gen int) {
	for {
		var ev rtEvent
		err := readEvent(r.ctx, c, &ev)
		select {
		case r.srv <- serverMsg{gen, err, ev}:
		case <-r.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func TestRealtime(ctx context.Context, cfg Config) (time.Duration, error) {
	t := time.Now()
	c, err := dial(ctx, RealtimeConfig{Config: cfg}, "")
	if err != nil {
		return time.Since(t), err
	}
	took := time.Since(t)
	c.Close(websocket.StatusNormalClosure, "")
	return took, nil
}

func (r *Realtime) take(x rtInput) error {
	if x.voice != nil {
		r.detect = batch.DetectionOn
		t := samplesOf(x.voice.T)
		switch {
		case x.voice.On && !r.voiceOn:
			r.voiceOn, r.onAt = true, t
		case !x.voice.On:
			r.voiceOn, r.offAt = false, t
		}
		return nil
	}
	n := samples(len(x.pcm) / 2)
	r.recent = append(r.recent, x.pcm[:n.bytes()]...)
	r.total += n
	if r.inTurn {
		r.send(r.total - n)
	}
	if err := r.step(); err != nil {
		return err
	}
	if keep := samplesOf(2); r.total-r.recentAt > 2*keep {
		cut := r.total - keep
		r.recent = append(r.recent[:0], r.recent[(cut-r.recentAt).bytes():]...)
		r.recentAt = cut
	}
	if r.total-r.clocked >= engineRate/10 {
		r.clocked = r.total
		r.on(engine.Event{Type: engine.EventClock, T: r.total.sec()})
	}
	return nil
}

func (r *Realtime) step() error {
	if r.detect == batch.DetectionUnknown && r.total >= samplesOf(undetected) {
		r.detect = batch.DetectionNone
		log.Printf("openai: the engine detects no voice: one turn every %g s", longest)
	}
	var cur *turn
	if r.inTurn {
		cur = r.turns[len(r.turns)-1]
	}
	switch {
	case r.detect == batch.DetectionUnknown:
	case !r.inTurn && (r.detect == batch.DetectionNone || r.voiceOn && r.total >= r.onAt+samplesOf(minSpeech)):
		start := max(r.sentTo, r.recentAt)
		if r.detect == batch.DetectionOn {
			start = max(start, r.onAt-samplesOf(preroll))
		}
		if r.conn == nil {
			if err := r.connect(); err != nil {
				return err
			}
		}
		r.inTurn, r.up = true, upsampler{}
		r.turns = append(r.turns, &turn{start: start})
		r.on(engine.Event{Type: engine.EventVoice, On: true, T: max(start, r.onAt).sec()})
		r.send(start)
	case r.inTurn && r.total-cur.start >= samplesOf(2*longest):
		r.commit(r.total)
	case r.inTurn && r.detect == batch.DetectionOn && !r.voiceOn:
		g := gap
		if r.total-cur.start >= samplesOf(longest) || strings.ContainsAny(cur.text.String(), "。．.！!？?") {
			g = 0
		}
		if r.total >= r.offAt+samplesOf(g) {
			r.commit(r.offAt)
		}
	case r.inTurn && r.detect == batch.DetectionNone && r.total-cur.start >= samplesOf(longest):
		r.commit(r.total)
	}
	return nil
}

func (r *Realtime) send(from samples) {
	from = max(from, r.sentTo, r.recentAt)
	if from >= r.total || r.conn == nil {
		return
	}
	pcm := r.up.convert(r.recent[(from - r.recentAt).bytes():])
	r.sentTo = r.total
	r.mu.Lock()
	r.sent += float64(r.total-from) / engineRate
	r.mu.Unlock()
	b, _ := json.Marshal(map[string]string{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(pcm)})
	r.write(b)
}

func (r *Realtime) write(b []byte) {
	if r.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	err := r.conn.Write(ctx, websocket.MessageText, b)
	if ev := batch.Stalled(time.Since(start)); ev.Type != "" {
		r.on(ev)
	}
	if err != nil {
		log.Printf("openai: realtime: %v", err)
		r.drop()
	}
}

func (r *Realtime) commit(end samples) {
	t := r.turns[len(r.turns)-1]
	r.inTurn = false
	t.end, t.committed = max(end, t.start), time.Now()
	r.on(engine.Event{Type: engine.EventVoice, On: false, T: t.end.sec()})
	if r.conn == nil {
		r.finalize(t)
		r.flush()
		return
	}
	r.write([]byte(`{"type":"input_audio_buffer.commit"}`))
}

func (r *Realtime) server(m serverMsg) {
	if m.gen != r.gen {
		return
	}
	if m.err != nil {
		log.Printf("openai: realtime: the connection ended: %v", m.err)
		r.drop()
		return
	}
	ev := m.ev
	switch ev.Type {
	case "conversation.item.input_audio_transcription.delta":
		if t := r.turnForDelta(ev.ItemID); t != nil {
			t.text.WriteString(ev.Delta)
			r.volatile()
		}
	case "conversation.item.input_audio_transcription.completed":
		if at, ok := r.lateCommits[ev.ItemID]; ok {
			delete(r.lateCommits, ev.ItemID)
			r.on(batch.Took(time.Since(at)))
			return
		}
		if t := r.turnForDelta(ev.ItemID); t != nil {
			if !t.committed.IsZero() && t.final == nil {
				r.on(batch.Took(time.Since(t.committed)))
			}
			text := ev.Transcript
			t.final = &text
			r.flush()
		}
	case "conversation.item.input_audio_transcription.failed":
		log.Printf("openai: realtime: a turn's transcription failed: %s", ev.Error.Message)
		if t := r.turnForDelta(ev.ItemID); t != nil {
			r.finalize(t)
			r.flush()
		}
	case "input_audio_buffer.committed":
		r.ackCommit(ev.ItemID)
	case "error":
		log.Printf("openai: realtime: %s: %s", ev.Error.Code, ev.Error.Message)
	}
}

func (r *Realtime) known(id itemID) *turn {
	for _, t := range r.turns {
		if t.id == id {
			return t
		}
	}
	return nil
}

func (r *Realtime) turnForDelta(id itemID) *turn {
	if t := r.known(id); t != nil {
		return t
	}
	for i := len(r.turns) - 1; i >= 0; i-- {
		if r.turns[i].id == "" {
			r.turns[i].id = id
			return r.turns[i]
		}
	}
	return nil
}

func (r *Realtime) ackCommit(id itemID) {
	if t := r.known(id); t != nil {
		t.acked = true
		return
	}
	for _, t := range r.turns {
		if !t.committed.IsZero() && !t.acked && t.id == "" {
			t.id, t.acked = id, true
			return
		}
	}
}

func (r *Realtime) drop() {
	if r.conn == nil {
		return
	}
	r.conn.CloseNow()
	r.conn, r.gen = nil, r.gen+1
	clear(r.lateCommits)
	if r.inTurn {
		r.inTurn = false
		r.on(engine.Event{Type: engine.EventVoice, On: false, T: r.total.sec()})
	}
	for _, t := range r.turns {
		r.finalize(t)
	}
	r.flush()
}

func (r *Realtime) overdue() {
	for _, t := range r.turns {
		if !t.committed.IsZero() && t.final == nil && time.Since(t.committed) > late {
			log.Printf("openai: realtime: a turn's final text did not come in %v", late)
			if t.id != "" {
				r.lateCommits[t.id] = t.committed
			}
			r.finalize(t)
		}
	}
	r.flush()
}

func (r *Realtime) finalize(t *turn) {
	if t.final == nil {
		text := t.text.String()
		t.final = &text
	}
	if t.end == 0 {
		t.end = max(r.total, t.start)
	}
}

func (r *Realtime) flush() {
	n := 0
	for n < len(r.turns) && r.turns[n].final != nil && !r.turns[n].committed.IsZero() {
		r.on(final(r.turns[n]))
		n++
	}
	if n > 0 {
		r.turns = append(r.turns[:0], r.turns[n:]...)
		r.volatile()
	}
}

func (r *Realtime) volatile() {
	if len(r.turns) == 0 {
		return
	}
	var b strings.Builder
	for _, t := range r.turns {
		if !t.committed.IsZero() && t.final == nil && t.text.Len() == 0 {
			break
		}
		b.WriteString(spaced(t.text.String()))
	}
	text := spaced(b.String())
	if text == "" {
		return
	}
	r.on(engine.Event{Type: engine.EventResult, Start: r.turns[0].start.sec(), End: r.total.sec(), Tokens: []engine.Token{{Text: text}}})
}

func (r *Realtime) finish() error {
	if r.inTurn {
		r.commit(r.total)
	}
	deadline := time.After(finishing)
	for len(r.turns) > 0 && r.conn != nil {
		select {
		case m := <-r.srv:
			r.server(m)
		case <-deadline:
			r.drop()
		}
	}
	for _, t := range r.turns {
		r.finalize(t)
	}
	r.flush()
	return nil
}

func final(t *turn) engine.Event {
	pieces := batch.Spaced(batch.Pieces(*t.final))
	t0, t1 := float64(t.start)/engineRate, float64(t.end)/engineRate
	batch.Spread(pieces, t1-t0)
	tokens := []engine.Token{}
	for i, p := range pieces {
		end := t1
		if i+1 < len(pieces) {
			end = t0 + pieces[i+1].At
		}
		start, until := batch.Round(t0+p.At), batch.Round(min(end, t1))
		tokens = append(tokens, engine.Token{Text: p.Text, Start: &start, End: &until})
	}
	return engine.Event{Type: engine.EventResult, Final: true, Start: batch.Round(t0), End: batch.Round(t1), Tokens: tokens}
}

func spaced(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return ""
	}
	return batch.Spaced([]batch.Piece{{Text: s}})[0].Text
}

const (
	phasePerInput  = serviceRate / rateUnit
	phasePerOutput = engineRate / rateUnit
)

type upsampler struct {
	last  int16
	have  bool
	phase int
}

func (u *upsampler) convert(pcm []byte) []byte {
	out := make([]byte, 0, len(pcm)*3/2+4)
	put := func(v int) { out = binary.LittleEndian.AppendUint16(out, uint16(int16(v))) }
	for i := 0; i+1 < len(pcm); i += 2 {
		x := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if !u.have {
			u.last, u.have, u.phase = x, true, phasePerOutput
			put(int(x))
			continue
		}
		for ; u.phase <= phasePerInput; u.phase += phasePerOutput {
			put(int(u.last) + (int(x)-int(u.last))*u.phase/phasePerInput)
		}
		u.phase -= phasePerInput
		u.last = x
	}
	return out
}
