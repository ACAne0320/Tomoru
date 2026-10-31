package segment

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReplay feeds recordings of the engine (testdata/*.jsonl, docs/engine-protocol.md) through the
// segmenter and compares what it commits with the .golden file next to each. The
// golden files were made by the Swift segmenter it was ported from, on the same recordings; run
// with -update to write them from this one instead, after a deliberate change.
func TestReplay(t *testing.T) {
	recs, _ := filepath.Glob("testdata/*.jsonl")
	if len(recs) == 0 {
		t.Fatal("no recordings")
	}
	for _, rec := range recs {
		lang := strings.TrimSuffix(filepath.Base(rec), ".jsonl")
		t.Run(lang, func(t *testing.T) {
			got := replay(t, rec, lang)
			golden := strings.TrimSuffix(rec, ".jsonl") + ".golden"
			if *update {
				os.WriteFile(golden, []byte(got), 0o644)
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func replay(t *testing.T, path, lang string) string {
	var out strings.Builder
	replayWith(t, path, lang, func(_ *Segmenter, acts []Action) {
		for _, a := range acts {
			fmt.Fprintf(&out, "commit\t%s\t%s\n", a.Commit.Reason, a.Commit.Text)
		}
	})
	return out.String()
}

// replayWith plays the recording at path through a segmenter of lang, giving emit what each event
// returns.
func replayWith(t *testing.T, path, lang string, emit func(*Segmenter, []Action)) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := New(Config{VolatileSentences: true, FinalsEnd: true}) // the recordings are Apple's
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var ev struct {
			Type       string
			T          float64
			On         bool
			Final      bool
			Start, End float64
			Tokens     []struct {
				Text       string
				Start, End *float64
			}
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatal(err)
		}
		switch ev.Type {
		case "clock":
			emit(s, s.Tick(ev.T))
		case "voice":
			emit(s, s.Voice(ev.On, ev.T))
		case "result":
			if ev.Final {
				var toks []Token
				for _, tok := range ev.Tokens {
					if tok.Start != nil && tok.End != nil { // the recognizer timed it
						toks = append(toks, Token{tok.Text, *tok.Start, *tok.End})
					}
				}
				emit(s, s.Final(toks))
			} else {
				var text strings.Builder
				for _, tok := range ev.Tokens {
					text.WriteString(tok.Text)
				}
				emit(s, s.Volatile(text.String(), ev.Start, ev.End))
			}
		}
	}
}

// TestEnded times every segment of the recordings: by its final tokens as it is committed, or, cut
// from the hypothesis, by those of the final result that comes after, at the end of its last word.
func TestEnded(t *testing.T) {
	recs, _ := filepath.Glob("testdata/*.jsonl")
	for _, rec := range recs {
		lang := strings.TrimSuffix(filepath.Base(rec), ".jsonl")
		t.Run(lang, func(t *testing.T) {
			type seg struct {
				Segment
				ended float64
			}
			var segs []seg
			byEnd := 0
			clock := 0.0
			replayWith(t, rec, lang, func(s *Segmenter, acts []Action) {
				for _, a := range acts {
					if a.Commit != nil {
						if a.Commit.Seq != len(segs)+1 {
							t.Errorf("segment %d numbered %d", len(segs)+1, a.Commit.Seq)
						}
						segs = append(segs, seg{Segment: *a.Commit, ended: -1})
						clock = max(clock, s.clock)
					}
				}
				for _, e := range s.Ended() {
					sg := &segs[e.Seq-1]
					if sg.Timed || sg.ended >= 0 {
						t.Errorf("%q timed again", sg.Text)
					}
					if e.End < sg.Start || e.End > s.clock {
						t.Errorf("%q ended at %.2f, started %.2f, clock %.2f", sg.Text, e.End, sg.Start, s.clock)
					}
					sg.ended = e.End
					byEnd++
				}
			})
			timed := 0
			for _, sg := range segs {
				if sg.Timed {
					timed++
				} else if sg.ended < 0 {
					t.Logf("never timed: %q", sg.Text)
				}
			}
			t.Logf("%d segments: %d timed as committed, %d after", len(segs), timed, byEnd)
			if timed+byEnd < len(segs)-1 { // the last may wait for a final result the recording ends before
				t.Errorf("%d of %d segments timed", timed+byEnd, len(segs))
			}
		})
	}
}
