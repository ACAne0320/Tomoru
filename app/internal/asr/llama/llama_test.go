package llama

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"tomoru/internal/models"
)

func TestAnswer(t *testing.T) {
	for in, want := range map[string]string{
		"language Japanese<asr_text>その後、大事はないか。": "その後、大事はないか。",
		"language None<asr_text>":                "",
		"language None":                          "",
		" language English<asr_text> Hello.":     "Hello.",
	} {
		if got := answer(in); got != want {
			t.Errorf("answer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGrammar(t *testing.T) {
	for lang, want := range map[string]string{"ja": `root ::= "language " ("Japanese" | "None") [^\x00]*`,
		"yue": `root ::= "language " ("Cantonese" | "None") [^\x00]*`, "zh-Hant": "", "xx": ""} {
		if got := grammar(lang); got != want {
			t.Errorf("grammar(%q) = %q, want %q", lang, got, want)
		}
	}
	auto := grammar("auto")
	if !strings.HasPrefix(auto, `root ::= "language " ("Chinese" | "English" | "Japanese" | "Korean" | "Cantonese" | `) ||
		!strings.HasSuffix(auto, ` | "None") [^\x00]*`) || strings.Count(auto, " | ") != len(models.Qwen3ASRLangs) {
		t.Errorf("grammar(auto) = %q", auto)
	}
}

func TestAuto(t *testing.T) {
	const told = `{"choices":[{"message":{"content":"language %s<asr_text>%s"},"logprobs":{"content":[` +
		`{"token":"language","top_logprobs":[{"token":"language","logprob":0}]},` +
		`{"token":" %s","top_logprobs":[{"token":" %s","logprob":0},{"token":" Korean","logprob":%d}]}]}}]}`
	var grammars []string
	answer := func(w http.ResponseWriter) {
		switch n := len(grammars); {
		case n == 1:
			w.Write([]byte(`{"choices":[{"message":{"content":"language Korean<asr_text>안녕하세요."}}]}`))
		case n == 2:
			fmt.Fprintf(w, told, "Chinese", "我们BLG就是最强的。", "Chinese", "Chinese", -20)
		case n == 3, n == 5:
			fmt.Fprintf(w, told, "Japanese", "食べい。", "Japanese", "Japanese", -2)
		default:
			w.Write([]byte(`{"choices":[{"message":{"content":"language Chinese<asr_text>吃吧。"}}]}`))
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got struct{ Grammar string }
		json.NewDecoder(r.Body).Decode(&got)
		grammars = append(grammars, got.Grammar)
		answer(w)
	}))
	defer srv.Close()
	d := New(srv.URL, "auto", "")
	defer d.Stop()
	var heard []string
	for _, n := range []int{3200, 3200, 3200, 2 * rate * brief} {
		ps, err := d.Decode(context.Background(), make([]byte, n))
		if err != nil || len(ps) == 0 {
			t.Fatalf("decoded %v: %v", ps, err)
		}
		heard = append(heard, ps[0].Lang)
	}
	if !slices.Equal(heard, []string{"ko", "zh", "zh", "ja"}) || !slices.Equal(grammars, []string{grammar("auto"), "", "", grammar("zh"), ""}) {
		t.Fatalf("heard %v, grammars %q", heard, grammars)
	}
}

func TestDecode(t *testing.T) {
	var got struct {
		Messages []struct {
			Role    string
			Content json.RawMessage
		}
		Grammar     string
		Logprobs    bool
		TopLogprobs int `json:"top_logprobs"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"choices":[{"message":{"content":"language Japanese<asr_text>狼よ。"}}]}`))
	}))
	defer srv.Close()
	d := New(srv.URL, "ja", "一心, 葦名")
	defer d.Stop()
	ps, err := d.Decode(context.Background(), make([]byte, 3200))
	if err != nil || len(ps) != 3 || ps[0].Text != "狼" || ps[2].Text != "。" {
		t.Fatalf("%v %v", ps, err)
	}
	var system string
	if len(got.Messages) == 2 {
		json.Unmarshal(got.Messages[0].Content, &system)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "system" || system != "一心, 葦名" || got.Messages[1].Role != "user" || got.Grammar != "" || !got.Logprobs || got.TopLogprobs != topLogprobs {
		t.Fatalf("messages %d, system %q, grammar %q, logprobs %v %d", len(got.Messages), system, got.Grammar, got.Logprobs, got.TopLogprobs)
	}
	got.Messages = got.Messages[1:]
	var parts []struct {
		Type       string
		InputAudio struct{ Data, Format string } `json:"input_audio"`
	}
	json.Unmarshal(got.Messages[0].Content, &parts)
	audio, _ := base64.StdEncoding.DecodeString(parts[0].InputAudio.Data)
	if len(parts) != 1 || parts[0].Type != "input_audio" || parts[0].InputAudio.Format != "wav" ||
		string(audio[:4]) != "RIFF" || string(audio[8:12]) != "WAVE" || len(audio) != 44+3200 {
		t.Fatalf("audio %+v, %d bytes", parts, len(audio))
	}
}

