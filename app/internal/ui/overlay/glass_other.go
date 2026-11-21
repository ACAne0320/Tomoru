//go:build !darwin

package overlay

import "github.com/egoist/mygo"

func hasGlass() bool { return false }

func looksActive(*mygo.Window) {}
