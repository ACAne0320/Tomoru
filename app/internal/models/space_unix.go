//go:build !windows

package models

import "golang.org/x/sys/unix"

func enoughSpace(dir string, need int64) error {
	var st unix.Statfs_t
	if unix.Statfs(dir, &st) != nil {
		return nil
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	if free < need+256<<20 {
		return ErrNoSpace{Need: need, Free: free}
	}
	return nil
}
