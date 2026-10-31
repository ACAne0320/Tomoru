// Package openai decodes stretches of speech with OpenAI's transcription API, POST
// /v1/audio/transcriptions, or a service's that is compatible with it (docs/recognition.md). The
// model punctuates what it hears, but times none of it; each decode is billed, by the audio sent.
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

// DefaultURL is OpenAI's API, without /v1.
const DefaultURL = "https://api.openai.com"

// Models are OpenAI's transcription models to choose, the default first. A compatible service
// names its own.
var Models = []string{"gpt-transcribe"}

// Caps are how the API is run. A decode is billed by the audio it is sent, and takes a round trip
// (1–1.8 s for 2–5 s of audio, 2.5 s for 10 s): a stretch is decoded once, as it ends, at a pause
// of half a second, or the first pause past 5 s. Shorter, at 3 s, it lost context: errors 11.5 →
// 12.9 %; at 8 s, as accurate, translations came 2–3 s later at p90 (docs/recognition.md, step
// 5). How long each decode took tells package asr whether the network is slow.
var Caps = batch.Caps{Gap: .5, Longest: 5, Untimed: true, Remote: true}

// Config is a service to recognize with, and what to tell it to expect.
type Config struct {
	URL   string // the base, as https://api.openai.com, with or without /v1; "" for DefaultURL
	Model string // "" for the first of Models
	Key   string // the API key
	// Keywords are words to expect, names: the first MaxKeywords. About describes what is heard,
	// for live transcription.
	Keywords []string
	About    string
}

// MaxKeywords are the most keywords a session is given, and maxAbout the most runes of About.
const (
	MaxKeywords = 100
	maxAbout    = 500
)

// keywords are the config's keywords, as the API takes them: one line each, without < or >,
// which it refuses.
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

// ErrAuth is an API key the service refused.
var ErrAuth = errors.New("openai: the API key was refused")

// Error is a failure the service reported: its HTTP status, 0 for an event of a realtime session,
// and its message.
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

// endpoint is the transcription API's address.
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

// Decoder decodes with the service, giving it the language and what the stretch before said.
type Decoder struct {
	cfg    Config
	lang   string // ISO-639-1, or "" to tell
	client *http.Client
	prev   string // the end of the last decode's text, for the next to go on from
	done   chan struct{}
	stop   sync.Once
}

// New decodes speech in lang with the service of cfg.
func New(cfg Config, lang string) *Decoder {
	return &Decoder{cfg: cfg, lang: iso639(lang), client: &http.Client{Timeout: 30 * time.Second}, done: make(chan struct{})}
}

// iso639 is the ISO-639-1 code the API takes for lang: "zh" for Traditional Chinese; "" for a
// language without one (Cantonese, Filipino), which the model tells itself.
func iso639(lang string) string {
	base, _ := language.Make(lang).Base()
	if s := base.String(); len(s) == 2 {
		return s
	}
	return ""
}

// prompt is how much of the text before a decode is given it: about a sentence or two.
const prompt = 120

// Decode decodes a stretch: its text in pieces, a character or a word each, untimed. A decode the
// connection failed is tried once more.
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
	var e *Error
	if err != nil && ctx.Err() == nil && !errors.Is(err, ErrAuth) && (!errors.As(err, &e) || e.Status >= 500 || e.Status == http.StatusTooManyRequests) {
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
		d.prev = tail(d.prev+" "+text, prompt)
	}
	return batch.Pieces(text), nil
}

// tail is the last n runes of s, or so: from a space or a punctuation mark, where one is near.
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
	// gpt-transcribe takes keywords; the others are given them in the prompt, before the text
	// before, as whisper-1 takes a prompt of keywords.
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

// message is what an error's body says: OpenAI's {"error": {"message": …}}, or the body.
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

// Test transcribes a second of silence with the service, as the settings test it: how long it took.
func Test(ctx context.Context, cfg Config) (time.Duration, error) {
	d := New(cfg, "")
	defer d.Stop()
	t := time.Now()
	_, err := d.transcribe(ctx, make([]byte, 32000))
	return time.Since(t), err
}

// Stop ends the decoder: a decode in flight is canceled.
func (d *Decoder) Stop() { d.stop.Do(func() { close(d.done) }) }

// Done is closed once the decoder has stopped.
func (d *Decoder) Done() <-chan struct{} { return d.done }

// Err is nil: a decode the service failed fails, and ends the runner with it.
func (d *Decoder) Err() error { return nil }

var _ batch.Decoder = (*Decoder)(nil)
