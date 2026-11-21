package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/asr/llama"
	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/provider"
	"tomoru/internal/resource"
	"tomoru/internal/translate"
)

func modelsDir() (string, error) {
	if dir, err := os.UserConfigDir(); err == nil {
		if p, err := prefs.Load(filepath.Join(dir, "Tomoru", "settings.json"), ""); err == nil && p.ModelsDir != "" {
			return p.ModelsDir, nil
		}
	}
	return models.Dir()
}

func translator(mt, url, model string, roots ...string) (translate.Translator, func(), error) {
	id, cloudModel, _ := strings.Cut(mt, ":")
	if s, ok := core.MTServiceByID(id); ok {
		key := os.Getenv("TOKENHUB_API_KEY")
		if key == "" {
			return nil, nil, fmt.Errorf("-mt %s needs TOKENHUB_API_KEY", id)
		}
		return provider.Endpoint(cmp.Or(os.Getenv("TOKENHUB_BASE_URL"), s.URL), cmp.Or(cloudModel, s.Models[0]), key, s.Format), func() {}, nil
	}
	switch id {
	case "cloud":
		tr := translate.Cloud()
		tr.Model = cmp.Or(cloudModel, tr.Model)
		return tr, func() {}, nil
	case "chat":
		if url == "" || model == "" {
			return nil, nil, errors.New("-mt chat needs -mt-url and -mt-model")
		}
		return provider.Endpoint(url, model, os.Getenv("MT_API_KEY"), translate.Chat), func() {}, nil
	}
	m, ok := models.ByID(mt)
	format, known := translate.FormatByID(m.Format)
	if !ok || !known {
		return nil, nil, fmt.Errorf("-mt %s: not a translation", mt)
	}
	dir, err := modelsDir()
	if err != nil {
		return nil, nil, err
	}
	path := models.NewManager(dir, models.HuggingFaceFirst, nil).Path(mt)
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
	tr, err := provider.Local(ctx, srv, path, format)
	if err != nil {
		srv.Stop()
		return nil, nil, err
	}
	return tr, srv.Stop, nil
}

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
		dir, err := modelsDir()
		if err != nil {
			return spec, env, nil, err
		}
		manager := models.NewManager(dir, models.HuggingFaceFirst, nil)
		if manager.Status(id).State != models.Ready {
			return spec, env, nil, fmt.Errorf("-asr %s: not downloaded to %s", id, dir)
		}
		env.ModelDir, env.ModelPath = filepath.Join(dir, spec.Model.Dir), manager.Path(id)
		if spec.Model.Served() {
			bin, err := resource.Bin("llama-server", roots...)
			if err != nil {
				return spec, env, nil, err
			}
			srv := models.NewServer(bin, nil, models.WithProjectors(manager.ProjectorOf), models.WithWarm(llama.Warm))
			env.Server, stop = srv, srv.Stop
		} else if env.ASR, err = engine.ASRPath(roots...); err != nil {
			return spec, env, nil, err
		}
	}
	if spec.Key {
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
	if spec.InEngine {
		return spec, env, stop, nil
	}
	if err := preload(spec, env); err != nil {
		stop()
		return spec, env, nil, err
	}
	return spec, env, stop, nil
}

func preload(spec asr.Spec, env asr.Env) error {
	_, err := spec.New(env)
	return err
}
