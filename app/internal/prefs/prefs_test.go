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

	os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "ja", "Shown": 99, "Font": " ", "MT": "custom:gone"}`), 0o644)
	got, _ = Load(path, "zh-Hans")
	if got.Src != "en" || got.Tgt != "zh" || got.Shown != MaxShown || got.Font != "system" || got.MT != "custom:gone" {
		t.Errorf("%+v", got)
	}
	os.WriteFile(path, []byte(`{"Src": "auto", "Tgt": "zh"}`), 0o644)
	if got, _ = Load(path, "en"); got.Src != "auto" || got.Tgt != "zh" {
		t.Errorf("auto → %s became %s → %s", "zh", got.Src, got.Tgt)
	}
	os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "auto"}`), 0o644)
	if got, _ = Load(path, "zh-Hans"); got.Src != "en" || got.Tgt != "zh" {
		t.Errorf("ja → auto became %s → %s", got.Src, got.Tgt)
	}
	os.WriteFile(path, []byte(`{"SrcColor": "white", "TgtColor": "#12345", "EdgeColor": "#ABCDEF80"}`), 0o644)
	if got, _ = Load(path, "en"); got.SrcColor != "#ffffff" || got.TgtColor != "#7fdbff" || got.EdgeColor != "#ABCDEF80" {
		t.Errorf("colors %q %q %q", got.SrcColor, got.TgtColor, got.EdgeColor)
	}
	os.WriteFile(path, []byte(`{"TextOpacity": 5, "BoxOpacity": 120}`), 0o644)
	if got, _ = Load(path, "en"); got.TextOpacity != MinTextOpacity || got.BoxOpacity != 100 || got.OlderOpacity != 54 {
		t.Errorf("opacity %v %v %v", got.TextOpacity, got.BoxOpacity, got.OlderOpacity)
	}
	os.WriteFile(path, []byte(`{"Font": "Comic Sans MS"}`), 0o644)
	if got, _ = Load(path, "en"); got.Font != "Comic Sans MS" {
		t.Errorf("font %q", got.Font)
	}
}

func TestImageShortcut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for shortcut, want := range map[string]string{"Ctrl+Alt+T": DefaultImageShortcut, "Alt+Shift+S": DefaultShortcut} {
		os.WriteFile(path, []byte(`{"Src": "ja", "Tgt": "zh", "Shortcut": "`+shortcut+`"}`), 0o644)
		if p, _ := Load(path, "zh-Hans"); p.ImageShortcut != want || p.Shortcut != shortcut {
			t.Errorf("shortcut %s: picture's %q, want %q", shortcut, p.ImageShortcut, want)
		}
	}
}

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
	os.WriteFile(path, []byte(`{"ASRCloud": {"Host": "", "Model": "", "Key": false}}`), 0o644)
	if p, _ := Load(path, "zh-Hans"); len(p.ASRAPIs) != 0 || p.ASRCloud != nil {
		t.Errorf("%+v", p.ASRAPIs)
	}
}

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
