// Package translate translates subtitle lines with a model behind an OpenAI-compatible chat API:
// Index-Translate's cloud, a local llama-server, or an endpoint the user adds.
package translate

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"tomoru/internal/lang"
)

// Request is a line to translate.
type Request struct {
	Src, Tgt string // languages, of lang.All
	Text     string
	// Draft tells that Text is the start of a sentence still being spoken: the model translates
	// what is there, without completing it.
	Draft bool
	// Terms are names in Text and how to translate them, from the scene's glossary.
	Terms []Term
	// About is what is being watched, as the scene's description says: a hint for the names and
	// the words that depend on it.
	About string
}

// Term is a name, as written in the source, and its translation.
type Term struct{ Src, Tgt string }

// Translator translates lines.
type Translator interface {
	// Translate returns the translation of r, calling onDelta with the translation so far as it
	// streams in.
	Translate(ctx context.Context, r Request, onDelta func(string)) (string, error)
}

// Style is how a model is prompted.
type Style int

const (
	// Index is Index-Translate's prompt: one turn, in Chinese, with the requirements numbered.
	Index Style = iota
	// Chat is a system prompt and the line, for general chat models.
	Chat
)

// Endpoint is a model behind an OpenAI-compatible /v1/chat/completions.
type Endpoint struct {
	URL    string // the base, without /v1
	Model  string
	APIKey string
	Style  Style
	// Llama sends llama.cpp's own parameters (prompt caching, no thinking), which Index-Translate's
	// cloud and llama-server take and other services may refuse.
	Llama  bool
	Client *http.Client
	// The waits of a live translation, which is no use late, and holds up the lines after it.
	// FirstWait is for the first words: past it, a line is asked again, once, alongside, and a
	// draft dropped. Gap is between words, past which the translation failed; Timeout is for all
	// of it. 0 waits, but for 30 s in all.
	FirstWait, Gap, Timeout time.Duration
	// Parallel is how many lines may be translated at once; 0 is one.
	Parallel int
	// WarmIdle is how long a connection may go unused before Warm opens one again; 0 never warms.
	WarmIdle time.Duration
	// ImageSide is the longest side, in pixels, of a picture sent (image.go); 0 is MaxSide.
	ImageSide int

	// plain is set once the server refused the parameters it may not know (those that turn a chat
	// model's thinking off, and asking for the tokens used): they go no more.
	plain atomic.Bool
	used  atomic.Int64 // when a request was last sent, in Unix nanoseconds
}

// Usage is the tokens a completion took, as the server counted them.
type Usage struct{ In, Out int }

type usageKey struct{}

// WithUsage is ctx, with which completions tell f the tokens they took, when the server says.
func WithUsage(ctx context.Context, f func(Usage)) context.Context {
	return context.WithValue(ctx, usageKey{}, f)
}

// CloudURL is Index-Translate's public cloud, and its page; CloudModel the model it serves.
const (
	CloudURL   = "https://index-translate.bilibili.com"
	CloudModel = "Index-Translate-35B-A3B"
)

// Cloud is Index-Translate's public cloud API, free to use. Its first words come within a second
// or so (2026-10-26: 0.5 s typically, 1.2 s at most with 4 requests at a time), or, now and then
// (half the requests, some hours), after 6 s, if at all, and once after 30 s; a whole line within
// 1.5 s. It closes a connection unused for 40 to 60 s, and a new one takes 0.4 to 0.5 s more.
func Cloud() *Endpoint {
	return &Endpoint{URL: CloudURL, Model: CloudModel, Style: Index, Llama: true,
		FirstWait: 2 * time.Second, Gap: 2 * time.Second, Timeout: 8 * time.Second, Parallel: 3, WarmIdle: 30 * time.Second}
}

// zhName is a language's name in Chinese, for Index-Translate's prompts: "日语".
func zhName(code string) string { return lang.Name(code, "zh-Hans") }

// enName is a language's name in English, for a chat model's prompts, Chinese with its script:
// "Simplified Chinese".
func enName(code string) string {
	if code == "zh" {
		code = "zh-Hans"
	}
	return lang.Name(code, "en")
}

