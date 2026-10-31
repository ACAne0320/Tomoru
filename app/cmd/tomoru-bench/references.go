package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tomoru/internal/lang"
	"tomoru/internal/translate"
)

// translateRefs translates a case's reference, sentence by sentence.
func translateRefs(c Case, media string, tr translate.Translator, about string) Result {
	tgt, tgtRef := target(c)
	r := Result{ID: c.ID, Lang: c.Lang, Scene: c.Scene, Tgt: tgt, TgtRef: tgtRef}
	cues, err := readSRT(filepath.Join(media, c.ID+".ref.srt"))
	if err != nil {
		r.Err = err.Error()
		return r
	}
	if len(cues) > 0 {
		r.Audio = cues[len(cues)-1].end
	}
	for _, src := range sentences(cues, c.Lang) {
		t0 := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		out, err := tr.Translate(ctx, translate.Request{Src: c.Lang, Tgt: tgt, Text: src, About: about}, nil)
		cancel()
		s := Sentence{Src: src, Tgt: out, Sec: round(time.Since(t0).Seconds())}
		if err != nil {
			s.Err = err.Error()
		}
		r.Sentences = append(r.Sentences, s)
	}
	return r
}

type cue struct {
	start, end float64
	text       string
}

var stamp = regexp.MustCompile(`(\d+):(\d+):(\d+),(\d+)\s*-->\s*(\d+):(\d+):(\d+),(\d+)`)

func readSRT(path string) ([]cue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []cue
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := stamp.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		n := make([]float64, 8)
		for i := range n {
			n[i], _ = strconv.ParseFloat(m[i+1], 64)
		}
		c := cue{start: n[0]*3600 + n[1]*60 + n[2] + n[3]/1000, end: n[4]*3600 + n[5]*60 + n[6] + n[7]/1000}
		var text []string
		for sc.Scan() && strings.TrimSpace(sc.Text()) != "" {
			text = append(text, sc.Text())
		}
		c.text = strings.Join(text, " ")
		out = append(out, c)
	}
	return out, sc.Err()
}

// sentences joins a reference's cues into sentences, as the segmenter would commit them: a cue
// ending a sentence, or a pause of 0.8 s, ends one, and one gets no longer than about two lines.
func sentences(cues []cue, code string) []string {
	var out []string
	var cur strings.Builder
	space := lang.Spaced(code)
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for i, c := range cues {
		if cur.Len() > 0 && space {
			cur.WriteByte(' ')
		}
		cur.WriteString(strings.TrimSpace(c.text))
		text := strings.TrimSpace(c.text)
		end := strings.ContainsAny(lastRune(text), "。！？!?.…")
		pause := i+1 < len(cues) && cues[i+1].start-c.end > 0.8
		if end || pause || len([]rune(cur.String())) > 80 {
			flush()
		}
	}
	flush()
	return out
}

func lastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[len(r)-1])
}
