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

// Realtime transcription: a transcription session of OpenAI's Realtime API, which streams the text
// of what it is sent as it hears it, and gives the final text of each turn the app commits
// (docs/recognition.md, step 6). The model detects no turns itself: the engine's voice detection
// cuts speech into turns, as the batch runner cuts stretches, and only they are sent, as the
// service bills the audio it is sent.

// LiveModel is OpenAI's model for live transcription.
const LiveModel = "gpt-live-transcribe"

// RealtimeModels are the models transcribed in a realtime session, the default first; others go to
// the transcription API.
var RealtimeModels = []string{LiveModel, "gpt-realtime-whisper"}

// Realtimed tells whether model ("" for the default) is transcribed in a realtime session.
func Realtimed(model string) bool { return model == "" || slices.Contains(RealtimeModels, model) }

const (
	rate = 16000 // the engine's audio
	// The service takes PCM at 24 kHz at least: the engine's is resampled.
	rtRate = 24000
	// minSpeech is the voice that starts a turn: shorter is a click, or a cough.
	minSpeech = .25
	// preroll is the audio before the voice a turn starts with: detection is late for it.
	preroll = .4
	// gap is the pause that ends a turn; once the model has ended a sentence in it, or past
	// longest, the first pause does; past twice longest, it ends wherever it is.
	gap     = .5
	longest = 8.0
	// undetected is the audio after which an engine that sent no voice event detects none: then
	// all of it is sent, a turn every longest.
	undetected = 1.0
	// late is how long a turn committed may go without its final text: then its streamed text is.
	// The turns after it wait for it, for their order: as long as 10 s held every subtitle after a
	// final text the service lost.
	late = 2 * time.Second
	// finishing is how long the last turns' final text is waited for, as the session stops.
	finishing = 5 * time.Second
)

// Realtime is a session of realtime transcription. Its methods are those of a recognizer to a live
// session: Audio, Voice, Finalize, Stop, Done, Err, and Sent.
type Realtime struct {
	cfg  Config
	lang string
	on   func(engine.Event)

	in       chan rtInput
	srv      chan rtEvent
	quit     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	err      error // why it ended, set before done closes
	ctx      context.Context
	cancel   context.CancelFunc

	mu   sync.Mutex
	sent float64 // seconds of audio sent, which the service bills

	// The loop's.
	conn     *websocket.Conn
	gen      int   // the connection's number, to tell its events from an earlier one's
	total    int64 // samples heard
	clocked  int64
	detect   int // the engine's voice detection: 0 unknown yet, 1 on, -1 none
	voiceOn  bool
	onAt     int64
	offAt    int64
	recent   []byte // the last seconds of audio, from recentAt: a turn's preroll
	recentAt int64
	open     bool  // in a turn
	sentTo   int64 // the audio sent goes through here
	up       upsampler
	turns    []*turn // the turns not final yet, in order
	// lateTurns are the turns given their streamed text for want of their final text, by item ID,
	// and when they were committed: their final text, coming, still tells how long it took.
	lateTurns map[string]time.Time
}

type turn struct {
	id         string
	acked      bool  // the service said it committed it
	start, end int64 // samples; end once committed
	committed  time.Time
	text       strings.Builder // the streamed text
	final      *string
}

type rtInput struct {
	pcm   []byte
	voice *engine.Event
}

// rtEvent is an event of the service, or the end of its connection (err).
type rtEvent struct {
	gen        int
	err        error
	Type       string `json:"type"`
	ItemID     string `json:"item_id"`
	Delta      string `json:"delta"`
	Transcript string `json:"transcript"`
	Error      struct {
		Type, Code, Message string
	} `json:"error"`
}

