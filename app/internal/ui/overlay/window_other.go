//go:build !windows && !linux

package overlay

import "github.com/egoist/mygo"

func toolWindow(*mygo.Window) {}

const pointerUnseen = false
