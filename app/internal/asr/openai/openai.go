package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/language"

	"tomoru/internal/asr/batch"
)

const DefaultURL = "https://api.openai.com"

var Models = []string{"gpt-transcribe"}

var Caps = batch.Caps{Gap: .5, Longest: 5, Untimed: true, Remote: true}

type Config struct {
	URL      string
	Model    string
	Key      string
	Keywords []string
}

const (
	MaxKeywords = 100
	maxAbout    = 500
)

func (c Config) keywords() []string {
	var out []string
	for _, k := range c.Keywords {
		k = strings.TrimSpace(strings.NewReplacer("<", "", ">", "", "\r", " ", "\n", " ").Replace(k))
		if k != "" && len(out) < MaxKeywords {
			out = append(out, k)
		}
	}
	return out
}

var ErrAuth = errors.New("openai: the API key was refused")

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Status == 0 {
		return "openai: " + e.Message
	}
	return fmt.Sprintf("openai: HTTP %d: %s", e.Status, e.Message)
}

func (c Config) endpoint() string {
	u := strings.TrimRight(strings.TrimSpace(c.URL), "/")
	if u == "" {
		u = DefaultURL
	}
	return strings.TrimSuffix(u, "/v1") + "/v1/audio/transcriptions"
}

func (c Config) model() string {
	if c.Model == "" {
		return Models[0]
	}
	return c.Model
}

type Decoder struct {
	cfg    Config
	lang   string
	client *http.Client
	prev   string
	done   chan struct{}
	stop   sync.Once
}

func New(cfg Config, lang string) *Decoder {
	return &Decoder{cfg: cfg, lang: iso639(lang), client: &http.Client{Timeout: 30 * time.Second}, done: make(chan struct{})}
}

func iso639(lang string) string {
	base, _ := language.Make(lang).Base()
	if s := base.String(); len(s) == 2 {
		return s
	}
	return ""
}

const promptRunes = 120

func (d *Decoder) Decode(ctx context.Context, pcm []byte) ([]batch.Piece, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-d.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	text, err := d.transcribe(ctx, pcm)
	if err != nil && ctx.Err() == nil && retryable(err) {
		select {
		case <-time.After(300 * time.Millisecond):
			text, err = d.transcribe(ctx, pcm)
		case <-ctx.Done():
		}
	}
	if err != nil {
		return nil, err
	}
	if text != "" {
		d.prev = tail(d.prev+" "+text, promptRunes)
	}
	return batch.Pieces(text), nil
}

func retryable(err error) bool {
	var e *Error
	return !errors.Is(err, ErrAuth) && (!errors.As(err, &e) || e.Status >= 500 || e.Status == http.StatusTooManyRequests)
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	s = string(r[len(r)-n:])
	if i := strings.IndexAny(s, " 。、，,.！？!?"); i >= 0 && i < len(s)/2 {
		_, w := utf8.DecodeRuneInString(s[i:])
		s = s[i+w:]
	}
	return strings.TrimSpace(s)
}

func (d *Decoder) transcribe(ctx context.Context, pcm []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields := [][2]string{{"model", d.cfg.model()}, {"response_format", "json"}}
	if d.lang != "" {
		fields = append(fields, [2]string{"language", d.lang})
	}
	prompt, keywords := d.prev, d.cfg.keywords()
	if d.cfg.model() == "gpt-transcribe" {
		for _, k := range keywords {
			fields = append(fields, [2]string{"keywords[]", k})
		}
	} else if len(keywords) > 0 {
		prompt = strings.TrimSpace(strings.Join(keywords, ", ") + ". " + prompt)
	}
	if prompt != "" {
		fields = append(fields, [2]string{"prompt", prompt})
	}
	for _, f := range fields {
		w.WriteField(f[0], f[1])
	}
	f, err := w.CreateFormFile("file", "speech.wav")
	if err != nil {
		return "", err
	}
	f.Write(batch.WAV(pcm))
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.endpoint(), &body)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+d.cfg.Key)
	r, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if r.StatusCode != http.StatusOK {
		if r.StatusCode == http.StatusUnauthorized {
			return "", ErrAuth
		}
		return "", &Error{r.StatusCode, message(b)}
	}
	var out struct{ Text *string }
	if err := json.Unmarshal(b, &out); err != nil || out.Text == nil {
		return "", &Error{r.StatusCode, "an answer without text: " + clip(string(b))}
	}
	return strings.TrimSpace(*out.Text), nil
}

func message(b []byte) string {
	var e struct {
		Error struct{ Message string }
	}
	if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return clip(strings.TrimSpace(string(b)))
}

func clip(s string) string {
	if r := []rune(s); len(r) > 200 {
		return string(r[:200]) + "…"
	}
	return s
}

func Test(ctx context.Context, cfg Config) (time.Duration, error) {
	d := New(cfg, "")
	defer d.Stop()
	t := time.Now()
	_, err := d.transcribe(ctx, make([]byte, batch.BytesPerSecond))
	return time.Since(t), err
}

func (d *Decoder) Stop() { d.stop.Do(func() { close(d.done) }) }

func (d *Decoder) Done() <-chan struct{} { return d.done }

func (d *Decoder) Err() error { return nil }

var _ batch.Decoder = (*Decoder)(nil)
