package shared

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"tomoru/internal/asr/batch"
	"tomoru/internal/asr/openai"
	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/models"
	"tomoru/internal/scene"
	"tomoru/locales"
)

// DownloadError says why a model's download failed, or "".
func DownloadError(err error) string {
	var space models.ErrNoSpace
	switch {
	case err == nil:
		return ""
	case errors.Is(err, models.ErrChecksum):
		return locales.DownloadDamaged()
	case errors.As(err, &space):
		return locales.NoSpace(Size(space.Need))
	}
	return locales.DownloadFailed(err.Error())
}

// LangName is a language's name in the language the app shows.
func LangName(code string) string { return lang.Name(code, i18n.Locale()) }

// LangChoices are languages as a picker lists them: those most watched first, then the others by
// name, after a separator ("", as kit.Choices takes it).
func LangChoices(codes []string) []string {
	var first, rest []string
	for _, code := range codes {
		if slices.Contains(lang.Featured, code) {
			first = append(first, code)
		} else {
			rest = append(rest, code)
		}
	}
	col := collate.New(language.Make(i18n.Locale()))
	slices.SortFunc(rest, func(a, b string) int { return col.CompareString(LangName(a), LangName(b)) })
	if len(first) > 0 && len(rest) > 0 {
		first = append(first, "")
	}
	return append(first, rest...)
}

// FontName is a font's name, by its key of prefs.Fonts.
func FontName(key string) string {
	switch key {
	case "serif":
		return locales.FontSerif()
	case "rounded":
		return locales.FontRounded()
	}
	return locales.FontSystem()
}

// CloudName is Index-Translate's cloud, 35B, as the app names it everywhere: by its model's size,
// as the local ones are. It is free and the default, and in the cloud alone, so the cloud goes
// without saying; its other sizes say it (locales.EngineCloudSize), as there are local ones.
const CloudName = "Index-Translate 35B"

// ModelNote is what a model's row says of it: its size, or how much of it is in when st is a
// download stopped before, then what it needs to run: nothing but the CPU; that this computer
// runs it, as core.Runs knows; else, on the GPU, its memory (a Mac's, the GPU's on Windows).
func ModelNote(m models.Model, st models.Status, runs bool) string {
	var needs string
	switch mem, vram := int(m.Needs.Memory>>30), int(m.Needs.VRAM>>30); {
	case m.Needs.Memory == 0:
		needs = locales.ModelRunsOnCpu()
	case runs:
		needs = locales.RunsHere()
	case m.ASR: // said apart from those that run on the CPU
		needs = OnWindows(locales.AsrNeedsMemory(mem), locales.AsrNeedsVram(vram))
	default:
		needs = OnWindows(locales.NeedsMemory(mem), locales.NeedsVram(vram))
	}
	size := Size(m.Size)
	if st.State == models.Missing && st.Done > 0 {
		size = locales.Downloading(Size(st.Done), size)
	}
	return size + " · " + needs
}

// SizeNote says what a model's size is for, against its family's other: quicker or more
// accurate, as measured (bench/README.md, docs/recognition.md). "" where the other measured no
// better.
func SizeNote(m models.Model) string {
	switch m.ID {
	case "local-2b", "hy-mt2-1.8b", "qwen3-asr-0.6b":
		return locales.SizeQuicker()
	case "local-9b", "qwen3-asr-1.7b":
		return locales.SizeMoreAccurate()
	}
	return ""
}

// FamilyName is a family of engines' name, as core.Families gives it: its model's, without a
// size; else its one engine's.
func FamilyName(f []core.Engine) string {
	if m := f[0].Model; m != nil {
		return m.Family
	}
	if f[0].Family() == "cloud" {
		return locales.EngineIndexCloud()
	}
	return EngineName(f[0])
}

