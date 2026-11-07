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
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

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

// Format is how a kind of model is asked to translate: its prompts and its sampling. The way they
// are sent, an OpenAI-compatible chat API, is the same for all.
type Format struct {
	ID   string // as settings name it: "chat", "index", "hy-mt"
	Name string // the model's, as shown; "" for Chat, which the app names in its language
	// prompt is the turns that ask for r's translation.
	prompt func(Request) []message
	// image asks for the text of a picture in tgt, with the names in terms as given; nil for a
	// model that reads no pictures.
	image func(tgt string, terms []Term) string
	// temperature is the sampling's for a line.
	temperature float64
	// chat tells it is a general chat model, which is told not to think in each of the ways
	// services take it.
	chat bool
}

var (
	// Index is Index-Translate's: one turn, in Chinese, with the requirements numbered (instTrans),
	// decoded greedily, as its authors advise.
	Index = &Format{
		ID: "index", Name: "Index-Translate",
		prompt: func(r Request) []message { return []message{{"user", indexPrompt(r)}} },
		image:  indexImagePrompt,
	}
	// Chat is a general chat model's: a system prompt and the line.
	Chat = &Format{
		ID:          "chat",
		prompt:      func(r Request) []message { return []message{{"system", chatSystem(r)}, {"user", r.Text}} },
		image:       chatImagePrompt,
		temperature: 0.2,
		chat:        true,
	}
	// HyMT is Hy-MT2's: one turn, in Chinese, its personalization template, decoded greedily. It
	// reads no pictures, and has no system prompt and no thinking.
	HyMT = &Format{
		ID: "hy-mt", Name: "Hy-MT2",
		prompt: func(r Request) []message { return []message{{"user", hymtPrompt(r)}} },
	}
)

// Formats are those a user's API may be prompted in, in the order to choose them.
var Formats = []*Format{Chat, Index, HyMT}

// ReadsPictures tells whether a model of f reads the text in pictures.
func (f *Format) ReadsPictures() bool { return f.image != nil }

// FormatByID finds a format by ID.
func FormatByID(id string) (*Format, bool) {
	i := slices.IndexFunc(Formats, func(f *Format) bool { return f.ID == id })
	if i < 0 {
		return nil, false
	}
	return Formats[i], true
}

// Endpoint is a model behind an OpenAI-compatible /v1/chat/completions.
type Endpoint struct {
	URL    string // the base, without /v1
	Model  string
	APIKey string
	Format *Format // nil for Chat
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
	// ImageWait is for the first words of a picture's translation (image.go): past it, the picture
	// is asked again, once, on a connection of its own. 0 waits a minute in all.
	ImageWait time.Duration

	// plain is set once the server refused the parameters it may not know (those that turn a chat
	// model's thinking off, and asking for the tokens used): they go no more.
	plain   atomic.Bool
	used    atomic.Int64 // when a request was last sent, in Unix nanoseconds
	limited atomic.Int64 // when the service last said too many requests, in Unix nanoseconds
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
	return &Endpoint{URL: CloudURL, Model: CloudModel, Format: Index, Llama: true,
		FirstWait: 2 * time.Second, Gap: 2 * time.Second, Timeout: 8 * time.Second, Parallel: 3, WarmIdle: 30 * time.Second,
		ImageWait: 10 * time.Second}
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

// hymtPrompt is the single turn Hy-MT2 takes: the text, then the task's items numbered, the
// language to translate into last. Without the glossary it got names wrong (2026-11-07, the 1.8B).
// It is not asked for colloquial subtitles, as Index-Translate is: asked, the 7B put 呗 and 啦 in
// lectures, and chrF on the full set fell from 40.5 to 38.1 (1.8B) and from 40.4 to 36.1 (7B).
func hymtPrompt(r Request) string {
	var task []string
	if len(r.Terms) > 0 {
		pairs := make([]string, len(r.Terms))
		for i, t := range r.Terms {
			pairs[i] = t.Src + "→" + t.Tgt
		}
		task = append(task, "专名按此翻译："+strings.Join(pairs, "、"))
	}
	if r.About != "" {
		task = append(task, "背景："+r.About)
	}
	if r.Draft {
		task = append(task, "【待翻译文本】是一句还没说完的话的开头：只翻译已有的部分，不要补全或猜测后文")
	}
	task = append(task, "只输出译文，不要额外解释", "将【待翻译文本】翻译为"+zhName(r.Tgt)+"。")
	var b strings.Builder
	fmt.Fprintf(&b, "【待翻译文本】\n%s\n\n【翻译任务】", r.Text)
	for i, t := range task {
		fmt.Fprintf(&b, "\n%d、%s", i+1, t)
	}
	return b.String()
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

// body is the request for r's translation. Its tokens are enough for a long line's, and bounded by
// the line, so that a model repeating itself stops.
func (e *Endpoint) body(r Request) map[string]any {
	f := e.format()
	return e.request(f.prompt(r), f.temperature, 128+utf8.RuneCountInString(r.Text))
}

func (e *Endpoint) format() *Format { return cmp.Or(e.Format, Chat) }

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
func (e *Endpoint) quiet() bool { return e.format().chat && !e.Llama && !e.plain.Load() }

// optional are the parameters of a request a server may refuse, which go no more once one did.
var optional = []string{"stream_options", "thinking", "reasoning_effort", "enable_thinking"}

// Chat asks a chat model, prompted with system, to answer user, in maxTokens at most, without
// thinking where it can: for the model that learns names (internal/learn), behind the same API.
func (e *Endpoint) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	out, _, err := e.complete(ctx, e.request([]message{{"system", system}, {"user", user}}, 0, maxTokens), 2*time.Minute, nil)
	return out, err
}

// Translate implements Translator.
func (e *Endpoint) Translate(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	if r.Draft && time.Since(time.Unix(0, e.limited.Load())) < LimitedPause {
		return "", ErrDraftPaused
	}
	run := e.hedged
	if e.FirstWait <= 0 {
		run = e.attempt
	}
	out, err := run(ctx, r, onDelta)
	if status := (*StatusError)(nil); errors.As(err, &status) && status.Status == http.StatusTooManyRequests {
		e.limited.Store(time.Now().UnixNano())
	}
	return out, err
}

// LimitedPause is how long drafts pause after a service said too many requests (429): a draft
// can go without, and the requests left are the lines'. Hy-MT2 Pro on Tencent Cloud takes 60 a
// minute, which the drafts of a fast talk alone go past.
const LimitedPause = time.Minute

// ErrDraftPaused is a draft not asked for, in the pause after the service limited requests.
var ErrDraftPaused = errors.New("translate: drafts pause while the service limits requests")

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
		return unfinished(e.complete(ctx, e.body(r), timeout, onDelta))
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	gap := time.AfterFunc(e.Gap, func() { cancel(ErrStalled) })
	gap.Stop() // until the first words
	out, cut, err := e.complete(ctx, e.body(r), timeout, func(s string) {
		gap.Reset(e.Gap)
		if onDelta != nil {
			onDelta(s)
		}
	})
	gap.Stop()
	if err != nil && errors.Is(context.Cause(ctx), ErrStalled) {
		err = ErrStalled
	}
	return unfinished(out, cut, err)
}

