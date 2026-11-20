//go:build !darwin

package overlay

import "github.com/egoist/mygo"

// hasGlass tells whether the system has the glass, which only macOS has.
func hasGlass() bool { return false }

func looksActive(*mygo.Window) {}
