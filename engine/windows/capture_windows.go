package main

// #cgo CFLAGS: -D_WIN32_WINNT=0x0A00
// #cgo LDFLAGS: -lole32 -lmmdevapi -luuid -luser32 -lgdi32 -ldwmapi -lversion -lshell32
// #include <stdlib.h>
// #include "engine_windows.h"
import "C"

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"tomoru-engine-shared"
)

func systemSource(app string) shared.Source { return &loopback{app: strings.ToLower(app)} }

type loopback struct{ app string }

var formats = []struct{ rate, channels int }{{shared.Rate, 1}, {48000, 2}}

func (l *loopback) Start(feed func([]int16)) error {
	if l.app == "" {
		opened := make(chan error, 1)
		go func(opened chan error) {
			for {
				l.run(0, feed, opened, nil)
				time.Sleep(time.Second)
				opened = make(chan error, 1)
			}
		}(opened)
		return <-opened
	}
	go func() {
		for {
			pid, proc := findApp(l.app)
			if proc == 0 {
				time.Sleep(time.Second)
				continue
			}
			opened := make(chan error, 1)
			l.run(pid, feed, opened, func() bool {
				ev, _ := windows.WaitForSingleObject(proc, 0)
				return ev == windows.WAIT_OBJECT_0
			})
			windows.CloseHandle(proc)
			if err := <-opened; err != nil {
				log.Printf("capture %s (%d): %v", l.app, pid, err)
				time.Sleep(time.Second)
			}
		}
	}()
	return nil
}

func (l *loopback) run(pid uint32, feed func([]int16), opened chan<- error, ended func() bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var c *C.tc_capture
	var hr C.long = -1
	var f struct{ rate, channels int }
	what := "default playback device"
	if pid != 0 {
		what = fmt.Sprintf("%s (%d)", l.app, pid)
	}
	for _, f = range formats {
		if hr = C.tc_open(C.ulong(pid), C.int(f.rate), C.int(f.channels), &c); hr >= 0 {
			break
		}
		log.Printf("capture at %d Hz, %d channels: %v", f.rate, f.channels, hresult(hr))
	}
	if hr < 0 {
		opened <- fmt.Errorf("capture %s: %v", what, hresult(hr))
		return
	}
	defer C.tc_close(c)
	log.Printf("capturing %s at %d Hz, %d channels", what, f.rate, f.channels)
	opened <- nil

	buf := make([]int16, 48000)
	conv := newConverter(f.rate, f.channels)
	var frames, peak int
	report, check := time.Now(), time.Now()
	for {
		n := C.tc_read(c, (*C.int16_t)(unsafe.Pointer(&buf[0])), C.int(len(buf)), 100)
		if n < 0 {
			log.Printf("capture: %v", hresult(C.long(n)))
			return
		}
		if n > 0 {
			got := conv.convert(buf[:n])
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
		if time.Since(check) >= time.Second {
			if pid == 0 {
				if hr := C.tc_default_changed(c); hr != C.TC_DEVICE_SAME {
					if hr < 0 {
						log.Printf("default playback device unavailable: %v", hresult(hr))
					} else {
						log.Print("default playback device changed")
					}
					return
				}
			}
			if ended != nil && ended() {
				log.Printf("%s ended", l.app)
				return
			}
			check = time.Now()
		}
	}
}

type converter struct {
	channels, factor int
	sum, n           int
}

func newConverter(r, channels int) *converter {
	return &converter{channels: channels, factor: r / shared.Rate}
}

func (c *converter) convert(in []int16) []int16 {
	if c.factor == 1 && c.channels == 1 {
		return append([]int16(nil), in...)
	}
	out := make([]int16, 0, len(in)/(c.channels*c.factor)+1)
	for i := 0; i+c.channels <= len(in); i += c.channels {
		for ch := range c.channels {
			c.sum += int(in[i+ch])
		}
		c.n++
		if c.n == c.factor {
			out = append(out, int16(c.sum/(c.factor*c.channels)))
			c.sum, c.n = 0, 0
		}
	}
	return out
}

func findApp(exe string) (uint32, windows.Handle) {
	procs := processes()
	for pid, p := range procs {
		if !strings.EqualFold(p.exe, exe) {
			continue
		}
		if parent, ok := procs[p.parent]; ok && strings.EqualFold(parent.exe, exe) {
			continue
		}
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			continue
		}
		return pid, h
	}
	return 0, 0
}

type process struct {
	exe    string
	parent uint32
}

func processes() map[uint32]process {
	out := map[uint32]process{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out
	}
	defer windows.CloseHandle(snap)
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[e.ProcessID] = process{exe: windows.UTF16ToString(e.ExeFile[:]), parent: e.ParentProcessID}
	}
	return out
}

func hresult(hr C.long) error { return fmt.Errorf("%#08x: %v", uint32(hr), windows.Errno(uint32(hr))) }
