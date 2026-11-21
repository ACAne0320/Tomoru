package models

import "golang.org/x/sys/windows"

var errOtherDisk error = windows.ERROR_NOT_SAME_DEVICE
