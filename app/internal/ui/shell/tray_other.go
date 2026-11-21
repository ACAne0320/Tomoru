//go:build !windows

package shell

func trayIcon(template []byte) ([]byte, bool) { return template, true }
