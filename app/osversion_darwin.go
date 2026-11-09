package main

import "golang.org/x/sys/unix"

// osVersion is the system and its version, for a report of a problem: "macOS 26.1".
func osVersion() string {
	v, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return "macOS"
	}
	return "macOS " + v
}