// FamilyNote says what is worth knowing of a family of engines before using it, in a line: what
// it costs, or how the system's does. "" for the others: their sizes say the rest.
func FamilyNote(f []core.Engine) string {
	e := f[0]
	switch e.Family() {
	case "apple":
		return locales.EngineAppleDescription()
	case "cloud":
		return locales.EngineCloudThanks()
	case core.QwenCloud:
		return locales.EngineQwenAsrDescription()
	case core.OpenAI:
		return locales.EngineOpenAiDescription()
	case core.HyMTCloud:
		return locales.EngineHyMtCloudDescription()
	}
	return ""
}

// EngineName is an engine's name: a model's, its family's and its size.
func EngineName(e core.Engine) string {
	switch {
	case e.ID == "":
		return locales.EngineNone()
	case e.Endpoint != nil:
		return e.Endpoint.Name
	case e.ID == "apple":
		return locales.EngineApple()
	case e.ID == "cloud":
		return CloudName
	case e.Family() == "cloud":
		size, _ := core.CloudSizeByID(e.ID)
		return locales.EngineCloudSize(size.Label)
	case e.ID == core.QwenCloud:
		return locales.EngineQwenAsr()
	case e.ID == core.OpenAI:
		return "OpenAI"
	case e.ID == core.HyMTCloud:
		return locales.EngineHyMtCloud()
	}
	if m, ok := models.ByID(e.ID); ok {
		return m.Name()
	}
	return e.ID
}

// ErrorText says why translating stopped, or "" for no error.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	if core.IsExited(err) {
		return locales.ErrEngineExited()
	}
	switch {
	case errors.Is(err, core.ErrModelMissing):
		return locales.ErrModelMissing()
	case errors.Is(err, core.ErrModelsMoving):
		return locales.ErrModelsMoving()
	case errors.Is(err, core.ErrModelBusy):
		return locales.ErrModelBusy()
	case errors.Is(err, core.ErrNoASR):
		return locales.ErrNoAsr()
	case errors.Is(err, core.ErrASRMissing):
		return locales.ErrAsrModelMissing()
	case errors.Is(err, batch.ErrBehind):
		return locales.ErrAsrBehind()
	}
	if why := asrCloudError(err); why != "" {
		return why
	}
	switch {
	case errors.Is(err, core.ErrNoEndpoint):
		return locales.ErrNoEndpoint()
	case errors.Is(err, core.ErrMTCloudKey):
		return locales.ErrMtCloudKey()
	case errors.As(err, new(core.LocalError)):
		return locales.ErrLocal()
	}
	var e *engine.Error
	if !errors.As(err, &e) {
		return locales.StatusStopped(err.Error())
	}
	switch e.Code {
	case "no-permission":
		return locales.StatusNoPermission()
	case "unsupported-locale":
		return locales.ErrUnsupportedLocale()
	case "assets-failed":
		return locales.ErrAssets()
	case "capture-failed":
		return locales.ErrCapture()
	}
	return locales.StatusStopped(e.Message)
}

// asrCloudError says why a cloud recognition service failed, or "" for an error not its own.
func asrCloudError(err error) string {
	var qe *qwencloud.Error
	var oe *openai.Error
	switch {
	case errors.Is(err, core.ErrASRCloudKey):
		return locales.ErrAsrCloudKey()
	case errors.Is(err, qwencloud.ErrAuth):
		return locales.ErrAsrCloudAuth()
	case errors.Is(err, openai.ErrAuth):
		return locales.ErrAsrApiAuth()
	case errors.As(err, &qe):
		return locales.ErrAsrCloud(qe.Message)
	case errors.As(err, &oe):
		return locales.ErrAsrCloud(oe.Message)
	}
	return ""
}

// Size is a size in bytes, as "1.3 GB", the number held to its unit by a no-break space.
func Size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f\u00a0GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f\u00a0MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d\u00a0KB", n>>10)
}

// SceneName is a scene's name as shown: the general one's, or a new one's until it is named.
func SceneName(s scene.Scene) string {
	switch {
	case s.ID == "":
		return locales.SceneGeneral()
	case strings.TrimSpace(s.Name) == "":
		return locales.NewScene()
	}
	return s.Name
}
