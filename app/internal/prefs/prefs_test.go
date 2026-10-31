package prefs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	for locale, want := range map[string][2]string{
		"zh-Hans": {"en", "zh"}, "zh-Hant": {"en", "zh-Hant"}, "ja": {"en", "ja"}, "ko": {"en", "ko"},
		"en": {"ja", "en"}, "de": {"en", "de"}, "sw": {"ja", "en"},
	} {
		if p := Default(locale); p.Src != want[0] || p.Tgt != want[1] {
			t.Errorf("%s: %s → %s, want %s → %s", locale, p.Src, p.Tgt, want[0], want[1])
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if p, err := Load(path, "ja"); err != nil || !reflect.DeepEqual(p, Default("ja")) {
		t.Errorf("no file: %+v, %v", p, err)
	}

	p := Default("ja")
	p.Src, p.Draft, p.Source = "ko", false, "com.apple.Safari"
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, "ja")
	if err != nil || got.Src != "ko" || got.Draft || got.SourceName != "com.apple.Safari" {
		t.Errorf("%+v, %v", got, err)
	}

	// Edited out of range: set right.
	os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "ja", "Shown": 99, "Font": "Comic", "MT": "custom:gone"}`), 0o644)
	got, _ = Load(path, "zh-Hans")
	if got.Src != "en" || got.Tgt != "zh" || got.Shown != MaxShown || got.Font != "system" || got.MT != "cloud" {
		t.Errorf("%+v", got)
	}
}

// TestImageShortcut gives settings from before pictures were translated the picture's shortcut,
// one that is not starting and stopping's.
func TestImageShortcut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for shortcut, want := range map[string]string{"Ctrl+Alt+T": DefaultImageShortcut, "Alt+Shift+S": DefaultShortcut} {
		os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "zh", "Shortcut": "`+shortcut+`"}`), 0o644)
		if p, _ := Load(path, "zh-Hans"); p.ImageShortcut != want || p.Shortcut != shortcut {
			t.Errorf("shortcut %s: picture's %q, want %q", shortcut, p.ImageShortcut, want)
		}
	}
}

// TestASRCloud moves Qwen's service, as settings kept it before ASRAPIs, into them, once.
func TestASRCloud(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"ASR": "qwen-cloud", "ASRCloud": {"Host": "dashscope-intl.aliyuncs.com", "Model": "", "Key": true}}`), 0o644)
	p, err := Load(path, "zh-Hans")
	a, ok := p.ASRAPI("qwen-cloud")
	if err != nil || !ok || len(p.ASRAPIs) != 1 || !a.Key || a.URL != "dashscope-intl.aliyuncs.com" || p.ASRCloud != nil || p.ASR != "qwen-cloud" {
		t.Fatalf("%+v %v", p.ASRAPIs, err)
	}
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); strings.Contains(string(b), `"ASRCloud"`) {
		t.Error("saved the old field")
	}
	// Never set up: nothing to move.
	os.WriteFile(path, []byte(`{"ASRCloud": {"Host": "", "Model": "", "Key": false}}`), 0o644)
	if p, _ := Load(path, "zh-Hans"); len(p.ASRAPIs) != 0 || p.ASRCloud != nil {
		t.Errorf("%+v", p.ASRAPIs)
	}
}
