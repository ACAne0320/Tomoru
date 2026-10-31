// Package qwencloud recognizes speech in the cloud: Qwen ASR's streaming models on Alibaba Cloud
// Model Studio (DashScope), over its WebSocket API. A Session takes the audio the engine hears with
// `listen --asr none` and reports what it recognizes as the engine's protocol does
// (docs/engine-protocol.md), so the segmenter takes it as it takes the engine's.
package qwencloud

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/coder/websocket"

	"tomoru/internal/asr/batch"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/models"
)

// Models are the streaming models, the newest first.
var Models = []string{"qwen-audio-3.1-asr-flash-streaming", "qwen-audio-3.0-asr-flash-streaming"}

// Langs are the languages the models recognize: Qwen3-ASR's, as its model card lists them.
var Langs = models.Qwen3ASRLangs

// DefaultHost is Model Studio's in Beijing; a workspace has a domain of its own too.
const DefaultHost = "dashscope.aliyuncs.com"

// Config is a service to recognize with, and what to tell it to expect.
type Config struct {
	Host  string // a domain, with or without https://; "" for DefaultHost
	Model string // one of Models; "" for the first
	Key   string // the API key
	// Vocabulary are words to expect, names, as the service's hot words: the first MaxVocabulary.
	Vocabulary []string
	// Context is text that helps recognition, a scene's description and names: ContextLimit runes
	// at most.
	Context string
}

// The service's limits on what it is told to expect: hot words, which it takes as many as 2000
// of, and context, 400 characters a turn of it.
const (
	MaxVocabulary = 100
	ContextLimit  = 400
	// weight is how much a hot word is preferred, of 1–5 (or 50, for 50 words at most).
	weight = 4
)

// ErrAuth is an API key the service refused.
var ErrAuth = errors.New("qwencloud: the API key was refused")

// Error is a failure the service reported, or of the connection to it (Code "connection").
type Error struct{ Code, Message string }

func (e *Error) Error() string { return "qwencloud: " + e.Code + ": " + e.Message }

// URL is the service's WebSocket address.
func (c Config) URL() (string, error) {
	h := strings.TrimSpace(c.Host)
	if h == "" {
		h = DefaultHost
	}
	if !strings.Contains(h, "://") {
		h = "https://" + h
	}
	u, err := url.Parse(h)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("qwencloud: not an address: %q", c.Host)
	}
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.Path, u.RawQuery = "/api-ws/v1/inference", ""
	return u.String(), nil
}

func (c Config) model() string {
	if c.Model == "" {
		return Models[0]
	}
	return c.Model
}

// silence is the pause, in ms, that ends a sentence for the service: its default, 1.3 s, made a
// twelve-second recording one sentence, final only at its end; 0.3 s made it final at the pause
// in it, 0.5–0.9 s after. 0.2 s, the least, and multi_threshold_mode_enabled split it no more: the
// pauses between TTS sentences are shorter. Sentences commit as the volatile text punctuates them.
const silence = 300

// The service bills the audio it is sent, silence too (usage.duration counted 15 s of silence
// before speech). So the session hangs up after idle seconds of silence: no voice, by the
// engine's voice detection, and no sound. Voice detection alone hung up on a song, whose voice it
// never heard under the music (the benchmark's en-music-billie, recognized for 10 s). It calls
// again when voice starts or sound comes, sending the preroll seconds of audio before it first:
// detection starts late, and the service drops a syllable it hears first thing.
const (
	idle    = 10
	preroll = 0.5
	keep    = 2 // seconds of audio kept to send as preroll: voice detection comes after its audio
	// loud is the peak, of 32768, of audio with sound in it: a paused video or nothing playing
	// is digital silence, 0; -50 dBFS is quieter than any music's quiet passage.
	loud = 100
)

// lead is the silence a call starts with, in samples: with none, the service missed the first
// syllable of speech it started on ("皆さん" as "なさん"); 0.1 s was enough.
const lead = 3200

