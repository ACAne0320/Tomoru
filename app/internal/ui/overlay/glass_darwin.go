package overlay

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func hasGlass() bool {
	glassOnce.Do(func() { glassClass = objc.GetClass("NSGlassEffectView") })
	return glassClass != 0
}

var (
	glassOnce  sync.Once
	glassClass objc.Class
)

func looksActive(win *mygo.Window) {
	if !hasGlass() {
		return
	}
	w := objc.ID(win.NativeHandle())
	if sel := objc.RegisterName("_setForceMainAppearance:"); objc.Send[bool](w, objc.RegisterName("respondsToSelector:"), sel) {
		w.Send(sel, true)
	}
}
