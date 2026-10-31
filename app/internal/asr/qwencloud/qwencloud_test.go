package qwencloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tomoru/internal/engine"
)

// fakeService is the service: it starts the task, counts the audio, and answers with the results
// given, a volatile one after the first audio and a final one after the second.
func fakeService(t *testing.T, key string) (*httptest.Server, *[]map[string]any, *int) {
	var run []map[string]any
	audio := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api-ws/v1/inference" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		for {
			typ, b, err := c.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageText {
				var m map[string]any
				json.Unmarshal(b, &m)
				run = append(run, m)
				c.Write(ctx, websocket.MessageText, []byte(`{"header":{"event":"task-started"},"payload":{}}`))
				continue
			}
			audio++
			switch audio {
			case 1:
				c.Write(ctx, websocket.MessageText, []byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"begin_time":200,"end_time":null,"text":"大家","sentence_end":false}}}}`))
			case 2:
				c.Write(ctx, websocket.MessageText, []byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"heartbeat":true}}}}`))
				c.Write(ctx, websocket.MessageText, []byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"begin_time":300,"end_time":380,"text":"大家好。","sentence_end":true,
					"words":[{"begin_time":300,"end_time":340,"text":"大家","punctuation":""},{"begin_time":340,"end_time":380,"text":"好","punctuation":"。"}]}}}}`))
			}
		}
	}))
	return srv, &run, &audio
}

func audioAt(t float64) engine.Event {
	return engine.Event{Type: "audio", T: t, PCM: base64.StdEncoding.EncodeToString(make([]byte, 3200))}
}

func TestSession(t *testing.T) {
	srv, run, _ := fakeService(t, "sk-1")
	defer srv.Close()
	var mu sync.Mutex
	var evs []engine.Event
	s := Start(Config{Host: srv.URL, Key: "sk-1", Vocabulary: []string{"一心", "葦名"}, Context: "只狼的对话\n一心、葦名"}, "ja", func(ev engine.Event) {
		mu.Lock()
		evs = append(evs, ev)
		mu.Unlock()
	})
	defer s.Stop()
	// The engine's time starts before the service's: its first audio is at 2 s, which the service
	// hears after the lead's 0.2 s of silence.
	s.Audio(audioAt(2))
	s.Audio(audioAt(2.1))
	var results []engine.Event
	for deadline := time.Now().Add(2 * time.Second); len(results) < 2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("events %+v", evs)
		}
		mu.Lock()
		results = nil
		for _, ev := range evs {
			if ev.Type == "result" {
				results = append(results, ev)
			}
		}
		mu.Unlock()
	}
	v, f := results[0], results[1]
	if v.Final || v.Text() != "大家" || v.Start != 2 {
		t.Errorf("volatile %+v", v)
	}
	if !f.Final || f.Text() != "大家好。" || f.Start != 2.1 || f.End != 2.18 || *f.Tokens[1].Start != 2.14 || *f.Tokens[1].End != 2.18 {
		t.Errorf("final %+v", f)
	}
	mu.Lock()
	if evs[0].Type != "clock" || evs[0].T != 2.1 {
		t.Errorf("first event %+v", evs[0])
	}
	mu.Unlock()
	p := (*run)[0]["payload"].(map[string]any)
	params := p["parameters"].(map[string]any)
	if p["model"] != Models[0] || params["sample_rate"] != 16000.0 || params["format"] != "pcm" || params["language_hints"].([]any)[0] != "ja" {
		t.Errorf("run-task %v", p)
	}
	// What it is told to expect: the names as hot words, and the scene as context.
	vocab, _ := params["vocabulary"].(map[string]any)
	ctxt, _ := p["input"].(map[string]any)["context"].([]any)
	if vocab["一心"] != float64(weight) || len(vocab) != 2 || len(ctxt) != 1 ||
		ctxt[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"] != "只狼的对话\n一心、葦名" {
		t.Errorf("hints %v, %v", vocab, ctxt)
	}
	// Billed: the lead's silence and the two pieces of audio.
	if sent := s.Sent(); sent < .399 || sent > .401 {
		t.Errorf("sent %v s", sent)
	}
	s.Stop()
	<-s.Done()
	if s.Err() != nil {
		t.Errorf("stopped: %v", s.Err())
	}
}

// TestIdle hangs up after idle seconds without voice, and calls again when voice starts, with
// the preroll before it.
func TestIdle(t *testing.T) {
	var mu sync.Mutex
	var calls []int // bytes of audio each call got
	ended := 0      // calls hung up
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		mu.Lock()
		n := len(calls)
		calls = append(calls, 0)
		mu.Unlock()
		for {
			typ, b, err := c.Read(r.Context())
			if err != nil {
				mu.Lock()
				ended++
				mu.Unlock()
				return
			}
			if typ == websocket.MessageText {
				c.Write(r.Context(), websocket.MessageText, []byte(`{"header":{"event":"task-started"},"payload":{}}`))
				continue
			}
			mu.Lock()
			calls[n] += len(b)
			got := calls[n]
			mu.Unlock()
			if n == 1 && got == 2*lead+3200 {
				c.Write(r.Context(), websocket.MessageText, []byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"begin_time":200,"end_time":400,"text":"はい。","sentence_end":true,
					"words":[{"begin_time":200,"end_time":400,"text":"はい","punctuation":"。"}]}}}}`))
			}
		}
	}))
	defer srv.Close()
	var evs []engine.Event
	s := Start(Config{Host: srv.URL, Key: "sk-1", Vocabulary: []string{"一心", "葦名"}, Context: "只狼的对话\n一心、葦名"}, "ja", func(ev engine.Event) {
		mu.Lock()
		evs = append(evs, ev)
		mu.Unlock()
	})
	defer s.Stop()
	wait := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			mu.Lock()
			done := ok()
			mu.Unlock()
			if done {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: calls %v, ended %d", what, calls, ended)
			}
		}
	}
	at := func(i int) float64 { return float64(i) / 10 }
	s.Voice(engine.Event{Type: "voice", On: false, T: 0})
	for i := range 105 {
		s.Audio(audioAt(at(i)))
	}
	// 10 s of audio, the last ending at 10 s, went before the hang-up.
	wait("hung up", func() bool { return ended == 1 })
	if calls[0] != 2*lead+100*3200 {
		t.Errorf("first call got %d bytes", calls[0])
	}
	for i := 105; i < 120; i++ {
		s.Audio(audioAt(at(i)))
	}
	s.Voice(engine.Event{Type: "voice", On: true, T: 11.5})
	for i := 120; i < 125; i++ {
		s.Audio(audioAt(at(i)))
	}
	// The second call: the lead, the audio from 11.0, and what came after.
	wait("called again", func() bool { return len(calls) == 2 && calls[1] == 2*lead+15*3200 })
	wait("result", func() bool {
		for _, ev := range evs {
			if ev.Type == "result" {
				return true
			}
		}
		return false
	})
	mu.Lock()
	defer mu.Unlock()
	voices := 0
	for _, ev := range evs {
		switch ev.Type {
		case "voice":
			voices++
		case "result":
			if ev.Start != 11 || ev.End != 11.2 || *ev.Tokens[0].Start != 11 {
				t.Errorf("result %+v", ev)
			}
		}
	}
	if voices != 2 {
		t.Errorf("%d voice events", voices)
	}
}

