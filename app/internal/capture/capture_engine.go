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

func selectInto(ctx context.Context, engine func() (string, error), path string) ([]byte, image.Rectangle, error) {
	bin, err := engine()
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	cmd := exec.CommandContext(ctx, bin, "select", "--out", path)
	resource.Hide(cmd)
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
