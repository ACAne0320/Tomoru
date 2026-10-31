// Package scene is the user's translation scenes: what they watch, each with its own languages
// and glossary of names (docs/scenes.md). A line goes to the translator with the terms of the
// glossary it concerns, and with the names it was heard wrong as written right.
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

// Scene is a translation scene. The general one, with no ID, is always first of a file's: it has
// no name or description, and its languages are those of translating outside the others.
type Scene struct {
	ID       string
	Name     string
	About    string // what is watched: sent with each sentence translated, and to the model that learns names
	Src, Tgt string // the languages, "" until the scene is first used
	Terms    []Term
	// Rejected are names suggested that the user did not take, as heard: they are not suggested
	// again.
	Rejected []string `json:",omitempty"`
}

// Term is a name of the glossary: as said, its translation, and how recognition heard it wrong.
type Term struct {
	Src   string
	Tgt   string
	Heard []string `json:",omitempty"`
	Auto  bool     `json:",omitempty"` // learned; the user's own are not
	Added time.Time
}

// Load reads the scenes at path, the general one first. A missing file has the general one only.
func Load(path string) ([]Scene, error) {
	var ss []Scene
	var err error
	if b, rerr := os.ReadFile(path); rerr == nil {
		err = json.Unmarshal(b, &ss)
	} else if !errors.Is(rerr, fs.ErrNotExist) {
		err = rerr
	}
	if i := slices.IndexFunc(ss, func(s Scene) bool { return s.ID == "" }); i > 0 {
		g := ss[i]
		ss = slices.Insert(slices.Delete(ss, i, i+1), 0, g)
	} else if i < 0 {
		ss = slices.Insert(ss, 0, Scene{})
	}
	return ss, err
}

// Save writes the scenes to path.
func Save(path string, ss []Scene) error {
	b, err := json.MarshalIndent(ss, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, b, 0o644)
}

// Clone is a copy of ss that shares nothing with it.
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

// MaxTerms is the most terms a line goes to the translator with.
const MaxTerms = 12

// Glossary is a scene's terms, ready to apply to lines. It does not change, and may be used from
// several goroutines.
type Glossary struct {
	entries []entry
}

type entry struct {
	tgt   string
	src   form
	heard []form
}

// form is a way a name is written, and how to find it: a Latin one as a whole word, whatever its
// case; another within the text, as written alike once normalized.
type form struct {
	text string
	word *regexp.Regexp // Latin: the form in any case, to check as a whole word
	norm string         // other
}

func newForm(s string) form {
	f := form{text: s}
	if latin(s) {
		f.word = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(s))
	} else {
		f.norm = normalize(s)
	}
	return f
}

// in tells whether the form is in text, whose normalized form is n.
func (f form) in(text, n string) bool {
	if f.word != nil {
		return len(f.words(text)) > 0
	}
	return f.norm != "" && strings.Contains(n, f.norm)
}

// words are where a Latin form is in text as a whole word.
func (f form) words(text string) [][]int {
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

// wordy tells whether r would go on a Latin word: a Latin letter or a digit. Another script's
// letter does not: "outra项目" has the word "outra".
func wordy(r rune) bool { return unicode.Is(unicode.Latin, r) || unicode.IsDigit(r) }

// Has tells whether name is in text, as a glossary finds it: a Latin name as a whole word, in any
// case; another as written alike once normalized.
func Has(text, name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && newForm(name).in(text, normalize(text))
}

// Distinct tells whether a way a name was heard is long enough to be told from other words: three
// letters in Latin, two characters else. Written right, "F" would be every F.
func Distinct(heard string) bool {
	n := len([]rune(normalize(heard)))
	if latin(heard) {
		return n >= 3
	}
	return n >= 2
}

// Same tells whether a and b are the same name, but for width, case, spaces and dots.
func Same(a, b string) bool { return normalize(a) == normalize(b) }

// latin tells whether s is written in Latin letters, with digits and marks between them.
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

// normalize makes alike what differs in width, case, spaces and the dots between a name's parts:
// "ファイナルファンタジー VII" and "ファイナルファンタジーⅦ".
func normalize(s string) string {
	s = strings.ToLower(norm.NFKC.String(s))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("・·･-‐－", r) {
			return -1
		}
		return r
	}, s)
}

// NewGlossary makes the glossary of terms: the user's own first, as they go before those learned
// when a line concerns more than MaxTerms.
func NewGlossary(terms []Term) *Glossary {
	g := &Glossary{}
	for _, own := range []bool{true, false} {
		for _, t := range terms {
			src, tgt := strings.TrimSpace(t.Src), strings.TrimSpace(t.Tgt)
			if t.Auto == own || src == "" || tgt == "" {
				continue
			}
			e := entry{tgt: tgt, src: newForm(src)}
			for _, h := range t.Heard {
				if h = strings.TrimSpace(h); h != src && Distinct(h) {
					e.heard = append(e.heard, newForm(h))
				}
			}
			g.entries = append(g.entries, e)
		}
	}
	return g
}

// Names are the names of the glossary as said, the user's own first: what recognition is told to
// expect. Nil for a nil glossary.
func (g *Glossary) Names() []string {
	if g == nil {
		return nil
	}
	names := make([]string, 0, len(g.entries))
	for _, e := range g.entries {
		if !slices.Contains(names, e.src.text) {
			names = append(names, e.src.text)
		}
	}
	return names
}

// Apply writes in text the names heard wrong as said, and gives the terms the text then concerns,
// MaxTerms at most. A nil glossary leaves text as it is, with none.
func (g *Glossary) Apply(text string) (string, []translate.Term) {
	if g == nil {
		return text, nil
	}
	for _, e := range g.entries {
		for _, h := range e.heard {
			text = h.replace(text, e.src.text)
		}
	}
	n := normalize(text)
	var terms []translate.Term
	for _, e := range g.entries {
		if len(terms) == MaxTerms {
			break
		}
		if e.src.in(text, n) {
			terms = append(terms, translate.Term{Src: e.src.text, Tgt: e.tgt})
		}
	}
	return text, terms
}

// replace writes to for each time the form is in text as it is written: a Latin one as a whole
// word.
func (f form) replace(text, to string) string {
	if f.word == nil {
		return strings.ReplaceAll(text, f.text, to)
	}
	var b strings.Builder
	last := 0
	for _, m := range f.words(text) {
		b.WriteString(text[last:m[0]])
		b.WriteString(to)
		last = m[1]
	}
	return b.String() + text[last:]
}
