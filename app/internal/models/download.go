package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// State is where a model is.
type State int

const (
	Missing     State = iota // not downloaded
	Downloading              // coming in: Done of Total bytes
	Verifying                // downloaded, its checksum being checked
	Ready                    // downloaded and checked
)

// Status is a model's state, and how its download goes or why it failed.
type Status struct {
	State       State
	Done, Total int64
	Err         error // why the last download failed, while Missing
}

// ErrChecksum is a download that is not the model: it is deleted.
var ErrChecksum = errors.New("models: the download is damaged")

// ErrNoSpace is a disk without room for the model.
type ErrNoSpace struct{ Need, Free int64 }

func (e ErrNoSpace) Error() string {
	return fmt.Sprintf("models: %d bytes free, %d needed", e.Free, e.Need)
}

// Manager downloads models into a directory, a model's files in its own directory when it has one.
// A file coming in has ".part" after its name, which a later download goes on from; a file is
// there only once its checksum is right, and a model is Ready once all of its files are. A Manager
// is safe for concurrent use.
type Manager struct {
	dir      string // guarded by mu: a move changes it
	china    bool
	slow     int64         // bytes a second below which a source that is not the last gives way
	window   time.Duration // speed sample and no-progress check interval
	client   *http.Client
	onChange func() // after a status changes; progress, a few times a second

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
	mu     sync.Mutex
	jobs   map[string]*job
	errs   map[string]error
	move   Move
	held   map[string]Status           // the statuses as a move found them, until it ends
	rename func(from, to string) error // os.Rename; tests make it fail, as across disks
}

type job struct {
	cancel      context.CancelFunc
	exited      chan struct{} // closed once its goroutine is done with the files
	state       State
	done, total int64
}

// NewManager keeps models in dir, downloading them first from ModelScope when china, and calls
// onChange, from any goroutine, as statuses change.
func NewManager(dir string, china bool, onChange func()) *Manager {
	return NewManagerContext(context.Background(), dir, china, onChange)
}

// NewManagerContext ties every download to the application lifetime. Close cancels
// and waits for work; onChange must return promptly and must not call Close itself.
func NewManagerContext(parent context.Context, dir string, china bool, onChange func()) *Manager {
	ctx, cancel := context.WithCancel(parent)
	return &Manager{dir: dir, china: china, slow: 1 << 20, window: 10 * time.Second, onChange: onChange, client: &http.Client{},
		ctx: ctx, cancel: cancel, jobs: map[string]*job{}, errs: map[string]error{}, rename: os.Rename}
}

// Close rejects new downloads, cancels active requests/checksums, and waits for them
// to close their files. The bound also covers an uncooperative custom HTTP transport.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

// Path is where a model's first file is, once Ready: the model itself. Its other files are beside
// it.
func (m *Manager) Path(id string) string {
	mod, _ := ByID(id)
	if len(mod.Files) == 0 {
		return ""
	}
	return m.file(mod, mod.Files[0])
}

// VisionOf is the vision projector of the translation model at path, if it has one and is
// downloaded, else "": what Server.Vision asks.
func (m *Manager) VisionOf(path string) string {
	for _, mod := range All {
		if mod.Vision != "" && m.Path(mod.ID) == path && m.Status(mod.ID).State == Ready {
			return filepath.Join(m.Dir(), mod.Dir, mod.Vision)
		}
	}
	return ""
}

// Dir is the directory the models are kept in.
func (m *Manager) Dir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dir
}

func (m *Manager) file(mod Model, f File) string { return filePath(m.Dir(), mod, f) }

func filePath(dir string, mod Model, f File) string { return filepath.Join(dir, mod.Dir, f.Name) }

// have is how much of a model is in dir: its files downloaded, and the parts of the others.
func have(dir string, mod Model) (done int64, all bool) {
	all = true
	for _, f := range mod.Files {
		if fi, err := os.Stat(filePath(dir, mod, f)); err == nil && fi.Size() == f.Size {
			done += f.Size
			continue
		}
		all = false
		if fi, err := os.Stat(filePath(dir, mod, f) + ".part"); err == nil && fi.Size() <= f.Size {
			done += fi.Size()
		}
	}
	return done, all
}

