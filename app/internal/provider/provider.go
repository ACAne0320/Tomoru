package provider

import (
	"context"

	"tomoru/internal/translate"
)

func Endpoint(url, model, key string, format *translate.Format) *translate.Endpoint {
	return &translate.Endpoint{URL: url, Model: model, APIKey: key, Format: format}
}

type Server interface {
	Ensure(context.Context, string) (string, error)
}

const LocalImageSide = 1280

type VisionServer interface {
	EnsureVision(context.Context, string) (string, error)
}

func Local(ctx context.Context, server Server, modelPath string, format *translate.Format) (*translate.Endpoint, error) {
	url, err := server.Ensure(ctx, modelPath)
	return local(url, modelPath, format, err)
}

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
