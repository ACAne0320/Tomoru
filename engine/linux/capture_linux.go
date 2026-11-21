package main

// #cgo pkg-config: libpulse
// #include <stdlib.h>
// #include "engine_linux.h"
import "C"

import (
	"fmt"
	"log"
	"time"
	"unsafe"

	"tomoru-engine-shared"
)

func systemSource(string) shared.Source { return monitor{} }

type monitor struct{}

func (monitor) Start(feed func([]int16)) error {
	c, err := open()
	if err != nil {
		return err
	}
	go func() {
		for {
			changed := false
			if c != nil {
				changed = hear(c, feed)
				C.tc_close(c)
			}
			if !changed {
				time.Sleep(time.Second)
			}
			if c, err = open(); err != nil {
				log.Print(err)
			}
		}
	}()
	return nil
}

func open() (*C.tc_capture, error) {
	var c *C.tc_capture
	var why [256]C.char
	if C.tc_open(&c, &why[0], C.int(len(why))) < 0 {
		return nil, fmt.Errorf("capture: %s", C.GoString(&why[0]))
	}
	log.Printf("capturing %s", C.GoString(C.tc_sink(c)))
	return c, nil
}

func hear(c *C.tc_capture, feed func([]int16)) (changed bool) {
	buf := make([]int16, shared.Rate)
	var frames, peak int
	report := time.Now()
	for {
		switch n := C.tc_read(c, (*C.int16_t)(unsafe.Pointer(&buf[0])), C.int(len(buf)), 100); {
		case n == C.TC_CHANGED:
			log.Print("default output changed")
			return true
		case n == C.TC_NONE:
		case n == C.TC_ENDED:
			log.Printf("capture ended: %s", C.GoString(C.tc_why(c)))
			return false
		case n > 0:
			got := append([]int16(nil), buf[:n]...)
			for _, s := range got {
				peak = max(peak, int(s), -int(s))
			}
			frames += len(got)
			feed(got)
		}
		if time.Since(report) >= 30*time.Second {
			log.Printf("capture: %.1f s of audio, peak %.4f", float64(frames)/shared.Rate, float64(peak)/32768)
			frames, peak, report = 0, 0, time.Now()
		}
	}
}