// Status says where a model is.
func (m *Manager) Status(id string) Status {
	mod, ok := ByID(id)
	if !ok {
		return Status{Err: fmt.Errorf("models: no %s", id)}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[id]; j != nil {
		return Status{State: j.state, Done: j.done, Total: j.total}
	}
	if st, ok := m.held[id]; ok {
		return st
	}
	st := onDisk(m.dir, mod)
	if st.State == Missing {
		st.Err = m.errs[id]
	}
	return st
}

// onDisk is a model's status as dir has it.
func onDisk(dir string, mod Model) Status {
	done, all := have(dir, mod)
	if all {
		return Status{State: Ready, Done: mod.Size, Total: mod.Size}
	}
	return Status{State: Missing, Done: done, Total: mod.Size} // Done: to go on from
}

// Download starts downloading a model, or goes on with it, unless it is under way or ready, or
// the models are moving.
func (m *Manager) Download(id string) {
	mod, ok := ByID(id)
	if !ok {
		return
	}
	if m.Status(id).State != Missing {
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil || m.jobs[id] != nil || m.move.Active {
		m.mu.Unlock()
		cancel()
		return
	}
	j := &job{cancel: cancel, exited: make(chan struct{}), state: Downloading, total: mod.Size}
	m.jobs[id] = j
	delete(m.errs, id)
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		defer cancel()
		m.changed()
		err := m.download(ctx, mod, j)
		m.mu.Lock()
		delete(m.jobs, id)
		if err != nil && !errors.Is(err, context.Canceled) {
			m.errs[id] = err
		}
		close(j.exited)
		m.mu.Unlock()
		m.changed()
	}()
}

// Cancel stops a download; what came in stays, to go on from.
func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[id]; j != nil {
		j.cancel()
	}
}

// Delete removes a model, and what came in of it; while the models are moving, nothing.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	dir, moving := m.dir, m.move.Active
	m.mu.Unlock()
	if moving {
		return nil
	}
	m.Cancel(id)
	m.mu.Lock()
	delete(m.errs, id)
	m.mu.Unlock()
	mod, _ := ByID(id)
	var err error
	for _, f := range mod.Files {
		if e := os.Remove(filePath(dir, mod, f)); e != nil && !os.IsNotExist(e) && err == nil {
			err = e
		}
		os.Remove(filePath(dir, mod, f) + ".part")
	}
	if mod.Dir != "" {
		os.Remove(filepath.Join(dir, mod.Dir)) // once empty
	}
	m.changed()
	return err
}

func (m *Manager) changed() {
	m.mu.Lock()
	closed := m.closed || m.ctx.Err() != nil
	m.mu.Unlock()
	if !closed && m.onChange != nil {
		m.onChange()
	}
}

func (m *Manager) download(ctx context.Context, mod Model, j *job) error {
	dir := m.Dir()
	if err := os.MkdirAll(filepath.Join(dir, mod.Dir), 0o755); err != nil {
		return err
	}
	done, _ := have(dir, mod)
	m.set(func() { j.done = done })
	if err := enoughSpace(dir, mod.Size-done); err != nil {
		return err
	}
	var before int64 // the bytes of the files before this one
	for _, f := range mod.Files {
		if err := m.downloadFile(ctx, mod, f, j, before); err != nil {
			return err
		}
		before += f.Size
	}
	return nil
}

