package main

// #include "engine_linux.h"
import "C"

import (
	"log"
	"path/filepath"
	"unsafe"

	"tomoru-engine-shared"
)

func speak(path string) {
	samples, rate, channels, err := shared.ReadWAV(path)
	if err != nil {
		shared.Fail("internal", "speak: %v", err)
	}
	log.Printf("speaking %s (%.1f s)", filepath.Base(path), float64(len(samples))/float64(rate*channels))
	shared.Send(shared.Event{Type: "state", State: "speaking"})
	if len(samples) == 0 {
		return
	}
	var why [256]C.char
	if C.tc_play_pcm((*C.int16_t)(unsafe.Pointer(&samples[0])), C.int(len(samples)), C.int(rate), C.int(channels), &why[0], C.int(len(why))) < 0 {
		shared.Fail("internal", "playback: %s", C.GoString(&why[0]))
	}
}
