package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
)

const rate = 16000

type Recognizer interface {
	Decode(samples []float32) Decoded
}

type Decoded struct {
	Pieces    []Piece
	HeardLang string
}

type Piece struct {
	Text      string
	AtSeconds float64
}

func decode(rec Recognizer, lang string, c Command) (Event, error) {
	b, err := base64.StdEncoding.DecodeString(c.PCM)
	if err != nil {
		return Event{}, fmt.Errorf("decode: %w", err)
	}
	if len(b)%2 != 0 {
		return Event{}, fmt.Errorf("decode: incomplete PCM sample")
	}
	samples := make([]float32, len(b)/2)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
	}
	tokens := []Token{}
	spoken := ""
	if len(samples) > 0 {
		d := rec.Decode(samples)
		if spoken = lang; lang == "auto" {
			spoken = d.HeardLang
		}
		for _, p := range clean(d.Pieces, spoken) {
			at := round(p.AtSeconds)
			tokens = append(tokens, Token{Text: p.Text, Start: &at})
		}
	}
	return Event{Type: "decoded", ID: c.ID, Tokens: tokens, Lang: spoken}, nil
}

func clean(pieces []Piece, lang string) []Piece {
	var out []Piece
	for _, p := range pieces {
		if lang == "zh" || lang == "yue" || lang == "ja" {
			p.Text = strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) {
					return -1
				}
				return r
			}, p.Text)
		}
		if len(out) == 0 {
			p.Text = strings.TrimLeft(p.Text, " ")
		}
		if p.Text != "" {
			out = append(out, p)
		}
	}
	return out
}

func round(x float64) float64 {
	return float64(int64(x*1000+.5)) / 1000
}
