package models

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	length, memoryLoad                  uint32
	totalPhys, availPhys                uint64
	totalPageFile, availPageFile        uint64
	totalVirtual, availVirtual, availEx uint64
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func Memory() uint64 {
	m := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); ok == 0 {
		return 0
	}
	return m.totalPhys
}
