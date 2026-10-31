package provider

import (
	"context"
	"errors"
	"testing"

	"tomoru/internal/translate"
)

type testServer struct {
	path string
	err  error
}

func (s *testServer) Ensure(ctx context.Context, path string) (string, error) {
	s.path = path
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "http://127.0.0.1:1234", s.err
}

func TestResolvedTranslationProviders(t *testing.T) {
	remote := Endpoint("https://example.test", "model", "secret", false)
	if remote.Style != translate.Chat || remote.APIKey != "secret" {
		t.Fatal("endpoint configuration lost")
	}
	s := &testServer{}
	tr, err := Local(context.Background(), s, "/models/model.gguf")
	if err != nil {
		t.Fatal(err)
	}
	local := tr
	if s.path != "/models/model.gguf" || local.Model != s.path || !local.Llama || local.Style != translate.Index || local.URL != "http://127.0.0.1:1234" {
		t.Fatalf("local configuration lost: %+v", local)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Local(ctx, s, "model"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
