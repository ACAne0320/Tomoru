// Package capture has the user select a part of the screen, through the engine's select: with the
// system's own selection where it has one, the crosshair of ⌃⇧⌘4 on macOS, else the engine's own
// (Windows); the engine tells where the part is. On macOS the capture needs Screen Recording, which
// the app asks for (the engine's screen-access); without it the system asks as the selection ends,
// and nothing is captured.
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

// Selector has the user select a part of the screen, and gives it as a PNG, with where it is in
// the screens' pixels where the system tells (Windows); nil if the user cancelled, with Escape. On
// Windows the engine, which engine finds, selects.
func Selector(engine func() (string, error)) func(context.Context) ([]byte, image.Rectangle, error) {
	return func(ctx context.Context) ([]byte, image.Rectangle, error) {
		dir, err := os.MkdirTemp("", "tomoru-capture-")
		if err != nil {
			return nil, image.Rectangle{}, err
		}
		defer os.RemoveAll(dir) // the picture is not kept
		path := filepath.Join(dir, "selection.png")
		out, at, err := selectInto(ctx, engine, path)
		pic, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			return pic, at, nil
		case err != nil:
			return nil, image.Rectangle{}, fmt.Errorf("capture: %v: %s", err, out) // as without Screen Recording
		case errors.Is(readErr, fs.ErrNotExist):
			return nil, image.Rectangle{}, nil // cancelled
		}
		return nil, image.Rectangle{}, readErr
	}
}

// area is where the engine's output says the selection is, if it says.
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
