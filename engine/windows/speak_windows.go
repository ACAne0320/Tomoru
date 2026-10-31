package main

// #include "engine_windows.h"
import "C"

import (
	"log"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// speak says lines aloud with the system's voice for locale, through the default output, where
// `listen` hears them as it hears a video: the first-run guide's sample. A pause follows each, as
// a speaker pauses between sentences; it exits once they are said.
func speak(locale string, lines []string) {
	runtime.LockOSThread() // the voice, on the thread that made it
	var name [256]uint16
	switch hr := C.tc_speak_open(wide(locale), (*C.wchar_t)(unsafe.Pointer(&name[0])), C.int(len(name))); {
	case hr == 1:
		fail("no-voice", "no voice for %s", locale)
	case hr < 0:
		fail("internal", "SAPI: %v", hresult(hr))
	}
	log.Printf("speaking with %s", windows.UTF16ToString(name[:]))
	send(Event{Type: "state", State: "speaking"})
	for i, line := range lines {
		if i > 0 {
			time.Sleep(600 * time.Millisecond)
		}
		if hr := C.tc_speak(wide(line)); hr < 0 {
			fail("internal", "SAPI: %v", hresult(hr))
		}
	}
}

// wide is s for Windows, in UTF-16, ended by 0, for the call it is passed to.
func wide(s string) *C.wchar_t {
	p, err := windows.UTF16FromString(s)
	if err != nil {
		p = []uint16{0}
	}
	return (*C.wchar_t)(unsafe.Pointer(&p[0]))
}
