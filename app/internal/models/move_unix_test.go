//go:build !windows

package models

import (
	"os"
	"syscall"
)

// anotherDisk is a rename across disks, as os.Rename reports it.
func anotherDisk(from, to string) error {
	return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.EXDEV}
}

// renameFail is a rename error that is not another disk.
func renameFail(kind string) error {
	var errno syscall.Errno
	switch kind {
	case "permission":
		errno = syscall.EACCES
	case "exists":
		errno = syscall.EEXIST
	default:
		panic(kind)
	}
	return &os.LinkError{Op: "rename", Old: "from", New: "to", Err: errno}
}
