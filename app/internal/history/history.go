// Package history keeps the subtitles of translating, when the user asks it to: on this computer
// only, a file a session, under the scene it was in (docs/scenes.md). A file is JSON lines:
// first what the session was, then its lines as they were translated.
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// Head is what a session was: its scene, as named then, its languages, and the app it heard.
type Head struct {
	Scene string    `json:"scene"` // "" for the general one
	Name  string    `json:"name,omitempty"`
	Src   string    `json:"src"`
	Tgt   string    `json:"tgt"`
	Start time.Time `json:"start"`
	App   string    `json:"app,omitempty"` // the app heard, "" for all the system's sound
}

// Entry is a line of a session.
type Entry struct {
	At    time.Time `json:"at"`
	Src   string    `json:"src"`             // as shown: the names heard wrong written right
	Heard string    `json:"heard,omitempty"` // as recognized, when the glossary changed it
	Tgt   string    `json:"tgt"`
	Err   string    `json:"err,omitempty"` // why it has no translation
}

// Record is a session kept: its file, what it was, and how many lines it has.
type Record struct {
	Path  string
	Head  Head
	Lines int
}

// deleted ends the name of a file deleted until it is put back or purged.
const deleted = ".deleted"

// general is the directory of the general scene's sessions.
const general = "general"

// A session's file is open only while a line is added to it, read, or put in its place, one at a
// time: a session still recording can be rewritten (its names learned) or deleted, which Windows
// does to no open file, and its lines added meanwhile stay.
var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}
)

// lock takes the lock of the file at path, and gives what lets it go.
func lock(path string) func() {
	locksMu.Lock()
	l := locks[path]
	if l == nil {
		l = &sync.Mutex{}
		locks[path] = l
	}
	locksMu.Unlock()
	l.Lock()
	return l.Unlock
}

// Writer adds a session's lines to its file, which it makes with the first.
type Writer struct {
	dir  string
	head Head
	path string // the file, once made
	err  error
}

// NewWriter is the writer of a session, head, kept under dir.
func NewWriter(dir string, head Head) *Writer { return &Writer{dir: dir, head: head} }

// Add adds a line. The first error stops the writer, and is given again.
func (w *Writer) Add(e Entry) error {
	if w.err != nil {
		return w.err
	}
	if w.path == "" {
		w.err = w.create()
		if w.err != nil {
			return w.err
		}
	}
	unlock := lock(w.path)
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND, 0)
	if errors.Is(err, fs.ErrNotExist) {
		// Deleted while recording: the lines after go to a session of their own, which may take
		// the same name, and its lock.
		unlock()
		w.path = ""
		return w.Add(e)
	}
	defer unlock()
	if err == nil {
		_, err = f.Write(line(e))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	w.err = err
	return err
}

func (w *Writer) create() error {
	scene := w.head.Scene
	if scene == "" {
		scene = general
	}
	dir := filepath.Join(w.dir, scene)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := w.head.Start.Format("20060102-150405")
	for i := 1; ; i++ { // two sessions in a second: the scene changed
		path := filepath.Join(dir, name+".jsonl")
		var f *os.File
		err := fs.ErrExist
		// Not the name of a session deleted, which undo would put back over this one: this one's,
		// deleted while it recorded.
		if !pathDeleted(path) {
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o644)
		}
		if errors.Is(err, fs.ErrExist) {
			name = w.head.Start.Format("20060102-150405") + "-" + string(rune('0'+i))
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.Write(line(w.head))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			w.path = path
		}
		return err
	}
}

func line(v any) []byte {
	b, _ := json.Marshal(v)
	return append(b, '\n')
}

// Close ends the writer. The file is closed between lines: there is nothing left to do.
func (w *Writer) Close() error { return nil }

// pathDeleted tells whether a session deleted, which undo would put back, has path's name.
func pathDeleted(path string) bool {
	_, err := os.Stat(path + deleted)
	return err == nil
}

