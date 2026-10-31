package main

import (
	"fmt"
	"os"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// senseVoice is SenseVoice through sherpa-onnx's offline recognizer.
type senseVoice struct{ r *sherpa.OfflineRecognizer }

func newSenseVoice(dir, lang string, threads int) (*senseVoice, error) {
	model, tokens := filepath.Join(dir, "model.int8.onnx"), filepath.Join(dir, "tokens.txt")
	for _, p := range []string{model, tokens} {
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
	}
	c := sherpa.OfflineRecognizerConfig{}
	c.FeatConfig = sherpa.FeatureConfig{SampleRate: rate, FeatureDim: 80}
	c.ModelConfig.SenseVoice = sherpa.OfflineSenseVoiceModelConfig{Model: model, Language: lang, UseInverseTextNormalization: 1}
	c.ModelConfig.Tokens = tokens
	c.ModelConfig.NumThreads = threads
	c.ModelConfig.Provider = "cpu" // CoreML was no faster (docs/local-asr.md)
	c.DecodingMethod = "greedy_search"
	r := sherpa.NewOfflineRecognizer(&c)
	if r == nil {
		return nil, fmt.Errorf("sherpa-onnx could not load %s", dir)
	}
	return &senseVoice{r}, nil
}

func (v *senseVoice) Decode(samples []float32) []Piece {
	if len(samples) == 0 {
		return nil
	}
	s := sherpa.NewOfflineStream(v.r)
	defer sherpa.DeleteOfflineStream(s)
	s.AcceptWaveform(rate, samples)
	v.r.Decode(s)
	res := s.GetResult()
	out := make([]Piece, 0, len(res.Tokens))
	for i, t := range res.Tokens {
		var at float64
		if i < len(res.Timestamps) {
			at = float64(res.Timestamps[i])
		}
		out = append(out, Piece{Text: t, At: at})
	}
	return out
}
