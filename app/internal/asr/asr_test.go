package asr

import (
	"context"
	"errors"
	"testing"

	"tomoru/internal/models"
)

func TestSpecsOrder(t *testing.T) {
	var ids []string
	for _, s := range specs("darwin") {
		ids = append(ids, s.ID)
	}
	if len(ids) < 4 || ids[0] != "apple" || ids[len(ids)-2] != QwenCloud || ids[len(ids)-1] != OpenAI {
		t.Fatalf("the system's first, the cloud's last: %v", ids)
	}
	for _, s := range specs("windows") {
		if s.ID == "apple" {
			t.Fatal("Apple's recognition listed off macOS")
		}
	}
	for _, m := range models.All {
		if s, ok := ByID(m.ID); ok != m.ASR || ok && (s.Model == nil || s.Model.ID != m.ID || s.Family != Batch) {
			t.Fatalf("model %s: listed %v as %+v", m.ID, ok, s)
		}
	}
}

type fakeServer struct{}

func (fakeServer) EnsureVision(context.Context, string) (string, error) {
	return "http://127.0.0.1:1", nil
}

func TestSpecsMake(t *testing.T) {
	apple := specs("darwin")[0]
	if f, err := apple.New(Env{}); f != nil || err != nil {
		t.Fatal("the system's recognition must stay in the engine")
	}
	var model, served Spec
	for _, s := range specs("windows") {
		switch {
		case s.Model == nil:
		case s.Model.Runtime == "llama":
			served = s
		default:
			model = s
		}
	}
	if _, err := served.New(Env{ASR: "/bin/asr", ModelDir: "/models/asr"}); !errors.Is(err, ErrLocal) {
		t.Fatalf("a served model without its server: %v", err)
	}
	if f, err := served.New(Env{Server: fakeServer{}, ModelPath: "/models/q.gguf"}); f == nil || err != nil {
		t.Fatalf("a served model: %v", err)
	}
	if !served.VolatileSentences || served.FinalsEnd {
		t.Error("an untimed model's volatile sentence ends commit; its finals end stretches")
	}
	if _, err := model.New(Env{}); !errors.Is(err, ErrLocal) {
		t.Fatalf("a local model without its paths: %v", err)
	}
	if f, err := model.New(Env{ASR: "/bin/asr", ModelDir: "/models/asr"}); f == nil || err != nil {
		t.Fatalf("a local model with its paths: %v", err)
	}
	if !apple.FinalsEnd || !apple.VolatileSentences || apple.Hint == nil || model.FinalsEnd || model.VolatileSentences {
		t.Error("only services end utterances with final results, and only theirs hold volatile sentence ends")
	}
	cloud, ok := ByID(QwenCloud)
	if !ok || !cloud.Cloud || !cloud.Billed || !cloud.Key || cloud.Langs == nil || !cloud.FinalsEnd || !cloud.VolatileSentences {
		t.Fatalf("the cloud's entry: %+v", cloud)
	}
	if f, err := cloud.New(Env{Service: Service{Key: "k"}}); f == nil || err != nil {
		t.Fatalf("the cloud's factory: %v", err)
	}
	oa, ok := ByID(OpenAI)
	if !ok || !oa.Cloud || !oa.Billed || !oa.Key || oa.FinalsEnd || oa.URL == "" || len(oa.Models) < 2 || oa.Models[0] != "gpt-live-transcribe" {
		t.Fatalf("OpenAI's entry: %+v", oa)
	}
	for _, model := range []string{"", "gpt-transcribe"} { // live transcription, the transcription API
		if f, err := oa.New(Env{Service: Service{Model: model, Key: "k"}}); f == nil || err != nil {
			t.Fatalf("OpenAI's factory of %q: %v", model, err)
		}
	}
	if _, ok := ByID("unknown"); ok {
		t.Fatal("an unknown recognizer found")
	}
}

func TestHints(t *testing.T) {
	s := Scene{Names: []string{"一心", "葦名", "竜胤"}, About: " 只狼的对话 "}
	for _, c := range []struct {
		max  int
		want string
	}{{100, "只狼的对话\n一心、葦名、竜胤"}, {12, "只狼的对话\n一心、葦名"}, {3, "只狼的"}} {
		if got := s.Hints("、", c.max); got != c.want {
			t.Errorf("Hints(%d) = %q, want %q", c.max, got, c.want)
		}
	}
	if got := (Scene{Names: []string{"a", "b"}}).Hints(", ", 100); got != "a, b" {
		t.Errorf("no description: %q", got)
	}
}
