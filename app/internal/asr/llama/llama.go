package llama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"tomoru/internal/asr/batch"
	"tomoru/internal/lang"
	"tomoru/internal/models"
)

var Caps = batch.Caps{Volatile: .5, Gap: 1, Longest: 15, Untimed: true}

const rate = 16000

const ContextLimit = 300

const (
	tokensPerSecond = 12
	tokensMore      = 16
)

const sure = -15

const topLogprobs = 10

type Decoder struct {
	url     string
	current spoken
	auto    bool
	context string
	client  *http.Client
	done    chan struct{}
	stop    sync.Once
}

type spoken struct {
	name, code, grammar string
}

func New(url, code, context string) *Decoder {
	d := &Decoder{url: url, context: context, client: &http.Client{}, done: make(chan struct{}), auto: code == lang.Auto}
	if !d.auto {
		d.follow(code)
	}
	return d
}

func (d *Decoder) follow(code string) {
	if g := grammar(code); g != "" && code != lang.Auto {
		d.current = spoken{name(code), code, g}
	}
}

var autoGrammar = sync.OnceValue(func() string { return grammar(lang.Auto) })

func name(code string) string { return display.English.Tags().Name(language.Make(code)) }

var codes = sync.OnceValue(func() map[string]string {
	m := map[string]string{}
	for _, c := range models.Qwen3ASRLangs {
		m[name(c)] = c
	}
	return m
})

func grammar(code string) string {
	var names []string
	switch {
	case code == lang.Auto:
		for _, c := range models.Qwen3ASRLangs {
			names = append(names, `"`+name(c)+`"`)
		}
	case slices.Contains(models.Qwen3ASRLangs, code):
		names = []string{`"` + name(code) + `"`}
	default:
		return ""
	}
	return `root ::= "language " (` + strings.Join(append(names, `"None"`), " | ") + `) [^\x00]*`
}

func (d *Decoder) Decode(ctx context.Context, pcm []byte) ([]batch.Piece, error) {
	text, heard, err := d.transcribe(ctx, pcm)
	if err != nil {
		return nil, err
	}
	if !lang.Fits(text, heard) {
		heard = ""
	}
	pieces := batch.Pieces(text)
	for i := range pieces {
		pieces[i].Lang = heard
	}
	return pieces, nil
}

func (d *Decoder) transcribe(ctx context.Context, pcm []byte) (string, string, error) {
	if d.current.code == "" {
		any := ""
		if d.auto {
			any = autoGrammar()
		}
		a, err := d.ask(ctx, pcm, any)
		heard := codes()[a.label]
		if d.auto {
			d.follow(heard)
		}
		return a.text, heard, err
	}
	a, err := d.ask(ctx, pcm, "")
	heard := codes()[a.label]
	switch {
	case err != nil || a.label == d.current.name || a.label == "None":
		return a.text, d.current.code, err
	case heard != "" && d.auto && len(pcm) >= 2*rate*brief:
		d.follow(heard)
		return a.text, heard, nil
	case a.sessionLogprobAtMost < sure:
		if d.auto {
			d.follow(heard)
		}
		return a.text, heard, nil
	}
	a, err = d.ask(ctx, pcm, d.current.grammar)
	return a.text, d.current.code, err
}

const brief = 1.5

type reply struct {
	text, label          string
	sessionLogprobAtMost float64
}

func (d *Decoder) ask(ctx context.Context, pcm []byte, grammar string) (reply, error) {
	audio := map[string]any{"type": "input_audio", "input_audio": map[string]string{
		"data": base64.StdEncoding.EncodeToString(batch.WAV(pcm)), "format": "wav"}}
	messages := []map[string]any{{"role": "system", "content": d.context}, {"role": "user", "content": []any{audio}}}
	limit := tokensMore + tokensPerSecond*len(pcm)/(2*rate)
	request := map[string]any{"messages": messages, "temperature": 0, "max_tokens": limit, "cache_prompt": false, "reasoning_format": "none"}
	if grammar != "" {
		request["grammar"] = grammar
	} else if d.current.code != "" {
		request["logprobs"], request["top_logprobs"] = true, topLogprobs
	}
	body, err := json.Marshal(request)
	if err != nil {
		return reply{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-d.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := d.client.Do(req)
	if err != nil {
		return reply{}, fmt.Errorf("llama: %w", err)
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return reply{}, fmt.Errorf("llama: %w", err)
	}
	if r.StatusCode != http.StatusOK {
		return reply{}, fmt.Errorf("llama: HTTP %d: %.200s", r.StatusCode, b)
	}
	var out struct {
		Choices []struct {
			Message      struct{ Content string }
			FinishReason string `json:"finish_reason"`
			Logprobs     struct{ Content []step }
		}
	}
	if err := json.Unmarshal(b, &out); err != nil || len(out.Choices) == 0 {
		return reply{}, fmt.Errorf("llama: an answer without text: %.200s", b)
	}
	c := out.Choices[0]
	head, _, _ := strings.Cut(c.Message.Content, "<asr_text>")
	a := reply{label: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(head), "language")), sessionLogprobAtMost: d.sessionLogprobAtMost(c.Logprobs.Content)}
	if c.FinishReason != "length" {
		a.text = answer(c.Message.Content)
	}
	return a, nil
}

type step struct {
	Token       string
	TopLogprobs []struct {
		Token   string
		Logprob float64
	} `json:"top_logprobs"`
}

func (d *Decoder) sessionLogprobAtMost(steps []step) float64 {
	said := ""
	for _, s := range steps {
		if strings.TrimSpace(said) != "language" || strings.TrimSpace(s.Token) == "" {
			said += s.Token
			continue
		}
		least := 0.0
		for i, t := range s.TopLogprobs {
			if word := strings.TrimSpace(t.Token); len(word) >= 3 && strings.HasPrefix(d.current.name, word) {
				return t.Logprob
			}
			if i == 0 || t.Logprob < least {
				least = t.Logprob
			}
		}
		return least
	}
	return 0
}

func answer(s string) string {
	head, text, found := strings.Cut(s, "<asr_text>")
	if !found || strings.TrimSpace(head) == "language None" {
		return ""
	}
	return strings.TrimSpace(text)
}

func Warm(ctx context.Context, url string) error {
	d := New(url, "", "")
	defer d.Stop()
	_, _, err := d.transcribe(ctx, make([]byte, batch.BytesPerSecond))
	return err
}

func (d *Decoder) Stop() { d.stop.Do(func() { close(d.done) }) }

func (d *Decoder) Done() <-chan struct{} { return d.done }

func (d *Decoder) Err() error { return nil }

var _ batch.Decoder = (*Decoder)(nil)
