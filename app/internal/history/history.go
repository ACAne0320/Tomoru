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
	"slices"
	"strings"
	"sync"
	"time"
)

type Head struct {
	Scene        string    `json:"scene"`
	Name         string    `json:"name,omitempty"`
	Src          string    `json:"src"`
	Tgt          string    `json:"tgt"`
	Start        time.Time `json:"start"`
	App          string    `json:"app,omitempty"`
	Untranslated bool      `json:"untranslated,omitempty"`
}

type Entry struct {
	At    time.Time `json:"at"`
	Start time.Time `json:"start,omitzero"`
	End   time.Time `json:"end,omitzero"`
	Src   string    `json:"src"`
	Heard string    `json:"heard,omitempty"`
	Lang  string    `json:"lang,omitempty"`
	Said  bool      `json:"said,omitempty"`
	Tgt   string    `json:"tgt"`
	Err   string    `json:"err,omitempty"`
}

type Record struct {
	Path  string
	Head  Head
	Lines int
}

const Skipped = "behind"

const deleted = ".deleted"

const general = "general"

var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}
)

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

type Writer struct {
	dir  string
	head Head
	path string
	err  error
}

func NewWriter(dir string, head Head) *Writer { return &Writer{dir: dir, head: head} }

func (w *Writer) Broken() bool { return w.err != nil }

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
	for i := 1; ; i++ {
		path := filepath.Join(dir, name+".jsonl")
		var f *os.File
		err := fs.ErrExist
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

func (w *Writer) Close() error { return nil }

func pathDeleted(path string) bool {
	_, err := os.Stat(path + deleted)
	return err == nil
}

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
			continue
		}
		out = append(out, Record{Path: p, Head: head, Lines: n})
	}
	slices.SortFunc(out, func(a, b Record) int { return b.Head.Start.Compare(a.Head.Start) })
	return out, nil
}

func Read(path string) ([]Entry, error) {
	var es []Entry
	_, err := scan(path, func(e Entry) { es = append(es, e) })
	return es, err
}

func count(path string) (Head, int, error) {
	defer lock(path)()
	var head Head
	f, err := os.Open(path)
	if err != nil {
		return head, 0, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 4<<10)
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

func scan(path string, each func(Entry)) (Head, error) {
	defer lock(path)()
	return scanLocked(path, each)
}

func scanLocked(path string, each func(Entry)) (Head, error) {
	var head Head
	f, err := os.Open(path)
	if err != nil {
		return head, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	if !sc.Scan() {
		return head, errors.New("history: an empty file")
	}
	if err := json.Unmarshal(sc.Bytes(), &head); err != nil || head.Start.IsZero() {
		return head, errors.New("history: not a session")
	}
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if each != nil {
			each(e)
		}
	}
	return head, sc.Err()
}

func Purge(dir string) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*", "*.jsonl"+deleted))
	for _, p := range paths {
		os.Remove(p)
	}
}

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

func Rewrite(path string, es []Entry) error {
	defer lock(path)()
	var now []Entry
	head, err := scanLocked(path, func(e Entry) { now = append(now, e) })
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

func rename(from, to string) error {
	err := os.Rename(from, to)
	for i := 0; err != nil && i < 5; i++ {
		time.Sleep(time.Duration(i+1) * 20 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}

func ReadHead(path string) (Head, error) {
	return scan(path, nil)
}