// loudAt is 100 ms of sound, a song's with no voice detected in it, from t.
func loudAt(t float64) engine.Event {
	pcm := make([]byte, 3200)
	for i := 0; i < len(pcm); i += 2 {
		pcm[i+1] = 0x10 // 4096
	}
	return engine.Event{Type: "audio", T: t, PCM: base64.StdEncoding.EncodeToString(pcm)}
}

// TestIdleSound keeps the call while sound plays with no voice detected, as music, and calls
// again when sound comes after it hung up.
func TestIdleSound(t *testing.T) {
	var mu sync.Mutex
	calls, ended := 0, 0
	resumed := make(chan []byte, 1)
	nextPCM := bytes.Repeat([]byte{0x23, 0x45}, 1600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		var pcm []byte
		for {
			typ, b, err := c.Read(r.Context())
			if err != nil {
				mu.Lock()
				ended++
				mu.Unlock()
				return
			}
			if typ == websocket.MessageText {
				c.Write(r.Context(), websocket.MessageText, []byte(`{"header":{"event":"task-started"},"payload":{}}`))
			} else if call == 2 {
				pcm = append(pcm, b...)
				if bytes.Equal(b, nextPCM) {
					resumed <- append([]byte(nil), pcm...)
				}
			}
		}
	}))
	defer srv.Close()
	s := Start(Config{Host: srv.URL, Key: "sk-1"}, "en", func(engine.Event) {})
	defer s.Stop()
	s.Voice(engine.Event{Type: "voice", On: false, T: 0})
	for i := range 150 {
		s.Audio(loudAt(float64(i) / 10)) // 15 s of a song
	}
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	if ended != 0 {
		t.Errorf("hung up on a song")
	}
	mu.Unlock()
	for i := 150; i < 260; i++ {
		s.Audio(audioAt(float64(i) / 10)) // 11 s of silence
	}
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		mu.Lock()
		e := ended
		mu.Unlock()
		if e == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("did not hang up in silence")
		}
	}
	s.Audio(loudAt(26))
	s.Audio(engine.Event{Type: "audio", T: 26.1, PCM: base64.StdEncoding.EncodeToString(nextPCM)})
	select {
	case got := <-resumed:
		// Lead silence, 0.5 s preroll, the sound that resumed the call once, and
		// the following chunk, in order and without adding time to the audio clock.
		want := make([]byte, 2*lead+5*3200)
		first, _ := base64.StdEncoding.DecodeString(loudAt(26).PCM)
		want = append(want, first...)
		want = append(want, nextPCM...)
		if !bytes.Equal(got, want) {
			t.Fatalf("resumed audio differs: got %d bytes, want %d", len(got), len(want))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive audio after sound resumed")
	}
}

