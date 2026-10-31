package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
)

// Decoding a stretch of speech: the app cuts speech into stretches and asks for each to be decoded
// as it grows and once more when it ends (app/internal/asr/batch, docs/recognition.md).

const rate = 16000

// Recognizer decodes a stretch of audio, 16 kHz mono: its tokens, each with its time from the
// stretch's start.
type Recognizer interface {
	Decode(samples []float32) []Piece
}

// Piece is a token a recognizer decoded, at At seconds into the stretch.
type Piece struct {
	Text string
	At   float64
}

// decode answers a decode command: the stretch's tokens, cleaned for lang, each timed from its
// start.
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
	if len(samples) > 0 {
		for _, p := range clean(rec.Decode(samples), lang) {
			at := round(p.At)
			tokens = append(tokens, Token{Text: p.Text, Start: &at})
		}
	}
	return Event{Type: "decoded", ID: c.ID, Tokens: tokens}, nil
}

// clean drops the spaces SenseVoice puts between Chinese, Cantonese and Japanese words, which they
// don't have, and the space a text starts with.
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

// round is seconds to the millisecond, as the protocol's times are.
func round(x float64) float64 {
	return float64(int64(x*1000+.5)) / 1000
}
