package models

import "testing"

func TestParseDevices(t *testing.T) {
	mac := parseDevices([]byte("Available devices:\n  MTL0: Apple M5 (18186 MiB, 18185 MiB free)\n  BLAS: Accelerate (0 MiB, 0 MiB free)\n"))
	if len(mac) != 1 || mac[0].Name != "Apple M5" || mac[0].Memory != 18186<<20 {
		t.Errorf("mac %+v", mac)
	}
	win := parseDevices([]byte("load_backend: loaded Vulkan backend\nAvailable devices:\n  Vulkan0: NVIDIA GeForce RTX 5060 (7895 MiB, 7127 MiB free)\n"))
	if len(win) != 1 || win[0].Name != "NVIDIA GeForce RTX 5060" || win[0].Memory != 7895<<20 {
		t.Errorf("windows %+v", win)
	}
	if none := parseDevices([]byte("Available devices:\n")); len(none) != 0 {
		t.Errorf("no GPU: %+v", none)
	}
}
