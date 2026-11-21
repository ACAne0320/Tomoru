package lang

import (
	"slices"
	"unicode"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var All = []string{
	"zh", "en", "ja", "ko", "zh-Hant", "yue",
	"ar", "cs", "da", "de", "el", "es", "fa", "fi", "fil", "fr", "hi", "hu", "id", "it",
	"mk", "ms", "nl", "pl", "pt", "ro", "ru", "sv", "th", "tr", "vi",
}

const Auto = "auto"

func Choose(side, other *string, l string) {
	if l == *other {
		*other = *side
		if *other == Auto {
			*other = "en"
			if l == "en" {
				*other = "zh"
			}
		}
	}
	*side = l
}

var Featured = []string{"zh", "en", "ja", "ko"}

func Spoken() []string {
	return slices.DeleteFunc(slices.Clone(All), func(code string) bool { return code == "zh-Hant" })
}

func Known(code string) bool { return slices.Contains(All, code) }

func Of(locale string) string {
	t, err := language.Parse(locale)
	if err != nil {
		return ""
	}
	base, _ := t.Base()
	code := base.String()
	if script, _ := t.Script(); code == "zh" && script.String() == "Hant" {
		code = "zh-Hant"
	}
	if !Known(code) {
		return ""
	}
	return code
}

func Name(code, locale string) string {
	t, err := language.Parse(code)
	if err != nil {
		return code
	}
	if in, err := language.Parse(locale); err == nil {
		if n := display.Tags(in).Name(t); n != "" {
			return n
		}
	}
	if n := display.Self.Name(t); n != "" {
		return n
	}
	return code
}

func Locale(code string) string {
	t, err := language.Parse(code)
	if err != nil {
		return code
	}
	base, _ := t.Base()
	region, _ := t.Region()
	return base.String() + "-" + region.String()
}

func Spaced(code string) bool {
	switch script(code) {
	case "Hans", "Hant", "Jpan", "Thai", "Laoo", "Khmr", "Mymr", "Tibt":
		return false
	}
	return true
}

func CJK(code string) bool {
	switch script(code) {
	case "Hans", "Hant", "Jpan", "Kore":
		return true
	}
	return false
}

func Chinese(code string) bool {
	t, err := language.Parse(code)
	if err != nil {
		return false
	}
	base, _ := t.Base()
	return base.String() == "zh" || base.String() == "yue"
}

func Fits(text, code string) bool {
	var hangul, kana, han, other bool
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul = true
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			kana = true
		case unicode.Is(unicode.Han, r):
			han = true
		case unicode.IsLetter(r):
			other = true
		}
	}
	switch {
	case hangul:
		return code == "ko"
	case kana:
		return code == "ja"
	case han:
		return Chinese(code) || code == "ja"
	case other:
		return !CJK(code)
	}
	return true
}

func script(code string) string {
	t, err := language.Parse(code)
	if err != nil {
		return ""
	}
	s, _ := t.Script()
	return s.String()
}
