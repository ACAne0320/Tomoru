package scene

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"tomoru/internal/translate"
)

func TestApply(t *testing.T) {
	g := NewGlossary([]Term{
		{Src: "メテオ", Tgt: "陨石", Heard: []string{"メデオ"}, Learned: true},
		{Src: "クラウド", Tgt: "克劳德"},
		{Src: "ファイナルファンタジーVII", Tgt: "最终幻想7"},
		{Src: "Eva", Tgt: "爱娃", Heard: []string{"Ava", "F", "A."}},
		{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北", "刘备"}},
		{Src: "", Tgt: "nothing"},
	}, false)
	for _, c := range []struct {
		in, out string
		terms   []translate.Term
	}{
		{"メデオの脅威からクラウドが", "メテオの脅威からクラウドが", []translate.Term{{Src: "クラウド", Tgt: "克劳德"}, {Src: "メテオ", Tgt: "陨石"}}},
		{"ファイナルファンタジー Ⅶ が完結", "ファイナルファンタジー Ⅶ が完結", []translate.Term{{Src: "ファイナルファンタジーVII", Tgt: "最终幻想7"}}},
		{"eva and Ava, Ava!", "eva and Eva, Eva!", []translate.Term{{Src: "Eva", Tgt: "爱娃"}}},
		{"We evaluate Avatar.", "We evaluate Avatar.", nil},
		{"开着车在刘北赛道上，挑战刘备", "开着车在纽北赛道上，挑战纽北", []translate.Term{{Src: "纽北", Tgt: "Nürburgring"}}},
		{"大家决定重启 Ava项目", "大家决定重启 Eva项目", []translate.Term{{Src: "Eva", Tgt: "爱娃"}}},
		{"Grade F is fine.", "Grade F is fine.", nil},
		{"nothing here", "nothing here", nil},
	} {
		out, terms := g.Apply(c.in)
		if out != c.out || !reflect.DeepEqual(terms, c.terms) {
			t.Errorf("%q: %q %v, want %q %v", c.in, out, terms, c.out, c.terms)
		}
	}
	if out, terms := (*Glossary)(nil).Apply("クラウド"); out != "クラウド" || terms != nil {
		t.Errorf("nil glossary: %q %v", out, terms)
	}
}

func TestMaxTerms(t *testing.T) {
	var terms []Term
	text := ""
	for i := range MaxTerms + 3 {
		w := string(rune('a'+i)) + "x"
		terms = append(terms, Term{Src: w, Tgt: w})
		text += w + " "
	}
	if _, got := NewGlossary(terms, false).Apply(text); len(got) != MaxTerms {
		t.Errorf("%d terms", len(got))
	}
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenes.json")
	if ss, err := Load(path); err != nil || len(ss) != 1 || ss[0].ID != "" {
		t.Fatalf("missing file: %v %v", ss, err)
	}
	if err := Save(path, []Scene{{ID: "ff7", Name: "FF7", Terms: []Term{{Src: "クラウド", Tgt: "克劳德"}}}, {Src: "ja", Tgt: "zh"}}); err != nil {
		t.Fatal(err)
	}
	ss, err := Load(path)
	if err != nil || len(ss) != 2 || ss[0].ID != "" || ss[0].Src != "ja" || ss[1].Terms[0].Tgt != "克劳德" {
		t.Errorf("%+v %v", ss, err)
	}
	c := Clone(ss)
	c[1].Terms[0].Tgt = "云"
	if ss[1].Terms[0].Tgt != "克劳德" {
		t.Error("a clone shares terms")
	}
}

func TestNames(t *testing.T) {
	g := NewGlossary([]Term{{Src: "葦名", Tgt: "苇名", Learned: true}, {Src: "一心", Tgt: "一心"}, {Src: "一心", Tgt: "一心", Learned: true}, {Src: "九郎"}}, false)
	if got := g.Names(); !slices.Equal(got, []string{"一心", "葦名"}) {
		t.Errorf("%q", got)
	}
	if (*Glossary)(nil).Names() != nil {
		t.Error("a nil glossary's names")
	}
}

func TestGeneral(t *testing.T) {
	g := NewGlossary([]Term{{Src: "Claude", Tgt: "Claude", Heard: []string{"cloud"}}}, true)
	if got := g.Names(); got != nil {
		t.Errorf("names %q", got)
	}
	if text, terms := g.Apply("Private Cloud Compute"); text != "Private Cloud Compute" || terms != nil {
		t.Errorf("%q %v", text, terms)
	}
	if _, terms := g.Apply("Claude Code"); len(terms) != 1 {
		t.Errorf("%v", terms)
	}
}

func TestLoadEngines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenes.json")
	old := `[{"ID":"","Name":"","About":"","Src":"ja","Tgt":"zh","Terms":null},{"ID":"ff7","Name":"FF7","About":"","Src":"ja","Tgt":"zh","Terms":null}]`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	ss, err := Load(path)
	if err != nil || len(ss) != 2 || ss[0].ASR != "" || ss[0].MT != "" || ss[1].ASR != "" || ss[1].MT != "" {
		t.Fatalf("an older file: %+v %v", ss, err)
	}
	ss[1].ASR, ss[1].MT = "sensevoice-small", "local-2b"
	if err := Save(path, ss); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Count(string(b), `"ASR"`) != 1 || strings.Count(string(b), `"MT"`) != 1 {
		t.Errorf("following the default writes no engines:\n%s", b)
	}
	if ss, err = Load(path); err != nil || ss[1].ASR != "sensevoice-small" || ss[1].MT != "local-2b" || ss[0].ASR != "" {
		t.Errorf("%+v %v", ss, err)
	}
}
