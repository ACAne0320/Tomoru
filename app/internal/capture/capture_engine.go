//go:build darwin || windows

package capture

import (
	"bytes"
	"context"
	"image"
	"os/exec"

	"tomoru/internal/resource"
)

// selectInto has the engine's selection write the part selected to path, as PNG: nothing when
// cancelled, with Escape (on Windows, the right button too). The engine tells where the part is,
// in the screens' pixels.
func selectInto(ctx context.Context, engine func() (string, error), path string) ([]byte, image.Rectangle, error) {
	bin, err := engine()
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	cmd := exec.CommandContext(ctx, bin, "select", "--out", path)
	resource.Hide(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return append(out, stderr.Bytes()...), area(out), err
}
