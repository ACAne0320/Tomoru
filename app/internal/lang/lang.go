// Package lang is the languages Tomoru knows, by BCP 47 code: which they are, their names, and
// how they are written. What each engine recognizes is the engine's to say.
package lang

import (
	"slices"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// All are the languages to translate into, and among which recognition hears: the 30 that Qwen
// ASR recognizes, which take in the system's and SenseVoice's, and Traditional Chinese, which is
// only written. "zh" is Mandarin, written in Simplified Chinese.
var All = []string{
	"zh", "en", "ja", "ko", "zh-Hant", "yue",
	"ar", "cs", "da", "de", "el", "es", "fa", "fi", "fil", "fr", "hi", "hu", "id", "it",
	"mk", "ms", "nl", "pl", "pt", "ro", "ru", "sv", "th", "tr", "vi",
}

// Featured are the languages most watched, which pickers show first.
var Featured = []string{"zh", "en", "ja", "ko"}

// Spoken are the languages of All that may be heard: all but Traditional Chinese, a way of writing
// what is spoken as Mandarin or Cantonese.
func Spoken() []string {
	return slices.DeleteFunc(slices.Clone(All), func(code string) bool { return code == "zh-Hant" })
}

// Known tells whether code is one of All.
func Known(code string) bool { return slices.Contains(All, code) }

// Of is the language of All that a reader of locale ("zh-Hans", "pt-BR") reads, or "".
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

// Name is the name of a language in the language of locale: "日语" for "ja" in "zh-Hans". Where
// that language has no name for it, it is the language's own.
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

// Locale is a language with the region it is most spoken in, as recognizers and voices take it:
// "ja-JP" for "ja", "zh-TW" for "zh-Hant".
func Locale(code string) string {
	t, err := language.Parse(code)
	if err != nil {
		return code
	}
	base, _ := t.Base()
	region, _ := t.Region()
	return base.String() + "-" + region.String()
}

// Spaced tells whether the language puts spaces between its words: English and Korean do;
// Chinese, Japanese and Thai do not.
func Spaced(code string) bool {
	switch script(code) {
	case "Hans", "Hant", "Jpan", "Thai", "Laoo", "Khmr", "Mymr", "Tibt":
		return false
	}
	return true
}

// CJK tells whether the language is written in Chinese characters, kana or hangul.
func CJK(code string) bool {
	switch script(code) {
	case "Hans", "Hant", "Jpan", "Kore":
		return true
	}
	return false
}

// Chinese tells whether the language is Chinese: Mandarin, in either script, or Cantonese.
func Chinese(code string) bool {
	t, err := language.Parse(code)
	if err != nil {
		return false
	}
	base, _ := t.Base()
	return base.String() == "zh" || base.String() == "yue"
}

func script(code string) string {
	t, err := language.Parse(code)
	if err != nil {
		return ""
	}
	s, _ := t.Script()
	return s.String()
}
