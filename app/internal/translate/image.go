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
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

const MaxSide = 2048

const MinSide = 1000

const imageTimeout = 2 * time.Minute

var errLooping = errors.New("translate: the model repeats itself")

var ErrNoPictures = errors.New("translate: the model reads no pictures")

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
	return Dedupe(s), err
}

func (e *Endpoint) readImage(ctx context.Context, body map[string]any, onDelta func(string)) (string, error) {
	if e.ImageWait <= 0 {
		return e.readOne(ctx, body, onDelta)
	}
	body2 := maps.Clone(body)
	var fresh *http.Client
	defer func() {
		if fresh != nil {
			fresh.CloseIdleConnections()
			cmp.Or(e.Client, &http.Client{}).CloseIdleConnections()
		}
	}()
	return hedge(ctx, e.ImageWait, 6*e.ImageWait, false, onDelta, func(ctx context.Context, n int32, onDelta func(string)) (string, error) {
		if n == 1 {
			return e.readOne(ctx, body, onDelta)
		}
		fresh = e.anew(imageTimeout)
		return e.readOne(context.WithValue(ctx, clientKey{}, fresh), body2, onDelta)
	})
}

func (e *Endpoint) readOne(ctx context.Context, body map[string]any, onDelta func(string)) (string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	s, _, err := e.complete(ctx, body, imageTimeout, func(s string) {
		if looping(s) {
			cancel(errLooping)
		} else if onDelta != nil {
			onDelta(Dedupe(s))
		}
	})
	if err != nil && errors.Is(context.Cause(ctx), errLooping) {
		err = nil
	}
	return s, err
}

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

func looping(s string) bool {
	lines := strings.Split(s, "\n")
	lines = lines[:len(lines)-1]
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
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var b bytes.Buffer
	if err := png.Encode(&b, dst); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

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
