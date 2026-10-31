package main

// #cgo LDFLAGS: -lole32 -lmmdevapi -luuid -luser32 -lgdi32 -ldwmapi -lversion
// #include <stdlib.h>
// #include "engine_windows.h"
import "C"

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemSource hears what the computer plays: everything but the engine itself, or app alone, a
// program's file name, with its helpers (a browser's audio process is a child of the browser). An
// app that is not running yet is heard once it starts, and again once it starts again.
func systemSource(app string) Source { return &loopback{app: strings.ToLower(app)} }

type loopback struct{ app string }

// The formats to capture, in order: the one sent, else one the audio engine surely gives, which is
// made it here.
var formats = []struct{ rate, channels int }{{rate, 1}, {48000, 2}}

func (l *loopback) Start(feed func([]int16)) error {
	if l.app == "" {
		opened := make(chan error, 1)
		go func() {
			// Should capture stop, as when the audio service restarts, it starts again.
			for {
				l.run(uint32(os.Getpid()), false, feed, opened, nil)
				time.Sleep(time.Second)
				opened = make(chan error, 1)
			}
		}()
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
			l.run(pid, true, feed, opened, func() bool {
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

// run captures the tree of pid, or everything but it, until ended says so (never, when nil),
// telling opened whether it started.
func (l *loopback) run(pid uint32, include bool, feed func([]int16), opened chan<- error, ended func() bool) {
	runtime.LockOSThread() // COM's objects, on the thread that made them
	defer runtime.UnlockOSThread()
	var c *C.tc_capture
	var hr C.long = -1
	var f struct{ rate, channels int }
	for _, f = range formats {
		if hr = C.tc_open(C.ulong(pid), boolInt(include), C.int(f.rate), C.int(f.channels), &c); hr >= 0 {
			break
		}
		log.Printf("capture at %d Hz, %d channels: %v", f.rate, f.channels, hresult(hr))
	}
	if hr < 0 {
		opened <- fmt.Errorf("process loopback: %v", hresult(hr))
		return
	}
	defer C.tc_close(c)
	what := "everything but the engine"
	if include {
		what = fmt.Sprintf("%s (%d)", l.app, pid)
	}
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
		// What arrives, logged every 30 s: silence, as much as sound, says capture works.
		if time.Since(report) >= 30*time.Second {
			log.Printf("capture: %.1f s of audio, peak %.4f", float64(frames)/rate, float64(peak)/32768)
			frames, peak, report = 0, 0, time.Now()
		}
		if ended != nil && time.Since(check) >= time.Second {
			if ended() {
				log.Printf("%s ended", l.app)
				return
			}
			check = time.Now()
		}
	}
}

// converter makes interleaved 16-bit audio of rate and channels 16 kHz mono, when it is a whole
// multiple of it, averaging.
type converter struct {
	channels, factor int
	sum, n           int
}

func newConverter(r, channels int) *converter {
	return &converter{channels: channels, factor: r / rate}
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

// findApp finds the process of a program, by its file name: the first that no other of it started,
// and a handle to wait for it to end. 0 when none runs.
func findApp(exe string) (uint32, windows.Handle) {
	procs := processes()
	for pid, p := range procs {
		if !strings.EqualFold(p.exe, exe) {
			continue
		}
		if parent, ok := procs[p.parent]; ok && strings.EqualFold(parent.exe, exe) {
			continue // a helper: its tree is the root's
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

// processes are the processes running, by ID.
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

func boolInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// hresult is an HRESULT, as an error that says it.
func hresult(hr C.long) error { return fmt.Errorf("%#08x: %v", uint32(hr), windows.Errno(uint32(hr))) }
