package main

// #include "engine_windows.h"
import "C"

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shell are the programs of Windows's own that have windows but nothing to hear on their own.
var shell = []string{"explorer.exe", "applicationframehost.exe", "textinputhost.exe", "shellexperiencehost.exe",
	"startmenuexperiencehost.exe", "searchhost.exe", "systemsettings.exe", "lockapp.exe"}

// listApps sends the apps whose sound can be heard on their own: those with a window on the
// taskbar, and any playing sound (playing), those playing first. An app is its program: a
// helper that plays for it, as a browser's audio process, is the same program.
func listApps() {
	var buf [1024]C.ulong
	playing := map[uint32]bool{}
	for _, pid := range buf[:C.tc_playing(&buf[0], C.int(len(buf)))] {
		playing[uint32(pid)] = true
	}
	pids := make([]uint32, 0, len(playing))
	for _, pid := range buf[:C.tc_windowed(&buf[0], C.int(len(buf)))] {
		pids = append(pids, uint32(pid))
	}
	windowed := len(pids)
	for pid := range playing {
		pids = append(pids, pid)
	}
	// Not the engine, nor the app that runs it: Tomoru itself.
	procs := processes()
	own := map[string]bool{strings.ToLower(filepath.Base(os.Args[0])): true}
	if p, ok := procs[uint32(os.Getpid())]; ok {
		own[strings.ToLower(procs[p.parent].exe)] = true
	}

	type app struct {
		id, name string
		playing  bool
	}
	var apps []*app
	byID := map[string]*app{}
	for i, pid := range pids {
		path, name := describe(pid)
		if path == "" {
			continue
		}
		id := strings.ToLower(filepath.Base(path))
		if own[id] || slices.Contains(shell, id) {
			continue
		}
		a := byID[id]
		if a == nil {
			a = &app{id: id}
			byID[id] = a
			apps = append(apps, a)
		}
		// A window's process names the app: a helper may describe itself otherwise.
		if a.name == "" || i < windowed && name != "" {
			a.name = name
		}
		if a.name == "" {
			a.name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		a.playing = a.playing || playing[pid]
	}
	slices.SortFunc(apps, func(a, b *app) int {
		if a.playing != b.playing {
			if a.playing {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
	for _, a := range apps {
		send(Event{Type: "app", ID: a.id, Name: a.name, Playing: a.playing})
	}
}

// describe gives a process's program, "" when it cannot tell, and the name its file gives it.
func describe(pid uint32) (path, name string) {
	var p [windows.MAX_LONG_PATH]uint16
	var n [256]uint16
	if C.tc_describe(C.ulong(pid), (*C.wchar_t)(unsafe.Pointer(&p[0])), C.int(len(p)), (*C.wchar_t)(unsafe.Pointer(&n[0])), C.int(len(n))) != 0 {
		return "", ""
	}
	return windows.UTF16ToString(p[:]), strings.TrimSpace(windows.UTF16ToString(n[:]))
}
