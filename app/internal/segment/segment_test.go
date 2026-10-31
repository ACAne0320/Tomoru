package segment

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// chars makes one token per character, as SpeechAnalyzer times Chinese and Japanese, dt seconds
// each from t.
func chars(s string, t float64) []Token {
	const dt = 0.15
	var out []Token
	for i, r := range []rune(s) {
		out = append(out, Token{Text: string(r), Start: t + float64(i)*dt, End: t + float64(i+1)*dt})
	}
	return out
}

func commits(acts []Action) []Segment {
	var out []Segment
	for _, a := range acts {
		if a.Commit != nil {
			out = append(out, *a.Commit)
		}
	}
	return out
}

func texts(segs []Segment) []string {
	var out []string
	for _, s := range segs {
		out = append(out, s.Text)
	}
	return out
}

func zh(maxWidth int) *Segmenter { return New(Config{MaxWidth: maxWidth}) }

// zhVol is a segmenter of a recognizer whose volatile sentence ends hold.
func zhVol() *Segmenter { return New(Config{VolatileSentences: true}) }

func TestFinal(t *testing.T) {
	t.Run("one segment per sentence, with times", func(t *testing.T) {
		got := commits(zh(0).Final(chars("皆さん、こんばんは。今日は頑張ります。", 0)))
		if !slices.Equal(texts(got), []string{"皆さん、こんばんは。", "今日は頑張ります。"}) {
			t.Fatalf("%q", texts(got))
		}
		if !got[0].Timed || got[0].Start != 0 || got[0].End != 1.5 {
			t.Errorf("%+v", got[0])
		}
	})
	t.Run("an unfinished tail waits for the speaker to stop", func(t *testing.T) {
		s := zh(0)
		if got := commits(s.Final(chars("先看看角色", 0))); len(got) > 0 {
			t.Errorf("committed %q", texts(got))
		}
		if got := commits(s.Tick(0.75 + 0.5)); len(got) > 0 {
			t.Errorf("committed %q before the pause", texts(got))
		}
		got := commits(s.Tick(0.75 + 0.61))
		if !slices.Equal(texts(got), []string{"先看看角色"}) || got[0].Reason != "pause" {
			t.Errorf("%+v", got)
		}
	})
	t.Run("finalized again over committed audio, nothing is repeated", func(t *testing.T) {
		s := zh(0)
		s.Final(chars("先看看角色创建界面吧。", 0))
		got := commits(s.Final(chars("界面吧。好的。", 1.05))) // 界 is the 8th character
		if !slices.Equal(texts(got), []string{"好的。"}) {
			t.Errorf("%q", texts(got))
		}
	})
	t.Run("punctuation alone is dropped, but its time is committed", func(t *testing.T) {
		s := zh(0)
		got := commits(s.Final([]Token{{Text: " 。", Start: 0, End: 0.2}}))
		if len(got) > 0 || s.CommittedUntil() != 0.2 {
			t.Errorf("%q, committed until %v", texts(got), s.CommittedUntil())
		}
	})
	t.Run("English abbreviations and decimals don't end a sentence", func(t *testing.T) {
		s := New(Config{})
		var toks []Token
		for i, w := range []string{" Mr.", " Smith", " paid", " 3.5", " dollars.", " Then", " he"} {
			toks = append(toks, Token{Text: w, Start: float64(i) * 0.3, End: float64(i+1) * 0.3})
		}
		got := commits(s.Final(toks))
		if !slices.Equal(texts(got), []string{"Mr. Smith paid 3.5 dollars."}) {
			t.Errorf("%q", texts(got))
		}
		if s.Pending() != "Then he" {
			t.Errorf("pending %q", s.Pending())
		}
	})
	t.Run("a long clause is cut at the last comma", func(t *testing.T) {
		got := commits(zh(50).Final(chars("我们今天要玩的是一款新出的角色扮演游戏，听说剧情非常感人", 0)))
		if !slices.Equal(texts(got), []string{"我们今天要玩的是一款新出的角色扮演游戏，"}) || got[0].Reason != "length" {
			t.Errorf("%+v", got)
		}
	})
}

