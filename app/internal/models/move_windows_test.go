package models

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// anotherDisk is a rename across disks, as os.Rename reports it.
func anotherDisk(from, to string) error {
	return &os.LinkError{Op: "rename", Old: from, New: to, Err: windows.ERROR_NOT_SAME_DEVICE}
}

// renameFail is a rename error that is not another disk.
func renameFail(kind string) error {
	var errno syscall.Errno
	switch kind {
	case "permission":
		errno = windows.ERROR_ACCESS_DENIED
	case "exists":
		errno = windows.ERROR_FILE_EXISTS
	default:
		panic(kind)
	}
	return &os.LinkError{Op: "rename", Old: "from", New: "to", Err: errno}
}
