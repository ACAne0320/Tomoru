package models

import "golang.org/x/sys/unix"

// Memory is the computer's memory, in bytes, or 0 when unknown.
func Memory() uint64 {
	n, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return n
}
