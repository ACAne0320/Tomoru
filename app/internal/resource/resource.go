package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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

func Licenses(roots ...string) string {
	for _, dir := range roots {
		for _, p := range []string{
			filepath.Join(dir, "licenses"),
			filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, "licenses"),
		} {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return p
			}
		}
	}
	return ""
}
