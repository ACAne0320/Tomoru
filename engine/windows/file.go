package main

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

// Hearing a file (`listen --file`): a recording played as if the system played it, at its own
// pace, for benchmarks and tests that need neither speakers nor capture.

// FileSource delivers a WAV file's audio in real time, 100 ms at a time; after its end, silence,
// as capture does once nothing plays.
type FileSource struct {
	path    string
	samples []int16 // the file at 16 kHz mono
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
	log.Printf("file %s: %.1f s", filepath.Base(f.path), float64(len(f.samples))/rate)
	go func() {
		started := time.Now()
		var delivered int64
		for range time.Tick(100 * time.Millisecond) {
			due := int64(time.Since(started).Seconds() * rate)
			if due <= delivered {
				continue
			}
			buf := make([]int16, due-delivered)
			if delivered < int64(len(f.samples)) {
				copy(buf, f.samples[delivered:]) // past the end, silence
			}
			delivered = due
			feed(buf)
		}
	}()
	return nil
}

// decodeWAV reads a WAV file of PCM, 8 to 32 bits, or of 32-bit floats, as 16 kHz mono.
func decodeWAV(b []byte) ([]int16, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("not a WAV file")
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
				return nil, errors.New("short fmt chunk")
			}
			format, channels = binary.LittleEndian.Uint16(body), binary.LittleEndian.Uint16(body[2:])
			srcRate, bits = binary.LittleEndian.Uint32(body[4:]), binary.LittleEndian.Uint16(body[14:])
			if format == 0xfffe && len(body) >= 26 { // WAVE_FORMAT_EXTENSIBLE: its subformat
				format = binary.LittleEndian.Uint16(body[24:])
			}
		case "data":
			data = body
		}
		next := size + size%2 // chunks are padded to even sizes
		if next > len(p) {
			break
		}
		p = p[next:]
	}
	if channels == 0 || srcRate == 0 || data == nil {
		return nil, io.ErrUnexpectedEOF
	}
	width := int(bits) / 8
	if format == 1 && (width < 1 || width > 4) || format == 3 && width != 4 || format != 1 && format != 3 {
		return nil, fmt.Errorf("unsupported format %d, %d bits", format, bits)
	}
	frame := width * int(channels)
	mono := make([]float64, len(data)/frame)
	for i := range mono {
		var sum float64
		for c := range int(channels) {
			s := data[i*frame+c*width:]
			switch {
			case format == 3:
				sum += float64(math.Float32frombits(binary.LittleEndian.Uint32(s)))
			case width == 1:
				sum += (float64(s[0]) - 128) / 128
			case width == 2:
				sum += float64(int16(binary.LittleEndian.Uint16(s))) / 32768
			case width == 3:
				sum += float64(int32(uint32(s[0])<<8|uint32(s[1])<<16|uint32(s[2])<<24)) / (1 << 31)
			default:
				sum += float64(int32(binary.LittleEndian.Uint32(s))) / (1 << 31)
			}
		}
		mono[i] = sum / float64(channels)
	}
	return resample(mono, float64(srcRate)), nil
}

// resample makes samples at from Hz 16 kHz 16-bit: higher rates by averaging the samples each
// one spans, which keeps most of what is over 8 kHz out; lower ones linearly. Enough for speech.
func resample(x []float64, from float64) []int16 {
	n := int(float64(len(x)) * rate / from)
	out := make([]int16, n)
	step := from / rate
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
