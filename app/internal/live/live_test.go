package live

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"tomoru/internal/engine"
	"tomoru/internal/translate"
)

// newSession is a session started already, as Start's is once its engines run.
func newSession(cfg Config, onChange func(State)) *Session {
	s := newStartingSession(cfg, onChange)
	close(s.started)
	return s
}

type upper struct{}

func (upper) Translate(_ context.Context, r translate.Request, onDelta func(string)) (string, error) {
	if r.Draft {
		return "draft: " + r.Text, nil
	}
	onDelta("…")
	return "[" + r.Src + "→" + r.Tgt + "] " + r.Text, nil
}

type finalizes struct{ ts []float64 }

func (f *finalizes) Finalize(t float64) error { f.ts = append(f.ts, t); return nil }

// TestReplay plays a recording of the engine through a session, and checks the lines it ends
// with.
func TestReplay(t *testing.T) {
	var mu sync.Mutex
	var last State
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: upper{}}, func(st State) {
		mu.Lock()
		last = st
		mu.Unlock()
	})
	defer s.Stop()
	s.proc = &finalizes{}

	f, err := os.Open("../segment/testdata/ja-engine.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var ev engine.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != "hello" {
			s.handle(ev)
		}
	}

	want := []string{"みなさん、こんばんは。", "今日は新しいゲームを最後までクリアしたいと思います。", "コメントもどんどん書いてくださいね。"}
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		st := last
		mu.Unlock()
		done := len(st.Lines) == len(want)
		for i, l := range st.Lines {
			done = done && l.ID == i+1 && l.Src == want[i] && l.Done && l.Tgt == "[ja→zh] "+want[i]
		}
		// Each was cut from the hypothesis, its end told by the final result after it: each times
		// the wait from its end to its translation.
		timed := st.Stats.Latency.EndToEnd.N == len(want)
		if done && timed && st.Phase == "listening" && st.Pending == "" {
			return
		}
		if time.Now().After(deadline) {
			var lines []string
			for _, l := range st.Lines {
				lines = append(lines, l.Src+" => "+l.Tgt)
			}
			t.Fatalf("phase %q, pending %q, %d timed from their end, lines:\n%s", st.Phase, st.Pending, st.Stats.Latency.EndToEnd.N, strings.Join(lines, "\n"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestDraft checks that pending text is drafted as it grows, and that a sentence keeps the
// draft of it when it is committed.
func TestDraft(t *testing.T) {
	var mu sync.Mutex
	var drafts []string
	var last State
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: upper{}, Draft: true}, func(st State) {
		mu.Lock()
		defer mu.Unlock()
		if st.Draft != "" && (len(drafts) == 0 || drafts[len(drafts)-1] != st.Draft) {
			drafts = append(drafts, st.Draft)
		}
		last = st
	})
	defer s.Stop()
	s.proc = &finalizes{}
	s.handle(engine.Event{Type: "result", Start: 0, End: 1, Tokens: []engine.Token{{Text: "今日は新しいゲーム"}}})
	waitFor(t, &mu, func() bool { return len(drafts) == 1 })
	if drafts[0] != "draft: 今日は新しいゲーム" {
		t.Fatalf("drafts %q", drafts)
	}
	// Grown by less than a step, or too soon: no new draft.
	s.handle(engine.Event{Type: "result", Start: 0, End: 1.2, Tokens: []engine.Token{{Text: "今日は新しいゲームを"}}})
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if len(drafts) != 1 {
		t.Errorf("drafts %q", drafts)
	}
	mu.Unlock()
	// The sentence ends: its line takes the draft, and the next starts without one.
	s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日は新しいゲームをやります。"}}})
	waitFor(t, &mu, func() bool { return len(last.Lines) == 1 && last.Lines[0].Done })
	if l := last.Lines[0]; l.Draft != "draft: 今日は新しいゲーム" || last.Draft != "" {
		t.Errorf("line %+v, draft %q", l, last.Draft)
	}
}

func waitFor(t *testing.T, mu *sync.Mutex, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		mu.Lock()
		done := ok()
		mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
	}
}

// glossed translates as upper does, and says the terms it was given.
type glossed struct{ upper }

func (g glossed) Translate(ctx context.Context, r translate.Request, onDelta func(string)) (string, error) {
	out, err := g.upper.Translate(ctx, r, onDelta)
	for _, t := range r.Terms {
		out += " {" + t.Src + "→" + t.Tgt + "}"
	}
	return out, err
}

