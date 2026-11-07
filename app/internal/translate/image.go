package translate

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

// Translating the text in a picture (docs/image-translation.md): Index-Translate reads it itself, with no
// OCR before it. The picture and the prompt go in one turn, and the translation comes back.

// MaxSide is the longest side, in pixels, of a picture sent: Index-Translate's advice. A larger
// one is scaled down.
const MaxSide = 2048

// MinSide is the longest side, in pixels, below which a picture sent is scaled up, two or three
// times: a selection on a screen at 100% scale (Windows' usual), its text 12 pixels high, came back
// with words lost, Korean garbled and Chinese said twice (2026-10-30, the guide's sample); twice
// as large, right.
const MinSide = 1000

// errLooping is a model saying a line over and over, which it would until it ran out of tokens.
var errLooping = errors.New("translate: the model repeats itself")

// ErrNoPictures is a picture to translate with a model whose format reads none: it is not sent.
var ErrNoPictures = errors.New("translate: the model reads no pictures")

// TranslateImage translates the text in pic, a PNG, into tgt, with the names in terms as given,
// calling onDelta with the translation so far as it streams in. A line the model repeats comes
// once, and one it keeps repeating ends the translation there.
func (e *Endpoint) TranslateImage(ctx context.Context, pic []byte, tgt string, terms []Term, onDelta func(string)) (string, error) {
	if !e.format().ReadsPictures() {
		return "", ErrNoPictures
	}
	pic, err := fit(pic, cmp.Or(e.ImageSide, MaxSide))
	if err != nil {
		return "", err
	}
	parts := []map[string]any{
		{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pic)}},
		{"type": "text", "text": e.format().image(tgt, terms)},
	}
	body := e.request([]message{{"user", parts}}, 0, 2048)
	s, err := e.readImage(ctx, body, onDelta)
	if errors.Is(err, ErrStalled) {
		// Through a proxy, one connection in five to the cloud took a picture and answered
		// nothing, not even a ping, until the server closed it minutes later; HTTP/2 sent each
		// picture asked again on it (2026-11-01). Its first words come in 2 s, 7.5 s at most.
		log.Printf("translate: no words in %v, asking again on a new connection", e.ImageWait)
		c := e.anew(time.Minute)
		defer c.CloseIdleConnections()
		// The one stalled goes too, for the pictures after: idle once its request is done with.
		defer cmp.Or(e.Client, &http.Client{}).CloseIdleConnections()
		s, err = e.readImage(context.WithValue(ctx, clientKey{}, c), body, onDelta)
	}
	return Dedupe(s), err
}

// readImage reads a picture's translation, given up as ErrStalled when its first words have not
// come within e.ImageWait.
func (e *Endpoint) readImage(ctx context.Context, body map[string]any, onDelta func(string)) (string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	wait := time.AfterFunc(cmp.Or(e.ImageWait, time.Minute), func() { cancel(ErrStalled) })
	defer wait.Stop()
	s, _, err := e.complete(ctx, body, time.Minute, func(s string) {
		wait.Stop()
		if looping(s) {
			cancel(errLooping)
		} else if onDelta != nil {
			onDelta(Dedupe(s))
		}
	})
	switch cause := context.Cause(ctx); {
	case err == nil:
	case errors.Is(cause, errLooping):
		err = nil
	case errors.Is(cause, ErrStalled):
		err = ErrStalled
	}
	return s, err
}

// anew is a client like the one e sends with, on connections of its own, within timeout unless
// e.Client says.
func (e *Endpoint) anew(timeout time.Duration) *http.Client {
	c := http.Client{Timeout: timeout}
	if e.Client != nil {
		c = *e.Client
	}
	t := c.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	if t, ok := t.(*http.Transport); ok {
		c.Transport = t.Clone()
	}
	return &c
}

// looping tells that the last line ended in s was said three times in a row: the 2B model, asked
// for English, at times says a name and nothing else, line after line.
func looping(s string) bool {
	lines := strings.Split(s, "\n")
	lines = lines[:len(lines)-1] // the one not ended yet
	n, last := 0, ""
	for _, l := range slices.Backward(lines) {
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			continue
		case last == "":
			last, n = t, 1
		case t == last:
			n++
		default:
			return false
		}
		if n == 3 {
			return true
		}
	}
	return false
}

// chatImagePrompt asks a chat model for the text of a picture in tgt. Its names are not known
// before, so the glossary goes whole.
func chatImagePrompt(tgt string, terms []Term) string {
	s := fmt.Sprintf("Translate all the text in the image into %s. Output only the translation.", enName(tgt))
	if len(terms) > 0 {
		pairs := make([]string, len(terms))
		for i, t := range terms {
			pairs[i] = t.Src + " → " + t.Tgt
		}
		s += " Render these names as given: " + strings.Join(pairs, "; ") + "."
	}
	return s
}

// indexImagePrompt asks Index-Translate for the text of a picture in tgt, the glossary whole, in
// its form of one (instTrans) as for subtitles.
func indexImagePrompt(tgt string, terms []Term) string {
	if len(terms) == 0 {
		return fmt.Sprintf("请将图片中的文本翻译为%s，直接输出翻译结果。", zhName(tgt))
	}
	pairs := make([]string, len(terms))
	for i, t := range terms {
		pairs[i] = t.Src + "→" + t.Tgt
	}
	return fmt.Sprintf("请将图片中的文本翻译为%s，并且严格遵循所有约束要求。\n\n【约束要求】\n1. 【硬性要求】专名/术语对照: %s\n只输出译文，不要有任何额外说明。",
		zhName(tgt), strings.Join(pairs, "、"))
}

// fit scales pic, a PNG, down to max pixels on its longest side, or one shorter than MinSide up
// two or three times, as far as max; one between goes as it is.
func fit(pic []byte, max int) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pic))
	if err != nil {
		return nil, fmt.Errorf("translate: the picture: %w", err)
	}
	long := cfg.Width
	if cfg.Height > long {
		long = cfg.Height
	}
	to := long
	switch {
	case long > max:
		to = max
	case long > 0 && long < MinSide:
		to = min(long*min((MinSide+long-1)/long, 3), max)
	}
	if to == long {
		return pic, nil
	}
	src, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil, fmt.Errorf("translate: the picture: %w", err)
	}
	w, h := cfg.Width*to/long, cfg.Height*to/long
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil) // keeps small text legible
	var b bytes.Buffer
	if err := png.Encode(&b, dst); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Dedupe drops a line that says again the line before it, as the model does at times with a
// status bar or a chat's last line.
func Dedupe(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	last := ""
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" && t == last {
			continue
		}
		if t != "" {
			last = t
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
