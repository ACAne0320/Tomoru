// Package engine runs the platform's engine, the process that hears the system and recognizes
// speech, and tomoru-asr, which recognizes what the engine hears with a downloaded model; and
// speaks their protocol (docs/engine-protocol.md).
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

// Protocol is the version of the protocol this package speaks.
const Protocol = 1

// Event is a line the engine sends. Type says which fields it has.
type Event struct {
	Type string `json:"type"`

	// hello
	Protocol int    `json:"protocol,omitempty"`
	Engine   string `json:"engine,omitempty"`
	Version  string `json:"version,omitempty"`

	// state: "downloading", "ready", "listening"; access: "granted", "denied", "unknown"
	State string `json:"state,omitempty"`

	// result
	Final  bool    `json:"final,omitempty"`
	Start  float64 `json:"start,omitempty"`
	End    float64 `json:"end,omitempty"`
	Tokens []Token `json:"tokens,omitempty"`

	// clock, voice, audio: seconds of audio
	T float64 `json:"t,omitempty"`

	// audio: 16 kHz mono 16-bit PCM, in base64
	PCM string `json:"pcm,omitempty"`

	// voice: the speaker's voice started (true) or stopped at T
	On bool `json:"on,omitempty"`

	// error
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`

	// app; decoded: the decode it answers
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Playing bool   `json:"playing,omitempty"`
}

// Token is a word or character of a result. Start and End are nil where the recognizer has not
// timed it, as in volatile results.
type Token struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
	Conf  *float64 `json:"conf,omitempty"`
}

// Text is the result's text: its tokens', with their spaces and punctuation.
func (e Event) Text() string {
	var b strings.Builder
	for _, t := range e.Tokens {
		b.WriteString(t.Text)
	}
	return b.String()
}

// Error is an error event, as a Go error.
type Error struct{ Code, Message string }

func (e *Error) Error() string { return "engine: " + e.Code + ": " + e.Message }

// Path finds the engine (scripts/build-engine-mac.sh builds it).
func Path(roots ...string) (string, error) {
	p, err := resource.Bin("tomoru-engine", roots...)
	if err != nil {
		return "", fmt.Errorf("engine: %w", err)
	}
	return p, nil
}

// ASRPath finds tomoru-asr (scripts/build-asr.sh builds it).
func ASRPath(roots ...string) (string, error) {
	p, err := resource.Bin("tomoru-asr", roots...)
	if err != nil {
		return "", fmt.Errorf("engine: %w", err)
	}
	return p, nil
}

// Log is where the engine's own log goes: its stderr.
var Log io.Writer = os.Stderr

// Process is a running `listen` of the engine, or `serve` of tomoru-asr.
type Process struct {
	cmd       *exec.Cmd
	mu        sync.Mutex // serializes writes; closing stdin must not wait on a blocked write
	stdin     io.WriteCloser
	done      chan struct{}
	err       error // why it ended, set before done closes
	stopOnce  sync.Once
	requested atomic.Bool
}

const shutdownGrace = time.Second

// Listen starts the engine at bin recognizing locale, a BCP 47 tag, from the whole system or, with
// app, from one app, or, with file, from a recording played at its pace; with hearOnly, it
// recognizes nothing and sends audio, for Recognize. It calls onEvent with each event after hello,
// in order, from a goroutine of its own, until the engine ends.
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

// Serve starts tomoru-asr at bin decoding lang ("ja") with the model in dir: each stretch of speech
// given it with Decode is answered with a decoded event. It calls onEvent as Listen does.
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
		// A protocol failure must also stop the child; waiting for an unread, full stdout
		// pipe can otherwise deadlock forever. EOF starts the same bounded shutdown.
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

// read reads the events of r: hello first, which must speak this protocol. An error event ends
// the reading with it.
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

// Audio gives tomoru-asr an audio event of the engine.
func (p *Process) Audio(ev Event) error {
	return p.send(struct {
		Type string  `json:"type"`
		T    float64 `json:"t"`
		PCM  string  `json:"pcm"`
	}{"audio", ev.T, ev.PCM})
}

// Decode asks tomoru-asr to decode a stretch of speech, 16 kHz mono 16-bit PCM in base64; the
// decoded event with id answers it.
func (p *Process) Decode(id, pcm string) error {
	return p.send(struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		PCM  string `json:"pcm"`
	}{"decode", id, pcm})
}

// Finalize asks the recognizer to make its result final through t, seconds of audio.
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

// Stop asks the engine to exit by closing stdin. An unresponsive child is killed after one
// second. It returns promptly even when an Audio write is blocked; Done reports completion.
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

// Done is closed when the engine has ended.
func (p *Process) Done() <-chan struct{} { return p.done }

// Err is why the engine ended, after Done: nil when stopped, an *Error when it said why.
func (p *Process) Err() error { return p.err }

// Run runs a command that answers and exits, as access, apps or prepare, and returns its events
// after hello.
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
	// Once stdout is closed, a one-shot command has no more protocol work to do.
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
