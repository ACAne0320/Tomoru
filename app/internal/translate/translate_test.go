package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEndpoint(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&body)
		for _, piece := range []string{"大家", "好", "！"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	e := &Endpoint{URL: srv.URL + "/", Model: "m", APIKey: "k", Style: Index}
	var deltas []string
	out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi all!"}, func(s string) { deltas = append(deltas, s) })
	if err != nil || out != "大家好！" || strings.Join(deltas, "|") != "大家|大家好|大家好！" {
		t.Fatalf("%q %v, deltas %q", out, err, deltas)
	}
	prompt := body["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, "请将以下英语文本翻译成中文") || !strings.Contains(prompt, "【源文】\nHi all!\n") || body["cache_prompt"] != nil {
		t.Errorf("prompt %q, body %v", prompt, body)
	}

	// A chat model gets a system prompt and the line.
	e.Style, e.Llama = Chat, true
	e.Translate(context.Background(), Request{Src: "ja", Tgt: "en", Text: "こんにちは", Draft: true}, nil)
	msgs := body["messages"].([]any)
	sys := msgs[0].(map[string]any)["content"].(string)
	if len(msgs) != 2 || !strings.Contains(sys, "Japanese line into natural, colloquial English") || !strings.Contains(sys, "unfinished") || body["cache_prompt"] != true {
		t.Errorf("messages %v, body %v", msgs, body)
	}
}

// TestTerms puts the glossary's terms in either prompt: as Index-Translate's hard requirement, and
// in a chat model's system prompt.
func TestTerms(t *testing.T) {
	terms := []Term{{"クラウド", "克劳德"}, {"メテオ", "陨石"}}
	p := indexPrompt(Request{Src: "ja", Tgt: "zh", Text: "クラウド。", Terms: terms})
	if !strings.Contains(p, "1. 【硬性要求】专名/术语对照: クラウド→克劳德、メテオ→陨石\n2. 【注意】") {
		t.Errorf("Index prompt %q", p)
	}
	if p := indexPrompt(Request{Src: "ja", Tgt: "zh", Text: "クラウド。"}); strings.Contains(p, "硬性要求") {
		t.Errorf("Index prompt without terms %q", p)
	}
	if s := chatSystem(Request{Src: "ja", Tgt: "zh", Terms: terms}); !strings.HasSuffix(s, "Render these names as given: クラウド → 克劳德; メテオ → 陨石.") {
		t.Errorf("system prompt %q", s)
	}
}

// TestAbout tells the model what is being watched, after the glossary, as a note (soft constraint).
func TestAbout(t *testing.T) {
	r := Request{Src: "ja", Tgt: "zh", Text: "クラウド。", Terms: []Term{{"クラウド", "克劳德"}}, About: "最终幻想7 重制版"}
	if p := indexPrompt(r); !strings.Contains(p, "1. 【硬性要求】专名/术语对照: クラウド→克劳德\n2. 【注意】背景：最终幻想7 重制版\n3. 【注意】") {
		t.Errorf("Index prompt %q", p)
	}
	if s := chatSystem(r); !strings.Contains(s, " What is being watched: 最终幻想7 重制版.") {
		t.Errorf("system prompt %q", s)
	}
	r.About = ""
	if p := indexPrompt(r); strings.Contains(p, "背景") {
		t.Errorf("Index prompt without it %q", p)
	}
}

// TestNoThinking asks a chat model not to think, as a line's translation would wait for it, in each
// of the ways services take it; a server that refuses them gets the request again without them,
// and never again.
func TestNoThinking(t *testing.T) {
	var bodies []map[string]any
	refuse := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		if _, ok := body["thinking"]; ok && refuse {
			http.Error(w, `{"error":{"message":"Unrecognized request argument supplied: thinking"}}`, http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	e := &Endpoint{URL: srv.URL, Model: "m", Style: Chat}
	if out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
		t.Fatalf("%q %v", out, err)
	}
	b := bodies[0]
	if fmt.Sprint(b["thinking"]) != "map[type:disabled]" || b["reasoning_effort"] != "none" || b["enable_thinking"] != false ||
		fmt.Sprint(b["chat_template_kwargs"]) != "map[enable_thinking:false]" {
		t.Errorf("body %v", b)
	}

	// A server that refuses them.
	bodies, refuse = nil, true
	e = &Endpoint{URL: srv.URL, Model: "m", Style: Chat}
	for range 2 {
		if out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
			t.Fatalf("%q %v", out, err)
		}
	}
	if len(bodies) != 3 || bodies[2]["thinking"] != nil {
		t.Errorf("%d requests: %v", len(bodies), bodies)
	}

	// Index-Translate does not think: its requests stay as they were.
	bodies = nil
	e = &Endpoint{URL: srv.URL, Model: "m", Style: Index}
	e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil)
	if len(bodies) != 1 || bodies[0]["thinking"] != nil || bodies[0]["reasoning_effort"] != nil {
		t.Errorf("Index-Translate: %v", bodies)
	}
}

