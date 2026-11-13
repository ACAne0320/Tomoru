package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
)

// wavSeconds is how long a WAV file of ffmpeg's lasts.
func wavSeconds(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	bytesPerSec, size, _, err := wavData(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	return float64(size) / float64(bytesPerSec), nil
}

// trimWav writes the first sec seconds of src to dst.
func trimWav(src, dst string, sec float64) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	bytesPerSec, size, header, err := wavData(f)
	if err != nil {
		return err
	}
	n := min(size, int64(sec*float64(bytesPerSec))&^1)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	h := slices.Clone(header)
	binary.LittleEndian.PutUint32(h[4:], uint32(len(h)-8)+uint32(n))
	binary.LittleEndian.PutUint32(h[len(h)-4:], uint32(n))
	if _, err := out.Write(h); err != nil {
		return err
	}
	_, err = io.CopyN(out, f, n)
	return err
}

// wavData reads a WAV file's chunks up to its data, leaving f there: its bytes a second, the
// data's size, and the header up to it.
func wavData(f *os.File) (bytesPerSec uint32, size int64, header []byte, err error) {
	h := make([]byte, 12)
	if _, err = io.ReadFull(f, h); err != nil || string(h[:4]) != "RIFF" || string(h[8:]) != "WAVE" {
		return 0, 0, nil, errors.New("not a WAV file")
	}
	header = h
	for {
		ch := make([]byte, 8)
		if _, err = io.ReadFull(f, ch); err != nil {
			return 0, 0, nil, err
		}
		n := binary.LittleEndian.Uint32(ch[4:])
		header = append(header, ch...)
		if string(ch[:4]) == "data" {
			return bytesPerSec, int64(n), header, nil
		}
		body := make([]byte, n+n%2)
		if _, err = io.ReadFull(f, body); err != nil {
			return 0, 0, nil, err
		}
		if string(ch[:4]) == "fmt " {
			bytesPerSec = binary.LittleEndian.Uint32(body[8:])
		}
		header = append(header, body...)
	}
}
