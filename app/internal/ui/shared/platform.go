package shared

import "runtime"

var Windows = runtime.GOOS == "windows"

var linux = runtime.GOOS == "linux"

func OnWindows(mac, win string) string {
	if Windows || linux {
		return win
	}
	return mac
}

func OnEach(mac, win, lin string) string {
	if linux {
		return lin
	}
	return OnWindows(mac, win)
}

var HidesFromCapture = !linux