// TestTerms applies the glossary to what is heard: a name heard wrong shows written right, in the
// line and in the text still pending, and the line goes to the translator with its terms.
func TestTerms(t *testing.T) {
	terms := func(text string) (string, []translate.Term) {
		text = strings.ReplaceAll(text, "ゲーム", "ゲイム")
		if strings.Contains(text, "ゲイム") {
			return text, []translate.Term{{Src: "ゲイム", Tgt: "游戏"}}
		}
		return text, nil
	}
	var mu sync.Mutex
	var last State
	pendings := map[string]bool{}
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: glossed{}, Terms: terms}, func(st State) {
		mu.Lock()
		last = st
		pendings[st.Pending] = true
		mu.Unlock()
	})
	defer s.Stop()
	s.proc = &finalizes{}
	f, err := os.Open("../segment/testdata/ja-engine.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var ev engine.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != "hello" {
			s.handle(ev)
		}
	}
	const src = "今日は新しいゲイムを最後までクリアしたいと思います。"
	want := "[ja→zh] " + src + " {ゲイム→游戏}"
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		st := last
		mu.Unlock()
		if len(st.Lines) == 3 && st.Lines[1].Src == src && st.Lines[1].Heard == strings.ReplaceAll(src, "ゲイム", "ゲーム") && st.Lines[0].Heard == "" &&
			st.Lines[1].Tgt == want && st.Lines[2].Done && !strings.Contains(st.Lines[2].Tgt, "{") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lines %+v", st.Lines)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	for p := range pendings {
		if strings.Contains(p, "ゲーム") {
			t.Errorf("pending %q", p)
		}
	}
}

// TestStats counts the sentences committed, the seconds spoken and the letters translated.
func TestStats(t *testing.T) {
	var mu sync.Mutex
	var last State
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: upper{}}, func(st State) {
		mu.Lock()
		defer mu.Unlock()
		last = st
	})
	defer s.Stop()
	s.proc = &finalizes{}
	s.handle(engine.Event{Type: "voice", On: true, T: 1})
	s.handle(engine.Event{Type: "voice", On: true, T: 1.5}) // still on: no new start
	s.handle(engine.Event{Type: "voice", On: false, T: 3})
	s.handle(engine.Event{Type: "voice", On: true, T: 4})
	s.handle(engine.Event{Type: "voice", On: false, T: 4.5})
	s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日はいい天気です。明日も晴れます。"}}})
	waitFor(t, &mu, func() bool { return len(last.Lines) == 2 && last.Lines[1].Done })
	want := 0
	for _, l := range last.Lines {
		want += utf8.RuneCountInString(strings.ReplaceAll(l.Tgt, " ", ""))
	}
	if st := last.Stats; st.Lines != 2 || st.Speech != 2.5 || st.Chars != want {
		t.Errorf("stats %+v, want 2 lines, 2.5 s, %d letters", st, want)
	}
}