// indexPrompt is the single turn Index-Translate takes: the source text, then numbered
// requirements.
func indexPrompt(r Request) string {
	var reqs []string
	if len(r.Terms) > 0 {
		// Index-Translate's own form of a glossary (instTrans), a hard requirement.
		pairs := make([]string, len(r.Terms))
		for i, t := range r.Terms {
			pairs[i] = t.Src + "→" + t.Tgt
		}
		reqs = append(reqs, "【硬性要求】专名/术语对照: "+strings.Join(pairs, "、"))
	}
	if r.About != "" {
		reqs = append(reqs, "【注意】背景："+r.About)
	}
	if r.Draft {
		reqs = append(reqs, "【注意】源文是一句还没说完的话的开头：只翻译已有的部分，不要补全或猜测后文")
	}
	reqs = append(reqs, "【注意】这是直播实时字幕，请译成口语化、简洁的字幕")
	var b strings.Builder
	for i, c := range reqs {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	return fmt.Sprintf("请将以下%s文本翻译成%s，并且严格遵循所有约束要求。\n\n【源文】\n%s\n\n【约束要求】\n%s\n只输出译文，不要有任何额外说明。",
		zhName(r.Src), zhName(r.Tgt), r.Text, b.String())
}

func chatSystem(r Request) string {
	s := fmt.Sprintf("You are a live subtitle translator. Translate the user's %s line into natural, colloquial %s suitable for on-screen subtitles. Output only the translation.",
		enName(r.Src), enName(r.Tgt))
	if r.About != "" {
		s += " What is being watched: " + r.About + "."
	}
	if r.Draft {
		s += " The line is the start of an unfinished sentence: translate only what is there, do not complete it."
	}
	if len(r.Terms) > 0 {
		pairs := make([]string, len(r.Terms))
		for i, t := range r.Terms {
			pairs[i] = t.Src + " → " + t.Tgt
		}
		s += " Render these names as given: " + strings.Join(pairs, "; ") + "."
	}
	return s
}

// message is a turn of a chat. Its content is text, or parts: a picture and text (image.go).
type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (e *Endpoint) body(r Request) map[string]any {
	if e.Style == Index {
		return e.request([]message{{"user", indexPrompt(r)}}, 0, 128)
	}
	return e.request([]message{{"system", chatSystem(r)}, {"user", r.Text}}, 0.2, 128)
}

// request is the body of a streamed completion of msgs.
func (e *Endpoint) request(msgs []message, temp float64, maxTokens int) map[string]any {
	b := map[string]any{"model": e.Model, "messages": msgs, "stream": true, "temperature": temp, "max_tokens": maxTokens}
	if !e.plain.Load() {
		b["stream_options"] = map[string]any{"include_usage": true} // the tokens, in the last event
	}
	if e.Llama {
		b["cache_prompt"] = true
		b["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	} else if e.quiet() {
		// A line's translation would wait for the thinking, and is no better for it: off, in each
		// of the ways services take it.
		b["thinking"] = map[string]any{"type": "disabled"}                   // DeepSeek, GLM, Doubao
		b["reasoning_effort"] = "none"                                       // DeepSeek, OpenAI, Gemini
		b["chat_template_kwargs"] = map[string]any{"enable_thinking": false} // vLLM, SGLang
		b["enable_thinking"] = false                                         // Qwen's cloud
	}
	return b
}

// quiet tells that requests ask a chat model not to think.
func (e *Endpoint) quiet() bool { return e.Style == Chat && !e.Llama && !e.plain.Load() }

// optional are the parameters of a request a server may refuse, which go no more once one did.
var optional = []string{"stream_options", "thinking", "reasoning_effort", "enable_thinking"}

// Chat asks a chat model, prompted with system, to answer user, in maxTokens at most, without
// thinking where it can: for the model that learns names (internal/learn), behind the same API.
func (e *Endpoint) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	return e.complete(ctx, e.request([]message{{"system", system}, {"user", user}}, 0, maxTokens), 2*time.Minute, nil)
}

// Translate implements Translator.
func (e *Endpoint) Translate(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	if e.FirstWait <= 0 {
		return e.attempt(ctx, r, onDelta)
	}
	return e.hedged(ctx, r, onDelta)
}

// ErrStalled is a translation whose words did not come in time: Endpoint.FirstWait for a draft's
// first, Endpoint.Gap between them.
var ErrStalled = errors.New("translate: the reply stalled")

// hedged translates r, and asks again when the first words have not come within e.FirstWait,
// without dropping the first request, which may yet answer first: the request that sends words
// first is read, and the other dropped. A draft is dropped instead, as one of more words follows.
func (e *Endpoint) hedged(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // the request not read
	type result struct {
		n   int32
		out string
		err error
	}
	var read atomic.Int32 // the request read, from 1; 0 until one sends words, -1 when dropped
	results := make(chan result, 2)
	ask := func(n int32) {
		go func() {
			out, err := e.attempt(ctx, r, func(s string) {
				if (read.CompareAndSwap(0, n) || read.Load() == n) && onDelta != nil {
					onDelta(s)
				}
			})
			results <- result{n, out, err}
		}()
	}
	ask(1)
	asked, pending := int32(1), 1
	wait := time.NewTimer(e.FirstWait)
	defer wait.Stop()
	for {
		select {
		case <-wait.C:
			switch {
			case ctx.Err() != nil:
			case r.Draft:
				if read.CompareAndSwap(0, -1) {
					return "", ErrStalled
				}
			case read.Load() == 0:
				log.Printf("translate: no words in %v, asking again", e.FirstWait)
				ask(2)
				asked, pending = 2, pending+1
			}
		case res := <-results:
			pending--
			switch n := read.Load(); {
			case n == res.n:
				return res.out, res.err
			case n == 0 && (pending == 0 || asked == 1):
				// Failed without words, with no other request to wait for. One that failed before
				// FirstWait failed fast, as asking again would.
				return res.out, res.err
			}
		}
	}
}

// attempt is a request for r's translation, given up when its words pause for e.Gap.
func (e *Endpoint) attempt(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	timeout := cmp.Or(e.Timeout, 30*time.Second)
	if e.Gap <= 0 {
		return e.complete(ctx, e.body(r), timeout, onDelta)
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	gap := time.AfterFunc(e.Gap, func() { cancel(ErrStalled) })
	gap.Stop() // until the first words
	out, err := e.complete(ctx, e.body(r), timeout, func(s string) {
		gap.Reset(e.Gap)
		if onDelta != nil {
			onDelta(s)
		}
	})
	gap.Stop()
	if err != nil && errors.Is(context.Cause(ctx), ErrStalled) {
		err = ErrStalled
	}
	return out, err
}

// Warm opens a connection to the server, unless one was used within e.WarmIdle, so that the
// translation after it need not. It returns when the server answered.
func (e *Endpoint) Warm(ctx context.Context) {
	if e.WarmIdle <= 0 || time.Since(time.Unix(0, e.used.Load())) < e.WarmIdle {
		return
	}
	e.used.Store(time.Now().UnixNano())
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(e.URL, "/")+"/v1/models", nil)
	if err != nil {
		return
	}
	resp, err := e.do(req, 5*time.Second)
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// complete sends a completion and reads it as it streams in, within timeout unless e.Client says.
func (e *Endpoint) complete(ctx context.Context, body map[string]any, timeout time.Duration, onDelta func(string)) (string, error) {
	quiet := e.quiet()
	_, asked := body["stream_options"]
	resp, err := e.post(ctx, body, timeout)
	if err == nil && (quiet || asked) && resp.StatusCode == http.StatusBadRequest {
		// A server that takes no parameter it does not know: again, as plain as it was.
		resp.Body.Close()
		e.plain.Store(true)
		for _, k := range optional {
			delete(body, k)
		}
		if quiet {
			delete(body, "chat_template_kwargs") // llama.cpp's, which a server of Llama needs
		}
		resp, err = e.post(ctx, body, timeout)
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("translate: %s: %s", resp.Status, bytes.TrimSpace(b))
	}
	out, used, err := readStream(resp.Body, onDelta)
	if f, ok := ctx.Value(usageKey{}).(func(Usage)); ok && used != (Usage{}) {
		f(used)
	}
	return out, err
}

// post sends a completion's body to the chat completions of e.
func (e *Endpoint) post(ctx context.Context, body map[string]any, timeout time.Duration) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(e.URL, "/")+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return e.do(req, timeout)
}

// do sends req to e, within timeout unless e.Client says.
func (e *Endpoint) do(req *http.Request, timeout time.Duration) (*http.Response, error) {
	req.Header.Set("User-Agent", "Index-Translate-Client/1.0") // the public endpoint's firewall wants it
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	e.used.Store(time.Now().UnixNano())
	return client.Do(req)
}

// ErrEmpty is a reply with no text: the Index-Translate cloud answered so while it was down
// (2026-10-20), and an empty line looked translated.
var ErrEmpty = errors.New("translate: the reply was empty")

// ErrOnlyThinking is a streamed completion with the model's thinking but no translation: it
// thought until it ran out of tokens.
var ErrOnlyThinking = errors.New("translate: the model only sent its thinking")

// readStream reads the server-sent events of a streamed completion, and the tokens it took, when
// the server says. One that ends with no translation says why, when it can: the model only
// thought, or the server sent an error.
func readStream(r io.Reader, onDelta func(string)) (string, Usage, error) {
	var out strings.Builder
	var thought bool
	var failed error
	var other string // the first line that is not an event: an error a server sent as it is
	var used Usage
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok && other == "" {
			other = strings.TrimSpace(sc.Text())
		}
		if !ok || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			// The tokens: OpenAI's, asked for with stream_options; llama.cpp's timings without.
			Usage *struct {
				Prompt     int `json:"prompt_tokens"`
				Completion int `json:"completion_tokens"`
			} `json:"usage"`
			Timings *struct {
				Prompt    int `json:"prompt_n"`
				Predicted int `json:"predicted_n"`
			} `json:"timings"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if chunk.Error != nil {
			failed = fmt.Errorf("translate: %s", chunk.Error.Message)
		}
		switch {
		case chunk.Usage != nil:
			used = Usage{chunk.Usage.Prompt, chunk.Usage.Completion}
		case chunk.Timings != nil && used == (Usage{}):
			used = Usage{chunk.Timings.Prompt, chunk.Timings.Predicted}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		thought = thought || d.ReasoningContent != ""
		if d.Content == "" {
			continue
		}
		out.WriteString(d.Content)
		if onDelta != nil {
			onDelta(out.String())
		}
	}
	s := strings.TrimSpace(out.String())
	switch {
	case sc.Err() != nil:
		return s, used, sc.Err()
	case failed != nil:
		return s, used, failed
	case s != "":
		return s, used, nil
	case thought:
		return "", used, ErrOnlyThinking
	case other != "":
		return "", used, fmt.Errorf("%w: %.200s", ErrEmpty, other)
	}
	return "", used, ErrEmpty
}
