//go:build !darwin && !windows

package capture

import (
	"context"
	"errors"
	"image"
)

// selectInto has nothing to select with on this system.
func selectInto(context.Context, func() (string, error), string) ([]byte, image.Rectangle, error) {
	return nil, image.Rectangle{}, errors.New("no selection of the screen on this system")
}
