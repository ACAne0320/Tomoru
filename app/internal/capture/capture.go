package capture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
)

func Selector(engine func() (string, error)) func(context.Context) ([]byte, image.Rectangle, error) {
	return func(ctx context.Context) ([]byte, image.Rectangle, error) {
		dir, err := os.MkdirTemp("", "tomoru-capture-")
		if err != nil {
			return nil, image.Rectangle{}, err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "selection.png")
		out, at, err := selectInto(ctx, engine, path)
		pic, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			return pic, at, nil
		case err != nil:
			return nil, image.Rectangle{}, fmt.Errorf("capture: %v: %s", err, out)
		case errors.Is(readErr, fs.ErrNotExist):
			return nil, image.Rectangle{}, nil
		}
		return nil, image.Rectangle{}, readErr
	}
}

func area(out []byte) image.Rectangle {
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		var e struct {
			Type                string
			X, Y, Width, Height int
		}
		if json.Unmarshal(lines.Bytes(), &e) == nil && e.Type == "area" {
			return image.Rect(e.X, e.Y, e.X+e.Width, e.Y+e.Height)
		}
	}
	return image.Rectangle{}
}