func TestVolatile(t *testing.T) {
	t.Run("a sentence end commits at once, and revisions before it repeat nothing", func(t *testing.T) {
		s := zhVol()
		got := commits(s.Volatile("大家好。欢迎来到", 0, 2))
		if !slices.Equal(texts(got), []string{"大家好。"}) || got[0].Timed {
			t.Fatalf("%+v", got)
		}
		// The hypothesis is revised before the committed point (。→！) and grows.
		got = commits(s.Volatile("大家好！欢迎来到今天的直播。我们", 0, 3))
		if !slices.Equal(texts(got), []string{"欢迎来到今天的直播。"}) {
			t.Errorf("%q", texts(got))
		}
		if s.Pending() != "我们" {
			t.Errorf("pending %q", s.Pending())
		}
	})
	t.Run("committed text rewritten (角色色 → 角色) still aligns", func(t *testing.T) {
		s := zhVol()
		s.Volatile("先看看角色色创建界面吧。", 0, 2)
		got := commits(s.Volatile("先看看角色创建界面吧。好的。", 0, 3))
		if !slices.Equal(texts(got), []string{"好的。"}) {
			t.Errorf("%q", texts(got))
		}
	})
	t.Run("the final result of committed volatile text adds only what is new", func(t *testing.T) {
		s := zhVol()
		s.Volatile("大家好。欢迎来到今天的直播。我", 0, 3)
		got := commits(s.Final(chars("大家好，欢迎来到今天的直播。我们", 0)))
		if len(got) > 0 {
			t.Errorf("committed again: %q", texts(got))
		}
		if s.Pending() != "我们" || len(s.stable) != 2 {
			t.Errorf("pending %q, stable %v", s.Pending(), s.stable)
		}
		if math.Abs(s.CommittedUntil()-14*0.15) > 1e-9 {
			t.Errorf("committed until %v", s.CommittedUntil())
		}
	})
	t.Run("volatile sentence ends wait for the final result where they don't hold", func(t *testing.T) {
		s := zh(0)
		if got := commits(s.Volatile("大家好。欢迎来到", 0, 2)); len(got) > 0 || s.Pending() != "大家好。欢迎来到" {
			t.Fatalf("committed %q, pending %q", texts(got), s.Pending())
		}
		if got := commits(s.Final(chars("大家好。欢迎来到", 0))); !slices.Equal(texts(got), []string{"大家好。"}) || !got[0].Timed {
			t.Errorf("%+v", got)
		}
	})
	t.Run("a hypothesis alone never commits by waiting: its final result does", func(t *testing.T) {
		s := zh(0)
		s.Volatile("先看看角色创建界面吧", 10, 12)
		if acts := s.Tick(20); len(acts) > 0 {
			t.Errorf("committed the hypothesis: %+v", acts)
		}
		got := commits(s.Final(chars("先看看角色创建界面吧", 10)))
		if !slices.Equal(texts(got), []string{"先看看角色创建界面吧"}) || got[0].Reason != "pause" || s.Pending() != "" {
			t.Errorf("%+v, pending %q", got, s.Pending())
		}
	})
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"大家好 ，欢迎 。":               "大家好，欢迎。",
		" Hey everyone,  welcome": "Hey everyone, welcome",
	} {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlignEnd(t *testing.T) {
	h := "先看看角色创建界面吧。好的"
	if got := string([]rune(h)[alignEnd("先看看角色色创建界面吧", h):]); got != "好的" {
		t.Errorf("left %q", got)
	}
}

func TestFirstSentenceEnd(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"大家好。欢迎", "大家好。"},
		{"「行くよ！」と言った", "「行くよ！」"},
		{"本当に？！そうか", "本当に？！"},
		{"还没说完", ""},
		{"Mr. Smith paid 3.5 dollars. Then", "Mr. Smith paid 3.5 dollars."},
		{"We met J. K. Rowling. She", "We met J. K. Rowling."},
		{"It's 5 p.m. and late", ""},
		{"Apples, pears, etc. and more", ""},
		{"Well... maybe. Yes", "Well... maybe."},
		{"Wait... Okay", "Wait..."},
		{"He said \"go.\" Then", "He said \"go.\""},
		{"Really?! No", "Really?!"},
		{"안녕하세요. 여러분", "안녕하세요."},
		{"See the file.txt now", ""},
	} {
		got := ""
		if b := firstSentenceEnd([]rune(c.text)); b >= 0 {
			got = string([]rune(c.text)[:b])
		}
		if got != c.want {
			t.Errorf("%q: first sentence %q, want %q", c.text, got, c.want)
		}
	}
}

