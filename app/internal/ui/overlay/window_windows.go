package overlay

import (
	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	getWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	setWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	setWindowPos     = user32.NewProc("SetWindowPos")
)

// toolWindow makes w a tool window (WS_EX_TOOLWINDOW): out of Alt+Tab, and passed over by the
// apps under it that ask what covers them. Chromium's browsers and Electron's apps stop drawing
// while a window covers them, a video black where it played; they deem a window opaque unless it
// lets clicks through, is a tool window or is see-through by its layering
// (ui/gfx/win/hwnd_util.cc, IsWindowVisibleAndFullyOpaque). The overlay spans the work area and
// takes clicks while hovered, see-through by DirectComposition, which they cannot tell: a drag in
// it brought it to the foreground, Chrome asked again, and a video under it went black until
// another window came to the foreground (2026-10-28). Done before it first shows.
func toolWindow(w *mygo.Window) {
	const gwlExStyle, wsExToolWindow = ^uintptr(19), 0x80 // GWL_EXSTYLE is -20
	const swpNoSize, swpNoMove, swpNoZOrder, swpNoActivate, swpFrameChanged = 0x1, 0x2, 0x4, 0x10, 0x20
	hwnd := w.NativeHandle()
	ex, _, _ := getWindowLongPtr.Call(hwnd, gwlExStyle)
	setWindowLongPtr.Call(hwnd, gwlExStyle, ex|wsExToolWindow)
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged)
}
