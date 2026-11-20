package overlay

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

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

// looksActive has win look active all along. The clear glass looks clear only in a window that
// looks active: the key window of the app in front. The overlay never is, as another app plays, and
// the system frosted it as the regular glass, until the block was clicked (2026-10-21). AppKit's
// private _setForceMainAppearance: does it; NSGlassEffectView has no public setting for it, as
// NSVisualEffectView's state. Without it, as on a later macOS that drops it, the glass only frosts.
func looksActive(win *mygo.Window) {
	if !hasGlass() {
		return
	}
	w := objc.ID(win.NativeHandle())
	if sel := objc.RegisterName("_setForceMainAppearance:"); objc.Send[bool](w, objc.RegisterName("respondsToSelector:"), sel) {
		w.Send(sel, true)
	}
}