// StartRealtime starts recognizing lang ("ja") with the service of cfg, calling on with each
// event, in order, from a goroutine of its own. It connects in the background, keeping the audio
// given meanwhile; a refused key or a configuration the service refused ends it.
func StartRealtime(cfg Config, lang string, on func(engine.Event)) *Realtime {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Realtime{cfg: cfg, lang: iso639(lang), on: on, in: make(chan rtInput, 600), srv: make(chan rtEvent, 256),
		quit: make(chan struct{}), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	r.lateTurns = map[string]time.Time{}
	go r.loop()
	return r
}

// Audio takes an audio event of the engine.
func (r *Realtime) Audio(ev engine.Event) error {
	b, err := base64.StdEncoding.DecodeString(ev.PCM)
	if err != nil {
		return fmt.Errorf("openai: audio: %w", err)
	}
	return r.put(rtInput{pcm: b})
}

// Voice takes a voice event of the engine.
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

// Stop ends the session: the turn going on is committed, and its final text waited for a while.
// It returns at once; Done tells when it has ended.
func (r *Realtime) Stop() { r.stopOnce.Do(func() { close(r.quit) }) }

// Done is closed when the session has ended.
func (r *Realtime) Done() <-chan struct{} { return r.done }

// Err is why the session ended, after Done: nil when stopped.
func (r *Realtime) Err() error { return r.err }

// Sent is the seconds of audio sent, which the service bills.
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
		case ev := <-r.srv:
			r.server(ev)
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

// connect opens a transcription session, and has it configured, before it reads its events.
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

// dial connects to the service and configures a session: transcription in lang ("" to tell),
// the turns the app's.
func dial(ctx context.Context, cfg Config, lang string) (*websocket.Conn, error) {
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
	// No "delay": the model's default was the most accurate, and as quick as "low"; "high" was
	// later and less accurate, the turns being committed at the speaker's pauses anyway.
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
			"format":         map[string]any{"type": "audio/pcm", "rate": rtRate},
			"transcription":  tr,
			"turn_detection": nil,
		}},
	}})
	if err := c.Write(dctx, websocket.MessageText, b); err != nil {
		c.CloseNow()
		return nil, fmt.Errorf("openai: %w", err)
	}
	// Refused, the session would go on as the service's default, hearing turns itself and
	// transcribing nothing: its error ends it.
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

// realtime is the service's WebSocket address for transcription.
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

