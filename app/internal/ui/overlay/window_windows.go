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

func toolWindow(w *mygo.Window) {
	const gwlExStyle, wsExToolWindow = ^uintptr(19), 0x80
	const swpNoSize, swpNoMove, swpNoZOrder, swpNoActivate, swpFrameChanged = 0x1, 0x2, 0x4, 0x10, 0x20
	hwnd := w.NativeHandle()
	ex, _, _ := getWindowLongPtr.Call(hwnd, gwlExStyle)
	setWindowLongPtr.Call(hwnd, gwlExStyle, ex|wsExToolWindow)
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged)
}

const pointerUnseen = false
