//go:build !windows

package models

import "golang.org/x/sys/unix"

var errOtherDisk error = unix.EXDEV
