package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func osVersion() string {
	v := windows.RtlGetVersion()
	name := "Windows 10"
	if v.MajorVersion == 10 && v.BuildNumber >= 22000 {
		name = "Windows 11"
	}
	return fmt.Sprintf("%s (%d.%d.%d)", name, v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
