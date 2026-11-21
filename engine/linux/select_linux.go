package main

// #cgo pkg-config: gtk+-3.0
// #include <stdlib.h>
// #include "engine_linux.h"
import "C"

import (
	"errors"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"tomoru-engine-shared"
)

func selectArea(path string) (image.Rectangle, error) {
	runtime.LockOSThread()
	app, out := C.CString(appID()), C.CString(path)
	defer C.free(unsafe.Pointer(app))
	defer C.free(unsafe.Pointer(out))
	var x, y, w, h C.int
	var why [256]C.char
	switch C.tc_select(app, out, &x, &y, &w, &h, &why[0], C.int(len(why))) {
	case C.TC_SELECTED:
		return image.Rect(int(x), int(y), int(x+w), int(y+h)), nil
	case C.TC_SELECT_CANCELLED:
		return image.Rectangle{}, shared.ErrCancelled
	default:
		return image.Rectangle{}, errors.New(C.GoString(&why[0]))
	}
}

func screenAccess(request bool) shared.Access {
	app := C.CString(appID())
	defer C.free(unsafe.Pointer(app))
	var why [256]C.char
	ask := C.int(0)
	if request {
		ask = 1
	}
	switch C.tc_screen_access(app, ask, &why[0], C.int(len(why))) {
	case C.TC_ACCESS_GRANTED:
		return shared.AccessGranted
	case C.TC_ACCESS_UNKNOWN:
		return shared.AccessUnknown
	default:
		log.Printf("screen access: %s", C.GoString(&why[0]))
		return shared.AccessUnknown
	}
}

func appID() string {
	for _, env := range []string{"GIO_LAUNCHED_DESKTOP_FILE", "BAMF_DESKTOP_FILE_HINT"} {
		if p := os.Getenv(env); p != "" {
			return strings.TrimSuffix(filepath.Base(p), ".desktop")
		}
	}
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(os.Getppid()), "exe"))
	if err != nil {
		return ""
	}
	return filepath.Base(exe)
}
