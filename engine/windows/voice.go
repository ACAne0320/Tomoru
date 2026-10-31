package main

import (
	"fmt"
	"os"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Voice activity: Silero VAD, through sherpa-onnx, on the audio sent, so that the app's cloud
// recognizer hangs up while nobody speaks. It decides as the macOS engine's does: in windows of
// 32 ms, voice starts after 64 ms over 0.5 and stops after 96 ms under 0.35 (sherpa-onnx's end
// threshold is 0.15 under its start's), each at the start of those.

const (
	window   = 512        // samples the detector decides on: 32 ms
	voiceOn  = 2 * window // the run of windows over the threshold that starts voice
	voiceOff = 3 * window // and under, that stops it
)

// Detector says when voice starts and stops, in seconds of the audio fed to it.
type Detector struct {
	vad      VAD
	onChange func(on bool, t float64)
	buf      []float32
	samples  int64 // the samples decided on
	voice    bool
}

// VAD tells whether a voice speaks, from audio fed to it a window at a time.
type VAD interface {
	Accept(window []float32)
	Speaking() bool
}

func NewDetector(vad VAD, onChange func(bool, float64)) *Detector {
	return &Detector{vad: vad, onChange: onChange}
}

// Feed takes samples at 16 kHz, after those before.
func (d *Detector) Feed(samples []float32) {
	d.buf = append(d.buf, samples...)
	for len(d.buf) >= window {
		d.vad.Accept(d.buf[:window])
		d.buf = d.buf[window:]
		d.samples += window
		if on := d.vad.Speaking(); on != d.voice {
			d.voice = on
			// The run that decided it started that much before.
			run := int64(voiceOn)
			if !on {
				run = voiceOff
			}
			d.onChange(on, float64(max(0, d.samples-run))/rate)
		}
	}
}

// Feeder runs a detector on a goroutine of its own, as Hearer's voice: the hearing goes on while it
// decides.
func Feeder(d *Detector) func([]float32) {
	c := make(chan []float32, 256)
	go func() {
		for x := range c {
			d.Feed(x)
		}
	}()
	return func(x []float32) { c <- x }
}

// silero is Silero VAD through sherpa-onnx.
type silero struct{ v *sherpa.VoiceActivityDetector }

func newSilero(model string) (*silero, error) {
	if _, err := os.Stat(model); err != nil {
		return nil, err
	}
	c := sherpa.VadModelConfig{SampleRate: rate, NumThreads: 1, Provider: "cpu"}
	// sherpa-onnx counts a run from its first window's end: a window short of the run.
	c.SileroVad = sherpa.SileroVadModelConfig{Model: model, Threshold: .5, MinSpeechDuration: float32(voiceOn-window) / rate,
		MinSilenceDuration: float32(voiceOff-window) / rate, WindowSize: window, MaxSpeechDuration: 3600}
	v := sherpa.NewVoiceActivityDetector(&c, 60)
	if v == nil {
		return nil, fmt.Errorf("sherpa-onnx could not load %s", model)
	}
	return &silero{v}, nil
}

func (s *silero) Accept(window []float32) {
	s.v.AcceptWaveform(window)
	// Only whether a voice speaks matters: not the detector's segments.
	for !s.v.IsEmpty() {
		s.v.Pop()
	}
}

func (s *silero) Speaking() bool { return s.v.IsSpeech() }

// defaultVAD is the model beside the engine: bin/../vad/silero_vad.onnx, as the app ships it.
func defaultVAD() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "vad", "silero_vad.onnx")
}