// List gives the sessions kept under dir, the latest first.
func List(dir string) ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "*.jsonl*"))
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, p := range paths {
		if !strings.HasSuffix(p, ".jsonl") {
			continue
		}
		head, n, err := count(p)
		if err != nil {
			continue // not one of ours, or broken: left alone
		}
		out = append(out, Record{Path: p, Head: head, Lines: n})
	}
	slices.SortFunc(out, func(a, b Record) int { return b.Head.Start.Compare(a.Head.Start) })
	return out, nil
}

// Read gives a session's lines.
func Read(path string) ([]Entry, error) {
	var es []Entry
	_, _, err := scan(path, func(e Entry) { es = append(es, e) })
	return es, err
}

// count reads a session's head and counts its lines, without reading each: those ended, as a line
// cut short as the app quit is not. Listing reads every session, each time.
func count(path string) (Head, int, error) {
	defer lock(path)()
	var head Head
	f, err := os.Open(path)
	if err != nil {
		return head, 0, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 4<<10) // a head is short
	first, err := r.ReadSlice('\n')
	if len(first) == 0 {
		return head, 0, errors.New("history: an empty file")
	}
	if errors.Is(err, bufio.ErrBufferFull) || json.Unmarshal(first, &head) != nil || head.Start.IsZero() {
		return head, 0, errors.New("history: not a session")
	}
	n, buf := 0, make([]byte, 32<<10)
	for {
		k, err := r.Read(buf)
		n += bytes.Count(buf[:k], []byte{'\n'})
		if errors.Is(err, io.EOF) {
			return head, n, nil
		}
		if err != nil {
			return head, n, err
		}
	}
}

// scan reads a session's file: its head, and its lines, each to each if it is not nil.
func scan(path string, each func(Entry)) (Head, int, error) {
	defer lock(path)()
	return scanLocked(path, each)
}

func scanLocked(path string, each func(Entry)) (Head, int, error) {
	var head Head
	f, err := os.Open(path)
	if err != nil {
		return head, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	if !sc.Scan() {
		return head, 0, errors.New("history: an empty file")
	}
	if err := json.Unmarshal(sc.Bytes(), &head); err != nil || head.Start.IsZero() {
		return head, 0, errors.New("history: not a session")
	}
	n := 0
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue // a line cut short as the app quit
		}
		n++
		if each != nil {
			each(e)
		}
	}
	return head, n, sc.Err()
}

// Purge removes for good the sessions deleted under dir. The app does it once a launch, before
// the user could delete one and want it back.
func Purge(dir string) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*", "*.jsonl"+deleted))
	for _, p := range paths {
		os.Remove(p)
	}
}

// Delete deletes the sessions at paths. undo puts them back until they are purged.
func Delete(paths []string) (undo func(), err error) {
	var done []string
	for _, p := range paths {
		unlock := lock(p)
		err = rename(p, p+deleted)
		unlock()
		if err != nil {
			break
		}
		done = append(done, p)
	}
	return func() {
		for _, p := range done {
			unlock := lock(p)
			rename(p+deleted, p)
			unlock()
		}
	}, err
}

// Rewrite writes a session's lines over those it has, as retranslated: a new file, put in its
// place once written. es are the lines as read, changed; those added since, while it still
// recorded, are kept after them.
func Rewrite(path string, es []Entry) error {
	defer lock(path)()
	var now []Entry
	head, _, err := scanLocked(path, func(e Entry) { now = append(now, e) })
	if err != nil {
		return err
	}
	if len(now) > len(es) {
		es = append(slices.Clip(es), now[len(es):]...)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	w.Write(line(head))
	for _, e := range es {
		w.Write(line(e))
	}
	if err = w.Flush(); err == nil {
		err = f.Close()
	} else {
		f.Close()
	}
	if err == nil {
		err = rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// rename renames a file, trying again a few times: on Windows a file a search indexer or a virus
// scanner opens for a moment cannot be replaced or moved meanwhile.
func rename(from, to string) error {
	err := os.Rename(from, to)
	for i := 0; err != nil && runtime.GOOS == "windows" && i < 5; i++ {
		time.Sleep(time.Duration(i+1) * 20 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}

// ReadHead gives what a session was.
func ReadHead(path string) (Head, error) {
	head, _, err := scan(path, nil)
	return head, err
}
