package shared

import "runtime"

var Windows = runtime.GOOS == "windows"

var linux = runtime.GOOS == "linux"

func MacOr(mac, other string) string {
	if Windows || linux {
		return other
	}
	return mac
}

func OnEach(mac, win, lin string) string {
	if linux {
		return lin
	}
	return MacOr(mac, win)
}

var HidesFromCapture = !linux