// TestSteady keeps only the sentence ends of the volatile texts the service settled on, as
// Model Studio sent them for a Korean and a Japanese recording.
func TestSteady(t *testing.T) {
	for _, c := range []struct{ lang, prev, cur, want string }{
		// A preview's end, joined to what follows.
		{"ko", "현역 은퇴 잘 놀다.", "현역 은퇴 잘 놀다.다는 게.", "현역 은퇴 잘 놀다다는 게"},
		// A settled end, with a space after it.
		{"ko", "가셨다고 그렇죠. 저 뭐", "가셨다고 그렇죠. 저 뭐 좋은 곳으로", "가셨다고 그렇죠. 저 뭐 좋은 곳으로"},
		// An end first seen, with the preview after it.
		{"ko", "그리고 가운데서", "그리고 가운데서 어버하고. 옛날은 찦가.", "그리고 가운데서 어버하고 옛날은 찦가"},
		// A sentence's first preview, in Chinese.
		{"ko", "", "没差", ""},
		{"ko", "", "뭐 자본지", "뭐 자본지"},
		// Japanese: no spaces; an end and the letter after it as they were.
		{"ja", "らしいです。でめちゃくちゃ厳", "らしいです。でめちゃくちゃ厳しくて", "らしいです。でめちゃくちゃ厳しくて"},
		{"ja", "らしいです", "らしいです。でめちゃくちゃ厳", "らしいですでめちゃくちゃ厳"},
		{"ja", "厳格な人で", "厳格な人で", "厳格な人で"},
		// Chinese keeps its ends.
		{"zh", "大家好", "大家好。今天我们。", "大家好。今天我们。"},
		{"yue", "大家好", "大家好。今日我哋。", "大家好。今日我哋。"},
		// Han characters go from any language not written with them.
		{"es", "", "没差", ""},
		// An end ending the text, sent again as it was: the speaker paused.
		{"ja", "らしいです。", "らしいです。", "らしいです。"},
		{"ko", "주주총회입니다.", "주주총회입니다.", "주주총회입니다."},
		{"en", "Hello everyone. Wel", "Hello everyone. Welcome to the.", "Hello everyone. Welcome to the"},
	} {
		if got := steady(c.prev, c.cur, c.lang); got != c.want {
			t.Errorf("%s %q after %q: %q, want %q", c.lang, c.cur, c.prev, got, c.want)
		}
	}
}

func TestSpaceAhead(t *testing.T) {
	var toks []engine.Token
	for _, s := range []string{"Hey", " everyone, ", "welcome", " to", " the ", "chat. "} {
		toks = append(toks, engine.Token{Text: s})
	}
	spaceAhead(toks)
	var got []string
	for _, tok := range toks {
		got = append(got, tok.Text)
	}
	if want := []string{"Hey", " everyone,", " welcome", " to", " the", " chat."}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%q", got)
	}
}

func TestAuth(t *testing.T) {
	srv, _, _ := fakeService(t, "sk-1")
	defer srv.Close()
	if _, err := Test(context.Background(), Config{Host: srv.URL, Key: "wrong"}); !errors.Is(err, ErrAuth) {
		t.Errorf("wrong key: %v", err)
	}
	if _, err := Test(context.Background(), Config{Host: srv.URL, Key: "sk-1"}); err != nil {
		t.Errorf("right key: %v", err)
	}
	s := Start(Config{Host: srv.URL, Key: "wrong"}, "zh", func(engine.Event) {})
	<-s.Done()
	if !errors.Is(s.Err(), ErrAuth) {
		t.Errorf("session: %v", s.Err())
	}
}

func TestURL(t *testing.T) {
	for host, want := range map[string]string{
		"":                                  "wss://dashscope.aliyuncs.com/api-ws/v1/inference",
		"ws-1.cn-beijing.maas.aliyuncs.com": "wss://ws-1.cn-beijing.maas.aliyuncs.com/api-ws/v1/inference",
		"https://dashscope-intl.aliyuncs.com/compatible-mode/v1": "wss://dashscope-intl.aliyuncs.com/api-ws/v1/inference",
		"http://127.0.0.1:8080":                                  "ws://127.0.0.1:8080/api-ws/v1/inference",
	} {
		if got, err := (Config{Host: host}).URL(); got != want || err != nil {
			t.Errorf("%q: %s %v", host, got, err)
		}
	}
}

// TestReal connects to the real service with DASHSCOPE_API_KEY and DASHSCOPE_API_HOST, when they
// are set.
func TestReal(t *testing.T) {
	key := os.Getenv("DASHSCOPE_API_KEY")
	if key == "" {
		t.Skip("set DASHSCOPE_API_KEY (and DASHSCOPE_API_HOST) to connect to Model Studio")
	}
	for _, m := range Models {
		took, err := Test(context.Background(), Config{Host: os.Getenv("DASHSCOPE_API_HOST"), Model: m, Key: key})
		t.Logf("%s: started in %v, %v", m, took, err)
		if err != nil {
			t.Error(err)
		}
	}
	if _, err := Test(context.Background(), Config{Host: os.Getenv("DASHSCOPE_API_HOST"), Key: "sk-wrong"}); !errors.Is(err, ErrAuth) {
		t.Errorf("a wrong key: %v", err)
	}
}
