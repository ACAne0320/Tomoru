package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osVersion is the system and its version, for a report of a problem: "Windows 11 (10.0.26100)".
// Windows 11 still says 10.0; its builds start at 22000.
func osVersion() string {
	v := windows.RtlGetVersion()
	name := "Windows 10"
	if v.MajorVersion == 10 && v.BuildNumber >= 22000 {
		name = "Windows 11"
	}
	return fmt.Sprintf("%s (%d.%d.%d)", name, v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
