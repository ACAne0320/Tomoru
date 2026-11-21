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

type Request struct {
	Src, Tgt string
	Text     string
	Draft    bool
	Terms    []Term
	About    string
}

type Term struct{ Src, Tgt string }

type Translator interface {
	Translate(ctx context.Context, r Request, onDelta func(string)) (string, error)
}

type Format struct {
	ID          string
	Name        string
	prompt      func(Request) []message
	image       func(tgt string, terms []Term) string
	temperature float64
	chat        bool
}

var (
	Index = &Format{
		ID: "index", Name: "Index-Translate",
		prompt: func(r Request) []message { return []message{{"user", indexPrompt(r)}} },
		image:  indexImagePrompt,
	}
	Chat = &Format{
		ID:          "chat",
		prompt:      func(r Request) []message { return []message{{"system", chatSystem(r)}, {"user", r.Text}} },
		image:       chatImagePrompt,
		temperature: 0.2,
		chat:        true,
	}
	HyMT = &Format{
		ID: "hy-mt", Name: "Hy-MT2",
		prompt: func(r Request) []message { return []message{{"user", hymtPrompt(r)}} },
	}
)

var Formats = []*Format{Chat, Index, HyMT}

func (f *Format) ReadsPictures() bool { return f.image != nil }

func FormatByID(id string) (*Format, bool) {
	i := slices.IndexFunc(Formats, func(f *Format) bool { return f.ID == id })
	if i < 0 {
		return nil, false
	}
	return Formats[i], true
}

type Endpoint struct {
	URL                     string
	Model                   string
	APIKey                  string
	Format                  *Format
	Llama                   bool
	Client                  *http.Client
	FirstWait, Gap, Timeout time.Duration
	Parallel                int
	WarmIdle                time.Duration
	ImageSide               int
	ImageWait               time.Duration

	plain   atomic.Bool
	used    atomic.Int64
	limited atomic.Int64
}

type Usage struct{ In, Out int }

type usageKey struct{}

func WithUsage(ctx context.Context, f func(Usage)) context.Context {
	return context.WithValue(ctx, usageKey{}, f)
}

const (
	CloudURL   = "https://index-translate.bilibili.com"
	CloudModel = "Index-Translate-35B-A3B"
)

func Cloud() *Endpoint {
	return &Endpoint{URL: CloudURL, Model: CloudModel, Format: Index,
		FirstWait: 2 * time.Second, Gap: 2 * time.Second, Timeout: 8 * time.Second, Parallel: 3, WarmIdle: 30 * time.Second,
		ImageWait: 10 * time.Second}
}

func zhName(code string) string { return lang.Name(code, "zh-Hans") }

func enName(code string) string {
	if code == "zh" {
		code = "zh-Hans"
	}
	return lang.Name(code, "en")
}