// TestVoice commits the tail of final results once the speaker has been silent for TailWait, by
// voice activity: a pause shorter than that, mid-sentence, keeps it for the next final result.
func TestVoice(t *testing.T) {
	s := New(Config{})
	s.Voice(true, 0)
	s.Final(chars("その後とは", 0)) // ends at 0.75
	s.Voice(false, 0.8)
	if got := commits(s.Tick(1.3)); len(got) > 0 {
		t.Fatalf("committed in a short pause: %q", texts(got))
	}
	s.Voice(true, 1.35)
	got := commits(s.Final(chars("あの世のことを覚えておらぬのか。", 1.7)))
	if !slices.Equal(texts(got), []string{"その後とはあの世のことを覚えておらぬのか。"}) || !got[0].Timed || got[0].Start != 0 {
		t.Fatalf("%+v", got)
	}
	s.Final(chars("はい", 4.2))
	s.Voice(false, 4.5)
	if got := commits(s.Tick(5.0)); len(got) > 0 {
		t.Fatalf("committed before TailWait: %q", texts(got))
	}
	if got := commits(s.Tick(5.2)); !slices.Equal(texts(got), []string{"はい"}) || got[0].Reason != "pause" {
		t.Fatalf("%+v", got)
	}
	// While the voice goes on, no wait commits.
	s.Voice(true, 6)
	s.Final(chars("いや", 6))
	if got := commits(s.Tick(20)); len(got) > 0 {
		t.Errorf("committed while speaking: %q", texts(got))
	}
}

// TestFinalsEnd commits the unfinished tail of a final result at once, for a recognizer whose final
// results end utterances.
func TestFinalsEnd(t *testing.T) {
	s := New(Config{FinalsEnd: true})
	got := commits(s.Final(chars("大事はないか。その後とは", 0)))
	if !slices.Equal(texts(got), []string{"大事はないか。", "その後とは"}) || got[1].Reason != "pause" || s.Pending() != "" {
		t.Errorf("%+v, pending %q", got, s.Pending())
	}
}

// TestLength cuts a sentence past two lines at its last clause mark, and only past them.
func TestLength(t *testing.T) {
	long := "我们今天要玩的是一款新出的角色扮演游戏，听说剧情非常感人，画面也很漂亮，所以大家一起来看看吧然后"
	got := commits(New(Config{}).Final(chars(long, 0)))
	if !slices.Equal(texts(got), []string{"我们今天要玩的是一款新出的角色扮演游戏，听说剧情非常感人，画面也很漂亮，"}) || got[0].Reason != "length" {
		t.Errorf("%+v", got)
	}
	if got := commits(New(Config{}).Final(chars("我们今天要玩的是一款新出的角色扮演游戏，听说剧情非常感人", 0))); len(got) > 0 {
		t.Errorf("cut a sentence of one line: %q", texts(got))
	}
	// No clause mark: between the last two words.
	var toks []Token
	for i, w := range []string{" so", " I", " went", " to", " the", " store", " and", " bought", " some", " apples", " and", " then", " I", " walked", " home", " because", " it", " was", " a", " really", " nice", " day", " outside", " today"} {
		toks = append(toks, Token{Text: w, Start: float64(i) * .3, End: float64(i+1) * .3})
	}
	got = commits(New(Config{}).Final(toks))
	if len(got) != 1 || got[0].Reason != "length" || got[0].Text != "so I went to the store and bought some apples and then I walked home because it was a" {
		t.Errorf("%+v", got)
	}
	if w := Width(got[0].Text); w > 90 {
		t.Errorf("cut %d wide", w)
	}
	// A whole sentence too long, final at once: cut within two lines too, then the rest.
	toks = nil
	for i, w := range strings.Fields("She was still struggling, often losing sleep in the middle of the night, feeling unfocused, unmotivated during the day.") {
		toks = append(toks, Token{Text: " " + w, Start: float64(i) * .3, End: float64(i+1) * .3})
	}
	got = commits(New(Config{FinalsEnd: true}).Final(toks))
	if !slices.Equal(texts(got), []string{"She was still struggling, often losing sleep in the middle of the night,", "feeling unfocused, unmotivated during the day."}) || got[0].Reason != "length" {
		t.Errorf("%q", texts(got))
	}
}
