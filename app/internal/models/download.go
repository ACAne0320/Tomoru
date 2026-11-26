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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type State int

const (
	Missing State = iota
	Downloading
	Verifying
	Ready
)

type Status struct {
	State       State
	Done, Total int64
	Err         error
}

var ErrChecksum = errors.New("models: the download is damaged")

type ErrNoSpace struct{ Need, Free int64 }

func (e ErrNoSpace) Error() string {
	return fmt.Sprintf("models: %d bytes free, %d needed", e.Free, e.Need)
}

type Manager struct {
	dir            string
	mirrors        MirrorOrder
	minMirrorSpeed int64
	sampleInterval time.Duration
	client         *http.Client
	notices        chan struct{}

	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	closed           bool
	mu               sync.Mutex
	jobs             map[string]*job
	errs             map[string]error
	move             Move
	statusDuringMove map[string]Status
	rename           func(from, to string) error
}

type job struct {
	cancel      context.CancelFunc
	exited      chan struct{}
	state       State
	done, total int64
}

func NewManager(dir string, mirrors MirrorOrder, onChange func()) *Manager {
	return NewManagerContext(context.Background(), dir, mirrors, onChange)
}

type ManagerOption func(*Manager)

func WithClient(c *http.Client) ManagerOption { return func(m *Manager) { m.client = c } }

func NewManagerContext(parent context.Context, dir string, mirrors MirrorOrder, onChange func(), opts ...ManagerOption) *Manager {
	ctx, cancel := context.WithCancel(parent)
	m := &Manager{dir: dir, mirrors: mirrors, minMirrorSpeed: 1 << 20, sampleInterval: 10 * time.Second, client: &http.Client{},
		ctx: ctx, cancel: cancel, jobs: map[string]*job{}, errs: map[string]error{}, rename: os.Rename, notices: make(chan struct{}, 1)}
	for _, o := range opts {
		o(m)
	}
	if onChange != nil {
		go m.notify(onChange)
	}
	return m
}

func (m *Manager) notify(onChange func()) {
	for {
		select {
		case <-m.notices:
			onChange()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	waitAtMost(done, shutdownWait)
}

const shutdownWait = 2 * time.Second

func waitAtMost(done <-chan struct{}, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

func (m *Manager) Path(id string) string {
	mod, _ := ByID(id)
	if len(mod.Files) == 0 {
		return ""
	}
	return m.file(mod, mod.MainFile())
}

func (m *Manager) ProjectorOf(path string) string {
	for _, mod := range All {
		if mod.Projector != "" && m.Path(mod.ID) == path && m.Status(mod.ID).State == Ready {
			return filepath.Join(m.Dir(), mod.Dir, mod.Projector)
		}
	}
	return ""
}

func (m *Manager) Dir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dir
}

func (m *Manager) file(mod Model, f File) string { return filePath(m.Dir(), mod, f) }

func filePath(dir string, mod Model, f File) string { return filepath.Join(dir, mod.Dir, f.Name) }

func have(dir string, mod Model) (done int64, all bool) {
	all = true
	for _, f := range mod.Files {
		if fi, err := os.Stat(filePath(dir, mod, f)); err == nil && mod.complete(f, fi.Size()) {
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
	if st, ok := m.statusDuringMove[id]; ok {
		return st
	}
	st := onDisk(m.dir, mod)
	if st.State == Missing {
		st.Err = m.errs[id]
	}
	return st
}

func onDisk(dir string, mod Model) Status {
	done, all := have(dir, mod)
	if all {
		return Status{State: Ready, Done: mod.Size, Total: mod.Size}
	}
	return Status{State: Missing, Done: done, Total: mod.Size}
}

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

func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[id]; j != nil {
		j.cancel()
	}
}

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
		os.Remove(filepath.Join(dir, mod.Dir))
	}
	m.changed()
	return err
}

func (m *Manager) changed() {
	m.mu.Lock()
	closed := m.closed || m.ctx.Err() != nil
	m.mu.Unlock()
	if !closed {
		select {
		case m.notices <- struct{}{}:
		default:
		}
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
	var before int64
	for _, f := range mod.Files {
		if err := m.downloadFile(ctx, mod, f, j, before); err != nil {
			return err
		}
		before += f.Size
	}
	return nil
}

func (m *Manager) downloadFile(ctx context.Context, mod Model, file File, j *job, before int64) error {
	path := m.file(mod, file)
	if fi, err := os.Stat(path); err == nil && mod.complete(file, fi.Size()) {
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
		have = 0
	}
	var last error
	urls := mod.URLs(file, m.mirrors)
	for i, url := range urls {
		slow := m.minMirrorSpeed
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

var errSlow = errors.New("models: too slow")

var errStalled = errors.New("models: download stalled")

func (m *Manager) fetch(ctx context.Context, url string, f *os.File, have, size int64, j *job, before, slow int64) (int64, error) {
	if have == size {
		return have, nil
	}
	requestCtx, cancel := context.WithCancelCause(ctx)
	var received atomic.Int64
	watchDone := make(chan struct{})
	ticker := time.NewTicker(m.sampleInterval)
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
				if first && slow > 0 && float64(n)/m.sampleInterval.Seconds() < float64(slow) {
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
			return ctx.Err()
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
		if total := rangeTotal(resp.Header.Get("Content-Range")); total >= 0 && total != size {
			return have, otherFile(url, total, size)
		}
	case http.StatusOK:
		if resp.ContentLength >= 0 && resp.ContentLength != size {
			return have, otherFile(url, resp.ContentLength, size)
		}
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
		if have+int64(n) > size {
			return 0, errors.Join(fmt.Errorf("models: %s sends more than %d bytes", url, size), f.Truncate(0))
		}
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

func otherFile(url string, got, want int64) error {
	return fmt.Errorf("models: %s is %d bytes, not %d", url, got, want)
}

func rangeTotal(contentRange string) int64 {
	_, total, ok := strings.Cut(contentRange, "/")
	if !ok {
		return -1
	}
	n, err := strconv.ParseInt(total, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func (m *Manager) set(f func()) {
	m.mu.Lock()
	f()
	m.mu.Unlock()
	m.changed()
}

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
