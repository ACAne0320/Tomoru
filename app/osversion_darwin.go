package main

import "golang.org/x/sys/unix"

func osVersion() string {
	v, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return "macOS"
	}
	return "macOS " + v
}
