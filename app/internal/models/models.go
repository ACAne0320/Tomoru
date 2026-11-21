package models

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Model struct {
	ID                    string
	Family, Label         string
	Dir                   string
	Files                 []File
	Size                  int64
	Repo, ModelScope, Rev string
	Needs                 struct{ Memory, VRAM uint64 }
	Rec                   bool
	ASR                   bool
	Runtime               string
	Langs                 []string
	Format                string
	Vision                string
	NoAbout               bool
}

func (m Model) Name() string { return m.Family + " " + m.Label }

func (m Model) Served() bool { return m.ASR && m.Runtime == "llama" }

func needs(memory, vram uint64) struct{ Memory, VRAM uint64 } {
	return struct{ Memory, VRAM uint64 }{memory << 30, vram << 30}
}

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
	{ID: "local-2b", Family: "Index-Translate", Label: "2B", Size: 1312164352 + 361518144, Repo: "IndexTeam/Index-Translate-2B-GGUF", Needs: needs(8, 4), NoAbout: true,
		Format: "index", Vision: "Index-Translate-2B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-2B.Q4_K_M.gguf", 1312164352, "044b313d29342bd3b2c77cbb64023ca0d209bd9b3247763f9d162767ef2d746a"},
			{"Index-Translate-2B.mmproj-Q8_0.gguf", 361518144, "b100ca29aaf437461d0b22ada7492d5d2dfeaa15959e329c4267598ddebe446d"},
		}},
	{ID: "local-9b", Family: "Index-Translate", Label: "9B", Size: 5780090304 + 620690272, Repo: "IndexTeam/Index-Translate-9B-GGUF", Needs: needs(16, 8), Rec: true,
		Format: "index", Vision: "Index-Translate-9B.mmproj-Q8_0.gguf",
		Files: []File{
			{"Index-Translate-9B.Q4_K_M.gguf", 5780090304, "9cc6758b0007f4cbf42ea3400768cbff91ae74ef8d640e467158348426d33713"},
			{"Index-Translate-9B.mmproj-Q8_0.gguf", 620690272, "0c654bbb219cea5dbb80656ec506e308c82b06aac7287f7636b9b6d06a44b957"},
		}},
	{ID: "hy-mt2-1.8b", Family: "Hy-MT2", Label: "1.8B", Size: 1133080448, Repo: "tencent/Hy-MT2-1.8B-GGUF", Needs: needs(8, 4),
		ModelScope: "Tencent-Hunyuan/Hy-MT2-1.8B-GGUF", Rev: "a0c709d9fac510f2c807aa3af52872340dc37a4a", Format: "hy-mt",
		Files: []File{
			{"Hy-MT2-1.8B-Q4_K_M.gguf", 1133080448, "dc5f44fcf1fa496ee7ad725982c0c8c553a4de00259b53af84c4b89fb0c06699"},
		}},
	{ID: "hy-mt2-7b", Family: "Hy-MT2", Label: "7B", Size: 4624648896, Repo: "tencent/Hy-MT2-7B-GGUF", Needs: needs(16, 8),
		ModelScope: "Tencent-Hunyuan/Hy-MT2-7B-GGUF", Rev: "ab8472660ac61fac25f1af43fac2599d52a8a775", Format: "hy-mt",
		Files: []File{
			{"Hy-MT2-7B-Q4_K_M.gguf", 4624648896, "9f96256500f3fc1ab4d64336b58f52a949a95ad7516b0c229476eef782f9f77b"},
		}},
	{ID: "sensevoice-small", Family: "SenseVoice", Label: "Small", Dir: "sensevoice-small-2024-07-17", Size: 239233841 + 315894, ASR: true, Langs: []string{"zh", "en", "ja", "ko", "yue"},
		Repo:       "csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17",
		Rev:        "2365baeacb507f821a0c8120fcee3d484dba7a07",
		ModelScope: "pengzhendong/sherpa-onnx-sense-voice-zh-en-ja-ko-yue",
		Files: []File{
			{"model.int8.onnx", 239233841, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"},
			{"tokens.txt", 315894, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"},
		}},
	{ID: "qwen3-asr-0.6b", Family: "Qwen3-ASR", Label: "0.6B", Dir: "qwen3-asr-0.6b", Size: 804749248 + 214392480, ASR: true, Runtime: "llama", Langs: Qwen3ASRLangs,
		Repo: "ggml-org/Qwen3-ASR-0.6B-GGUF", Rev: "928ab958557df9aa2ef1c93e0e83c7ad0933fae2", Needs: needs(2, 2),
		Vision: "mmproj-Qwen3-ASR-0.6B-Q8_0.gguf",
		Files: []File{
			{"Qwen3-ASR-0.6B-Q8_0.gguf", 804749248, "bca259818b50ca7c4c05e9bdb35a5dc04fa039653a6d6f3f0f331f96f6aa1971"},
			{"mmproj-Qwen3-ASR-0.6B-Q8_0.gguf", 214392480, "41a342b5e4c514e968cb756de6cd1b7be39eff43c44c57a2ef5fc6522e36603d"},
		}},
	{ID: "qwen3-asr-1.7b", Family: "Qwen3-ASR", Label: "1.7B", Dir: "qwen3-asr-1.7b", Size: 2165034944 + 355709344, ASR: true, Runtime: "llama", Langs: Qwen3ASRLangs,
		Repo: "ggml-org/Qwen3-ASR-1.7B-GGUF", Rev: "36a678687ba7d07a74ca70ccb0e36902e005fb80", Needs: needs(4, 3), Rec: true,
		Vision: "mmproj-Qwen3-ASR-1.7B-Q8_0.gguf",
		Files: []File{
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
