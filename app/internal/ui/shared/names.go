package shared

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"tomoru/internal/asr/batch"
	"tomoru/internal/asr/openai"
	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/proxy"
	"tomoru/internal/scene"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

func DownloadError(err error) string {
	var space models.ErrNoSpace
	switch {
	case err == nil:
		return ""
	case errors.Is(err, models.ErrChecksum):
		return locales.DownloadDamaged()
	case errors.Is(err, proxy.ErrInvalid):
		return locales.ErrProxyInvalid()
	case errors.As(err, &space):
		return locales.NoSpace(Size(space.Need))
	}
	return locales.DownloadFailed(err.Error())
}

func LangName(code string) string {
	if code == lang.Auto {
		return locales.AutoDetect()
	}
	return lang.Name(code, i18n.Locale())
}

func LangChoices(codes []string) [][]string {
	var auto, first, rest []string
	for _, code := range codes {
		if code == lang.Auto {
			auto = []string{code}
		} else if slices.Contains(lang.Featured, code) {
			first = append(first, code)
		} else {
			rest = append(rest, code)
		}
	}
	col := collate.New(language.Make(i18n.Locale()))
	slices.SortFunc(rest, func(a, b string) int { return col.CompareString(LangName(a), LangName(b)) })
	var groups [][]string
	for _, g := range [][]string{auto, first, rest} {
		if len(g) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

func FontName(font prefs.Font) string {
	switch font {
	case prefs.FontSystem:
		return locales.FontSystem()
	case prefs.FontSerif:
		return locales.FontSerif()
	case prefs.FontRounded:
		return locales.FontRounded()
	}
	return string(font)
}

const CloudName = "Index-Translate 35B"

func ModelNote(m models.Model, st models.Status, runs bool) string {
	var needs string
	switch mem, vram := int(m.Needs.Memory>>30), int(m.Needs.VRAM>>30); {
	case m.Needs.Memory == 0:
		needs = locales.ModelRunsOnCpu()
	case runs:
		needs = locales.RunsHere()
	case m.ASR:
		needs = MacOr(locales.AsrNeedsMemory(mem), locales.AsrNeedsVram(vram))
	default:
		needs = MacOr(locales.NeedsMemory(mem), locales.NeedsVram(vram))
	}
	size := Size(m.Size)
	if st.State == models.Missing && st.Done > 0 {
		size = locales.Downloading(Size(st.Done), size)
	}
	return size + " · " + needs
}

func SizeNote(m models.Model) string {
	switch m.ID {
	case "local-2b", "hy-mt2-1.8b", "qwen3-asr-0.6b":
		return locales.SizeQuicker()
	case "local-9b", "qwen3-asr-1.7b":
		return locales.SizeMoreAccurate()
	}
	return ""
}

func FamilyName(f []core.Engine) string {
	if m := f[0].Model; m != nil {
		return m.Family
	}
	if f[0].Family() == "cloud" {
		return locales.EngineIndexCloud()
	}
	return EngineName(f[0])
}

func FamilyNote(f []core.Engine) string {
	e := f[0]
	switch e.Family() {
	case "apple":
		return locales.EngineAppleDescription()
	case "Oruk Fourier":
		return locales.EngineOrukFourierDescription()
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

func EngineName(e core.Engine) string {
	switch {
	case e.ID == "":
		return locales.EngineNone()
	case e.Endpoint != nil:
		return e.Endpoint.Name
	case prefs.IsEndpoint(e.ID):
		return locales.EngineDeleted()
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
	case errors.Is(err, proxy.ErrInvalid):
		return locales.ErrProxyInvalid()
	}
	if why := asrCloudError(err); why != "" {
		return why
	}
	switch {
	case errors.Is(err, core.ErrNoEndpoint):
		return locales.ErrNoEndpoint()
	case errors.Is(err, core.ErrNoVision):
		return locales.ErrNoVision()
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

type Fix int

const (
	FixRetry Fix = iota
	FixEngines
	FixAccess
)

func ErrorFix(err error) Fix {
	var e *engine.Error
	switch {
	case core.IsExited(err):
		return FixRetry
	case errors.Is(err, core.ErrModelMissing), errors.Is(err, core.ErrASRMissing), errors.Is(err, core.ErrNoASR),
		errors.Is(err, core.ErrASRCloudKey), errors.Is(err, core.ErrMTCloudKey), errors.Is(err, core.ErrNoEndpoint), errors.Is(err, core.ErrNoVision),
		errors.Is(err, qwencloud.ErrAuth), errors.Is(err, openai.ErrAuth), errors.Is(err, batch.ErrBehind):
		return FixEngines
	case errors.As(err, &e) && e.Code == "no-permission":
		return FixAccess
	case errors.As(err, &e) && e.Code == "unsupported-locale":
		return FixEngines
	}
	return FixRetry
}

func FixButton(c *ui.Context, k *core.Core, err error, engines func()) {
	icon, label, do := assets.Reset, locales.Retry(), k.Start
	switch ErrorFix(err) {
	case FixEngines:
		icon, label, do = assets.Languages, locales.OpenEngines(), engines
	case FixAccess:
		icon, label, do = assets.Settings, locales.OpenSystemSettings(), kit.OpenScreenSettings
	}
	if kit.Button(c, kit.Plain, kit.Small, icon, label).Clicked() {
		do()
	}
}

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

func Size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f\u00a0GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f\u00a0MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d\u00a0KB", n>>10)
}

func SceneName(s scene.Scene) string {
	switch {
	case s.IsGeneral():
		return locales.SceneGeneral()
	case strings.TrimSpace(s.Name) == "":
		return locales.NewScene()
	}
	return s.Name
}
