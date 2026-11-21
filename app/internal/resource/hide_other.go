//go:build !windows

package resource

import "os/exec"

func Hide(*exec.Cmd) {}
