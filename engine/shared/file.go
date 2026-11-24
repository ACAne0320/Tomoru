package shared

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"
)

type FileSource struct {
	path    string
	samples []int16
}

func (f *FileSource) Start(feed func([]int16)) error {
	b, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	f.samples, err = decodeWAV(b)
	if err != nil {
		return fmt.Errorf("%s: %w", f.path, err)
	}
	log.Printf("file %s: %.1f s", filepath.Base(f.path), float64(len(f.samples))/Rate)
	go func() {
		started := time.Now()
		var delivered int64
		for range time.Tick(100 * time.Millisecond) {
			due := int64(time.Since(started).Seconds() * Rate)
			if due <= delivered {
				continue
			}
			buf := make([]int16, due-delivered)
			if delivered < int64(len(f.samples)) {
				copy(buf, f.samples[delivered:])
			}
			delivered = due
			feed(buf)
		}
	}()
	return nil
}

const (
	wavePCM        = 1
	waveFloat      = 3
	waveExtensible = 0xfffe
)

func ReadWAV(path string) ([]int16, int, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	pcm, rate, channels, err := parseWAV(b)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	return pcm, rate, channels, nil
}

func parseWAV(b []byte) ([]int16, int, int, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, 0, errors.New("not a WAV file")
	}
	var format, channels, bits uint16
	var srcRate uint32
	var data []byte
	for p := b[12:]; len(p) >= 8; {
		id, size := string(p[:4]), int(binary.LittleEndian.Uint32(p[4:8]))
		p = p[8:]
		size = min(size, len(p))
		body := p[:size]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return nil, 0, 0, errors.New("short fmt chunk")
			}
			format, channels = binary.LittleEndian.Uint16(body), binary.LittleEndian.Uint16(body[2:])
			srcRate, bits = binary.LittleEndian.Uint32(body[4:]), binary.LittleEndian.Uint16(body[14:])
			if format == waveExtensible && len(body) >= 26 {
				format = binary.LittleEndian.Uint16(body[24:])
			}
		case "data":
			data = body
		}
		next := size + size%2
		if next > len(p) {
			break
		}
		p = p[next:]
	}
	if channels == 0 || srcRate == 0 || data == nil {
		return nil, 0, 0, io.ErrUnexpectedEOF
	}
	width := int(bits) / 8
	if format == wavePCM && (width < 1 || width > 4) || format == waveFloat && width != 4 || format != wavePCM && format != waveFloat {
		return nil, 0, 0, fmt.Errorf("unsupported format %d, %d bits", format, bits)
	}
	count := len(data) / width
	pcm := make([]int16, count)
	for i := range pcm {
		s := data[i*width:]
		var v float64
		switch {
		case format == waveFloat:
			v = float64(math.Float32frombits(binary.LittleEndian.Uint32(s)))
		case width == 1:
			v = (float64(s[0]) - 128) / 128
		case width == 2:
			pcm[i] = int16(binary.LittleEndian.Uint16(s))
			continue
		case width == 3:
			v = float64(int32(uint32(s[0])<<8|uint32(s[1])<<16|uint32(s[2])<<24)) / (1 << 31)
		default:
			v = float64(int32(binary.LittleEndian.Uint32(s))) / (1 << 31)
		}
		pcm[i] = int16(max(-32768, min(32767, math.Round(v*32767))))
	}
	return pcm, int(srcRate), int(channels), nil
}

func decodeWAV(b []byte) ([]int16, error) {
	pcm, srcRate, channels, err := parseWAV(b)
	if err != nil {
		return nil, err
	}
	frames := len(pcm) / channels
	mono := make([]float64, frames)
	for i := range mono {
		var sum float64
		for c := range channels {
			sum += float64(pcm[i*channels+c]) / 32768
		}
		mono[i] = sum / float64(channels)
	}
	return resample(mono, float64(srcRate)), nil
}

func resample(x []float64, from float64) []int16 {
	n := int(float64(len(x)) * Rate / from)
	out := make([]int16, n)
	step := from / Rate
	for i := range out {
		at := float64(i) * step
		j := min(int(at), len(x)-1)
		var v float64
		if step > 1 {
			end := min(int((float64(i)+1)*step), len(x))
			for _, s := range x[j:max(end, j+1)] {
				v += s
			}
			v /= float64(max(end-j, 1))
		} else {
			v = x[j]
			if j+1 < len(x) {
				v += (x[j+1] - v) * (at - float64(j))
			}
		}
		out[i] = int16(max(-32768, min(32767, math.Round(v*32767))))
	}
	return out
}
