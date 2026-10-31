package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFailedReplacementLeavesDestinationAndCleansTemporaryFile(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "destination")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "previous")
	if err := os.WriteFile(marker, []byte("intact"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("replacement"), 0600); err == nil {
		t.Fatal("directory replacement succeeded")
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "intact" {
		t.Fatalf("destination changed: %q %v", b, err)
	}
	es, err := os.ReadDir(parent)
	if err != nil || len(es) != 1 {
		t.Fatalf("temporary files remain: %v %v", es, err)
	}
}

func TestReplacementIsCompleteAndUsesRequestedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new complete content"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "new complete content" {
		t.Fatalf("replacement: %q %v", b, err)
	}
	st, err := os.Stat(path)
	if runtime.GOOS == "windows" {
		return // Windows keeps no Unix permissions
	}
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v %v", st, err)
	}
}