func indexPrompt(r Request) string {
	var reqs []string
	if len(r.Terms) > 0 {
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
	src := "line"
	if r.Src != "" {
		src = enName(r.Src) + " line"
	}
	s := fmt.Sprintf("You are a live subtitle translator. Translate the user's %s into natural, colloquial %s suitable for on-screen subtitles. Output only the translation.",
		src, enName(r.Tgt))
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

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (e *Endpoint) body(r Request) map[string]any {
	f := e.format()
	return e.request(f.prompt(r), f.temperature, 128+utf8.RuneCountInString(r.Text))
}

func (e *Endpoint) format() *Format { return cmp.Or(e.Format, Chat) }

func (e *Endpoint) request(msgs []message, temp float64, maxTokens int) map[string]any {
	b := map[string]any{"model": e.Model, "messages": msgs, "stream": true, "temperature": temp, "max_tokens": maxTokens}
	if !e.plain.Load() {
		b["stream_options"] = map[string]any{"include_usage": true}
	}
	if e.Llama {
		b["cache_prompt"] = true
		b["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	} else if e.quiet() {
		b["thinking"] = map[string]any{"type": "disabled"}
		b["reasoning_effort"] = "none"
		b["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
		b["enable_thinking"] = false
	}
	return b
}

func (e *Endpoint) quiet() bool { return e.format().chat && !e.Llama && !e.plain.Load() }

var optional = []string{"stream_options", "thinking", "reasoning_effort", "enable_thinking"}

func (e *Endpoint) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	out, _, err := e.complete(ctx, e.request([]message{{"system", system}, {"user", user}}, 0, maxTokens), 2*time.Minute, nil)
	return out, err
}

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

const LimitedPause = time.Minute

var ErrDraftPaused = errors.New("translate: drafts pause while the service limits requests")

var ErrStalled = errors.New("translate: the reply stalled")

func (e *Endpoint) hedged(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	return hedge(ctx, e.FirstWait, 0, r.Draft, onDelta, func(ctx context.Context, _ int32, onDelta func(string)) (string, error) {
		return e.attempt(ctx, r, onDelta)
	})
}

func hedge(ctx context.Context, wait, giveUp time.Duration, drop bool, onDelta func(string),
	ask func(ctx context.Context, n int32, onDelta func(string)) (string, error)) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	type result struct {
		n   int32
		out string
		err error
	}
	var read atomic.Int32
	results := make(chan result, 2)
	pending := 0
	start := func(n int32) {
		pending++
		go func() {
			out, err := ask(ctx, n, func(s string) {
				if (read.CompareAndSwap(0, n) || read.Load() == n) && onDelta != nil {
					onDelta(s)
				}
			})
			results <- result{n, out, err}
		}()
	}
	defer func() {
		cancel()
		for ; pending > 0; pending-- {
			<-results
		}
	}()
	start(1)
	again := time.NewTimer(wait)
	defer again.Stop()
	var stalled <-chan time.Time
	if giveUp > 0 {
		t := time.NewTimer(giveUp)
		defer t.Stop()
		stalled = t.C
	}
	for {
		select {
		case <-again.C:
			switch {
			case ctx.Err() != nil:
			case drop:
				if read.CompareAndSwap(0, -1) {
					return "", ErrStalled
				}
			case read.Load() == 0:
				log.Printf("translate: no words in %v, asking again", wait)
				start(2)
			}
		case <-stalled:
			if read.CompareAndSwap(0, -1) {
				return "", ErrStalled
			}
		case res := <-results:
			pending--
			if n := read.Load(); n == res.n || n == 0 && pending == 0 {
				return res.out, res.err
			}
		}
	}
}

func (e *Endpoint) attempt(ctx context.Context, r Request, onDelta func(string)) (string, error) {
	timeout := cmp.Or(e.Timeout, 30*time.Second)
	if e.Gap <= 0 {
		return unfinished(e.complete(ctx, e.body(r), timeout, onDelta))
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	gap := time.AfterFunc(e.Gap, func() { cancel(ErrStalled) })
	gap.Stop()
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

func unfinished(out string, cut bool, err error) (string, error) {
	if cut && err == nil {
		out += "…"
	}
	return out, err
}

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

type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("translate: %d %s: %s", e.Status, http.StatusText(e.Status), e.Body)
}

func (e *Endpoint) complete(ctx context.Context, body map[string]any, timeout time.Duration, onDelta func(string)) (out string, cut bool, err error) {
	quiet := e.quiet()
	_, asked := body["stream_options"]
	resp, err := e.post(ctx, body, timeout)
	if err == nil && (quiet || asked) && resp.StatusCode == http.StatusBadRequest {
		resp.Body.Close()
		e.plain.Store(true)
		for _, k := range optional {
			delete(body, k)
		}
		if quiet {
			delete(body, "chat_template_kwargs")
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

func (e *Endpoint) post(ctx context.Context, body map[string]any, timeout time.Duration) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(e.URL, "/")+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return e.do(req, timeout)
}

type clientKey struct{}

func (e *Endpoint) do(req *http.Request, timeout time.Duration) (*http.Response, error) {
	req.Header.Set("User-Agent", "Index-Translate-Client/1.0")
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

var ErrEmpty = errors.New("translate: the reply was empty")

var ErrOnlyThinking = errors.New("translate: the model only sent its thinking")

func readStream(r io.Reader, onDelta func(string)) (string, Usage, bool, error) {
	var out strings.Builder
	var thought, cut bool
	var failed error
	var other string
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
