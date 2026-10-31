// Package atomicfile replaces small data files only after their new contents are complete.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write preserves the previous file if writing or syncing its replacement fails.
// The caller owns the parent directory; a missing directory is reported, not hidden.
func Write(path string, data []byte, mode os.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Close has already succeeded; avoid reporting its second call as a write failure.
	return os.Rename(f.Name(), path)
}
