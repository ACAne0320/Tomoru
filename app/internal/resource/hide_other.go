//go:build !windows

package resource

import "os/exec"

// Hide keeps the console of a program the app runs out of sight, on Windows; elsewhere there is
// none.
func Hide(*exec.Cmd) {}
