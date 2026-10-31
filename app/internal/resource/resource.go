// Package resource finds the programs the app ships with: the engine, llama-server.
package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Bin finds a bundled program in explicitly supplied resource roots, in order. A packaged root
// contains bin/; a development root contains GOOS-GOARCH/bin/. Entry points choose the roots:
// lookup never consults a UI runtime, the working directory, or a repository marker.
func Bin(name string, roots ...string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, dir := range roots {
		for _, p := range []string{
			filepath.Join(dir, "bin", name),
			filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, "bin", name),
		} {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("no %s in resource roots [%s]", name, strings.Join(roots, ", "))
}
