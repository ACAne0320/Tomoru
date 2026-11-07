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
	remote := Endpoint("https://example.test", "model", "secret", translate.Index)
	if remote.Format != translate.Index || remote.APIKey != "secret" {
		t.Fatal("endpoint configuration lost")
	}
	s := &testServer{}
	for _, format := range []*translate.Format{translate.Index} {
		local, err := Local(context.Background(), s, "/models/model.gguf", format)
		if err != nil {
			t.Fatal(err)
		}
		if s.path != "/models/model.gguf" || local.Model != s.path || !local.Llama || local.Format != format || local.URL != "http://127.0.0.1:1234" {
			t.Fatalf("local configuration lost: %+v", local)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Local(ctx, s, "model", translate.Index); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
