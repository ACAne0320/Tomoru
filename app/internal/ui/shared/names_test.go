package shared

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/i18n"

	"tomoru/internal/engine"
	"tomoru/locales"
)

func TestErrorText(t *testing.T) {
	for err, want := range map[error]string{
		nil:                                      "",
		&engine.Error{Code: "no-permission"}:     locales.StatusNoPermission(),
		&engine.Error{Code: "assets-failed"}:     locales.ErrAssets(),
		&engine.Error{Code: "new", Message: "m"}: locales.StatusStopped("m"),
		errors.New("x"):                          locales.StatusStopped("x"),
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
