// Package asr is speech recognition's base: what a session runs to recognize what the engine
// hears, and the recognizers there are to choose, each an entry of Specs (docs/recognition.md).
// Adding a recognizer is adding an adapter and its entry: the catalog, the settings and the
// session read the entries.
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

// Recognizer recognizes the audio the engine hears, reporting as the engine does
// (docs/engine-protocol.md): a *batch.Runner of a decoder, a *qwencloud.Session or an
// *openai.Realtime. It decides itself where results are final.
type Recognizer interface {
	Audio(engine.Event) error
	Stop()
	Done() <-chan struct{}
	Err() error
}

// Factory starts a session's recognizer for the language lang, reporting its events to on.
type Factory func(lang string, on func(engine.Event)) (Recognizer, error)

// The cloud services' IDs.
const (
	QwenCloud = "qwen-cloud" // Qwen ASR on Alibaba Cloud Model Studio
	OpenAI    = "openai"     // OpenAI's realtime transcription, or its transcription API, or a service's compatible with it
)

// Spec is a recognizer to choose.
type Spec struct {
	ID string // as the settings name it
	// Cloud tells it runs on a server, which gets what is heard; else on this computer.
	Cloud bool
	// Billed tells the service bills the audio it is sent: it is sent only while somebody speaks.
	Billed bool
	// URL is the cloud service's address unless the user gives one, and Models the models to
	// choose, the default first: "" and nil for one on this computer.
	URL    string
	Models []string
	// Langs are the languages it recognizes, in the order of lang.All; nil for those the system
	// says (Apple's).
	Langs []string
	// Model is the model to download before it can be used, or nil.
	Model *models.Model
	// Key tells it needs an API key.
	Key bool
	// Hints tells it is told what to expect, its Env's Scene: a scene of other names wants a new
	// session.
	Hints bool
	// VolatileSentences tells the sentences its volatile text ends hold: they are committed at
	// once, without waiting for its final result.
	VolatileSentences bool
	// FinalsEnd tells its final results end utterances, where it heard the speaker stop: a service's.
	// A batch decoder's end where voice detection cut, which may be mid-sentence.
	FinalsEnd bool
	// Hint, for a recognizer in the capture engine, tells it when to make its result final, from
	// the engine's events, for speech in lang; nil for none.
	Hint func(lang string) func(engine.Event) (through float64, ok bool)
	// New makes its factory with env. A nil factory leaves recognition to the capture engine.
	New func(env Env) (Factory, error)
	// Test connects to the cloud service, as the settings test it: how long it took to answer. Nil
	// for one on this computer.
	Test func(ctx context.Context, svc Service) (time.Duration, error)
}

// Env is what a recognizer is made with: what the app provides, and the user's choices. New may
// wait on it (a model loading, credentials): it is called in the background.
type Env struct {
	Context context.Context
	// ASR is tomoru-asr's path, for a model it runs.
	ASR string
	// ModelDir is the directory of the model, downloaded; ModelPath its model file.
	ModelDir, ModelPath string
	// Server runs a model llama-server runs, apart from translation's: a *models.Server.
	Server Server
	// Service is the cloud service chosen, with its API key.
	Service Service
	// Scene is what the session knows of what it hears.
	Scene Scene
}

// Scene is what a session knows of what it hears: the names of its scene's glossary, as said, the
// user's own first, and the scene's description. A recognizer that takes hints gives them to its
// model as it can (docs/recognition.md, "Hints"); the others hear without them.
type Scene struct {
	Names []string
	About string
}

// Hints are a scene's names, and its description, as one text for a model that takes context:
// "<about>\n<name>, <name>…", at most max runes, names cut off past it.
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

// Server runs a model with its projector, which has it hear: a *models.Server.
type Server interface {
	EnsureVision(ctx context.Context, path string) (url string, err error)
}

// Service is a cloud service: its address ("" for the default), model ("" for the default) and API
// key.
type Service struct {
	URL, Model, Key string
}

// ErrLocal is a local model made without what runs it: tomoru-asr and the model's directory, or
// llama-server and the model's file.
var ErrLocal = errors.New("asr: a local model needs what runs it")

// Specs are the recognizers this computer can use, in the order to list them: the system's, then
// the models to download, then the cloud's.
func Specs() []Spec { return slices.Clone(specsOnce()) }

// ByID finds a recognizer of Specs.
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
			// Untimed: the sentences it ends while a stretch goes on commit from its volatile text.
			ss = append(ss, Spec{ID: m.ID, Langs: m.Langs, Model: m, VolatileSentences: true, Hints: true, New: served})
		default:
			ss = append(ss, Spec{ID: m.ID, Langs: m.Langs, Model: m, New: local})
		}
	}
	// Qwen's volatile sentence ends hold once qwencloud has steadied them.
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
	// OpenAI's streams its text as it hears, in turns the engine's voice detection cuts, whose
	// final results end where the speaker paused, maybe mid-sentence; with a model of the
	// transcription API, a compatible service's, it decodes stretches as they end, untimed, which
	// is more accurate, and later (docs/recognition.md, steps 5 and 6).
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
	// The cloud's: what each measures of its network, the session's subtitles say of it.
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

// served is a model llama-server decodes with, in stretches the batch runner cuts: loaded, and
// warmed, before the session starts.
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
	// The names alone: a description in the reader's language, Chinese for Japanese speech, had
	// the model answer in it ("アシナ" as "阿信").
	hints := Scene{Names: env.Scene.Names}.Hints(", ", llama.ContextLimit)
	return func(lang string, on func(engine.Event)) (Recognizer, error) {
		return batch.Start(llama.New(url, lang, hints), llama.Caps, on), nil
	}, nil
}

// local is a model tomoru-asr decodes with, in stretches the batch runner cuts.
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
