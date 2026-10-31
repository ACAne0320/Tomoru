package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHist(t *testing.T) {
	var h Hist
	if _, ok := h.Quantile(.5); ok {
		t.Error("a median of nothing")
	}
	for _, ms := range []int{150, 250, 350, 450, 2600} {
		h.Add(time.Duration(ms) * time.Millisecond)
	}
	h.Add(-time.Second) // a clock that went back: none
	if h.N != 6 || h.B[0] != 1 || h.B[1] != 1 || h.B[16] != 1 {
		t.Errorf("counted %+v", h)
	}
	if d, _ := h.Quantile(.5); d != 300*time.Millisecond {
		t.Errorf("median %v", d)
	}
	var long Hist
	long.Add(time.Minute)
	if d, _ := long.Quantile(.9); d != 10*time.Second {
		t.Errorf("past the bounds: %v", d)
	}
	var m Hist
	m.Merge(h)
	m.Merge(long)
	if m.N != 7 || m.Sum != h.Sum+long.Sum {
		t.Errorf("merged %+v", m)
	}
}

func TestLedger(t *testing.T) {
	asr := Engines{ASR: "apple", MT: "cloud"}
	var l Ledger
	l.Add("2026-10-21", asr, Counts{Run: 60, Lines: 3})
	l.Add("2026-10-19", asr, Counts{Run: 10})
	l.Add("2026-10-21", asr, Counts{Run: 30, Lines: 1})
	l.Add("2026-10-21", Engines{ASR: "apple", MT: "custom:a", MTName: "OpenRouter"}, Counts{Lines: 5})
	if len(l.Days) != 2 || l.Days[0].Day != "2026-10-19" {
		t.Fatalf("days %+v", l.Days)
	}
	if d := l.Days[1]; len(d.Uses) != 2 || d.Uses[0].Run != 90 || d.Uses[0].Lines != 4 {
		t.Errorf("the day's uses %+v", d.Uses)
	}
	if c := l.Day("2026-10-21"); c.Run != 90 || c.Lines != 9 {
		t.Errorf("the day %+v", c)
	}

	path := filepath.Join(t.TempDir(), "stats.json")
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got.Days) != 2 || got.Day("2026-10-21").Lines != 9 {
		t.Errorf("loaded %+v, %v", got, err)
	}
	c := got.Clone()
	c.Days[0].Uses[0].Run = 0
	if got.Days[0].Uses[0].Run != 10 {
		t.Error("the clone shares its uses")
	}
	if l, err := Load(filepath.Join(t.TempDir(), "none.json")); err != nil || len(l.Days) != 0 {
		t.Errorf("none: %+v, %v", l, err)
	}
}

// TestLoadDay reads the file of a day's numbers the app kept before the ledger.
func TestLoadDay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	os.WriteFile(path, []byte(`{"Day": "2026-10-21", "Run": 29149, "Lines": 555, "Speech": 9000, "Chars": 12000}`), 0o644)
	l, err := Load(path)
	if err != nil || len(l.Days) != 1 {
		t.Fatalf("%+v, %v", l, err)
	}
	if u := l.Days[0].Uses[0]; u.Engines != (Engines{}) || u.Run != 29149 || u.Lines != 555 || u.Speech != 9000 || u.Chars != 12000 {
		t.Errorf("the day %+v", u)
	}
}
