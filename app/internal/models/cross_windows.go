package models

import (
	"errors"

	"golang.org/x/sys/windows"
)

// crossDevice reports a rename onto another disk: MoveFileEx's ERROR_NOT_SAME_DEVICE.
func crossDevice(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
