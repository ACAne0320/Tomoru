// Package provider constructs translators from resolved configuration (recognizers are asr's). Entry points own
// configuration, credentials, model availability and resource discovery.
package provider

import (
	"context"

	"tomoru/internal/translate"
)

// Endpoint constructs a remote translator, prompted in format. Credentials are supplied by the
// caller.
func Endpoint(url, model, key string, format *translate.Format) *translate.Endpoint {
	return &translate.Endpoint{URL: url, Model: model, APIKey: key, Format: format}
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

// Local prepares an already resolved model and constructs its translator, prompted in its format.
func Local(ctx context.Context, server Server, modelPath string, format *translate.Format) (*translate.Endpoint, error) {
	url, err := server.Ensure(ctx, modelPath)
	return local(url, modelPath, format, err)
}

// LocalVision prepares an already resolved model with its vision projector, to translate pictures.
func LocalVision(ctx context.Context, server VisionServer, modelPath string, format *translate.Format) (*translate.Endpoint, error) {
	url, err := server.EnsureVision(ctx, modelPath)
	return local(url, modelPath, format, err)
}

func local(url, modelPath string, format *translate.Format, err error) (*translate.Endpoint, error) {
	if err != nil {
		return nil, err
	}
	e := Endpoint(url, modelPath, "", format)
	e.Llama, e.ImageSide = true, LocalImageSide
	return e, nil
}
