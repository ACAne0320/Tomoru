package overlay

import (
	"os"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

func toolWindow(w *mygo.Window) {
	gtk, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	var setAcceptFocus func(window uintptr, accept int32)
	purego.RegisterLibFunc(&setAcceptFocus, gtk, "gtk_window_set_accept_focus")
	setAcceptFocus(w.NativeHandle(), 0)
}

var pointerUnseen = os.Getenv("WAYLAND_DISPLAY") != ""