// TestUsage asks for the tokens a completion takes, and tells them: OpenAI's usage, or llama.cpp's
// timings. A server that refuses the asking gets the request again without it.
func TestUsage(t *testing.T) {
	var bodies []map[string]any
	refuse := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		if _, ok := body["stream_options"]; ok && refuse {
			http.Error(w, `{"error":{"message":"stream_options: not supported"}}`, http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		if _, ok := body["stream_options"]; ok {
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":32,\"completion_tokens\":2}}\n\n")
		} else {
			fmt.Fprint(w, "data: {\"choices\":[],\"timings\":{\"prompt_n\":30,\"predicted_n\":3}}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	var used []Usage
	ctx := WithUsage(context.Background(), func(u Usage) { used = append(used, u) })
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Index, Llama: true}
	if out, err := e.Translate(ctx, Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
		t.Fatalf("%q %v", out, err)
	}
	if fmt.Sprint(bodies[0]["stream_options"]) != "map[include_usage:true]" || len(used) != 1 || used[0] != (Usage{32, 2}) {
		t.Errorf("body %v, used %v", bodies[0], used)
	}

	bodies, used, refuse = nil, nil, true
	for range 2 {
		if out, err := e.Translate(ctx, Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
			t.Fatalf("%q %v", out, err)
		}
	}
	if len(bodies) != 3 || bodies[1]["cache_prompt"] != true || bodies[1]["chat_template_kwargs"] == nil || len(used) != 2 || used[1] != (Usage{30, 3}) {
		t.Errorf("%d requests: %v, used %v", len(bodies), bodies, used)
	}
}

// TestNoTranslation reads streams that end without a translation: a model that spent its tokens
// thinking, and an error sent in the stream, say so; nothing at all is an empty translation.
func TestNoTranslation(t *testing.T) {
	for _, c := range []struct {
		stream string
		want   error
		says   string
	}{
		{"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Let me think\"}}]}\n\ndata: [DONE]\n\n", ErrOnlyThinking, ""},
		{"data: {\"error\":{\"message\":\"Model Not Exist\"}}\n\n", nil, "Model Not Exist"},
		{"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Hm\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n", nil, ""},
		{"data: [DONE]\n\n", ErrEmpty, ""},
		{"Internal Server Error", ErrEmpty, "Internal Server Error"},
	} {
		out, _, err := readStream(strings.NewReader(c.stream), nil)
		switch {
		case c.want != nil && !errors.Is(err, c.want), c.says != "" && (err == nil || !strings.Contains(err.Error(), c.says)):
			t.Errorf("%q: %q, %v", c.stream, out, err)
		case c.want == nil && c.says == "" && err != nil:
			t.Errorf("%q: %v", c.stream, err)
		}
	}
}

// A provider can fail after streaming some text. Keep that text and its usage, but
// do not report the incomplete translation as a successful completion.
func TestStreamErrorAfterPartial(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"部分译文\"}}]}\n\n" +
		"data: {\"error\":{\"message\":\"upstream failure\"},\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"
	var partial string
	out, used, err := readStream(strings.NewReader(stream), func(text string) { partial = text })
	if out != "部分译文" || partial != out || used != (Usage{8, 3}) {
		t.Fatalf("partial output or usage lost: out=%q, partial=%q, usage=%+v", out, partial, used)
	}
	if err == nil || !strings.Contains(err.Error(), "upstream failure") {
		t.Fatalf("provider failure lost after partial output: %v", err)
	}
}

// TestCloud translates with the real cloud API, when TOMORU_CLOUD is set.
func TestCloud(t *testing.T) {
	if os.Getenv("TOMORU_CLOUD") == "" {
		t.Skip("set TOMORU_CLOUD=1 to call the cloud")
	}
	out, err := Cloud().Translate(context.Background(), Request{Src: "ja", Tgt: "zh", Text: "みなさん、こんばんは。"}, nil)
	t.Logf("%q", out)
	if err != nil || out == "" {
		t.Fatal(err)
	}
}

// TestChat asks a chat model with a system prompt, without its thinking.
func TestChat(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"terms\\\":[]}\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Chat}
	out, err := e.Chat(context.Background(), "sys", "user", 1024)
	msgs := body["messages"].([]any)
	if err != nil || out != `{"terms":[]}` || len(msgs) != 2 || msgs[0].(map[string]any)["content"] != "sys" || body["max_tokens"] != 1024.0 || body["reasoning_effort"] != "none" {
		t.Errorf("%q %v, body %v", out, err, body)
	}
}

// TestFirstWait asks again when the first words do not come in time, keeping the first request,
// which may yet answer first: the cloud now and then leaves a request hanging. A translation whose
// words started is not asked again, however slow.
func TestFirstWait(t *testing.T) {
	const hang = -1
	type reply struct {
		after time.Duration // before the first words; hang: never
		empty bool
		slow  bool // a pause between the words
	}
	var mu sync.Mutex
	var plan []reply // of each request in turn
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body) // so that the server hears the client hang up
		mu.Lock()
		p := plan[requests]
		requests++
		mu.Unlock()
		if p.after == hang {
			<-r.Context().Done()
			return
		}
		select {
		case <-time.After(p.after):
		case <-r.Context().Done():
			return
		}
		if !p.empty {
			for _, piece := range []string{"大家", "好"} {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
				w.(http.Flusher).Flush()
				if p.slow {
					time.Sleep(300 * time.Millisecond)
				}
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Index, FirstWait: 100 * time.Millisecond}

	for _, c := range []struct {
		name     string
		plan     []reply
		requests int
	}{
		{"hanging", []reply{{after: hang}, {}}, 2},
		{"late, but first", []reply{{after: 200 * time.Millisecond}, {after: hang}}, 2},
		{"late and empty", []reply{{after: 150 * time.Millisecond, empty: true}, {after: 200 * time.Millisecond}}, 2},
		{"slow", []reply{{slow: true}}, 1},
	} {
		mu.Lock()
		plan, requests = c.plan, 0
		mu.Unlock()
		var deltas []string
		start := time.Now()
		out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi all"}, func(s string) { deltas = append(deltas, s) })
		mu.Lock()
		n := requests
		mu.Unlock()
		if err != nil || out != "大家好" || n != c.requests || strings.Join(deltas, "|") != "大家|大家好" || time.Since(start) > 2*time.Second {
			t.Errorf("%s: %q %v, %d requests in %v, deltas %q", c.name, out, err, n, time.Since(start), deltas)
		}
	}

	// Stopped while waiting: not asked again.
	mu.Lock()
	plan, requests = []reply{{after: hang}, {}}, 0
	mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := e.Translate(ctx, Request{Src: "en", Tgt: "zh", Text: "Hi all"}, nil)
	time.Sleep(100 * time.Millisecond) // past FirstWait
	mu.Lock()
	n := requests
	mu.Unlock()
	if err == nil || n != 1 {
		t.Errorf("stopped: %v, %d requests", err, n)
	}
}

// TestDraftStalled drops a draft whose first words do not come in time, without asking again: one
// of more words follows.
func TestDraftStalled(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		requests.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Index, FirstWait: 100 * time.Millisecond}
	start := time.Now()
	_, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi", Draft: true}, nil)
	time.Sleep(150 * time.Millisecond) // time to ask again, were it to
	if !errors.Is(err, ErrStalled) || requests.Load() != 1 || time.Since(start) > time.Second {
		t.Errorf("%v, %d requests in %v", err, requests.Load(), time.Since(start))
	}
}

// TestGap gives up a translation whose words stop for Gap, keeping those that came.
func TestGap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Index, FirstWait: time.Second, Gap: 100 * time.Millisecond}
	start := time.Now()
	out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi all"}, nil)
	if !errors.Is(err, ErrStalled) || out != "大家" || time.Since(start) > time.Second {
		t.Errorf("%q %v in %v", out, err, time.Since(start))
	}
}

// TestWarm opens a connection when none was used within WarmIdle.
func TestWarm(t *testing.T) {
	var models, completions atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			models.Add(1)
			fmt.Fprint(w, `{"object":"list","data":[]}`)
		default:
			completions.Add(1)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n")
		}
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Style: Index}
	e.Warm(context.Background())
	if models.Load() != 0 {
		t.Fatal("warmed without WarmIdle")
	}
	e.WarmIdle = 100 * time.Millisecond
	e.Warm(context.Background())
	e.Warm(context.Background()) // just warmed
	e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil)
	time.Sleep(50 * time.Millisecond)
	e.Warm(context.Background()) // just used
	if models.Load() != 1 {
		t.Errorf("warmed %d times", models.Load())
	}
	time.Sleep(100 * time.Millisecond)
	e.Warm(context.Background())
	if models.Load() != 2 || completions.Load() != 1 {
		t.Errorf("warmed %d times, %d completions", models.Load(), completions.Load())
	}
}
