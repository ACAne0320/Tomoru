package core

import (
	"slices"
	"testing"

	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/engine"
	"tomoru/internal/lang"
	"tomoru/internal/prefs"
)

func TestSourceLangs(t *testing.T) {
	c, _ := newCore(t)
	for _, tc := range []struct {
		asr  string
		want []string
	}{
		{"apple", lang.Spoken()},
		{"sensevoice-small", []string{"auto", "zh", "en", "ja", "ko", "yue"}},
		{QwenCloud, qwencloud.Langs},
		{"", lang.Spoken()},
	} {
		c.Prefs.ASR = tc.asr
		if got := c.SourceLangs(); !slices.Equal(got, tc.want) {
			t.Errorf("%q: %v, want %v", tc.asr, got, tc.want)
		}
	}
	if got := c.TargetLangs(); !slices.Equal(got, lang.All) {
		t.Errorf("targets %v", got)
	}
}

func TestListLangs(t *testing.T) {
	if prefs.SystemASR() == "" {
		t.Skip("the system has no recognition")
	}
	c, f := newCore(t)
	f.answers["locales"] = []engine.Event{{Type: "locale", ID: "en-US"}, {Type: "locale", ID: "en-GB"}, {Type: "locale", ID: "zh-TW"},
		{Type: "locale", ID: "zh-HK"}, {Type: "locale", ID: "yue-CN"}, {Type: "locale", ID: "ja-JP"}, {Type: "locale", ID: "sw-KE"}}
	c.Prefs.Src = "th"
	c.ListLangs()
	f.flush(t, 1)
	if got := c.SourceLangs(); !slices.Equal(got, []string{"zh", "en", "ja", "yue"}) {
		t.Errorf("hears %v", got)
	}
	if c.Prefs.Src != "en" {
		t.Errorf("Thai, unheard, became %q", c.Prefs.Src)
	}
	c.ListLangs()
	if slices.Index(f.ran, "locales") != len(f.ran)-1 {
		t.Errorf("ran %v", f.ran)
	}
}

func TestFitLangs(t *testing.T) {
	c, _ := newCore(t)
	model := Engine{ID: "sensevoice-small", Kind: ASR}

	c.Prefs.Src, c.Prefs.Tgt = "fr", "zh"
	c.use(model)
	if c.Prefs.Src != "en" || c.Prefs.Tgt != "zh" {
		t.Errorf("fr → zh became %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.Prefs.ASR = "apple"
	c.Prefs.Src, c.Prefs.Tgt = "fr", "en"
	c.use(model)
	if c.Prefs.Src != "ja" {
		t.Errorf("fr → en became %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}

	c.Prefs.Src, c.Prefs.Tgt = "ja", "fr"
	if c.CanSwap() {
		t.Error("can swap into French")
	}
	c.Swap()
	if c.Prefs.Src != "ja" || c.Prefs.Tgt != "fr" {
		t.Errorf("swapped to %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetTgt("ja")
	if c.Prefs.Src != "en" || c.Prefs.Tgt != "ja" {
		t.Errorf("translating into the spoken language: %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetTgt("zh-Hant")
	c.SetSrc("yue")
	if c.Prefs.Src != "yue" || c.Prefs.Tgt != "zh-Hant" || c.CanSwap() {
		t.Errorf("Cantonese into Traditional Chinese: %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
}

func TestPrepareOnlyHeard(t *testing.T) {
	c, f := newCore(t)
	c.PrepareASR("zh-Hant")
	if len(f.ran) != 0 || c.ASR["zh-Hant"] != "" {
		t.Errorf("prepared Traditional Chinese: ran %v", f.ran)
	}
}

func TestAuto(t *testing.T) {
	c, _ := newCore(t)
	c.Prefs.ASR, c.Prefs.Src, c.Prefs.Tgt = "sensevoice-small", "ja", "zh"
	c.SetSrc(lang.Auto)
	if c.Prefs.Src != lang.Auto || c.Prefs.Tgt != "zh" || c.CanSwap() {
		t.Fatalf("auto → %s, can swap %v", c.Prefs.Tgt, c.CanSwap())
	}
	c.Swap()
	if c.Prefs.Src != lang.Auto {
		t.Errorf("swapped to %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetSrc("zh")
	if c.Prefs.Src != "zh" || c.Prefs.Tgt != "en" {
		t.Errorf("zh, the target, spoken: %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetSrc(lang.Auto)
	c.SetSrc("en")
	if c.Prefs.Src != "en" || c.Prefs.Tgt != "zh" {
		t.Errorf("en, the target, spoken: %s → %s", c.Prefs.Src, c.Prefs.Tgt)
	}
	c.SetSrc(lang.Auto)
	c.use(Engine{ID: QwenCloud, Kind: ASR})
	if c.Prefs.Src != "en" {
		t.Errorf("a recognition that does not tell the language: %s", c.Prefs.Src)
	}
}
