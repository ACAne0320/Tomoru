// Package applog keeps the app's logs in files: what the app, the engine and llama-server say,
// for a user to attach to a report.
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

// MaxSize is how large a log grows before it starts afresh, the last one kept as name.1.log.
const MaxSize = 5 << 20

// File is a log file. Writes go to it line by line, each stamped with the time, and are safe from
// any goroutine.
type File struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
	buf  []byte // a line not ended yet
}

// Open opens the log name.log in dir, made if need be.
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

// Path is where the log is.
func (l *File) Path() string { return l.path }

// Write writes p, stamping each line it ends with the time.
func (l *File) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), l.buf[:i])
		l.buf = l.buf[i+1:]
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

// And is a writer to the log and to console as well, as far as console takes it: an app started
// from Windows' Explorer has no console, whose every write fails, and the log goes on without it.
func (l *File) And(console io.Writer) io.Writer { return and{l, console} }

type and struct {
	log     *File
	console io.Writer
}

func (a and) Write(p []byte) (int, error) {
	a.console.Write(p)
	return a.log.Write(p)
}

// Close closes the log's file.
func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// rotate keeps the log as name.1.log, over the one before, and starts afresh.
func (l *File) rotate() {
	l.f.Close()
	ext := filepath.Ext(l.path)
	os.Rename(l.path, l.path[:len(l.path)-len(ext)]+".1"+ext)
	if l.open() != nil {
		l.f, _ = os.Open(os.DevNull)
	}
}