// Session is a stream of audio being recognized. Its methods are those of a recognizer to a live
// session: Audio, Voice, Finalize, Stop, Done and Err.
type Session struct {
	cfg  Config
	lang string

	audio  chan chunk // audio to send, in order, and hang-ups
	events chan engine.Event
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	err     error   // why it ended, set before done closes
	t0      float64 // the time of the call's first audio, in the engine's: the service counts from it
	heard   float64 // the time the audio given reaches
	clocked float64
	recent  []chunk // the last keep seconds of audio
	calling bool    // audio goes to the service: a call is on, or wanted
	gated   bool    // voice detection is on: the session hangs up while nobody speaks
	speak   bool    // someone speaks
	quiet   float64 // when voice last stopped
	sound   float64 // when the audio last had sound in it
	sent    float64 // the seconds of audio sent, which the service bills
}

// The connection is pinged every pingEvery during a call, a ping given up after pingWait: how long
// its round trips take tells package asr whether the network is slow (0.37 s at the median, 0.83 s
// at p90 on a good connection). What a final result is behind the audio tells nothing of it: 1.4 s
// at the median and 7.4 s on a good connection, as the service ends sentences at its silences,
// which music hides.
const (
	pingEvery = 3 * time.Second
	pingWait  = 5 * time.Second
)

// chunk is audio of the engine's from t, or, with no PCM, a hang-up.
type chunk struct {
	t   float64
	pcm []byte
}

// Start starts recognizing lang ("ja") with cfg. It connects in the background, keeping the
// audio given meanwhile, and calls onEvent with each event, in order, from a goroutine of its
// own, until it ends.
func Start(cfg Config, lang string, onEvent func(engine.Event)) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{cfg: cfg, lang: lang, audio: make(chan chunk, 600), events: make(chan engine.Event, 256),
		ctx: ctx, cancel: cancel, done: make(chan struct{}), calling: true}
	go func() {
		for {
			select {
			case ev := <-s.events:
				onEvent(ev)
			case <-s.done:
				return
			}
		}
	}()
	go func() {
		err := s.run()
		var e *Error
		switch {
		case ctx.Err() != nil:
			err = nil // stopped
		case err != nil && !errors.Is(err, ErrAuth) && !errors.As(err, &e):
			err = &Error{"connection", strings.TrimPrefix(err.Error(), "qwencloud: ")}
		}
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		cancel()
		close(s.done)
	}()
	return s
}

// Audio takes an audio event of the engine: its PCM goes to the service while a call is on, and
// the audio clock on. Sound in it calls the service again, if the session hung up.
func (s *Session) Audio(ev engine.Event) error {
	pcm, err := base64.StdEncoding.DecodeString(ev.PCM)
	if err != nil {
		return err
	}
	c := chunk{ev.T, pcm}
	s.mu.Lock()
	s.heard = ev.T + float64(len(pcm))/32000
	clock := s.heard-s.clocked >= .1
	if clock {
		s.clocked = s.heard
	}
	heard := s.heard
	if peak(pcm) >= loud {
		s.sound = heard
		if !s.calling {
			s.callAgain(ev.T)
		}
	}
	// Replay only the earlier chunks when sound resumes; this chunk goes through
	// the normal send below exactly once, then remains available for later preroll.
	s.recent = append(s.recent, c)
	for len(s.recent) > 0 && s.recent[0].t < heard-keep {
		s.recent = s.recent[1:]
	}
	if s.calling {
		select {
		case s.audio <- c:
		default: // the connection is far behind: this audio goes without
		}
		if s.gated && !s.speak && heard-max(s.quiet, s.sound) >= idle {
			select {
			case s.audio <- chunk{t: heard}:
				s.calling = false
			default: // no room to hang up: the call goes on
			}
		}
	}
	s.mu.Unlock()
	if clock {
		s.emit(engine.Event{Type: "clock", T: round(heard)})
	}
	return nil
}

// Voice takes a voice event of the engine, which goes on as the session's: voice starting calls
// the service again, if the session hung up.
func (s *Session) Voice(ev engine.Event) {
	s.mu.Lock()
	s.gated, s.speak = true, ev.On
	if !ev.On {
		s.quiet = ev.T
	} else if !s.calling {
		s.callAgain(ev.T)
	}
	s.mu.Unlock()
	s.emit(ev)
}

