package lang

import "testing"

func TestNames(t *testing.T) {
	for _, tc := range []struct{ code, locale, want string }{
		{"zh", "zh-Hans", "中文"}, {"en", "zh-Hans", "英语"}, {"ja", "zh-Hans", "日语"}, {"ko", "zh-Hans", "韩语"},
		{"zh", "en", "Chinese"}, {"ja", "ja", "日本語"}, {"ko", "ko", "한국어"},
		{"zh-Hans", "en", "Simplified Chinese"}, {"zh-Hant", "zh-Hans", "繁体中文"}, {"yue", "zh-Hans", "粤语"},
		{"zh-Hant", "ko", "繁體中文"},
		{"xx-!", "en", "xx-!"},
	} {
		if got := Name(tc.code, tc.locale); got != tc.want {
			t.Errorf("Name(%q, %q) = %q, want %q", tc.code, tc.locale, got, tc.want)
		}
	}
	for _, code := range All {
		for _, locale := range []string{"en", "ja", "ko", "zh-Hans", "zh-Hant"} {
			if Name(code, locale) == code {
				t.Errorf("%s has no name in %s", code, locale)
			}
		}
	}
}

func TestOf(t *testing.T) {
	for locale, want := range map[string]string{"zh-Hans": "zh", "zh-Hant": "zh-Hant", "zh-TW": "zh-Hant", "zh-HK": "zh-Hant",
		"en-GB": "en", "pt-BR": "pt", "ja": "ja", "sw": "", "": ""} {
		if got := Of(locale); got != want {
			t.Errorf("Of(%q) = %q, want %q", locale, got, want)
		}
	}
}

func TestLocale(t *testing.T) {
	for code, want := range map[string]string{"zh": "zh-CN", "ja": "ja-JP", "en": "en-US", "ko": "ko-KR", "zh-Hant": "zh-TW", "pt": "pt-BR"} {
		if got := Locale(code); got != want {
			t.Errorf("Locale(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestWriting(t *testing.T) {
	for _, tc := range []struct {
		code                 string
		spaced, cjk, chinese bool
	}{
		{"zh", false, true, true}, {"zh-Hant", false, true, true}, {"yue", false, true, true},
		{"ja", false, true, false}, {"ko", true, true, false}, {"th", false, false, false},
		{"en", true, false, false}, {"ru", true, false, false}, {"ar", true, false, false},
	} {
		if Spaced(tc.code) != tc.spaced || CJK(tc.code) != tc.cjk || Chinese(tc.code) != tc.chinese {
			t.Errorf("%s: spaced %v, CJK %v, Chinese %v", tc.code, Spaced(tc.code), CJK(tc.code), Chinese(tc.code))
		}
	}
}

func TestFits(t *testing.T) {
	for _, c := range []struct {
		text, code string
		want       bool
	}{
		{"금이 될지도 몰라.", "ko", true},
		{"금이 될지도 몰라.", "zh", false},
		{"金になるかもしれない。", "zh", false},
		{"金になるかもしれない。", "ja", true},
		{"噂？", "ja", true},
		{"我们BLG就是最强的。", "zh", true},
		{"我们BLG就是最强的。", "en", false},
		{"We BLG are the strongest.", "zh", false},
		{"We BLG are the strongest.", "fr", true},
		{"…", "ko", true},
	} {
		if got := Fits(c.text, c.code); got != c.want {
			t.Errorf("Fits(%q, %s) = %v", c.text, c.code, got)
		}
	}
}
