package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tomoru/internal/resource"
)

const Protocol = 1

type Event struct {
	Type string `json:"type"`

	Protocol int    `json:"protocol,omitempty"`
	Engine   string `json:"engine,omitempty"`
	Version  string `json:"version,omitempty"`

	State string `json:"state,omitempty"`

	Final  bool    `json:"final,omitempty"`
	Start  float64 `json:"start,omitempty"`
	End    float64 `json:"end,omitempty"`
	Tokens []Token `json:"tokens,omitempty"`
	Lang   string  `json:"lang,omitempty"`

	T float64 `json:"t,omitempty"`

	PCM string `json:"pcm,omitempty"`

	On bool `json:"on,omitempty"`

	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`

	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Playing bool   `json:"playing,omitempty"`
}

type Token struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
	Conf  *float64 `json:"conf,omitempty"`
}

func (e Event) Text() string {
	var b strings.Builder
	for _, t := range e.Tokens {
		b.WriteString(t.Text)
	}
	return b.String()
}

type Error struct{ Code, Message string }

func (e *Error) Error() string { return "engine: " + e.Code + ": " + e.Message }

func Path(roots ...string) (string, error) {
	p, err := resource.Bin("tomoru-engine", roots...)
	if err != nil {
		return "", fmt.Errorf("engine: %w", err)
	}
	return p, nil
}

func ASRPath(roots ...string) (string, error) {
	p, err := resource.Bin("tomoru-asr", roots...)
	if err != nil {
		return "", fmt.Errorf("engine: %w", err)
	}
	return p, nil
}

var Log io.Writer = os.Stderr

type Process struct {
	cmd       *exec.Cmd
	mu        sync.Mutex
	stdin     io.WriteCloser
	done      chan struct{}
	err       error
	stopOnce  sync.Once
	requested atomic.Bool
}

const shutdownGrace = time.Second

func Listen(bin, locale, app, file string, hearOnly bool, onEvent func(Event)) (*Process, error) {
	args := []string{"listen", "--locale", locale}
	if app != "" {
		args = append(args, "--app", app)
	}
	if file != "" {
		args = append(args, "--file", file)
	}
	if hearOnly {
		args = append(args, "--asr", "none")
	}
	return start(bin, args, onEvent)
}

func Serve(bin, dir, lang string, onEvent func(Event)) (*Process, error) {
	return start(bin, []string{"serve", "--model", dir, "--lang", lang}, onEvent)
}

func start(bin string, args []string, onEvent func(Event)) (*Process, error) {
	cmd := exec.Command(bin, args...)
	resource.Hide(cmd)
	cmd.Stderr = Log
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		out.Close()
		return nil, err
	}
	p := &Process{cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go func() {
		err := read(out, onEvent)
		if err != nil {
			_ = cmd.Process.Kill()
		}
		p.terminate()
		if werr := cmd.Wait(); err == nil && werr != nil && !p.requested.Load() {
			err = werr
		}
		p.err = err
		close(p.done)
	}()
	return p, nil
}

func read(r io.Reader, onEvent func(Event)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	first := true
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return fmt.Errorf("engine: %w: %q", err, sc.Text())
		}
		if first {
			first = false
			if ev.Type != "hello" || ev.Protocol != Protocol {
				return fmt.Errorf("engine: speaks protocol %d, not %d", ev.Protocol, Protocol)
			}
			continue
		}
		if ev.Type == "error" {
			return &Error{ev.Code, ev.Message}
		}
		onEvent(ev)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if first {
		return errors.New("engine: missing hello")
	}
	return nil
}

func (p *Process) Audio(ev Event) error {
	return p.send(struct {
		Type string  `json:"type"`
		T    float64 `json:"t"`
		PCM  string  `json:"pcm"`
	}{"audio", ev.T, ev.PCM})
}

func (p *Process) Decode(id, pcm string) error {
	return p.send(struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		PCM  string `json:"pcm"`
	}{"decode", id, pcm})
}

func (p *Process) Finalize(t float64) error {
	return p.send(struct {
		Type    string  `json:"type"`
		Through float64 `json:"through"`
	}{"finalize", t})
}

func (p *Process) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.requested.Load() {
		return errors.New("engine: stopped")
	}
	_, err = p.stdin.Write(append(b, '\n'))
	return err
}

func (p *Process) Stop() {
	p.requested.Store(true)
	p.terminate()
}

func (p *Process) terminate() {
	p.stopOnce.Do(func() {
		_ = p.stdin.Close()
		go func() {
			timer := time.NewTimer(shutdownGrace)
			defer timer.Stop()
			select {
			case <-p.done:
			case <-timer.C:
				_ = p.cmd.Process.Kill()
			}
		}()
	})
}

func (p *Process) Done() <-chan struct{} { return p.done }

func (p *Process) Err() error { return p.err }

func Run(ctx context.Context, bin string, args ...string) ([]Event, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	resource.Hide(cmd)
	cmd.Stderr = Log
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		out.Close()
		return nil, err
	}
	var evs []Event
	err = read(out, func(ev Event) { evs = append(evs, ev) })
	if err != nil {
		_ = cmd.Process.Kill()
	}
	timer := time.AfterFunc(shutdownGrace, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()
	if werr := cmd.Wait(); err == nil {
		err = werr
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return evs, err
}
