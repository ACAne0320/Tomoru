package asr

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"tomoru/internal/asr/apple"
	"tomoru/internal/asr/batch"
	"tomoru/internal/asr/llama"
	"tomoru/internal/asr/openai"
	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/asr/sensevoice"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/models"
)

type Recognizer interface {
	Audio(engine.Event) error
	Stop()
	Done() <-chan struct{}
	Err() error
}

type VoiceHearer interface {
	Voice(engine.Event)
}

type Factory func(lang string, on func(engine.Event)) (Recognizer, error)

const (
	QwenCloud = "qwen-cloud"
	OpenAI    = "openai"
)

type Spec struct {
	ID                string
	Cloud             bool
	Billed            bool
	URL               string
	Models            []string
	Langs             []string
	Auto              bool
	Model             *models.Model
	Key               bool
	Hints             bool
	VolatileSentences bool
	FinalsEnd         bool
	Hint              func(lang string) func(engine.Event) (through float64, ok bool)
	New               func(env Env) (Factory, error)
	Test              func(ctx context.Context, svc Service) (time.Duration, error)
}

type Env struct {
	Context             context.Context
	ASR                 string
	ModelDir, ModelPath string
	Server              Server
	Service             Service
	Scene               Scene
}

type Scene struct {
	Names []string
	About string
}

func (s Scene) Hints(sep string, max int) string {
	text := strings.TrimSpace(s.About)
	if len([]rune(text)) > max {
		text = string([]rune(text)[:max])
	}
	for i, n := range s.Names {
		add := n
		if i == 0 && text != "" {
			add = "\n" + n
		} else if i > 0 {
			add = sep + n
		}
		if len([]rune(text))+len([]rune(add)) > max {
			break
		}
		text += add
	}
	return text
}

type Server interface {
	EnsureVision(ctx context.Context, path string) (url string, err error)
}

type Service struct {
	URL, Model, Key string
}

var ErrLocal = errors.New("asr: a local model needs what runs it")

func Specs() []Spec { return slices.Clone(specsOnce()) }

func ByID(id string) (Spec, bool) {
	for _, s := range specsOnce() {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

var specsOnce = sync.OnceValue(func() []Spec { return specs(runtime.GOOS) })

func specs(goos string) []Spec {
	var ss []Spec
	if goos == "darwin" {
		ss = append(ss, Spec{ID: "apple", VolatileSentences: true, FinalsEnd: true, Hint: apple.Hint,
			New: func(Env) (Factory, error) { return nil, nil }})
	}
	for i := range models.All {
		m := &models.All[i]
		switch {
		case !m.ASR:
		case m.Served():
			ss = append(ss, Spec{ID: m.ID, Langs: m.Langs, Auto: true, Model: m, VolatileSentences: true, Hints: true, New: served})
		default:
			ss = append(ss, Spec{ID: m.ID, Langs: m.Langs, Auto: true, Model: m, New: local})
		}
	}
	ss = append(ss, Spec{ID: QwenCloud, Cloud: true, Billed: true, Langs: qwencloud.Langs, Key: true, Hints: true, VolatileSentences: true, FinalsEnd: true,
		URL: qwencloud.DefaultHost, Models: qwencloud.Models,
		New: func(env Env) (Factory, error) {
			cfg := qwen(env.Service)
			cfg.Vocabulary, cfg.Context = env.Scene.Names, env.Scene.Hints("、", qwencloud.ContextLimit)
			return func(lang string, on func(engine.Event)) (Recognizer, error) {
				return qwencloud.Start(cfg, lang, on), nil
			}, nil
		},
		Test: func(ctx context.Context, svc Service) (time.Duration, error) { return qwencloud.Test(ctx, qwen(svc)) }})
	ss = append(ss, Spec{ID: OpenAI, Cloud: true, Billed: true, Langs: lang.Spoken(), Key: true, Hints: true, VolatileSentences: true,
		URL: openai.DefaultURL, Models: append(slices.Clone(openai.RealtimeModels[:1]), openai.Models...),
		New: func(env Env) (Factory, error) {
			cfg := openAI(env.Service)
			cfg.Keywords, cfg.About = env.Scene.Names, env.Scene.About
			return func(lang string, on func(engine.Event)) (Recognizer, error) {
				if openai.Realtimed(cfg.Model) {
					return openai.StartRealtime(cfg, lang, on), nil
				}
				return batch.Start(openai.New(cfg, lang), openai.Caps, on), nil
			}, nil
		},
		Test: func(ctx context.Context, svc Service) (time.Duration, error) {
			if openai.Realtimed(svc.Model) {
				return openai.TestRealtime(ctx, openAI(svc))
			}
			return openai.Test(ctx, openAI(svc))
		}})
	for i := range ss {
		if ss[i].Cloud {
			ss[i].New = watched(ss[i].New)
		}
	}
	return ss
}

func qwen(svc Service) qwencloud.Config {
	return qwencloud.Config{Host: svc.URL, Model: svc.Model, Key: svc.Key}
}

func openAI(svc Service) openai.Config {
	return openai.Config{URL: svc.URL, Model: svc.Model, Key: svc.Key}
}

func served(env Env) (Factory, error) {
	if env.Server == nil || env.ModelPath == "" {
		return nil, ErrLocal
	}
	ctx := env.Context
	if ctx == nil {
		ctx = context.Background()
	}
	url, err := env.Server.EnsureVision(ctx, env.ModelPath)
	if err != nil {
		return nil, err
	}
	hints := Scene{Names: env.Scene.Names}.Hints(", ", llama.ContextLimit)
	return func(lang string, on func(engine.Event)) (Recognizer, error) {
		return batch.Start(llama.New(url, lang, hints), llama.Caps, on), nil
	}, nil
}

func local(env Env) (Factory, error) {
	if env.ASR == "" || env.ModelDir == "" {
		return nil, ErrLocal
	}
	return func(lang string, on func(engine.Event)) (Recognizer, error) {
		d, err := sensevoice.Start(env.ASR, env.ModelDir, lang)
		if err != nil {
			return nil, err
		}
		return batch.Start(d, sensevoice.Caps, on), nil
	}, nil
}
