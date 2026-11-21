package applog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const MaxSize = 5 << 20

type File struct {
	mu          sync.Mutex
	path        string
	f           *os.File
	size        int64
	partialLine []byte
}

func Open(dir, name string) (*File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l := &File{path: filepath.Join(dir, name+".log")}
	return l, l.open()
}

func (l *File) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.f, l.size = f, fi.Size()
	return nil
}

func (l *File) Path() string { return l.path }

func (l *File) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.partialLine = append(l.partialLine, p...)
	for {
		i := bytes.IndexByte(l.partialLine, '\n')
		if i < 0 {
			break
		}
		line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), l.partialLine[:i])
		l.partialLine = l.partialLine[i+1:]
		if l.size+int64(len(line)) > MaxSize {
			l.rotate()
		}
		n, err := l.f.WriteString(line)
		l.size += int64(n)
		if err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func (l *File) And(console io.Writer) io.Writer { return and{l, console} }

type and struct {
	log     *File
	console io.Writer
}

func (a and) Write(p []byte) (int, error) {
	a.console.Write(p)
	return a.log.Write(p)
}

func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

func (l *File) rotate() {
	l.f.Close()
	ext := filepath.Ext(l.path)
	os.Rename(l.path, l.path[:len(l.path)-len(ext)]+".1"+ext)
	if l.open() != nil {
		l.f, _ = os.Open(os.DevNull)
	}
}
