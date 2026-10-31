package segment

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/lang"
)

// TestLive runs the segmenter on the real engine while a recording plays, when TOMORU_LIVE names
// it ("zh:/path/to/zh.wav"), with voice activity and without (TOMORU_LIVE_VOICE=0), and logs each
// commit with how long after the speaker's last pause it came. The recording plays out loud.
func TestLive(t *testing.T) {
	spec := os.Getenv("TOMORU_LIVE")
	if spec == "" {
		t.Skip(`set TOMORU_LIVE="zh:/path/to/zh.wav" to run`)
	}
	code, wav, _ := strings.Cut(spec, ":")
	bin, err := engine.Path(filepath.Join("..", "..", "resources"))
	if err != nil {
		t.Fatal(err)
	}
	useVoice := os.Getenv("TOMORU_LIVE_VOICE") != "0"
	var mu sync.Mutex
	s := New(Config{VolatileSentences: true}) // Apple's, in the engine
	lastOff, listening := 0.0, make(chan struct{})
	var log []string
	act := func(acts []Action) {
		for _, a := range acts {
			log = append(log, fmt.Sprintf("%-8s +%.2fs  %s", a.Commit.Reason, s.clock-lastOff, a.Commit.Text))
		}
	}
	proc, err := engine.Listen(bin, lang.Locale(code), "", "", false, func(ev engine.Event) {
		mu.Lock()
		defer mu.Unlock()
		switch ev.Type {
		case "state":
			if ev.State == "listening" {
				close(listening)
			}
		case "voice":
			if !ev.On {
				lastOff = ev.T
			}
			if useVoice {
				act(s.Voice(ev.On, ev.T))
			}
		case "clock":
			act(s.Tick(ev.T))
		case "result":
			if ev.Final {
				var toks []Token
				for _, tok := range ev.Tokens {
					if tok.Start != nil {
						toks = append(toks, Token{tok.Text, *tok.Start, *tok.End})
					}
				}
				act(s.Final(toks))
			} else {
				act(s.Volatile(ev.Text(), ev.Start, ev.End))
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Stop()
	<-listening
	time.Sleep(500 * time.Millisecond)
	if out, err := exec.Command("afplay", wav).CombinedOutput(); err != nil {
		t.Fatalf("afplay: %v %s", err, out)
	}
	time.Sleep(4 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	t.Logf("%s voice=%v\n  %s", code, useVoice, strings.Join(log, "\n  "))
}