// callAgain has the session call the service again, from the preroll before t: the audio kept
// from then goes first. It runs under s.mu.
func (s *Session) callAgain(t float64) {
	s.calling = true
	for _, c := range s.recent {
		if c.t+float64(len(c.pcm))/32000 > t-preroll {
			select {
			case s.audio <- c:
			default:
			}
		}
	}
}

// peak is the largest sample of 16-bit little-endian PCM, as a magnitude.
func peak(pcm []byte) int {
	m := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		m = max(m, v, -v)
	}
	return m
}

// Sent is the seconds of audio sent to the service so far, which it bills.
func (s *Session) Sent() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

// send sends audio on c, counting it.
func (s *Session) send(c *websocket.Conn, pcm []byte) error {
	start := time.Now()
	if err := c.Write(s.ctx, websocket.MessageBinary, pcm); err != nil {
		return fmt.Errorf("qwencloud: %w", err)
	}
	if ev := batch.Stalled(time.Since(start)); ev.Type != "" {
		s.emit(ev)
	}
	s.mu.Lock()
	s.sent += float64(len(pcm)) / 32000 // 16 kHz, 16-bit
	s.mu.Unlock()
	return nil
}

// Stop ends the session.
func (s *Session) Stop() { s.cancel() }

// Done is closed when the session has ended.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err is why the session ended, after Done: nil when stopped.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Session) emit(ev engine.Event) {
	select {
	case s.events <- ev:
	case <-s.ctx.Done():
	}
}

// run calls the service, and again each time the session hung up and voice starts.
func (s *Session) run() error {
	// The first call is at once, for a refused key to show as translating starts.
	var first *chunk
	for {
		c, err := connect(s.ctx, s.cfg, s.lang)
		if err != nil {
			return err
		}
		if err := s.call(c, first); err != nil || s.ctx.Err() != nil {
			return err
		}
		// Hung up: the next call is when there is audio to send.
		log.Println("qwencloud: hung up, nothing heard")
		first = nil
		for first == nil {
			select {
			case <-s.ctx.Done():
				return nil
			case a := <-s.audio:
				if a.pcm != nil {
					first = &a
				}
			}
		}
		log.Printf("qwencloud: calling again for sound or voice at %.1fs", first.t)
	}
}

// start sends a call's first audio, after the lead's silence: the service counts from there.
func (s *Session) start(c *websocket.Conn, first chunk) error {
	s.mu.Lock()
	s.t0 = first.t - lead/16000.
	s.mu.Unlock()
	for _, pcm := range [][]byte{make([]byte, 2*lead), first.pcm} {
		if err := s.send(c, pcm); err != nil {
			return err
		}
	}
	return nil
}

// call sends the audio on c, starting with first if not nil, while the results come, until the
// session hangs up (nil), is stopped (nil), or the call fails.
func (s *Session) call(c *websocket.Conn, first *chunk) error {
	defer c.CloseNow()
	if first != nil {
		if err := s.start(c, *first); err != nil {
			return err
		}
	}
	started := first != nil
	read := make(chan error, 1)
	go func() { read <- s.read(c) }()
	pings := time.NewTicker(pingEvery)
	defer pings.Stop()
	for {
		select {
		case <-pings.C:
			go s.ping(c)
		case <-s.ctx.Done():
			// Stopped: finishing the task would only bring results nobody reads.
			c.Close(websocket.StatusNormalClosure, "")
			return nil
		case err := <-read:
			return err
		case a := <-s.audio:
			switch {
			case a.pcm == nil:
				// Nobody has spoken for a while; what was said has had its results.
				c.Close(websocket.StatusNormalClosure, "")
				<-read
				return nil
			case !started:
				if err := s.start(c, a); err != nil {
					return err
				}
			default:
				if err := s.send(c, a.pcm); err != nil {
					return err
				}
			}
			started = true
		}
	}
}

