//go:build !windows

package models

import "golang.org/x/sys/unix"

// enoughSpace tells whether dir's disk has need bytes free, and a little more.
func enoughSpace(dir string, need int64) error {
	var st unix.Statfs_t
	if unix.Statfs(dir, &st) != nil {
		return nil // cannot tell: try
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	if free < need+256<<20 {
		return ErrNoSpace{Need: need, Free: free}
	}
	return nil
}
