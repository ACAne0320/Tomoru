package main

// #include <stdlib.h>
// #include "engine_windows.h"
import "C"

import (
	"image"
	"image/png"
	"os"
	"runtime"
	"unsafe"
)

// selectArea has the user select a part of the screen and writes its picture to path, as PNG, and
// tells where it is. Cancelled, it writes nothing, as macOS's screencapture does, and tells nothing.
func selectArea(path string) (*image.Rectangle, error) {
	runtime.LockOSThread() // the window, on the thread whose messages it gets
	var px *C.uint8_t
	var x, y, w, h C.int
	switch r := C.tc_select(&px, &x, &y, &w, &h); {
	case r < 0:
		return nil, hresult(r)
	case r == 0:
		return nil, nil
	}
	defer C.free(unsafe.Pointer(px))
	src := unsafe.Slice((*byte)(unsafe.Pointer(px)), int(w)*int(h)*4)
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	for i := 0; i < len(src); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = src[i+2], src[i+1], src[i], 255
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	at := image.Rect(int(x), int(y), int(x+w), int(y+h))
	return &at, f.Close()
}