func TestLanguage(t *testing.T) {
	const other = `{"choices":[{"message":{"content":"language Chinese<asr_text>我们BLG就是最强的。"},"logprobs":{"content":[` +
		`{"token":"language","top_logprobs":[{"token":"language","logprob":0}]},` +
		`{"token":" Chinese","top_logprobs":[{"token":" Chinese","logprob":0},{"token":" Cantonese","logprob":-9}%s]}]}}]}`
	for _, c := range []struct {
		name, alternatives string
		asks               int
		want, heard        string
	}{
		{"sure", `,{"token":" Eng","logprob":-20}`, 1, "我们BLG就是最强的。", "zh"},
		{"not sure", `,{"token":" English","logprob":-4}`, 2, "We BLG are the strongest.", "en"},
		{"not listed, below the least listed", "", 2, "We BLG are the strongest.", "en"},
		{"not listed, the least listed unlikely", `,{"token":" Japanese","logprob":-20}`, 1, "我们BLG就是最强的。", "zh"},
	} {
		asks := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var got struct{ Grammar string }
			json.NewDecoder(r.Body).Decode(&got)
			asks++
			if got.Grammar == grammar("en") {
				w.Write([]byte(`{"choices":[{"message":{"content":"language English<asr_text>We BLG are the strongest."}}]}`))
				return
			}
			fmt.Fprintf(w, other, c.alternatives)
		}))
		d := New(srv.URL, "en", "")
		text, heard, err := d.transcribe(context.Background(), make([]byte, 3200))
		d.Stop()
		srv.Close()
		if err != nil || text != c.want || heard != c.heard || asks != c.asks {
			t.Errorf("%s: %q in %q after %d asks, %v", c.name, text, heard, asks, err)
		}
	}
}

func TestBound(t *testing.T) {
	var got struct {
		Messages  []struct{ Role string }
		MaxTokens int `json:"max_tokens"`
	}
	reason := "length"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"choices":[{"finish_reason":"` + reason + `","message":{"content":"language English<asr_text>Congratulations! Congratulations!"}}]}`))
	}))
	defer srv.Close()
	d := New(srv.URL, "en", "")
	defer d.Stop()
	ps, err := d.Decode(context.Background(), make([]byte, 3*2*rate))
	if err != nil || len(ps) != 0 {
		t.Fatalf("an answer cut at its limit: %v %v", ps, err)
	}
	if got.MaxTokens != tokensMore+3*tokensPerSecond || len(got.Messages) != 2 || got.Messages[0].Role != "system" {
		t.Fatalf("max_tokens %d for 3 s, messages %+v", got.MaxTokens, got.Messages)
	}
	reason = "stop"
	if ps, err := d.Decode(context.Background(), make([]byte, 3*2*rate)); err != nil || len(ps) == 0 {
		t.Fatalf("an answer that ended: %v %v", ps, err)
	}
}

func TestStop(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	d := New(srv.URL, "en", "")
	errc := make(chan error, 1)
	go func() { _, err := d.Decode(context.Background(), nil); errc <- err }()
	time.Sleep(50 * time.Millisecond)
	d.Stop()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("a stopped decode succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not cancel the decode")
	}
	<-d.Done()

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "loading model", http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	if _, err := New(failing.URL, "en", "").Decode(context.Background(), nil); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("a server error: %v", err)
	}
}

func TestUnfit(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			w.Write([]byte(`{"choices":[{"message":{"content":"language Chinese<asr_text>我不要。"}}]}`))
		case 2:
			w.Write([]byte(`{"choices":[{"message":{"content":"language Japanese<asr_text>金になる。"},"logprobs":{"content":[` +
				`{"token":"language","top_logprobs":[{"token":"language","logprob":0}]},` +
				`{"token":" Japanese","top_logprobs":[{"token":" Japanese","logprob":0},{"token":" Chinese","logprob":-2}]}]}}]}`))
		default:
			w.Write([]byte(`{"choices":[{"message":{"content":"language Chinese<asr_text>金になるかもしれない。"}}]}`))
		}
	}))
	defer srv.Close()
	d := New(srv.URL, "auto", "")
	defer d.Stop()
	d.Decode(context.Background(), make([]byte, 3200))
	ps, err := d.Decode(context.Background(), make([]byte, 3200))
	if err != nil || len(ps) == 0 || ps[0].Lang != "" || n != 3 {
		t.Fatalf("pieces %+v after %d asks: %v", ps, n, err)
	}
}

func TestReasoningContent(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"language English","content":"Hello world."}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"language None","content":""}}]}`))
	}))
	defer srv.Close()
	d := New(srv.URL, "auto", "")
	defer d.Stop()
	ps, err := d.Decode(context.Background(), make([]byte, 3200))
	if err != nil || len(ps) == 0 || ps[0].Lang != "en" {
		t.Fatalf("speech: %+v %v", ps, err)
	}
	var text strings.Builder
	for _, p := range ps {
		text.WriteString(p.Text)
	}
	if text.String() != "Hello world." {
		t.Fatalf("text = %q, want %q", text.String(), "Hello world.")
	}
	silent, err := d.Decode(context.Background(), make([]byte, 3200))
	if err != nil || len(silent) != 0 {
		t.Fatalf("silence: %+v %v", silent, err)
	}
}
