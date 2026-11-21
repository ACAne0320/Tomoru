package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"tomoru/internal/lang"
	"tomoru/internal/scene"
)

const Chunk = 2 * time.Minute

type Line struct{ Src, Tgt string }

type Context struct {
	SrcLang, TgtLang string
	Name, About      string
	Terms            []scene.Term
	Rejected         []string
}

type Suggestion struct {
	Src, Tgt string
	Heard    []string
	Count    int
	Example  string
}

type Asker interface {
	Chat(ctx context.Context, system, user string, maxTokens int) (string, error)
}

func Ask(ctx context.Context, m Asker, c Context, lines []Line) ([]Suggestion, error) {
	reply, err := m.Chat(ctx, system(c), user(c, lines), 1024)
	if err != nil {
		return nil, err
	}
	found, err := parse(reply)
	if err != nil {
		return nil, err
	}
	return Filter(found, c, lines), nil
}

func langName(l string) string {
	if l == lang.Auto {
		return ""
	}
	return lang.Name(l, "zh-Hans")
}

func system(c Context) string {
	src, tgt := langName(c.SrcLang), langName(c.TgtLang)
	return fmt.Sprintf(`你帮实时字幕纠正专名。下面每行是语音识别出的%[1]s原文，和它当时的%[2]s译文。
只找出确实出了问题的专名（人名、角色名、作品名、地名、组织、产品名、专有术语）：
- 原文被识别成了同音、近音的错字：heard 写原文里的原样（只写专名本身，不带助词、词尾），src 写正确写法；
- 原文对了但译文把它译错了（例如把角色名按字面意思翻译）：heard 和 src 都写原样。
tgt 写通行的%[2]s译名（官方译名优先）。已经译对的不要写，普通词不要写，只差大小写、空格、标点的不要写。没把握就不要写，sure 表示你是否确信。
只输出 JSON：{"terms":[{"heard":"...","src":"...","tgt":"...","sure":true}]}`, src, tgt)
}

func user(c Context, lines []Line) string {
	var b strings.Builder
	about := strings.TrimSpace(strings.TrimSpace(c.Name) + "\n" + strings.TrimSpace(c.About))
	if about == "" {
		about = "（用户没说）"
	}
	fmt.Fprintf(&b, "在看：%s\n\n已有对照表：\n", about)
	if len(c.Terms) == 0 {
		b.WriteString("（空）\n")
	}
	for _, t := range c.Terms {
		fmt.Fprintf(&b, "%s → %s\n", t.Src, t.Tgt)
	}
	if len(c.Rejected) > 0 {
		fmt.Fprintf(&b, "\n用户不要的建议，不要再写：%s\n", strings.Join(c.Rejected, "、"))
	}
	b.WriteString("\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "原文：%s\n", l.Src)
		if l.Tgt != "" {
			fmt.Fprintf(&b, "译文：%s\n", l.Tgt)
		}
	}
	return b.String()
}

type found struct {
	Heard, Src, Tgt string
	Sure            bool
}

var object = regexp.MustCompile(`(?s)\{.*\}`)

func parse(reply string) ([]found, error) {
	var v struct {
		Terms []found `json:"terms"`
	}
	m := object.FindString(reply)
	if m == "" {
		return nil, fmt.Errorf("learn: no JSON in the reply: %.80q", reply)
	}
	if err := json.Unmarshal([]byte(m), &v); err != nil {
		return nil, fmt.Errorf("learn: %w: %.80q", err, reply)
	}
	return v.Terms, nil
}

func withParticle(heard, src string) bool { return strings.HasPrefix(norm(heard), norm(src)) }

func translatedRight(in []Line, tgt string) bool {
	return !slices.ContainsFunc(in, func(l Line) bool { return !strings.Contains(norm(l.Tgt), norm(tgt)) })
}

func Filter(fs []found, c Context, lines []Line) []Suggestion {
	var out []Suggestion
	for _, f := range fs {
		h, src, tgt := strings.TrimSpace(f.Heard), strings.TrimSpace(f.Src), strings.TrimSpace(f.Tgt)
		if !f.Sure || h == "" || src == "" || tgt == "" {
			continue
		}
		if scene.Same(h, src) || withParticle(h, src) {
			h = src
		}
		var in []Line
		for _, l := range lines {
			if scene.Has(l.Src, h) {
				in = append(in, l)
			}
		}
		if len(in) == 0 || h != src && !scene.Distinct(h) || rejected(c, h) || mine(c, src) {
			continue
		}
		if h == src && translatedRight(in, tgt) {
			continue
		}
		if have(c, src, tgt, h) {
			continue
		}
		s := Suggestion{Src: src, Tgt: tgt, Count: len(in), Example: in[0].Src}
		if h != src {
			s.Heard = []string{h}
		}
		out = Merge(out, s)
	}
	return out
}

func norm(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }

func rejected(c Context, h string) bool {
	return slices.ContainsFunc(c.Rejected, func(r string) bool { return scene.Same(r, h) })
}

func mine(c Context, src string) bool {
	return slices.ContainsFunc(c.Terms, func(t scene.Term) bool { return !t.Auto && scene.Same(t.Src, src) })
}

func have(c Context, src, tgt, h string) bool {
	return slices.ContainsFunc(c.Terms, func(t scene.Term) bool {
		return scene.Same(t.Src, src) && t.Tgt == tgt && (h == src || slices.ContainsFunc(t.Heard, func(x string) bool { return scene.Same(x, h) }))
	})
}

func Merge(ss []Suggestion, s Suggestion) []Suggestion {
	i := slices.IndexFunc(ss, func(o Suggestion) bool { return scene.Same(o.Src, s.Src) })
	if i < 0 {
		return append(ss, s)
	}
	for _, h := range s.Heard {
		if !slices.Contains(ss[i].Heard, h) {
			ss[i].Heard = append(ss[i].Heard, h)
		}
	}
	ss[i].Count += s.Count
	return ss
}
