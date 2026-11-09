package main

import (
	"runtime"
	"strings"
	"testing"
)

// TestOSVersion names the system with its version, for a report of a problem.
func TestOSVersion(t *testing.T) {
	got := osVersion()
	t.Log(got)
	want := map[string]string{"darwin": "macOS 2", "windows": "Windows 1"}[runtime.GOOS]
	if !strings.HasPrefix(got, want) {
		t.Errorf("osVersion() = %q, want it to start with %q", got, want)
	}
}
