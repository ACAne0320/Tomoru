package overlay

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// panes are panes of the system's Liquid Glass (NSGlassEffectView) in the overlay's window, under
// what MyGo draws there: what plays under the window shows through them, frosted and bent, as
// under the Dock. MyGo's glass cannot: it shows only what its window draws, and the overlay draws
// nothing under the block. The system composes them with the windows under it, needing no
// permission to see them. The views go in the window's content view, below MyGo's, as MyGo puts
// its own vibrancy.
type panes struct {
	content objc.ID   // the window's content view
	views   []objc.ID // one for each pane shown so far, hidden when not
	shown   []pane    // where each shows, as last set
}

// pane is a pane of glass: its box in the window, in DIPs from the top left, and its corners'
// radius.
type pane struct {
	r      ui.Rect
	radius float32
}

// hasGlass tells whether the system has the glass: macOS 26 and later. Asked once AppKit is
// loaded, as the app runs: at the program's start, it is not yet.
func hasGlass() bool {
	glassOnce.Do(func() { glassClass = objc.GetClass("NSGlassEffectView") })
	return glassClass != 0
}

var (
	glassOnce  sync.Once
	glassClass objc.Class
)

func sel(s string) objc.SEL { return objc.RegisterName(s) }

// set shows the panes, and hides the others, in win, h DIPs tall. Run on the main thread.
func (g *panes) set(win *mygo.Window, h float32, want ...pane) {
	if g.content == 0 {
		if win == nil || !hasGlass() {
			return
		}
		w := objc.ID(win.NativeHandle())
		g.content = w.Send(sel("contentView"))
		// The clear glass looks clear only in a window that looks active: the key window of the app
		// in front. The overlay never is, as another app plays, and the system frosted it as the
		// regular glass, until the block was clicked (2026-10-21). AppKit's private
		// _setForceMainAppearance: has the window look active all along; NSGlassEffectView has no
		// public setting for it, as NSVisualEffectView's state. Without it, as on a later macOS that
		// drops it, the glass only frosts.
		if objc.Send[bool](w, sel("respondsToSelector:"), sel("_setForceMainAppearance:")) {
			w.Send(sel("_setForceMainAppearance:"), true)
		}
	}
	// The frames change at once: by itself, AppKit animates the change, and the glass trails.
	tx := objc.ID(objc.GetClass("CATransaction"))
	tx.Send(sel("begin"))
	tx.Send(sel("setDisableActions:"), true)
	defer tx.Send(sel("commit"))
	for i, p := range want {
		if i == len(g.views) {
			v := objc.ID(objc.GetClass("NSGlassEffectView")).Send(sel("alloc")).Send(sel("initWithFrame:"), nsRect{})
			g.content.Send(sel("addSubview:positioned:relativeTo:"), v, -1, objc.ID(0)) // below MyGo's view
			g.views = append(g.views, v)
			g.shown = append(g.shown, pane{})
		}
		if g.shown[i] == p {
			continue
		}
		v := g.views[i]
		if g.shown[i] == (pane{}) {
			// Clear, as over photos and video: what plays shows through, barely frosted. What is on
			// it brings its own shade: over white, it is white.
			v.Send(sel("setStyle:"), 1) // NSGlassEffectViewStyleClear
			v.Send(sel("setHidden:"), false)
		}
		v.Send(sel("setCornerRadius:"), float64(p.radius))
		v.Send(sel("setFrame:"), nsRect{float64(p.r.X), float64(h - p.r.Y - p.r.H), float64(p.r.W), float64(p.r.H)}) // from the bottom left
		g.shown[i] = p
	}
	for i := len(want); i < len(g.views); i++ {
		if g.shown[i] != (pane{}) {
			g.views[i].Send(sel("setHidden:"), true)
			g.shown[i] = pane{}
		}
	}
}

// nsRect is an NSRect: its origin from the bottom left, and its size.
type nsRect struct{ X, Y, W, H float64 }
