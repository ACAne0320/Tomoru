package capture

import (
	"bytes"
	"context"
	"image"
	"os/exec"
	"syscall"
	"time"

	"tomoru/internal/resource"
)

// selectInto has the engine's selection write the part selected to path, as PNG: nothing when
// cancelled, with Escape (on Windows, the right button too). The engine tells where the part is,
// in the screens' pixels. Linux's selects over a screenshot, the Screenshot portal's under
// Wayland.
func selectInto(ctx context.Context, engine func() (string, error), path string) ([]byte, image.Rectangle, error) {
	bin, err := engine()
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	cmd := exec.CommandContext(ctx, bin, "select", "--out", path)
	resource.Hide(cmd)
	// Ended, it is asked to stop, so that it ends the selection it runs (macOS's screencapture);
	// Windows has no SIGTERM, and its selection is the engine's own window.
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 2 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return append(out, stderr.Bytes()...), area(out), err
}
