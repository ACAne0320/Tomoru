package main

import (
	"os"
	"strings"
)

func useDesktop() {
	if os.Getenv("WAYLAND_DISPLAY") == "" || os.Getenv("DISPLAY") == "" || os.Getenv("GDK_BACKEND") != "" {
		return
	}
	if strings.Contains(strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP")), "GNOME") {
		os.Setenv("GDK_BACKEND", "x11")
	}
}
