package overlay

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

const fitsWindow = true

func hasChildWindows(win *mygo.Window) bool {
	children := objc.ID(win.NativeHandle()).Send(objc.RegisterName("childWindows"))
	return children != 0 && objc.Send[int](children, objc.RegisterName("count")) > 0
}

func anchorBottomLeft(win *mygo.Window) {
	view := objc.ID(win.NativeHandle()).Send(objc.RegisterName("contentView"))
	if view == 0 {
		return
	}
	metal := objc.ID(objc.GetClass("CAMetalLayer"))
	gravity := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "topLeft")
	var walk func(layer objc.ID)
	walk = func(layer objc.ID) {
		if layer == 0 {
			return
		}
		if objc.Send[bool](layer, objc.RegisterName("isKindOfClass:"), metal) {
			layer.Send(objc.RegisterName("setContentsGravity:"), gravity)
		}
		subs := layer.Send(objc.RegisterName("sublayers"))
		if subs == 0 {
			return
		}
		for i := range objc.Send[int](subs, objc.RegisterName("count")) {
			walk(subs.Send(objc.RegisterName("objectAtIndex:"), i))
		}
	}
	walk(view.Send(objc.RegisterName("layer")))
}
