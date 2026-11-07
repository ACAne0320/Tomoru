package shared

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"tomoru/internal/asr/batch"
	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/locales"
)

// TestModelNote says SenseVoice runs on the CPU, Qwen3-ASR on the GPU with what it needs, and a
// translation model what it needs; one this computer runs, that it does.
func TestModelNote(t *testing.T) {
	note := func(id string) string {
		m, _ := models.ByID(id)
		return ModelNote(m, models.Status{}, false)
	}
	sv, _ := models.ByID("sensevoice-small")
	for id, want := range map[string]string{
		"sensevoice-small": Size(sv.Size) + " · " + locales.ModelRunsOnCpu(),
		"qwen3-asr-1.7b":   OnWindows(locales.AsrNeedsMemory(4), locales.AsrNeedsVram(3)),
		"local-2b":         OnWindows(locales.NeedsMemory(8), locales.NeedsVram(4)),
	} {
		if got := note(id); !strings.HasSuffix(got, want) {
			t.Errorf("%s: %q, want it to end with %q", id, got, want)
		}
	}
	for id, want := range map[string]string{"sensevoice-small": locales.ModelRunsOnCpu(), "local-9b": locales.RunsHere()} {
		m, _ := models.ByID(id)
		if got := ModelNote(m, models.Status{}, true); !strings.HasSuffix(got, want) {
			t.Errorf("%s run: %q, want it to end with %q", id, got, want)
		}
	}
}

// TestEngineName names a model by its family and its size, whether the engine carries the model
// or only its ID, as the statistics and the history name it.
func TestEngineName(t *testing.T) {
	for id, want := range map[string]string{
		"local-9b":         "Index-Translate 9B",
		"sensevoice-small": "SenseVoice Small",
		"qwen3-asr-1.7b":   "Qwen3-ASR 1.7B",
	} {
		m, _ := models.ByID(id)
		if got := EngineName(core.Engine{ID: id, Model: &m}); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
		if got := EngineName(core.Engine{ID: id}); got != want {
			t.Errorf("%s by its ID: %q, want %q", id, got, want)
		}
	}
	if got := EngineName(core.Engine{ID: "cloud"}); got != CloudName {
		t.Errorf("the cloud: %q", got)
	}
}

func TestErrorText(t *testing.T) {
	for err, want := range map[error]string{
		nil:                                      "",
		&engine.Error{Code: "no-permission"}:     locales.StatusNoPermission(),
		&engine.Error{Code: "assets-failed"}:     locales.ErrAssets(),
		&engine.Error{Code: "new", Message: "m"}: locales.StatusStopped("m"),
		errors.New("x"):                          locales.StatusStopped("x"),
		batch.ErrBehind:                          locales.ErrAsrBehind(),
	} {
		if got := ErrorText(err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}

func TestLangChoices(t *testing.T) {
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	for locale, want := range map[string][]string{
		"en":      {"zh", "en", "ja", "ko", "", "fr", "de", "es"}, // French, German, Spanish
		"zh-Hans": {"zh", "en", "ja", "ko", "", "de", "fr", "es"}, // 德语, 法语, 西班牙语: by pinyin
	} {
		i18n.SetLocaleFunc(func() string { return locale })
		if got := LangChoices([]string{"zh", "en", "ja", "ko", "de", "es", "fr"}); !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", locale, got, want)
		}
	}
	if got := LangChoices([]string{"zh", "en"}); !slices.Equal(got, []string{"zh", "en"}) {
		t.Errorf("no others: %q", got)
	}
}
