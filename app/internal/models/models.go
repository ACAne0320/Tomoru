package models

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

type Model struct {
	ID                string
	Family, Label     string
	Dir               string
	Files             []File
	Older             []File
	Size              int64
	Repo, Rev         string
	ModelScope, MSRev string
	Needs             Needs
	Recommended       bool
	ASR               bool
	Runtime           Runtime
	Langs             []string
	Format            string
	Projector         string
	NoAbout           bool
}

func (m Model) Name() string { return m.Family + " " + m.Label }

func (m Model) Served() bool { return m.ASR && m.Runtime == RuntimeLlama }

func (m Model) MainFile() File { return m.Files[0] }

func (m Model) complete(f File, size int64) bool {
	return size == f.Size || slices.ContainsFunc(m.Older, func(o File) bool { return o.Name == f.Name && o.Size == size })
}

type Runtime string

const (
	RuntimeLlama  Runtime = "llama"
	RuntimeSherpa Runtime = "sherpa"
)

type Needs struct{ Memory, VRAM uint64 }

func needsGB(memory, vram uint64) Needs { return Needs{memory << 30, vram << 30} }

type MirrorOrder int

const (
	HuggingFaceFirst MirrorOrder = iota
	ModelScopeFirst
)

const spaceHeadroom = 256 << 20

type File struct {
	Name   string
	Size   int64
	SHA256 string
}

var Qwen3ASRLangs = []string{
	"zh", "en", "ja", "ko", "yue",
	"ar", "cs", "da", "de", "el", "es", "fa", "fi", "fil", "fr", "hi", "hu", "id", "it",
	"mk", "ms", "nl", "pl", "pt", "ro", "ru", "sv", "th", "tr", "vi",
}

