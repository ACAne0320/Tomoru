package models

import "golang.org/x/sys/windows"

func enoughSpace(dir string, need int64) error {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil
	}
	var free, total, all uint64
	if windows.GetDiskFreeSpaceEx(p, &free, &total, &all) != nil {
		return nil
	}
	if int64(free) < need+256<<20 {
		return ErrNoSpace{Need: need, Free: int64(free)}
	}
	return nil
}