// read passes the connection's events to the loop, then its end.
func (r *Realtime) read(c *websocket.Conn, gen int) {
	for {
		var ev rtEvent
		err := readEvent(r.ctx, c, &ev)
		ev.gen, ev.err = gen, err
		select {
		case r.srv <- ev:
		case <-r.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

// Test connects to the service and configures a session, as recognizing would, then ends it: it
// tells whether cfg works, and how long that took.
func TestRealtime(ctx context.Context, cfg Config) (time.Duration, error) {
	t := time.Now()
	c, err := dial(ctx, cfg, "")
	if err != nil {
		return time.Since(t), err
	}
	took := time.Since(t)
	c.Close(websocket.StatusNormalClosure, "")
	return took, nil
}

// take follows the engine's audio and voice.
func (r *Realtime) take(x rtInput) error {
	if x.voice != nil {
		r.detect = 1
		t := int64(x.voice.T*rate + .5)
		switch {
		case x.voice.On && !r.voiceOn:
			r.voiceOn, r.onAt = true, t
		case !x.voice.On:
			r.voiceOn, r.offAt = false, t
		}
		return nil
	}
	n := int64(len(x.pcm) / 2)
	r.recent = append(r.recent, x.pcm[:2*n]...)
	r.total += n
	if r.open {
		if err := r.send(r.total - n); err != nil {
			return err
		}
	}
	if err := r.step(); err != nil {
		return err
	}
	if keep := int64(2 * rate); r.total-r.recentAt > 2*keep {
		cut := r.total - keep
		r.recent = append(r.recent[:0], r.recent[2*(cut-r.recentAt):]...)
		r.recentAt = cut
	}
	if r.total-r.clocked >= rate/10 {
		r.clocked = r.total
		r.on(engine.Event{Type: "clock", T: sec(r.total)})
	}
	return nil
}

// step opens a turn when voice starts, and commits it when it ends.
func (r *Realtime) step() error {
	if r.detect == 0 && r.total >= int64(undetected*rate) {
		r.detect = -1
		log.Printf("openai: the engine detects no voice: one turn every %g s", longest)
	}
	var cur *turn
	if r.open {
		cur = r.turns[len(r.turns)-1]
	}
	switch {
	case r.detect == 0:
	case !r.open && (r.detect < 0 || r.voiceOn && r.total >= r.onAt+int64(minSpeech*rate)):
		start := max(r.sentTo, r.recentAt)
		if r.detect > 0 {
			start = max(start, r.onAt-int64(preroll*rate))
		}
		if r.conn == nil {
			if err := r.connect(); err != nil {
				return err
			}
		}
		r.open, r.up = true, upsampler{}
		r.turns = append(r.turns, &turn{start: start})
		r.on(engine.Event{Type: "voice", On: true, T: sec(max(start, r.onAt))})
		return r.send(start)
	case r.open && r.total-cur.start >= int64(2*longest*rate):
		return r.commit(r.total) // no pause came: where it is
	case r.open && r.detect > 0 && !r.voiceOn:
		g := gap
		if r.total-cur.start >= int64(longest*rate) || strings.ContainsAny(cur.text.String(), "。．.！!？?") {
			g = 0 // past a sentence's end or longest, the first pause ends it
		}
		if r.total >= r.offAt+int64(g*rate) {
			return r.commit(r.offAt)
		}
	case r.open && r.detect < 0 && r.total-cur.start >= int64(longest*rate):
		return r.commit(r.total)
	}
	return nil
}

// send sends the audio heard from, through what is heard.
func (r *Realtime) send(from int64) error {
	from = max(from, r.sentTo, r.recentAt)
	if from >= r.total || r.conn == nil {
		return nil
	}
	pcm := r.up.convert(r.recent[2*(from-r.recentAt):])
	r.sentTo = r.total
	r.mu.Lock()
	r.sent += float64(r.total-from) / rate
	r.mu.Unlock()
	b, _ := json.Marshal(map[string]string{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(pcm)})
	return r.write(b)
}

// write sends a message; a connection that failed is dropped, to be made again by the next turn.
func (r *Realtime) write(b []byte) error {
	if r.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	err := r.conn.Write(ctx, websocket.MessageText, b)
	if ev := batch.Stalled(time.Since(start)); ev.Type != "" {
		r.on(ev) // what a turn's final text takes says the rest
	}
	if err != nil {
		log.Printf("openai: realtime: %v", err)
		r.drop()
	}
	return nil
}

// commit ends the turn going on at end, for the service to give its final text.
func (r *Realtime) commit(end int64) error {
	t := r.turns[len(r.turns)-1]
	r.open = false
	t.end, t.committed = max(end, t.start), time.Now()
	r.on(engine.Event{Type: "voice", On: false, T: sec(t.end)})
	if r.conn == nil {
		r.finalize(t)
		r.flush()
		return nil
	}
	return r.write([]byte(`{"type":"input_audio_buffer.commit"}`))
}

// server follows the service's events.
func (r *Realtime) server(ev rtEvent) {
	if ev.gen != r.gen {
		return // an earlier connection's
	}
	if ev.err != nil {
		log.Printf("openai: realtime: the connection ended: %v", ev.err)
		r.drop()
		return
	}
	switch ev.Type {
	case "conversation.item.input_audio_transcription.delta":
		if t := r.turnFor(ev.ItemID, false); t != nil {
			t.text.WriteString(ev.Delta)
			r.volatile()
		}
	case "conversation.item.input_audio_transcription.completed":
		if at, ok := r.lateTurns[ev.ItemID]; ok {
			delete(r.lateTurns, ev.ItemID)
			r.on(batch.Took(time.Since(at))) // a turn given its streamed text: it came, late
			return
		}
		if t := r.turnFor(ev.ItemID, false); t != nil {
			if !t.committed.IsZero() && t.final == nil {
				r.on(batch.Took(time.Since(t.committed)))
			}
			text := ev.Transcript
			t.final = &text
			r.flush()
		}
	case "conversation.item.input_audio_transcription.failed":
		log.Printf("openai: realtime: a turn's transcription failed: %s", ev.Error.Message)
		if t := r.turnFor(ev.ItemID, false); t != nil {
			r.finalize(t)
			r.flush()
		}
	case "input_audio_buffer.committed":
		r.turnFor(ev.ItemID, true)
	case "error":
		log.Printf("openai: realtime: %s: %s", ev.Error.Code, ev.Error.Message)
	}
}

// turnFor is the turn of the service's item id. A new item is the latest turn's, as the text of a
// turn streams before it is committed; committed says the service committed it, which it does in
// the order the turns were committed.
func (r *Realtime) turnFor(id string, committed bool) *turn {
	for _, t := range r.turns {
		if t.id == id {
			t.acked = t.acked || committed
			return t
		}
	}
	if committed {
		for _, t := range r.turns {
			if !t.committed.IsZero() && !t.acked && t.id == "" {
				t.id, t.acked = id, true
				return t
			}
		}
		return nil
	}
	for i := len(r.turns) - 1; i >= 0; i-- {
		if r.turns[i].id == "" {
			r.turns[i].id = id
			return r.turns[i]
		}
	}
	return nil
}

// drop forgets the connection: the turns it owed final text have their streamed text.
func (r *Realtime) drop() {
	if r.conn == nil {
		return
	}
	r.conn.CloseNow()
	r.conn, r.gen = nil, r.gen+1
	clear(r.lateTurns) // the next connection's items are others
	if r.open {
		r.open = false
		r.on(engine.Event{Type: "voice", On: false, T: sec(r.total)})
	}
	for _, t := range r.turns {
		r.finalize(t)
	}
	r.flush()
}

// overdue gives the turns whose final text is late their streamed text.
func (r *Realtime) overdue() {
	for _, t := range r.turns {
		if !t.committed.IsZero() && t.final == nil && time.Since(t.committed) > late {
			log.Printf("openai: realtime: a turn's final text did not come in %v", late)
			if t.id != "" {
				r.lateTurns[t.id] = t.committed
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

// flush sends the turns' final results, in order, as far as they have come.
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

// volatile sends the text streamed of the turns not final yet, from the first, up to a turn
// committed whose text has not come: a short turn's comes a second late, and the next turn's
// sentence, committed from its volatile text, went before it (Sekiro's 「狼よ、」 after the line
// it starts).
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
	text := spaced(b.String()) // after the final text before it, as a turn's is
	if text == "" {
		return
	}
	r.on(engine.Event{Type: "result", Start: sec(r.turns[0].start), End: sec(r.total), Tokens: []engine.Token{{Text: text}}})
}

// finish commits the turn going on, and waits a while for the final text of those committed.
func (r *Realtime) finish() error {
	if r.open {
		r.commit(r.total)
	}
	deadline := time.After(finishing)
	for len(r.turns) > 0 && r.conn != nil {
		select {
		case ev := <-r.srv:
			r.server(ev)
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

// final is a turn's final result, its pieces timed by their letters over its audio.
func final(t *turn) engine.Event {
	pieces := batch.Spaced(batch.Pieces(*t.final))
	t0, t1 := float64(t.start)/rate, float64(t.end)/rate
	batch.Spread(pieces, t1-t0)
	tokens := []engine.Token{}
	for i, p := range pieces {
		end := t1
		if i+1 < len(pieces) {
			end = t0 + pieces[i+1].At
		}
		start, until := round(t0+p.At), round(min(end, t1))
		tokens = append(tokens, engine.Token{Text: p.Text, Start: &start, End: &until})
	}
	return engine.Event{Type: "result", Final: true, Start: round(t0), End: round(t1), Tokens: tokens}
}

// spaced is a turn's text, starting with a space where its script spaces words.
func spaced(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return ""
	}
	return batch.Spaced([]batch.Piece{{Text: s}})[0].Text
}

func sec(n int64) float64 { return round(float64(n) / rate) }

func round(x float64) float64 { return float64(int64(x*1000+.5)) / 1000 }

// upsampler makes the engine's 16 kHz 16-bit PCM 24 kHz, interpolating linearly, a turn of it at
// a time, chunk by chunk.
type upsampler struct {
	last int16
	have bool
	pos  int // the next output's position after last, in thirds of a 16 kHz sample
}

func (u *upsampler) convert(pcm []byte) []byte {
	out := make([]byte, 0, len(pcm)*3/2+4)
	put := func(v int) { out = binary.LittleEndian.AppendUint16(out, uint16(int16(v))) }
	for i := 0; i+1 < len(pcm); i += 2 {
		x := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if !u.have {
			u.last, u.have, u.pos = x, true, 2
			put(int(x))
			continue
		}
		for ; u.pos <= 3; u.pos += 2 {
			put(int(u.last) + (int(x)-int(u.last))*u.pos/3)
		}
		u.pos -= 3
		u.last = x
	}
	return out
}
