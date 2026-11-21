//go:build !windows

package models

import (
	"errors"
	"syscall"
)

// crossDevice reports a rename onto another disk: EXDEV.
func crossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