// TestLatency times a sentence's steps, from the voice starting to its translation, and counts
// the tokens its translation took.
func TestLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家好。\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":32,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	var mu sync.Mutex
	var last State
	tr := &translate.Endpoint{URL: srv.URL, Model: "m", Style: translate.Index, Llama: true}
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr}, func(st State) {
		mu.Lock()
		last = st
		mu.Unlock()
	})
	defer s.Stop()
	s.proc = &finalizes{}

	at := func(v float64) *float64 { return &v }
	s.handle(engine.Event{Type: "clock", T: 1})
	s.handle(engine.Event{Type: "voice", On: true, T: 1})
	time.Sleep(50 * time.Millisecond)
	s.handle(engine.Event{Type: "result", Start: 1, End: 1.05, Tokens: []engine.Token{{Text: "皆さん"}}})
	s.handle(engine.Event{Type: "result", Final: true, Start: 1, End: 1.02, Tokens: []engine.Token{{Text: "皆さん", Start: at(1), End: at(1.01)}, {Text: "こんばんは。", Start: at(1.01), End: at(1.02)}}})

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		st := last.Stats
		mu.Unlock()
		if st.Latency.Translation.N == 1 && st.InTokens > 0 {
			l := st.Latency
			if l.FirstText.N != 1 || l.FirstText.Sum < 50 || l.FirstText.Sum > 500 {
				t.Errorf("first text %+v", l.FirstText)
			}
			if l.Final.N != 1 || l.Final.Sum < 30 {
				t.Errorf("final %+v", l.Final)
			}
			if l.FirstTranslation.N != 1 || l.FirstTranslation.Sum < 30 || l.EndToEnd.N != 1 || l.EndToEnd.Sum < l.FirstTranslation.Sum+30 {
				t.Errorf("translation %+v, end to end %+v", l.FirstTranslation, l.EndToEnd)
			}
			if st.InTokens != 32 || st.OutTokens != 2 || st.Lines != 1 || st.Chars != 4 || st.Failed != 0 {
				t.Errorf("stats %+v", st)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stats %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// held translates the sentence starting "今日" once hold is closed, and the others at once.
type held struct{ hold chan struct{} }

func (h held) Translate(ctx context.Context, r translate.Request, onDelta func(string)) (string, error) {
	if strings.HasPrefix(r.Text, "今日") {
		select {
		case <-h.hold:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	onDelta(r.Text)
	return r.Text, nil
}

// TestParallel translates sentences at once, as Config.Parallel says: one held up holds up the
// next only when they are translated one at a time. Each counts its wait in the queue.
func TestParallel(t *testing.T) {
	for _, parallel := range []int{0, 2} {
		var mu sync.Mutex
		var last State
		tr := held{make(chan struct{})}
		s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, Parallel: parallel}, func(st State) {
			mu.Lock()
			defer mu.Unlock()
			last = st
		})
		s.proc = &finalizes{}
		s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日はいい天気です。明日も晴れます。"}}})
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		second := len(last.Lines) == 2 && last.Lines[1].Done && !last.Lines[0].Done
		mu.Unlock()
		if second != (parallel > 1) {
			t.Errorf("parallel %d: the second translated while the first was held: %v", parallel, second)
		}
		close(tr.hold)
		waitFor(t, &mu, func() bool {
			return len(last.Lines) == 2 && last.Lines[0].Done && last.Lines[1].Done && last.Stats.Latency.Queue.N == 2
		})
		s.Stop()
	}
}

// stuck leaves its first draft hanging until it is dropped.
type stuck struct {
	mu               sync.Mutex
	drafts, canceled int
}

func (s *stuck) Translate(ctx context.Context, r translate.Request, onDelta func(string)) (string, error) {
	if !r.Draft {
		return r.Text, nil
	}
	s.mu.Lock()
	s.drafts++
	first := s.drafts == 1
	s.mu.Unlock()
	if first {
		<-ctx.Done()
		s.mu.Lock()
		s.canceled++
		s.mu.Unlock()
		return "", ctx.Err()
	}
	return "draft: " + r.Text, nil
}

// TestDraftDropped drops the draft in flight when its sentence is committed: the next sentence's
// drafts need not wait for it.
func TestDraftDropped(t *testing.T) {
	var mu sync.Mutex
	var last State
	tr := &stuck{}
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr, Draft: true}, func(st State) {
		mu.Lock()
		defer mu.Unlock()
		last = st
	})
	defer s.Stop()
	s.proc = &finalizes{}
	s.handle(engine.Event{Type: "result", Start: 0, End: 1, Tokens: []engine.Token{{Text: "今日は新しいゲーム"}}})
	s.handle(engine.Event{Type: "result", Start: 0, End: 2, Tokens: []engine.Token{{Text: "今日は新しいゲームをやります。"}}})
	time.Sleep(650 * time.Millisecond) // drafts are 0.6 s apart at least
	s.handle(engine.Event{Type: "result", Start: 0, End: 3, Tokens: []engine.Token{{Text: "今日は新しいゲームをやります。明日もゲームをします"}}})
	waitFor(t, &mu, func() bool { return last.Draft == "draft: 明日もゲームをします" })
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.drafts != 2 || tr.canceled != 1 {
		t.Errorf("%d drafts, %d dropped", tr.drafts, tr.canceled)
	}
}

// warmed translates as upper does, and counts the times it was asked to warm.
type warmed struct {
	upper
	n atomic.Int32
}

func (w *warmed) Warm(context.Context) { w.n.Add(1) }

// TestWarm has the translator warm as the session starts, and as someone starts speaking.
func TestWarm(t *testing.T) {
	tr := &warmed{}
	s := newSession(Config{VolatileSentences: true, Src: "ja", Tgt: "zh", Translator: tr}, func(State) {})
	defer s.Stop()
	s.proc = &finalizes{}
	s.handle(engine.Event{Type: "voice", On: true, T: 1})
	s.handle(engine.Event{Type: "voice", On: true, T: 1.5}) // still on
	s.handle(engine.Event{Type: "voice", On: false, T: 3})
	s.handle(engine.Event{Type: "voice", On: true, T: 4})
	deadline := time.Now().Add(2 * time.Second)
	for tr.n.Load() != 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n := tr.n.Load(); n != 3 {
		t.Errorf("warmed %d times", n)
	}
}

// TestHint has the engine finalize where the recognizer's Hint says, as its events come.
func TestHint(t *testing.T) {
	hint := func(ev engine.Event) (float64, bool) { return ev.T - 1, ev.Type == "clock" && ev.T >= 3 }
	s := newSession(Config{Src: "zh", Tgt: "en", Translator: upper{}, Hint: hint}, func(State) {})
	f := &finalizes{}
	s.proc = f
	for _, t := range []float64{1, 2, 3, 4} {
		s.handle(engine.Event{Type: "clock", T: t})
	}
	s.Stop()
	s.Wait()
	if !slices.Equal(f.ts, []float64{2, 3}) {
		t.Errorf("finalized %v", f.ts)
	}
}
