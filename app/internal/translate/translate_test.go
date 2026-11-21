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

	e := &Endpoint{URL: srv.URL + "/", Model: "m", APIKey: "k", Format: Index}
	var deltas []string
	out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi all!"}, func(s string) { deltas = append(deltas, s) })
	if err != nil || out != "大家好！" || strings.Join(deltas, "|") != "大家|大家好|大家好！" {
		t.Fatalf("%q %v, deltas %q", out, err, deltas)
	}
	prompt := body["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, "请将以下英语文本翻译成中文") || !strings.Contains(prompt, "【源文】\nHi all!\n") || body["cache_prompt"] != nil {
		t.Errorf("prompt %q, body %v", prompt, body)
	}

	e.Format, e.Llama = Chat, true
	e.Translate(context.Background(), Request{Src: "ja", Tgt: "en", Text: "こんにちは", Draft: true}, nil)
	msgs := body["messages"].([]any)
	sys := msgs[0].(map[string]any)["content"].(string)
	if len(msgs) != 2 || !strings.Contains(sys, "Japanese line into natural, colloquial English") || !strings.Contains(sys, "unfinished") || body["cache_prompt"] != true {
		t.Errorf("messages %v, body %v", msgs, body)
	}
}

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

func TestHyMT(t *testing.T) {
	var body struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Temperature *float64 `json:"temperature"`
		Thinking    any      `json:"thinking"`
		Effort      any      `json:"reasoning_effort"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"克劳德！\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: HyMT}
	for _, c := range []struct {
		r    Request
		want string
	}{
		{Request{Src: "ja", Tgt: "zh", Text: "クラウド！"},
			"【待翻译文本】\nクラウド！\n\n【翻译任务】\n1、只输出译文，不要额外解释\n2、将【待翻译文本】翻译为中文。"},
		{Request{Src: "ja", Tgt: "en", Text: "クラウドが", Draft: true, Terms: []Term{{"クラウド", "Cloud"}, {"メテオ", "Meteor"}}, About: "最终幻想7 重制版"},
			"【待翻译文本】\nクラウドが\n\n【翻译任务】\n1、专名按此翻译：クラウド→Cloud、メテオ→Meteor\n2、背景：最终幻想7 重制版\n" +
				"3、【待翻译文本】是一句还没说完的话的开头：只翻译已有的部分，不要补全或猜测后文\n4、只输出译文，不要额外解释\n5、将【待翻译文本】翻译为英语。"},
	} {
		if out, err := e.Translate(context.Background(), c.r, nil); err != nil || out != "克劳德！" {
			t.Fatalf("%q %v", out, err)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != c.want {
			t.Errorf("%+v: messages %+v", c.r, body.Messages)
		}
		if body.Temperature == nil || *body.Temperature != 0 || body.Thinking != nil || body.Effort != nil {
			t.Errorf("temperature %v, thinking %v, effort %v", body.Temperature, body.Thinking, body.Effort)
		}
	}
}

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

	e := &Endpoint{URL: srv.URL, Model: "m", Format: Chat}
	if out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
		t.Fatalf("%q %v", out, err)
	}
	b := bodies[0]
	if fmt.Sprint(b["thinking"]) != "map[type:disabled]" || b["reasoning_effort"] != "none" || b["enable_thinking"] != false ||
		fmt.Sprint(b["chat_template_kwargs"]) != "map[enable_thinking:false]" {
		t.Errorf("body %v", b)
	}

	bodies, refuse = nil, true
	e = &Endpoint{URL: srv.URL, Model: "m", Format: Chat}
	for range 2 {
		if out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil); err != nil || out != "好" {
			t.Fatalf("%q %v", out, err)
		}
	}
	if len(bodies) != 3 || bodies[2]["thinking"] != nil {
		t.Errorf("%d requests: %v", len(bodies), bodies)
	}

	bodies = nil
	e = &Endpoint{URL: srv.URL, Model: "m", Format: Index}
	e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil)
	if len(bodies) != 1 || bodies[0]["thinking"] != nil || bodies[0]["reasoning_effort"] != nil {
		t.Errorf("Index-Translate: %v", bodies)
	}
}

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
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, Llama: true}
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
		out, _, _, err := readStream(strings.NewReader(c.stream), nil)
		switch {
		case c.want != nil && !errors.Is(err, c.want), c.says != "" && (err == nil || !strings.Contains(err.Error(), c.says)):
			t.Errorf("%q: %q, %v", c.stream, out, err)
		case c.want == nil && c.says == "" && err != nil:
			t.Errorf("%q: %v", c.stream, err)
		}
	}
}

func TestStreamErrorAfterPartial(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"部分译文\"}}]}\n\n" +
		"data: {\"error\":{\"message\":\"upstream failure\"},\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"
	var partial string
	out, used, _, err := readStream(strings.NewReader(stream), func(text string) { partial = text })
	if out != "部分译文" || partial != out || used != (Usage{8, 3}) {
		t.Fatalf("partial output or usage lost: out=%q, partial=%q, usage=%+v", out, partial, used)
	}
	if err == nil || !strings.Contains(err.Error(), "upstream failure") {
		t.Fatalf("provider failure lost after partial output: %v", err)
	}
}

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

func TestChat(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"terms\\\":[]}\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Chat}
	out, err := e.Chat(context.Background(), "sys", "user", 1024)
	msgs := body["messages"].([]any)
	if err != nil || out != `{"terms":[]}` || len(msgs) != 2 || msgs[0].(map[string]any)["content"] != "sys" || body["max_tokens"] != 1024.0 || body["reasoning_effort"] != "none" {
		t.Errorf("%q %v, body %v", out, err, body)
	}
}

func TestStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := (&Endpoint{URL: srv.URL, Model: "m"}).Translate(context.Background(), Request{Src: "ja", Tgt: "zh", Text: "はい"}, nil)
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusUnauthorized || err.Error() != "translate: 401 Unauthorized: invalid api key" {
		t.Errorf("%v", err)
	}
}

func TestLimited(t *testing.T) {
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if asked.Add(1) == 1 {
			http.Error(w, `{"error":{"message":"The request rate exceeds the current model RPM limit 60."}}`, http.StatusTooManyRequests)
			return
		}
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m"}
	draft := Request{Src: "ja", Tgt: "zh", Text: "はい", Draft: true}
	if _, err := e.Translate(context.Background(), draft, nil); err == nil {
		t.Fatal("no 429")
	}
	if _, err := e.Translate(context.Background(), draft, nil); !errors.Is(err, ErrDraftPaused) || asked.Load() != 1 {
		t.Errorf("draft after the 429: %v, %d asked", err, asked.Load())
	}
	if out, err := e.Translate(context.Background(), Request{Src: "ja", Tgt: "zh", Text: "はい"}, nil); out != "好" || err != nil {
		t.Errorf("line after the 429: %q, %v", out, err)
	}
}

func TestTokenLimit(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"So the plan was\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Chat}
	line := strings.Repeat("长", 100)
	out, err := e.Translate(context.Background(), Request{Src: "zh", Tgt: "en", Text: line}, nil)
	if err != nil || out != "So the plan was…" || body["max_tokens"] != 228.0 {
		t.Errorf("%q %v, max_tokens %v", out, err, body["max_tokens"])
	}
	e.Format, e.Gap = Index, time.Second
	if out, err := e.Translate(context.Background(), Request{Src: "zh", Tgt: "en", Text: "好"}, nil); err != nil || out != "So the plan was…" || body["max_tokens"] != 129.0 {
		t.Errorf("Index, with a gap: %q %v, max_tokens %v", out, err, body["max_tokens"])
	}
	if out, err := e.Chat(context.Background(), "sys", "user", 1024); err != nil || out != "So the plan was" {
		t.Errorf("chat: %q %v", out, err)
	}
}

func TestFirstWait(t *testing.T) {
	const hang = -1
	type reply struct {
		after time.Duration
		empty bool
		slow  bool
	}
	var mu sync.Mutex
	var plan []reply
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
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
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, FirstWait: 100 * time.Millisecond}

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

	mu.Lock()
	plan, requests = []reply{{after: hang}, {}}, 0
	mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := e.Translate(ctx, Request{Src: "en", Tgt: "zh", Text: "Hi all"}, nil)
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	n := requests
	mu.Unlock()
	if err == nil || n != 1 {
		t.Errorf("stopped: %v, %d requests", err, n)
	}
}

func TestDraftStalled(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		requests.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, FirstWait: 100 * time.Millisecond}
	start := time.Now()
	_, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi", Draft: true}, nil)
	time.Sleep(150 * time.Millisecond)
	if !errors.Is(err, ErrStalled) || requests.Load() != 1 || time.Since(start) > time.Second {
		t.Errorf("%v, %d requests in %v", err, requests.Load(), time.Since(start))
	}
}

func TestGap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, FirstWait: time.Second, Gap: 100 * time.Millisecond}
	start := time.Now()
	out, err := e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Hi all"}, nil)
	if !errors.Is(err, ErrStalled) || out != "大家" || time.Since(start) > time.Second {
		t.Errorf("%q %v in %v", out, err, time.Since(start))
	}
}

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
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index}
	e.Warm(context.Background())
	if models.Load() != 0 {
		t.Fatal("warmed without WarmIdle")
	}
	e.WarmIdle = 100 * time.Millisecond
	e.Warm(context.Background())
	e.Warm(context.Background())
	e.Translate(context.Background(), Request{Src: "en", Tgt: "zh", Text: "Good"}, nil)
	time.Sleep(50 * time.Millisecond)
	e.Warm(context.Background())
	if models.Load() != 1 {
		t.Errorf("warmed %d times", models.Load())
	}
	time.Sleep(100 * time.Millisecond)
	e.Warm(context.Background())
	if models.Load() != 2 || completions.Load() != 1 {
		t.Errorf("warmed %d times, %d completions", models.Load(), completions.Load())
	}
}

func TestUnknownSource(t *testing.T) {
	r := Request{Tgt: "zh", Text: "안녕하세요."}
	if p := indexPrompt(r); !strings.HasPrefix(p, "请将以下文本翻译成中文，") {
		t.Errorf("Index prompt %q", p)
	}
	if s := chatSystem(r); !strings.HasPrefix(s, "You are a live subtitle translator. Translate the user's line into natural, colloquial Simplified Chinese ") {
		t.Errorf("system prompt %q", s)
	}
}