// ping times a round trip to the service: as the connection is slow, or the audio sent waits,
// the pong comes late. One that does not come in pingWait is that late.
func (s *Session) ping(c *websocket.Conn) {
	ctx, cancel := context.WithTimeout(s.ctx, pingWait)
	defer cancel()
	start := time.Now()
	err := c.Ping(ctx)
	if s.ctx.Err() != nil {
		return
	}
	took := time.Since(start)
	if err != nil {
		took = pingWait
	}
	s.emit(batch.Took(took))
}

// connect dials the service and starts a task of recognizing lang, waiting for it to start.
func connect(ctx context.Context, cfg Config, lang string) (*websocket.Conn, error) {
	u, err := cfg.URL()
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
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, ErrAuth
		}
		return nil, fmt.Errorf("qwencloud: %w", err)
	}
	c.SetReadLimit(1 << 20)
	id := make([]byte, 16)
	rand.Read(id)
	task := hex.EncodeToString(id)
	params := map[string]any{"format": "pcm", "sample_rate": 16000, "max_sentence_silence": silence, "heartbeat": true}
	if lang != "" {
		params["language_hints"] = []string{lang}
	}
	if len(cfg.Vocabulary) > 0 {
		words := map[string]int{}
		for _, w := range cfg.Vocabulary[:min(len(cfg.Vocabulary), MaxVocabulary)] {
			words[w] = weight
		}
		params["vocabulary"] = words
	}
	input := map[string]any{}
	if cfg.Context != "" {
		input["context"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": cfg.Context}}}}
	}
	b, _ := json.Marshal(map[string]any{
		"header": map[string]any{"action": "run-task", "task_id": task, "streaming": "duplex"},
		"payload": map[string]any{"task_group": "audio", "task": "asr", "function": "recognition",
			"model": cfg.model(), "parameters": params, "input": input},
	})
	if err := c.Write(dctx, websocket.MessageText, b); err != nil {
		c.CloseNow()
		return nil, fmt.Errorf("qwencloud: %w", err)
	}
	for {
		var ev event
		if err := readEvent(dctx, c, &ev); err != nil {
			c.CloseNow()
			return nil, err
		}
		switch ev.Header.Event {
		case "task-started":
			return c, nil
		case "task-failed":
			c.CloseNow()
			return nil, ev.err()
		}
	}
}

// Test connects to the service and starts a task, as recognizing would, then ends it: it tells
// whether cfg works, and how long starting took.
func Test(ctx context.Context, cfg Config) (time.Duration, error) {
	t := time.Now()
	c, err := connect(ctx, cfg, "")
	if err != nil {
		return time.Since(t), err
	}
	took := time.Since(t)
	c.Close(websocket.StatusNormalClosure, "")
	return took, nil
}

type event struct {
	Header struct {
		Event   string `json:"event"`
		Code    string `json:"error_code"`
		Message string `json:"error_message"`
	} `json:"header"`
	Payload struct {
		Output struct {
			Sentence struct {
				Begin     float64  `json:"begin_time"`
				End       *float64 `json:"end_time"`
				Text      string   `json:"text"`
				Final     bool     `json:"sentence_end"`
				Heartbeat bool     `json:"heartbeat"`
				Words     []struct {
					Begin       float64 `json:"begin_time"`
					End         float64 `json:"end_time"`
					Text        string  `json:"text"`
					Punctuation string  `json:"punctuation"`
				} `json:"words"`
			} `json:"sentence"`
		} `json:"output"`
	} `json:"payload"`
}

func (ev event) err() error {
	if strings.Contains(strings.ToLower(ev.Header.Code+ev.Header.Message), "auth") ||
		strings.Contains(strings.ToLower(ev.Header.Message), "api key") {
		return ErrAuth
	}
	return &Error{ev.Header.Code, ev.Header.Message}
}

func readEvent(ctx context.Context, c *websocket.Conn, ev *event) error {
	typ, b, err := c.Read(ctx)
	if err != nil {
		var ce websocket.CloseError
		if errors.As(err, &ce) {
			return &Error{fmt.Sprint(ce.Code), ce.Reason}
		}
		return fmt.Errorf("qwencloud: %w", err)
	}
	if typ != websocket.MessageText {
		return nil
	}
	return json.Unmarshal(b, ev)
}

