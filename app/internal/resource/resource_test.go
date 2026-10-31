package resource

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExplicitRoots(t *testing.T) {
	root, second := t.TempDir(), t.TempDir()
	name := "tool"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	makeBin := func(root, sub string) string {
		t.Helper()
		path := filepath.Join(root, sub, "bin", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("binary"), 0755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	platform := makeBin(root, runtime.GOOS+"-"+runtime.GOARCH)
	packaged := makeBin(second, "")
	for _, tc := range []struct {
		roots []string
		want  string
	}{
		{[]string{root, second}, platform}, {[]string{second, root}, packaged},
	} {
		got, err := Bin("tool", tc.roots...)
		if err != nil || got != tc.want {
			t.Fatalf("got %q, %v; want %q", got, err, tc.want)
		}
	}
	if _, err := Bin("tool"); err == nil {
		t.Fatal("lookup without roots succeeded")
	}
	if _, err := Bin("missing", root); err == nil {
		t.Fatal("missing binary succeeded")
	}
	if err := os.MkdirAll(filepath.Join(root, "bin", "directory"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Bin("directory", root); err == nil {
		t.Fatal("directory accepted as a binary")
	}
}
