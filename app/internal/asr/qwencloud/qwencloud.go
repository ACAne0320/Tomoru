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

var Models = []string{"qwen-audio-3.1-asr-flash-streaming", "qwen-audio-3.0-asr-flash-streaming"}

var Langs = models.Qwen3ASRLangs

const DefaultHost = "dashscope.aliyuncs.com"

type Config struct {
	Host       string
	Model      string
	Key        string
	Vocabulary []string
	Context    string
}

const (
	MaxVocabulary = 100
	ContextLimit  = 400
	weight        = 4
)

var ErrAuth = errors.New("qwencloud: the API key was refused")

type Error struct{ Code, Message string }

func (e *Error) Error() string { return "qwencloud: " + e.Code + ": " + e.Message }

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

const silence = 300

const (
	idle    = 10
	preroll = 0.5
	keep    = 2
	loud    = 100
)

const lead = 3200

type Session struct {
	cfg  Config
	lang string

	audio  chan chunk
	events chan engine.Event
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	err     error
	t0      float64
	heard   float64
	clocked float64
	recent  []chunk
	calling bool
	behind  bool
	gated   bool
	speak   bool
	quiet   float64
	sound   float64
	sent    float64
}

const (
	pingEvery = 3 * time.Second
	pingWait  = 5 * time.Second
)

type chunk struct {
	t   float64
	pcm []byte
}

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
			err = nil
		case err != nil && !errors.Is(err, ErrAuth) && !errors.As(err, &e):
			err = &Error{"connection", strings.TrimPrefix(err.Error(), "qwencloud: ")}
		}
		s.mu.Lock()
		if s.behind {
			err = batch.ErrBehind
		}
		s.err = err
		s.mu.Unlock()
		cancel()
		close(s.done)
	}()
	return s
}

func (s *Session) Audio(ev engine.Event) error {
	pcm, err := base64.StdEncoding.DecodeString(ev.PCM)
	if err != nil {
		return err
	}
	c := chunk{ev.T, pcm}
	s.mu.Lock()
	s.heard = ev.T + float64(len(pcm))/batch.BytesPerSecond
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
	s.recent = append(s.recent, c)
	for len(s.recent) > 0 && s.recent[0].t < heard-keep {
		s.recent = s.recent[1:]
	}
	if s.calling {
		s.put(c)
		if s.gated && !s.speak && heard-max(s.quiet, s.sound) >= idle {
			select {
			case s.audio <- chunk{t: heard}:
				s.calling = false
			default:
			}
		}
	}
	s.mu.Unlock()
	if clock {
		s.emit(engine.Event{Type: engine.EventClock, T: batch.Round(heard)})
	}
	return nil
}

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

func (s *Session) callAgain(t float64) {
	s.calling = true
	for _, c := range s.recent {
		if c.t+float64(len(c.pcm))/batch.BytesPerSecond > t-preroll {
			s.put(c)
		}
	}
}

func (s *Session) put(c chunk) {
	select {
	case s.audio <- c:
	default:
		s.behind = true
		s.cancel()
	}
}

func peak(pcm []byte) int {
	m := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		m = max(m, v, -v)
	}
	return m
}

func (s *Session) Sent() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

func (s *Session) send(c *websocket.Conn, pcm []byte) error {
	start := time.Now()
	if err := c.Write(s.ctx, websocket.MessageBinary, pcm); err != nil {
		return fmt.Errorf("qwencloud: %w", err)
	}
	if ev := batch.Stalled(time.Since(start)); ev.Type != "" {
		s.emit(ev)
	}
	s.mu.Lock()
	s.sent += float64(len(pcm)) / batch.BytesPerSecond
	s.mu.Unlock()
	return nil
}

func (s *Session) Stop() { s.cancel() }

func (s *Session) Done() <-chan struct{} { return s.done }

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

func (s *Session) run() error {
	var first *chunk
	for {
		c, err := connect(s.ctx, s.cfg, s.lang)
		if err != nil {
			return err
		}
		if err := s.call(c, first); err != nil || s.ctx.Err() != nil {
			return err
		}
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
			c.Close(websocket.StatusNormalClosure, "")
			return nil
		case err := <-read:
			return err
		case a := <-s.audio:
			switch {
			case a.pcm == nil:
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

func (s *Session) read(c *websocket.Conn) error {
	prev := ""
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
		at := func(ms float64) float64 { return batch.Round(t0 + ms/1000) }
		if !sen.Final {
			text := steady(prev, sen.Text, s.lang)
			prev = sen.Text
			if text != "" {
				s.emit(engine.Event{Type: engine.EventResult, Start: at(sen.Begin), End: batch.Round(heard), Tokens: []engine.Token{{Text: text}}})
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
		s.emit(engine.Event{Type: engine.EventResult, Final: true, Start: at(sen.Begin), End: batch.Round(end), Tokens: toks})
	}
}

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

func spaceAhead(toks []engine.Token) {
	for i := range toks {
		t := strings.TrimRight(toks[i].Text, " ")
		if t != toks[i].Text && i+1 < len(toks) && !strings.HasPrefix(toks[i+1].Text, " ") {
			toks[i+1].Text = " " + toks[i+1].Text
		}
		toks[i].Text = t
	}
}
