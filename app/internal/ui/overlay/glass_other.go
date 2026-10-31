//go:build !darwin

package overlay

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// panes are panes of the system's glass, which only macOS has: elsewhere, none show.
type panes struct{}

func hasGlass() bool { return false }

type pane struct {
	r      ui.Rect
	radius float32
}

func (g *panes) set(*mygo.Window, float32, ...pane) {}
