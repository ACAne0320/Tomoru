// Package models keeps the models that run on this computer, of translation and of recognition:
// what they are, where they are downloaded to, downloading them, and llama-server, which runs the
// translation models.
package models

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Model is a model to download: a file, or a few in a directory of its own.
type Model struct {
	ID    string // as the settings name the engine
	Dir   string // its directory among the models, "" for none
	Files []File // the first is the model itself, at Manager.Path
	Size  int64  // of all its files
	// Repo is the model's repository, by the same name on Hugging Face and ModelScope unless
	// ModelScope names another. Rev pins its revision on Hugging Face, "main" when "".
	Repo, ModelScope, Rev string
	// Memory is the least memory, in bytes, it runs well with; 0 for any. A Mac has less, and is
	// told so.
	Memory uint64
	// Needs is what a translation model is said to need to run, in bytes: memory on a Mac, whose
	// GPU shares it, and the GPU's own memory on Windows, where llama.cpp runs it on Vulkan.
	// Estimates from the file's size and its context, not measured on every machine.
	Needs struct{ Memory, VRAM uint64 }
	// ASR tells it recognizes speech; else it translates, with llama-server.
	ASR bool
	// Runtime is what runs a recognition model: "" tomoru-asr (sherpa-onnx), "llama" llama-server,
	// which hears through the model's audio projector, its Vision file.
	Runtime string
	// Langs are the languages a recognition model hears, in the order of lang.All.
	Langs []string
	// Vision is the file among Files with which llama-server has a translation model read pictures
	// (docs/image-translation.md), or a recognition model hear; "" for none.
	Vision string
	// NoAbout leaves the scene's description out of a translation model's prompts: given it, the
	// 2B model put the names in it where none was said (docs/scenes.md).
	NoAbout bool
}

// needs is a model's Needs: memory and the GPU's memory, in GB.
func needs(memory, vram uint64) struct{ Memory, VRAM uint64 } {
	return struct{ Memory, VRAM uint64 }{memory << 30, vram << 30}
}

// File is a file of a model, by its name in the repository and in the model's directory.
type File struct {
	Name   string
	Size   int64
	SHA256 string
}

// Qwen3ASRLangs are the languages Qwen3-ASR recognizes, as its model card lists them, in the order
// of lang.All.
var Qwen3ASRLangs = []string{
	"zh", "en", "ja", "ko", "yue",
	"ar", "cs", "da", "de", "el", "es", "fa", "fi", "fil", "fr", "hi", "hu", "id", "it",
	"mk", "ms", "nl", "pl", "pt", "ro", "ru", "sv", "th", "tr", "vi",
}

// All are the models. The translation models are from the official Index-Translate repositories
// (Q4_K_M: in our tests, the same mistakes as Q8_0 at less than two thirds of the size), each with
// its vision projector (Q8_0), so that it translates pictures too. The recognition models are
// SenseVoice as sherpa-onnx publishes it, the 2024-07-17 release: the 2025-09-09 one recognizes
// everything as Cantonese with sherpa-onnx 1.13.8 (docs/local-asr.md); and Qwen3-ASR 1.7B as
// llama.cpp's own (ggml-org) publish it, Q8_0 (bf16 recognized the same; docs/recognition.md),
// which needs a GPU: 2.9–3.1 GB of memory on an M5, 2.5 GB of VRAM on an RTX 5060.
var All = []Model{
	{ID: "local-2b", Size: 1312164352 + 361518144, Repo: "IndexTeam/Index-Translate-2B-GGUF", Needs: needs(8, 4), NoAbout: true,
		Vision: "Index-Translate-2B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-2B.Q4_K_M.gguf", 1312164352, "044b313d29342bd3b2c77cbb64023ca0d209bd9b3247763f9d162767ef2d746a"},
			{"Index-Translate-2B.mmproj-Q8_0.gguf", 361518144, "b100ca29aaf437461d0b22ada7492d5d2dfeaa15959e329c4267598ddebe446d"},
		}},
	{ID: "local-9b", Size: 5780090304 + 620690272, Repo: "IndexTeam/Index-Translate-9B-GGUF", Memory: 16 << 30, Needs: needs(16, 8),
		Vision: "Index-Translate-9B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-9B.Q4_K_M.gguf", 5780090304, "9cc6758b0007f4cbf42ea3400768cbff91ae74ef8d640e467158348426d33713"},
			{"Index-Translate-9B.mmproj-Q8_0.gguf", 620690272, "0c654bbb219cea5dbb80656ec506e308c82b06aac7287f7636b9b6d06a44b957"},
		}},
	{ID: "sensevoice-small", Dir: "sensevoice-small-2024-07-17", Size: 239233841 + 315894, ASR: true, Langs: []string{"zh", "en", "ja", "ko", "yue"},
		Repo:       "csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17",
		Rev:        "2365baeacb507f821a0c8120fcee3d484dba7a07",
		ModelScope: "pengzhendong/sherpa-onnx-sense-voice-zh-en-ja-ko-yue",
		Files: []File{
			{"model.int8.onnx", 239233841, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"},
			{"tokens.txt", 315894, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"},
		}},
	{ID: "qwen3-asr-1.7b", Dir: "qwen3-asr-1.7b", Size: 2165034944 + 355709344, ASR: true, Runtime: "llama", Langs: Qwen3ASRLangs,
		Repo: "ggml-org/Qwen3-ASR-1.7B-GGUF", Rev: "36a678687ba7d07a74ca70ccb0e36902e005fb80", Needs: needs(4, 3),
		Vision: "mmproj-Qwen3-ASR-1.7B-Q8_0.gguf",
		Files: []File{
			{"Qwen3-ASR-1.7B-Q8_0.gguf", 2165034944, "58e22d0532d4eacaf034cfac17a6fed159f37c41390c710186783be439d1fc57"},
			{"mmproj-Qwen3-ASR-1.7B-Q8_0.gguf", 355709344, "46c1d533af3f354ceb37ce855dbceff7da7fa7cf1e6a523df3b13440bd164c0d"},
		}},
}

// ByID finds a model.
func ByID(id string) (Model, bool) {
	for _, m := range All {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// URLs are where to download a file of m from, in the order to try: ModelScope first for a reader
// in China, where Hugging Face is slow or out of reach, else Hugging Face first.
func (m Model) URLs(f File, china bool) []string {
	rev, msRepo := m.Rev, m.ModelScope
	if rev == "" {
		rev = "main"
	}
	if msRepo == "" {
		msRepo = m.Repo
	}
	hf := "https://huggingface.co/" + m.Repo + "/resolve/" + rev + "/" + f.Name
	ms := "https://modelscope.cn/models/" + msRepo + "/resolve/master/" + f.Name
	if china {
		return []string{ms, hf}
	}
	return []string{hf, ms}
}

// Dir is where models are kept: the user's data, not the app, which updates replace whole, nor the
// cache, which cleaners empty. Not MyGo's user data directory either: on Windows that is the
// roaming profile, on Linux ~/.config.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("LocalAppData")
		if base == "" {
			return "", errors.New("models: no LocalAppData")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "Tomoru", "models"), nil
}
