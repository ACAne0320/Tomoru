//go:build !windows

package shell

// trayIcon is a template icon as the system shows it: macOS tints it for the menu bar.
func trayIcon(template []byte) ([]byte, bool) { return template, true }
