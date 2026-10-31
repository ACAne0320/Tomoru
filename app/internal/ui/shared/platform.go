package shared

import "runtime"

// Windows tells whether the app runs on Windows, which has words of its own for some of what the
// app says: the taskbar's corner for the menu bar, File Explorer for Finder, a computer for a Mac.
var Windows = runtime.GOOS == "windows"

// OnWindows is win on Windows, else mac.
func OnWindows(mac, win string) string {
	if Windows {
		return win
	}
	return mac
}
