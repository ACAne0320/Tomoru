package shared

import (
	"fmt"
	"os"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const (
	window   = 512
	voiceOn  = 2 * window
	voiceOff = 3 * window
)

type Detector struct {
	vad      VAD
	onChange func(on bool, t float64)
	buf      []float32
	samples  int64
	voice    bool
}

type VAD interface {
	Accept(window []float32)
	Speaking() bool
}

func NewDetector(vad VAD, onChange func(bool, float64)) *Detector {
	return &Detector{vad: vad, onChange: onChange}
}

func (d *Detector) Feed(samples []float32) {
	d.buf = append(d.buf, samples...)
	for len(d.buf) >= window {
		d.vad.Accept(d.buf[:window])
		d.buf = d.buf[window:]
		d.samples += window
		if on := d.vad.Speaking(); on != d.voice {
			d.voice = on
			run := int64(voiceOn)
			if !on {
				run = voiceOff
			}
			d.onChange(on, float64(max(0, d.samples-run))/Rate)
		}
	}
}

func Feeder(d *Detector) func([]float32) {
	c := make(chan []float32, 256)
	go func() {
		for x := range c {
			d.Feed(x)
		}
	}()
	return func(x []float32) { c <- x }
}

type silero struct{ v *sherpa.VoiceActivityDetector }

func newSilero(model string) (*silero, error) {
	if _, err := os.Stat(model); err != nil {
		return nil, err
	}
	c := sherpa.VadModelConfig{SampleRate: Rate, NumThreads: 1, Provider: "cpu"}
	c.SileroVad = sherpa.SileroVadModelConfig{Model: model, Threshold: .5, MinSpeechDuration: float32(voiceOn-window) / Rate,
		MinSilenceDuration: float32(voiceOff-window) / Rate, WindowSize: window, MaxSpeechDuration: 3600}
	v := sherpa.NewVoiceActivityDetector(&c, 60)
	if v == nil {
		return nil, fmt.Errorf("sherpa-onnx could not load %s", model)
	}
	return &silero{v}, nil
}

func (s *silero) Accept(window []float32) {
	s.v.AcceptWaveform(window)
	for !s.v.IsEmpty() {
		s.v.Pop()
	}
}

func (s *silero) Speaking() bool { return s.v.IsSpeech() }

func defaultVAD() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "vad", "silero_vad.onnx")
}
