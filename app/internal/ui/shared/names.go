package shared

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"

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

// CloudName is Index-Translate's cloud as the app names it everywhere: by its model's size, as the
// local ones are. It is free and the default, so the cloud goes without saying.
const CloudName = "Index-Translate 35B"

// ModelNote is what a model's row says of it: its size, or how much of it is in when st is a
// download stopped before, then, for a translation model, what it needs to run (memory on a Mac,
// the GPU's on Windows), else desc.
func ModelNote(m models.Model, st models.Status, desc string) string {
	if m.Needs.Memory != 0 {
		desc = OnWindows(locales.NeedsMemory(int(m.Needs.Memory>>30)), locales.NeedsVram(int(m.Needs.VRAM>>30)))
	}
	size := Size(m.Size)
	if st.State == models.Missing && st.Done > 0 {
		size = locales.Downloading(Size(st.Done), size)
	}
	return size + " · " + desc
}

// CloudSends is what the cloud's services of kind are sent: the sound for recognition, the text
// recognized for translation. It shows under them, once for all.
func CloudSends(kind core.Kind) string {
	if kind == core.ASR {
		return locales.CloudSendsAudio()
	}
	return locales.CloudSendsText()
}

// EngineName is an engine's name.
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
	case e.ID == "local-2b":
		return "Index-Translate 2B"
	case e.ID == "local-9b":
		return "Index-Translate 9B"
	case e.ID == "sensevoice-small":
		return "SenseVoice Small"
	case e.ID == "qwen3-asr-1.7b":
		return "Qwen3-ASR 1.7B"
	case e.ID == core.QwenCloud:
		return locales.EngineQwenAsr()
	case e.ID == core.OpenAI:
		return "OpenAI"
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
	case errors.Is(err, core.ErrNoASR):
		return locales.ErrNoAsr()
	case errors.Is(err, core.ErrASRMissing):
		return locales.ErrAsrModelMissing()
	}
	if why := asrCloudError(err); why != "" {
		return why
	}
	switch {
	case errors.Is(err, core.ErrNoEndpoint):
		return locales.ErrNoEndpoint()
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

// Size is a size in bytes, as "1.3 GB".
func Size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", n>>10)
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
