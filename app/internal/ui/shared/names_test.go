package shared

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"tomoru/internal/asr/batch"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/locales"
)

// TestModelNote says SenseVoice runs on the CPU, Qwen3-ASR on the GPU with what it needs, and a
// translation model what it needs.
func TestModelNote(t *testing.T) {
	note := func(id string) string {
		m, _ := models.ByID(id)
		return ModelNote(m, models.Status{}, locales.EngineSenseVoiceDescription())
	}
	sv, _ := models.ByID("sensevoice-small")
	for id, want := range map[string]string{
		"sensevoice-small": Size(sv.Size) + " · " + locales.EngineSenseVoiceDescription(),
		"qwen3-asr-1.7b":   OnWindows(locales.AsrNeedsMemory(4), locales.AsrNeedsVram(3)),
		"local-2b":         OnWindows(locales.NeedsMemory(8), locales.NeedsVram(4)),
	} {
		if got := note(id); !strings.HasSuffix(got, want) {
			t.Errorf("%s: %q, want it to end with %q", id, got, want)
		}
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