// downloadFile downloads a file of a model unless it is here, j's progress counting the files
// before it.
func (m *Manager) downloadFile(ctx context.Context, mod Model, file File, j *job, before int64) error {
	path := m.file(mod, file)
	if fi, err := os.Stat(path); err == nil && fi.Size() == file.Size {
		return nil
	}
	part := path + ".part"
	if err := m.fetchFile(ctx, mod, file, part, j, before); err != nil {
		return err
	}
	m.set(func() { j.state = Verifying })
	ok, err := checksum(ctx, part, file.SHA256)
	if err != nil {
		return err
	}
	if !ok {
		os.Remove(part)
		return ErrChecksum
	}
	if err := os.Rename(part, path); err != nil {
		return err
	}
	m.set(func() { j.state = Downloading })
	return nil
}

// fetchFile downloads a file of a model into part, going on from what part has. It closes part
// before it returns: Windows renames and removes no file that is open.
func (m *Manager) fetchFile(ctx context.Context, mod Model, file File, part string, j *job, before int64) error {
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	have, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if have > file.Size {
		have = 0 // not this file's
	}
	var last error
	urls := mod.URLs(file, m.china)
	for i, url := range urls {
		// A slow source gives way to the next, which goes on from there; the last
		// goes on, however slow, while it keeps making progress.
		slow := m.slow
		if i == len(urls)-1 {
			slow = 0
		}
		if have, last = m.fetch(ctx, url, f, have, file.Size, j, before, slow); last == nil || ctx.Err() != nil {
			break
		}
	}
	if last != nil {
		return last
	}
	return f.Close()
}

// errSlow is a source slower than Manager.slow.
var errSlow = errors.New("models: too slow")

var errStalled = errors.New("models: download stalled")

// fetch downloads url into f from byte have on, and returns how much f has. With slow, it gives up
// when its first m.window came at fewer bytes a second. Every source gives up after a full
// sampling interval without progress, including while waiting for headers. There is no total
// download deadline. j's progress counts the before bytes of the files before.
func (m *Manager) fetch(ctx context.Context, url string, f *os.File, have, size int64, j *job, before, slow int64) (int64, error) {
	if have == size {
		return have, nil
	}
	requestCtx, cancel := context.WithCancelCause(ctx)
	var received atomic.Int64
	watchDone := make(chan struct{})
	ticker := time.NewTicker(m.window)
	go func() {
		defer close(watchDone)
		defer ticker.Stop()
		var previous int64
		first := true
		for {
			select {
			case <-requestCtx.Done():
				return
			case <-ticker.C:
				n := received.Load()
				if first && slow > 0 && float64(n)/m.window.Seconds() < float64(slow) {
					cancel(errSlow)
					return
				}
				if n == previous {
					cancel(errStalled)
					return
				}
				previous, first = n, false
			}
		}
	}()
	defer func() { cancel(nil); <-watchDone }()
	requestError := func(err error) error {
		if ctx.Err() != nil {
			return ctx.Err() // user cancellation never advances to another mirror
		}
		if cause := context.Cause(requestCtx); cause != nil {
			return cause
		}
		return err
	}
	req, err := http.NewRequestWithContext(requestCtx, "GET", url, nil)
	if err != nil {
		return have, err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return have, requestError(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK: // the server sends it all: start over
		have = 0
		if err := f.Truncate(0); err != nil {
			return have, err
		}
	default:
		return have, fmt.Errorf("models: %s: %s", url, resp.Status)
	}
	if _, err := f.Seek(have, io.SeekStart); err != nil {
		return have, err
	}
	buf := make([]byte, 256<<10)
	var told time.Time
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return have, err
			}
			have += int64(n)
			received.Add(int64(n))
			if time.Since(told) > 250*time.Millisecond {
				told = time.Now()
				m.set(func() { j.done = before + have })
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return have, requestError(rerr)
		}
	}
	m.set(func() { j.done = before + have })
	if have != size {
		return have, fmt.Errorf("models: %s ended at %d of %d bytes", url, have, size)
	}
	return have, nil
}

// set changes a job's status, and tells.
func (m *Manager) set(f func()) {
	m.mu.Lock()
	f()
	m.mu.Unlock()
	m.changed()
}

// checksum tells whether the file at path has the SHA-256 want.
func checksum(ctx context.Context, path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, err := f.Read(buf)
		h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}
