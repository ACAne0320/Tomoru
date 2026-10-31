// Package provider constructs translators from resolved configuration (recognizers are asr's). Entry points own
// configuration, credentials, model availability and resource discovery.
package provider

import (
	"context"

	"tomoru/internal/translate"
)

// Endpoint constructs a remote translator. Credentials are supplied by the caller.
func Endpoint(url, model, key string, index bool) *translate.Endpoint {
	style := translate.Chat
	if index {
		style = translate.Index
	}
	return &translate.Endpoint{URL: url, Model: model, APIKey: key, Style: style}
}

// Server is the model-server capability needed for local translation.
type Server interface {
	Ensure(context.Context, string) (string, error)
}

// LocalImageSide is the longest side of a picture a local model reads: 1280 pixels take about 1200
// of its 4096 tokens of context, which 2048 would nearly fill.
const LocalImageSide = 1280

// VisionServer is the model-server capability needed for local picture translation.
type VisionServer interface {
	EnsureVision(context.Context, string) (string, error)
}

// Local prepares an already resolved model and constructs its translator.
func Local(ctx context.Context, server Server, modelPath string) (*translate.Endpoint, error) {
	return local(server.Ensure(ctx, modelPath))(modelPath)
}

// LocalVision prepares an already resolved model with its vision projector, to translate pictures.
func LocalVision(ctx context.Context, server VisionServer, modelPath string) (*translate.Endpoint, error) {
	return local(server.EnsureVision(ctx, modelPath))(modelPath)
}

func local(url string, err error) func(string) (*translate.Endpoint, error) {
	return func(modelPath string) (*translate.Endpoint, error) {
		if err != nil {
			return nil, err
		}
		e := Endpoint(url, modelPath, "", true)
		e.Llama, e.ImageSide = true, LocalImageSide
		return e, nil
	}
}
