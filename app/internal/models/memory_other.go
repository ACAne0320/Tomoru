//go:build !darwin && !windows

package models

// Memory is the computer's memory, in bytes, or 0 when unknown.
func Memory() uint64 { return 0 }
