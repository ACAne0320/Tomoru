package llama

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnswer(t *testing.T) {
	for in, want := range map[string]string{
		"language Japanese<asr_text>その後、大事はないか。": "その後、大事はないか。",
		"その後、大事はないか。":                            "その後、大事はないか。", // the prefill's continuation alone
		"language None<asr_text>":                "",
		"language None":                          "",
		" language English<asr_text> Hello.":     "Hello.",
	} {
		if got := answer(in); got != want {
			t.Errorf("answer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrefill(t *testing.T) {
	for lang, want := range map[string]string{"ja": "language Japanese<asr_text>", "yue": "language Cantonese<asr_text>",
		"zh": "language Chinese<asr_text>", "zh-Hant": "", "xx": ""} {
		if got := prefill(lang); got != want {
			t.Errorf("prefill(%q) = %q, want %q", lang, got, want)
		}
	}
}

// TestDecode sends the stretch as a WAV with the language prefilled, and reads the answer.
func TestDecode(t *testing.T) {
	var got struct {
		Messages []struct {
			Role    string
			Content json.RawMessage
		}
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
	var prefilled string
	var system string
	if len(got.Messages) == 3 {
		json.Unmarshal(got.Messages[0].Content, &system)
		json.Unmarshal(got.Messages[2].Content, &prefilled)
	}
	if len(got.Messages) != 3 || got.Messages[0].Role != "system" || system != "一心, 葦名" || got.Messages[2].Role != "assistant" || prefilled != "language Japanese<asr_text>" {
		t.Fatalf("messages %d, system %q, prefilled %q", len(got.Messages), system, prefilled)
	}
	got.Messages = got.Messages[1:] // the user's, with the audio, first
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

// TestStop cancels a decode in flight, and a server error fails a decode.
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