// unfinished ends a translation the token limit cut with an ellipsis, so that it does not read as
// finished: its start is still worth showing.
func unfinished(out string, cut bool, err error) (string, error) {
	if cut && err == nil {
		out += "…"
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

// StatusError is a reply other than 200 OK: its HTTP status, and the start of its body.
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("translate: %d %s: %s", e.Status, http.StatusText(e.Status), e.Body)
}

// complete sends a completion and reads it as it streams in, within timeout unless e.Client says;
// cut tells the token limit ended it.
func (e *Endpoint) complete(ctx context.Context, body map[string]any, timeout time.Duration, onDelta func(string)) (out string, cut bool, err error) {
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
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", false, &StatusError{resp.StatusCode, string(bytes.TrimSpace(b))}
	}
	out, used, cut, err := readStream(resp.Body, onDelta)
	if f, ok := ctx.Value(usageKey{}).(func(Usage)); ok && used != (Usage{}) {
		f(used)
	}
	return out, cut, err
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

// clientKey is the client a request goes with, in place of e.Client: one of its own connections.
type clientKey struct{}

// do sends req to e, within timeout unless e.Client, or the client in req's context, says.
func (e *Endpoint) do(req *http.Request, timeout time.Duration) (*http.Response, error) {
	req.Header.Set("User-Agent", "Index-Translate-Client/1.0") // the public endpoint's firewall wants it
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}
	client := e.Client
	if c, ok := req.Context().Value(clientKey{}).(*http.Client); ok {
		client = c
	}
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

// readStream reads the server-sent events of a streamed completion, the tokens it took, when the
// server says, and whether the token limit cut it. One that ends with no translation says why,
// when it can: the model only thought, or the server sent an error.
func readStream(r io.Reader, onDelta func(string)) (string, Usage, bool, error) {
	var out strings.Builder
	var thought, cut bool
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
				FinishReason string `json:"finish_reason"`
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
		cut = cut || chunk.Choices[0].FinishReason == "length"
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
		return s, used, false, sc.Err()
	case failed != nil:
		return s, used, false, failed
	case s != "":
		return s, used, cut, nil
	case thought:
		return "", used, false, ErrOnlyThinking
	case other != "":
		return "", used, false, fmt.Errorf("%w: %.200s", ErrEmpty, other)
	}
	return "", used, false, ErrEmpty
}
