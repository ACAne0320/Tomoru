package live

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/resource"
	"tomoru/internal/translate"
)

func TestEndToEnd(t *testing.T) {
	spec := os.Getenv("TOMORU_E2E")
	if spec == "" {
		t.Skip(`set TOMORU_E2E="ja:/path/to/speech.wav" to run, and optionally TOMORU_E2E_LOCAL, TOMORU_E2E_ASR, TOMORU_E2E_CLOUD with DASHSCOPE_API_KEY and DASHSCOPE_API_HOST (docs/development.md); it plays the recording aloud and needs the System Audio Recording permission`)
	}
	lang, wav, _ := strings.Cut(spec, ":")
	bin, err := engine.Path(filepath.Join("..", "..", "resources"))
	if err != nil {
		t.Fatal(err)
	}
	tgt := "zh"
	if lang == "zh" {
		tgt = "en"
	}
	var tr translate.Translator = translate.Cloud()
	if model := os.Getenv("TOMORU_E2E_LOCAL"); model != "" {
		server, err := resource.Bin("llama-server", filepath.Join("..", "..", "resources"))
		if err != nil {
			t.Fatal(err)
		}
		srv := models.NewServer(server, nil)
		defer srv.Stop()
		t0 := time.Now()
		url, err := srv.Ensure(context.Background(), model)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("loaded %s in %.1fs", filepath.Base(model), time.Since(t0).Seconds())
		tr = &translate.Endpoint{URL: url, Model: model, Format: translate.Index, Llama: true}
	}
	cfg := Config{Engine: bin, Src: lang, Tgt: tgt, Translator: tr}
	if dir := os.Getenv("TOMORU_E2E_ASR"); dir != "" {
		recognizer, err := engine.ASRPath(filepath.Join("..", "..", "resources"))
		if err != nil {
			t.Fatal(err)
		}
		spec, _ := asr.ByID("sensevoice-small")
		if cfg.Recognize, err = spec.New(asr.Env{ASR: recognizer, ModelDir: dir}); err != nil {
			t.Fatal(err)
		}
	}
	if key := os.Getenv("DASHSCOPE_API_KEY"); key != "" && os.Getenv("TOMORU_E2E_CLOUD") != "" {
		spec, _ := asr.ByID(asr.QwenCloud)
		svc := asr.Service{URL: os.Getenv("DASHSCOPE_API_HOST"), Model: os.Getenv("TOMORU_E2E_CLOUD"), Key: key}
		if cfg.Recognize, err = spec.New(asr.Env{Service: svc}); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var last State
	start := time.Now()
	s, err := Start(cfg, func(st State) {
		mu.Lock()
		defer mu.Unlock()
		if st.Phase != last.Phase {
			t.Logf("%5.2fs phase %s", time.Since(start).Seconds(), st.Phase)
		}
		for _, l := range st.Lines {
			if l.Done() && !doneBefore(last, l.ID) {
				t.Logf("%5.2fs %s => %s %s", time.Since(start).Seconds(), l.Src, l.Tgt, l.Err)
			}
		}
		last = st
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		mu.Lock()
		phase := last.Phase
		mu.Unlock()
		if phase == PhaseListening {
			break
		}
		if phase == PhaseStopped || time.Now().After(deadline) {
			t.Fatalf("phase %q: %v", phase, last.Err)
		}
	}
	t.Logf("%5.2fs playing %s", time.Since(start).Seconds(), filepath.Base(wav))
	if out, err := exec.Command("afplay", wav).CombinedOutput(); err != nil {
		t.Fatalf("afplay: %v %s", err, out)
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		mu.Lock()
		settled := last.Pending == "" && len(last.Lines) > 0 && last.Lines[len(last.Lines)-1].Done()
		mu.Unlock()
		if settled || time.Now().After(deadline) {
			break
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(last.Lines) == 0 {
		t.Fatalf("no lines; pending %q", last.Pending)
	}
	for _, l := range last.Lines {
		if !l.Done() || l.Tgt == "" {
			t.Errorf("line %d not translated: %+v", l.ID, l)
		}
	}
}

func doneBefore(st State, id int) bool {
	for _, l := range st.Lines {
		if l.ID == id {
			return l.Done()
		}
	}
	return false
}
