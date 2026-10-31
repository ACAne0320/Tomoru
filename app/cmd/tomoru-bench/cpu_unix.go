//go:build !windows

package main

import "syscall"

// cpuTimes are the seconds of CPU the children and the benchmark itself took.
func cpuTimes() (children, self float64) {
	return cpu(syscall.RUSAGE_CHILDREN), cpu(syscall.RUSAGE_SELF)
}

func cpu(who int) float64 {
	var ru syscall.Rusage
	syscall.Getrusage(who, &ru)
	return float64(ru.Utime.Sec+ru.Stime.Sec) + float64(ru.Utime.Usec+ru.Stime.Usec)/1e6
}
