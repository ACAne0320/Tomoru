package main

// #include "engine_windows.h"
import "C"

import (
	"log"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"tomoru-engine-shared"
)

func speak(locale string, lines []string) {
	runtime.LockOSThread()
	var name [256]uint16
	switch hr := C.tc_speak_open(wide(locale), (*C.wchar_t)(unsafe.Pointer(&name[0])), C.int(len(name))); {
	case hr == C.TC_NO_VOICE:
		shared.Fail("no-voice", "no voice for %s", locale)
	case hr < 0:
		shared.Fail("internal", "SAPI: %v", hresult(hr))
	}
	log.Printf("speaking with %s", windows.UTF16ToString(name[:]))
	shared.Send(shared.Event{Type: "state", State: "speaking"})
	for i, line := range lines {
		if i > 0 {
			time.Sleep(600 * time.Millisecond)
		}
		if hr := C.tc_speak(wide(line)); hr < 0 {
			shared.Fail("internal", "SAPI: %v", hresult(hr))
		}
	}
}

func wide(s string) *C.wchar_t {
	p, err := windows.UTF16FromString(s)
	if err != nil {
		p = []uint16{0}
	}
	return (*C.wchar_t)(unsafe.Pointer(&p[0]))
}
