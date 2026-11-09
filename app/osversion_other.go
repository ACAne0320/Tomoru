//go:build !darwin && !windows

package main

import "runtime"

// osVersion is the system, for a report of a problem.
func osVersion() string { return runtime.GOOS }