var All = []Model{
	{ID: "local-2b", Family: "Index-Translate", Label: "2B", Size: 1312164352 + 361518144, Repo: "IndexTeam/Index-Translate-2B-GGUF", Rev: "3a0eca6ce931d88d00614d427ab3ed8ff0743902", MSRev: "b92a7c7bf499a6f05da1a66fab8549da3de5b603", Needs: needsGB(8, 4), NoAbout: true,
		Format: "index", Projector: "Index-Translate-2B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-2B.Q4_K_M.gguf", 1312164352, "044b313d29342bd3b2c77cbb64023ca0d209bd9b3247763f9d162767ef2d746a"},
			{"Index-Translate-2B.mmproj-Q8_0.gguf", 361518144, "b100ca29aaf437461d0b22ada7492d5d2dfeaa15959e329c4267598ddebe446d"},
		}},
	{ID: "local-9b", Family: "Index-Translate", Label: "9B", Size: 5780090304 + 620690272, Repo: "IndexTeam/Index-Translate-9B-GGUF", Rev: "645939fbab839a656f1bc39975a872bc8ad749f8", MSRev: "e97205e0e68986a643eab5ebfd75cebeb67234d0", Needs: needsGB(16, 8), Recommended: true,
		Format: "index", Projector: "Index-Translate-9B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-9B.Q4_K_M.gguf", 5780090304, "9cc6758b0007f4cbf42ea3400768cbff91ae74ef8d640e467158348426d33713"},
			{"Index-Translate-9B.mmproj-Q8_0.gguf", 620690272, "0c654bbb219cea5dbb80656ec506e308c82b06aac7287f7636b9b6d06a44b957"},
		}},
	{ID: "hy-mt2-1.8b", Family: "Hy-MT2", Label: "1.8B", Size: 1133080448, Repo: "tencent/Hy-MT2-1.8B-GGUF", Needs: needsGB(8, 4),
		ModelScope: "Tencent-Hunyuan/Hy-MT2-1.8B-GGUF", Rev: "a0c709d9fac510f2c807aa3af52872340dc37a4a", MSRev: "e97e6af0ec0950385644d7c0a8e5c899071f722b", Format: "hy-mt",
		Files: []File{
			{"Hy-MT2-1.8B-Q4_K_M.gguf", 1133080448, "dc5f44fcf1fa496ee7ad725982c0c8c553a4de00259b53af84c4b89fb0c06699"},
		}},
	{ID: "hy-mt2-7b", Family: "Hy-MT2", Label: "7B", Size: 4624648896, Repo: "tencent/Hy-MT2-7B-GGUF", Needs: needsGB(16, 8),
		ModelScope: "Tencent-Hunyuan/Hy-MT2-7B-GGUF", Rev: "ab8472660ac61fac25f1af43fac2599d52a8a775", MSRev: "47b1dd35c1f984e23ec7b3c72e5d01620dce8f40", Format: "hy-mt",
		Files: []File{
			{"Hy-MT2-7B-Q4_K_M.gguf", 4624648896, "9f96256500f3fc1ab4d64336b58f52a949a95ad7516b0c229476eef782f9f77b"},
		}},
	{ID: "sensevoice-small", Family: "SenseVoice", Label: "Small", Dir: "sensevoice-small-2024-07-17", Size: 239233841 + 315894, ASR: true, Runtime: RuntimeSherpa, Langs: []string{"zh", "en", "ja", "ko", "yue"},
		Repo:       "csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17",
		Rev:        "2365baeacb507f821a0c8120fcee3d484dba7a07",
		ModelScope: "pengzhendong/sherpa-onnx-sense-voice-zh-en-ja-ko-yue",
		MSRev:      "73eca47697f980daa3d16112404174b6b950b514",
		Files: []File{
			{"model.int8.onnx", 239233841, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"},
			{"tokens.txt", 315894, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"},
		}},
	{ID: "qwen3-asr-0.6b", Family: "Qwen3-ASR", Label: "0.6B", Dir: "qwen3-asr-0.6b", Size: 804749536 + 214392544, ASR: true, Runtime: RuntimeLlama, Langs: Qwen3ASRLangs,
		Repo: "ggml-org/Qwen3-ASR-0.6B-GGUF", Rev: "b98bf4381df3b400b929dcaff0dc83b6dafe221b", MSRev: "0d9f3fc743e072629872858883bca089035aff1e", Needs: needsGB(2, 2),
		Projector: "mmproj-Qwen3-ASR-0.6B-Q8_0.gguf",
		Files: []File{
			{"Qwen3-ASR-0.6B-Q8_0.gguf", 804749536, "05c9e10cecb37f6fc0cac0e786195dba82f12c15021f0b8a09473dbf3b6f090d"},
			{"mmproj-Qwen3-ASR-0.6B-Q8_0.gguf", 214392544, "3b5e371a03e7d14bc1b5a66f38ff3c5eec3a99b3f96deb72b3814da9adafb16d"},
		},
		Older: []File{
			{"Qwen3-ASR-0.6B-Q8_0.gguf", 804749248, "bca259818b50ca7c4c05e9bdb35a5dc04fa039653a6d6f3f0f331f96f6aa1971"},
			{"mmproj-Qwen3-ASR-0.6B-Q8_0.gguf", 214392480, "41a342b5e4c514e968cb756de6cd1b7be39eff43c44c57a2ef5fc6522e36603d"},
		}},
	{ID: "qwen3-asr-1.7b", Family: "Qwen3-ASR", Label: "1.7B", Dir: "qwen3-asr-1.7b", Size: 2165035232 + 355709376, ASR: true, Runtime: RuntimeLlama, Langs: Qwen3ASRLangs,
		Repo: "ggml-org/Qwen3-ASR-1.7B-GGUF", Rev: "3b2c29585c2180452ccb14d3b6b58127f352400d", MSRev: "cb3c7bffeb9af54ad88cf0e7a7168f25f6aa1827", Needs: needsGB(4, 3), Recommended: true,
		Projector: "mmproj-Qwen3-ASR-1.7B-Q8_0.gguf",
		Files: []File{
			{"Qwen3-ASR-1.7B-Q8_0.gguf", 2165035232, "cd5f5ce9d7c9c03ff47d54d7f2726e63a39e61e3001640c5d08b6e4d3aed1e6d"},
			{"mmproj-Qwen3-ASR-1.7B-Q8_0.gguf", 355709376, "24eaf9a96fa94fe36ed0c1de68d44cccdb3f2229ee614e5b1b5bab951bc37883"},
		},
		Older: []File{
			{"Qwen3-ASR-1.7B-Q8_0.gguf", 2165034944, "58e22d0532d4eacaf034cfac17a6fed159f37c41390c710186783be439d1fc57"},
			{"mmproj-Qwen3-ASR-1.7B-Q8_0.gguf", 355709344, "46c1d533af3f354ceb37ce855dbceff7da7fa7cf1e6a523df3b13440bd164c0d"},
		}},
}

func ByID(id string) (Model, bool) {
	for _, m := range All {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

func (m Model) URLs(f File, order MirrorOrder) []string {
	msRepo := m.ModelScope
	if msRepo == "" {
		msRepo = m.Repo
	}
	hf := "https://huggingface.co/" + m.Repo + "/resolve/" + m.Rev + "/" + f.Name
	ms := "https://modelscope.cn/models/" + msRepo + "/resolve/" + m.MSRev + "/" + f.Name
	if order == ModelScopeFirst {
		return []string{ms, hf}
	}
	return []string{hf, ms}
}

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
