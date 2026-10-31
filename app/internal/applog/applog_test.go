package applog

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestFile(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close() // before the directory goes: Windows removes no open file
	l.Write([]byte("one\ntw"))
	l.Write([]byte("o\n"))
	b, _ := os.ReadFile(l.Path())
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], " one") || !strings.HasSuffix(lines[1], " two") {
		t.Errorf("%q", b)
	}

	// Past MaxSize, it starts afresh, keeping the last.
	big := strings.Repeat("x", 1<<20) + "\n"
	for range 6 {
		l.Write([]byte(big))
	}
	if fi, _ := os.Stat(l.Path()); fi.Size() > MaxSize {
		t.Errorf("%d bytes", fi.Size())
	}
	if _, err := os.Stat(dir + "/test.1.log"); err != nil {
		t.Error(err)
	}
}

type broken struct{}

func (broken) Write([]byte) (int, error) { return 0, errors.New("the handle is invalid") }

// TestAnd logs what the console cannot take.
func TestAnd(t *testing.T) {
	l, err := Open(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if n, err := l.And(broken{}).Write([]byte("kept\n")); n != 5 || err != nil {
		t.Errorf("wrote %d: %v", n, err)
	}
	if b, _ := os.ReadFile(l.Path()); !strings.HasSuffix(string(b), " kept\n") {
		t.Errorf("%q", b)
	}
}
