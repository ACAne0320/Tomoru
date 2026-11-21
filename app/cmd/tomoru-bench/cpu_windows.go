package main

import (
	"syscall"
	"time"
)

func cpuTimes() (children, self float64) {
	var c, e, k, u syscall.Filetime
	h, err := syscall.GetCurrentProcess()
	if err != nil || syscall.GetProcessTimes(h, &c, &e, &k, &u) != nil {
		return 0, 0
	}
	d := func(f syscall.Filetime) time.Duration {
		return time.Duration(int64(f.HighDateTime)<<32|int64(f.LowDateTime)) * 100
	}
	return 0, (d(k) + d(u)).Seconds()
}
