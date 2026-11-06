package models

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Move is how moving the models to another directory goes: Done of Total bytes while Active, and
// why the last move failed.
type Move struct {
	Active      bool
	Done, Total int64
	Err         error
}

// FolderName is the folder of their own the models get in a folder the user chooses.
const FolderName = "Tomoru Models"

// Destination is where the models go when the user chooses the folder chosen: a folder of their
// own in it, unless it is that folder already, or the one they are in now.
func Destination(chosen, current string) string {
	chosen = filepath.Clean(chosen)
	if strings.EqualFold(filepath.Base(chosen), FolderName) {
		return chosen
	}
	if a, err := os.Stat(chosen); err == nil {
		if b, err := os.Stat(current); err == nil && os.SameFile(a, b) {
			return chosen
		}
	}
	return filepath.Join(chosen, FolderName)
}

// errBusy is a move asked for while one goes on, or the manager is closed.
var errBusy = errors.New("models: busy")

// Moving says how moving the models goes.
func (m *Manager) Moving() Move {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.move
}

// Move moves the models, what came in of the others too, into the directory to, and keeps them
// there from then on. Downloads stop, and none starts or is deleted until it ends; Status says
// what was there as it began. Within a disk a file is renamed; across disks it is copied, and the
// files left where they were only once all are there. A file already there, of the size, stays.
// On failure everything goes back as it was. done is called once, from any goroutine, as it ends
// or is refused.
func (m *Manager) Move(to string, done func(error)) {
	to = filepath.Clean(to)
	m.mu.Lock()
	switch {
	case m.closed || m.ctx.Err() != nil || m.move.Active:
		m.mu.Unlock()
		done(errBusy)
		return
	case to == filepath.Clean(m.dir):
		m.mu.Unlock()
		done(nil)
		return
	}
	from := m.dir
	m.move = Move{Active: true}
	var jobs []*job
	for _, j := range m.jobs {
		j.cancel()
		jobs = append(jobs, j)
	}
	m.wg.Add(1)
	m.mu.Unlock()
	m.changed()
	go func() {
		defer m.wg.Done()
		for _, j := range jobs {
			<-j.exited // a file still open can't be moved on Windows
		}
		m.mu.Lock()
		m.held = map[string]Status{}
		for _, mod := range All {
			m.held[mod.ID] = onDisk(from, mod)
		}
		m.mu.Unlock()
		err := m.moveFiles(m.ctx, from, to)
		m.mu.Lock()
		m.move.Active, m.move.Err, m.held = false, err, nil
		m.mu.Unlock()
		m.changed()
		done(err)
	}()
}

// moving is a file being moved: from where to where, its size, and how it got there, to undo it.
type moving struct {
	from, to string
	size     int64
	how      moved
}

type moved int

const (
	notYet  moved = iota
	renamed       // undone by renaming it back
	copied        // undone by removing the copy
	there         // there before: kept
)

func (m *Manager) moveFiles(ctx context.Context, from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	if a, err := os.Stat(from); err == nil {
		// The same directory by another name, as in another case on macOS: nothing to move, and
		// its files are not to be removed as moved. It keeps the name it had.
		if b, err := os.Stat(to); err == nil && os.SameFile(a, b) {
			return nil
		}
	}
	var files []*moving
	var total int64
	for _, mod := range All {
		for _, f := range mod.Files {
			for _, name := range []string{f.Name, f.Name + ".part"} {
				src := filepath.Join(from, mod.Dir, name)
				if fi, err := os.Stat(src); err == nil && fi.Mode().IsRegular() {
					files = append(files, &moving{from: src, to: filepath.Join(to, mod.Dir, name), size: fi.Size()})
					total += fi.Size()
				}
			}
		}
	}
	var done int64
	var told time.Time
	progress := func(n int64) {
		done += n
		if time.Since(told) > 250*time.Millisecond {
			told = time.Now()
			m.mu.Lock()
			m.move.Done, m.move.Total = done, total
			m.mu.Unlock()
			m.changed()
		}
	}
	progress(0)
	err := func() error {
		copying := false
		for _, f := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(f.to), 0o755); err != nil {
				return err
			}
			if fi, err := os.Stat(f.to); err == nil && fi.Mode().IsRegular() && fi.Size() == f.size {
				f.how = there
				progress(f.size)
				continue
			}
			if !copying {
				if m.rename(f.from, f.to) == nil {
					f.how = renamed
					progress(f.size)
					continue
				}
				// Another disk: copy what is left, if it has the room.
				copying = true
				if err := enoughSpace(to, total-done); err != nil {
					return err
				}
			}
			if err := copyFile(ctx, f.from, f.to, progress); err != nil {
				return err
			}
			f.how = copied
		}
		return nil
	}()
	if err != nil {
		for _, f := range files {
			switch f.how {
			case renamed:
				if err := os.Rename(f.to, f.from); err != nil {
					log.Println("models: not moved back:", err)
				}
			case copied:
				os.Remove(f.to)
			}
		}
		for _, mod := range All {
			if mod.Dir != "" {
				os.Remove(filepath.Join(to, mod.Dir)) // once empty
			}
		}
		return err
	}
	m.mu.Lock()
	m.dir = to
	m.move.Done = total
	m.mu.Unlock()
	// The core stopped what read the models; a file still open stays behind, unused.
	for _, f := range files {
		if f.how != renamed {
			if err := os.Remove(f.from); err != nil {
				log.Println("models: leaving", f.from+":", err)
			}
		}
	}
	for _, mod := range All {
		if mod.Dir != "" {
			os.Remove(filepath.Join(from, mod.Dir))
		}
	}
	os.Remove(from) // once empty
	return nil
}

// copyFile copies a file through dst+".moving", so that dst is there only once all of it is.
func copyFile(ctx context.Context, src, dst string, progress func(int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".moving"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	err = func() error {
		buf := make([]byte, 1<<20)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, rerr := in.Read(buf)
			if n > 0 {
				if _, err := out.Write(buf[:n]); err != nil {
					return err
				}
				progress(int64(n))
			}
			if rerr == io.EOF {
				return nil
			}
			if rerr != nil {
				return rerr
			}
		}
	}()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
