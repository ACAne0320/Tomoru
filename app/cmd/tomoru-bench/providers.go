package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/asr/llama"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/provider"
	"tomoru/internal/resource"
	"tomoru/internal/translate"
)

// translator makes the translator of -mt, and what stops it at the end.
func translator(mt, url, model string, roots ...string) (translate.Translator, func(), error) {
	switch mt {
	case "cloud":
		return translate.Cloud(), func() {}, nil
	case "chat":
		if url == "" || model == "" {
			return nil, nil, errors.New("-mt chat needs -mt-url and -mt-model")
		}
		return provider.Endpoint(url, model, os.Getenv("MT_API_KEY"), false), func() {}, nil
	}
	if _, ok := models.ByID(mt); !ok {
		return nil, nil, fmt.Errorf("-mt %s: not a translation", mt)
	}
	dir, err := models.Dir()
	if err != nil {
		return nil, nil, err
	}
	path := models.NewManager(dir, false, nil).Path(mt)
	if _, err := os.Stat(path); err != nil {
		return nil, nil, fmt.Errorf("-mt %s: %w", mt, err)
	}
	bin, err := resource.Bin("llama-server", roots...)
	if err != nil {
		return nil, nil, err
	}
	srv := models.NewServer(bin, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tr, err := provider.Local(ctx, srv, path)
	if err != nil {
		srv.Stop()
		return nil, nil, err
	}
	return tr, srv.Stop, nil
}

// recognizer is the recognizer of -asr: its entry, what it is made with but the scene, which each
// case has its own of, and what stops it at the end.
func recognizer(name string, roots ...string) (asr.Spec, asr.Env, func(), error) {
	id, model, _ := strings.Cut(name, ":")
	switch id {
	case "sensevoice":
		id = "sensevoice-small"
	}
	spec, ok := asr.ByID(id)
	if !ok {
		return spec, asr.Env{}, nil, fmt.Errorf("-asr %s: not a recognizer", name)
	}
	env, stop := asr.Env{Context: context.Background()}, func() {}
	if spec.Model != nil {
		dir, err := models.Dir()
		if err != nil {
			return spec, env, nil, err
		}
		manager := models.NewManager(dir, false, nil)
		if manager.Status(id).State != models.Ready {
			return spec, env, nil, fmt.Errorf("-asr %s: not downloaded to %s", id, dir)
		}
		env.ModelDir, env.ModelPath = filepath.Join(dir, spec.Model.Dir), manager.Path(id)
		if spec.Model.Runtime == "llama" {
			bin, err := resource.Bin("llama-server", roots...)
			if err != nil {
				return spec, env, nil, err
			}
			srv := models.NewServer(bin, nil)
			srv.Vision, srv.Warm = manager.VisionOf, llama.Warm
			env.Server, stop = srv, srv.Stop
		} else if env.ASR, err = engine.ASRPath(roots...); err != nil {
			return spec, env, nil, err
		}
	}
	if spec.Key {
		// The service's key and address, from the environment: DASHSCOPE_API_KEY and
		// DASHSCOPE_API_HOST for Qwen's, OPENAI_API_KEY and OPENAI_BASE_URL for OpenAI's.
		keyVar, urlVar := "DASHSCOPE_API_KEY", "DASHSCOPE_API_HOST"
		if id == asr.OpenAI {
			keyVar, urlVar = "OPENAI_API_KEY", "OPENAI_BASE_URL"
		}
		key := os.Getenv(keyVar)
		if key == "" {
			return spec, env, nil, fmt.Errorf("-asr %s needs %s", id, keyVar)
		}
		env.Service = asr.Service{URL: os.Getenv(urlVar), Model: model, Key: key}
	}
	// Made once, to fail at once: a model llama-server runs loads now, for the cases after.
	if _, err := spec.New(env); err != nil {
		stop()
		return spec, env, nil, err
	}
	return spec, env, stop, nil
}
