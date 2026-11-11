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

	// Edited out of range: set right. An API deleted stays chosen: translating says it is gone
	// rather than sending the subtitles elsewhere.
	os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "ja", "Shown": 99, "Font": "Comic", "MT": "custom:gone"}`), 0o644)
	got, _ = Load(path, "zh-Hans")
	if got.Src != "en" || got.Tgt != "zh" || got.Shown != MaxShown || got.Font != "system" || got.MT != "custom:gone" {
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

// TestEndpointFormat reads an API's format, and moves Index, as settings kept it before Format,
// into it.
func TestEndpointFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"Endpoints": [{"ID": "a", "Index": true}, {"ID": "b", "Index": false}, {"ID": "c"}, {"ID": "d", "Format": "hy-mt"}]}`), 0o644)
	p, err := Load(path, "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"a": "index", "b": "chat", "c": "chat", "d": "hy-mt"} {
		if e, ok := p.Endpoint(id); !ok || e.Format != want || e.Index {
			t.Errorf("%s: %+v, want %q", id, e, want)
		}
	}
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); strings.Contains(string(b), `"Index"`) {
		t.Error("saved the old field")
	}
	if p, _ := Load(path, "zh-Hans"); p.Endpoints[0].Format != "index" || p.Endpoints[3].Format != "hy-mt" {
		t.Errorf("read again: %+v", p.Endpoints)
	}
}

// TestASRCloud moves Qwen's service, as settings kept it before ASRAPIs, into them, once.
func TestASRCloud(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"ASR": "qwen-cloud", "ASRCloud": {"Host": "dashscope-intl.aliyuncs.com", "Model": "", "Key": true}}`), 0o644)
	p, err := Load(path, "zh-Hans")
	a, ok := p.API("qwen-cloud")
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

// TestDisplay keeps the original only, which translates nothing, and sets a choice it does not
// know back to both.
func TestDisplay(t *testing.T) {
	for in, want := range map[string]int{`{"Display":2}`: DisplaySrc, `{"Display":1}`: DisplayTgt, `{"Display":7}`: DisplayBoth, `{"Display":-1}`: DisplayBoth} {
		path := filepath.Join(t.TempDir(), "settings.json")
		os.WriteFile(path, []byte(in), 0o644)
		p, err := Load(path, "en")
		if err != nil || p.Display != want || p.Translates() != (want != DisplaySrc) {
			t.Errorf("%s: display %d, translates %v, %v", in, p.Display, p.Translates(), err)
		}
	}
}
