package scene

import (
	"encoding/json"
	"errors"
	"io/fs"

	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"tomoru/internal/atomicfile"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"tomoru/internal/translate"
)

type Scene struct {
	ID       string
	Name     string
	About    string
	Src, Tgt string
	ASR      string `json:",omitempty"`
	MT       string `json:",omitempty"`
	Terms    []Term
	Rejected []string `json:",omitempty"`
}

type Term struct {
	Src     string
	Tgt     string
	Heard   []string `json:",omitempty"`
	Learned bool     `json:"Auto,omitempty"`
	Added   time.Time
}

const GeneralID = ""

func (s Scene) IsGeneral() bool { return s.ID == GeneralID }

func Load(path string) ([]Scene, error) {
	var ss []Scene
	var err error
	if b, rerr := os.ReadFile(path); rerr == nil {
		err = json.Unmarshal(b, &ss)
	} else if !errors.Is(rerr, fs.ErrNotExist) {
		err = rerr
	}
	if i := slices.IndexFunc(ss, Scene.IsGeneral); i > 0 {
		g := ss[i]
		ss = slices.Insert(slices.Delete(ss, i, i+1), 0, g)
	} else if i < 0 {
		ss = slices.Insert(ss, 0, Scene{})
	}
	return ss, err
}

func Save(path string, ss []Scene) error {
	b, err := json.MarshalIndent(ss, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, b, 0o644)
}

func Clone(ss []Scene) []Scene {
	ss = slices.Clone(ss)
	for i := range ss {
		ss[i].Rejected = slices.Clone(ss[i].Rejected)
		ss[i].Terms = slices.Clone(ss[i].Terms)
		for j := range ss[i].Terms {
			ss[i].Terms[j].Heard = slices.Clone(ss[i].Terms[j].Heard)
		}
	}
	return ss
}

const MaxTerms = 12

type Glossary struct {
	entries []entry
	general bool
}

type entry struct {
	tgt   string
	src   form
	heard []form
}

type form interface {
	name() string
	in(text, normalized string) bool
	replace(text, to string) string
}

type wordForm struct {
	text string
	word *regexp.Regexp
}

type runForm struct {
	text, norm string
}

func newForm(s string) form {
	if latin(s) {
		return wordForm{s, regexp.MustCompile(`(?i)` + regexp.QuoteMeta(s))}
	}
	return runForm{s, normalize(s)}
}

func (f wordForm) name() string { return f.text }

func (f runForm) name() string { return f.text }

func (f wordForm) in(text, _ string) bool { return len(f.words(text)) > 0 }

func (f runForm) in(_, normalized string) bool {
	return f.norm != "" && strings.Contains(normalized, f.norm)
}

func (f runForm) replace(text, to string) string { return strings.ReplaceAll(text, f.text, to) }

func (f wordForm) words(text string) [][]int {
	var out [][]int
	for _, m := range f.word.FindAllStringIndex(text, -1) {
		before, _ := utf8.DecodeLastRuneInString(text[:m[0]])
		after, _ := utf8.DecodeRuneInString(text[m[1]:])
		if !wordy(before) && !wordy(after) {
			out = append(out, m)
		}
	}
	return out
}

func wordy(r rune) bool { return unicode.Is(unicode.Latin, r) || unicode.IsDigit(r) }

func Has(text, name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && newForm(name).in(text, normalize(text))
}

func Distinct(heard string) bool {
	n := len([]rune(normalize(heard)))
	if latin(heard) {
		return n >= 3
	}
	return n >= 2
}

func Same(a, b string) bool { return normalize(a) == normalize(b) }

func latin(s string) bool {
	has := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Latin, r):
			has = true
		case r < 0x80 || unicode.IsSpace(r):
		default:
			return false
		}
	}
	return has
}

func normalize(s string) string {
	s = strings.ToLower(norm.NFKC.String(s))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("・·･-‐－", r) {
			return -1
		}
		return r
	}, s)
}

func NewGlossary(terms []Term, general bool) *Glossary {
	g := &Glossary{general: general}
	for _, own := range []bool{true, false} {
		for _, t := range terms {
			src, tgt := strings.TrimSpace(t.Src), strings.TrimSpace(t.Tgt)
			if t.Learned == own || src == "" || tgt == "" {
				continue
			}
			e := entry{tgt: tgt, src: newForm(src)}
			for _, h := range t.Heard {
				if h = strings.TrimSpace(h); h != src && Distinct(h) && !general {
					e.heard = append(e.heard, newForm(h))
				}
			}
			g.entries = append(g.entries, e)
		}
	}
	return g
}

func (g *Glossary) Names() []string {
	if g == nil || g.general {
		return nil
	}
	names := make([]string, 0, len(g.entries))
	for _, e := range g.entries {
		if !slices.Contains(names, e.src.name()) {
			names = append(names, e.src.name())
		}
	}
	return names
}

func (g *Glossary) Apply(text string) (string, []translate.Term) {
	if g == nil {
		return text, nil
	}
	for _, e := range g.entries {
		for _, h := range e.heard {
			text = h.replace(text, e.src.name())
		}
	}
	n := normalize(text)
	var terms []translate.Term
	for _, e := range g.entries {
		if len(terms) == MaxTerms {
			break
		}
		if e.src.in(text, n) {
			terms = append(terms, translate.Term{Src: e.src.name(), Tgt: e.tgt})
		}
	}
	return text, terms
}

func (f wordForm) replace(text, to string) string {
	var b strings.Builder
	last := 0
	for _, m := range f.words(text) {
		b.WriteString(text[last:m[0]])
		b.WriteString(to)
		last = m[1]
	}
	return b.String() + text[last:]
}
