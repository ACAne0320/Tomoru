package models

import "golang.org/x/sys/windows"

// enoughSpace tells whether dir's disk has need bytes free, and a little more.
func enoughSpace(dir string, need int64) error {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil
	}
	var free, total, all uint64
	if windows.GetDiskFreeSpaceEx(p, &free, &total, &all) != nil {
		return nil // cannot tell: try
	}
	if int64(free) < need+256<<20 {
		return ErrNoSpace{Need: need, Free: int64(free)}
	}
	return nil
}
