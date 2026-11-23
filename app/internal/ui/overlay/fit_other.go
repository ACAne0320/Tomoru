//go:build !darwin

package overlay

import "github.com/egoist/mygo"

const fitsWindow = false

func hasChildWindows(*mygo.Window) bool { return false }

func anchorBottomLeft(*mygo.Window) {}