// read turns the service's results into the protocol's until the connection ends.
func (s *Session) read(c *websocket.Conn) error {
	prev := "" // the sentence's last volatile text, as the service sent it
	for {
		var ev event
		if err := readEvent(s.ctx, c, &ev); err != nil {
			return err
		}
		switch ev.Header.Event {
		case "task-failed":
			return ev.err()
		case "task-finished":
			return errors.New("qwencloud: the service ended the task")
		case "result-generated":
		default:
			continue
		}
		sen := ev.Payload.Output.Sentence
		if sen.Heartbeat {
			continue
		}
		s.mu.Lock()
		t0, heard := s.t0, s.heard
		s.mu.Unlock()
		at := func(ms float64) float64 { return round(t0 + ms/1000) }
		if !sen.Final {
			// The sentence as heard so far, which the next replaces.
			text := steady(prev, sen.Text, s.lang)
			prev = sen.Text
			if text != "" {
				s.emit(engine.Event{Type: "result", Start: at(sen.Begin), End: round(heard), Tokens: []engine.Token{{Text: text}}})
			}
			continue
		}
		prev = ""
		var toks []engine.Token
		for _, w := range sen.Words {
			start, end := at(w.Begin), at(w.End)
			toks = append(toks, engine.Token{Text: w.Text + w.Punctuation, Start: &start, End: &end})
		}
		spaceAhead(toks)
		end := heard
		if sen.End != nil {
			end = at(*sen.End)
		}
		s.emit(engine.Event{Type: "result", Final: true, Start: at(sen.Begin), End: round(end), Tokens: toks})
	}
}

// steady is a sentence's volatile text cur, after prev, with the sentence ends it can be trusted
// with. The service's volatile text is what it settled on so far, then a preview of what it hears
// last, a guess that the next text often rewrites, ending sentences as it likes: "잘 놀다.다는 게."
// became "잘 놀다 간다는 게 회사", "어버하고. 옛날은 찦가." "워런 버핏하고 옛날에". The segmenter commits
// a sentence where the volatile text ends one, and the guess, rewritten, came again: Korean and
// Japanese words inserted, 8 and 11 in 100 (the benchmark's qwen31_cloud, 2026-10-17). So an end
// stays only where the text through it and the letter after it are as they were: no preview is,
// and in languages that space words (English, Korean), only with a space after it: a preview
// is joined to the end before it ("놀다.다는"). An end that ends the text stays once the text comes
// again as it was: nothing new was heard, as the speaker pauses. Han characters in the preview of
// a language not written with them, as Korean or English, go too: a sentence's first previews are
// often Chinese ("不不", "没差").
// Chinese keeps its ends: its errors came out the same without them, a third of a second later.
func steady(prev, cur, code string) string {
	if lang.Chinese(code) {
		return cur
	}
	p, c := []rune(prev), []rune(cur)
	same := 0
	for same < len(p) && same < len(c) && p[same] == c[same] {
		same++
	}
	spaced := lang.Spaced(code)
	var b strings.Builder
	for i, r := range c {
		switch {
		case strings.ContainsRune(".。!?！？", r):
			last := i == len(c)-1
			if last && prev != cur || !last && (i+1 >= same || spaced && !unicode.IsSpace(c[i+1])) {
				continue
			}
		case i >= same && code != "ja" && unicode.Is(unicode.Han, r):
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// spaceAhead moves the spaces that end tokens to the start of the next: the service puts a word's
// space before it (" chat") or after it ("the ", "chat. "), and a space ending a sentence's last
// token puts the sentence's end inside it, where the segmenter cannot tell it covers the token.
func spaceAhead(toks []engine.Token) {
	for i := range toks {
		t := strings.TrimRight(toks[i].Text, " ")
		if t != toks[i].Text && i+1 < len(toks) && !strings.HasPrefix(toks[i+1].Text, " ") {
			toks[i+1].Text = " " + toks[i+1].Text
		}
		toks[i].Text = t
	}
}

// round is seconds to the millisecond, as the protocol's times are.
func round(x float64) float64 { return float64(int64(x*1000+.5)) / 1000 }
