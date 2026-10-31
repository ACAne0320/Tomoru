package models

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"tomoru/internal/resource"
)

// Server runs llama-server, one model at a time, on a port of its own. It is safe for concurrent
// use.
type Server struct {
	bin string
	// Vision is the vision projector of the model at a path, which EnsureVision loads with it; ""
	// for none: Manager.VisionOf. Nil loads none.
	Vision func(model string) string
	// Warm has a server that answers do its first request, for the first real one not to wait:
	// recognition's, a stretch of speech. Nil translates a sentence, and reads a picture with the
	// vision projector (warm).
	Warm func(ctx context.Context, url string) error

	mu   sync.Mutex
	cur  *process
	logs io.Writer
}

type process struct {
	cmd    *exec.Cmd
	model  string // the model file's path
	vision string // its vision projector's, "" for none
	port   int
	url    string
	done   chan struct{} // closed when it exits

	warming sync.Once
	warmed  chan struct{} // closed once warm returned
}

// NewServer runs the llama-server at bin, writing its log to logs (nil for none).
func NewServer(bin string, logs io.Writer) *Server { return &Server{bin: bin, logs: logs} }

// Ensure has llama-server serve the model at path, starting it unless it does already, and waits
// until it answers and has translated once (warm): a few seconds, the model loading, and up to a
// minute the first time on a GPU. It returns the server's base URL.
func (s *Server) Ensure(ctx context.Context, path string) (string, error) {
	return s.ensure(ctx, path, false)
}

// EnsureVision is Ensure, with the model's vision projector loaded too, for it to read pictures. A
// server of the model without it starts again with it, on the same port, where the translator of a
// session going on sends its sentences, and keeps it. Its memory (430 MB more for 2B, 680 MB for
// 9B) is taken only once a picture is translated.
func (s *Server) EnsureVision(ctx context.Context, path string) (string, error) {
	return s.ensure(ctx, path, true)
}

func (s *Server) ensure(ctx context.Context, path string, sees bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var vision string
	if sees && s.Vision != nil {
		vision = s.Vision(path)
	}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return "", err
	}
	p, port := s.cur, 0
	same := p != nil && p.model == path && !p.exited()
	if !same || vision != "" && p.vision != vision {
		if same {
			port = p.port // the same model, now with its vision projector
		}
		s.stopLocked()
		if err := ctx.Err(); err != nil {
			s.mu.Unlock()
			return "", err
		}
		var err error
		if p, err = s.start(path, vision, port); err != nil {
			s.mu.Unlock()
			return "", err
		}
		s.cur = p
	}
	s.mu.Unlock()
	if err := p.wait(ctx); err != nil {
		if port != 0 && ctx.Err() == nil {
			// The port was not free again: on another.
			s.mu.Lock()
			if s.cur == p {
				s.stopLocked()
			}
			s.mu.Unlock()
			return s.ensure(ctx, path, sees)
		}
		return "", err
	}
	p.warming.Do(func() {
		go func() {
			if err := p.warm(s.Warm); err != nil && s.logs != nil {
				fmt.Fprintln(s.logs, "models: warming:", err)
			}
			close(p.warmed)
		}()
	})
	select {
	case <-p.warmed:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return p.url, nil
}

// Stop stops llama-server, which gives its memory back.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *Server) stopLocked() {
	if s.cur != nil {
		p := s.cur
		s.cur = nil
		_ = p.cmd.Process.Kill()
		// Wait reaps the child before a replacement starts, without allowing shutdown
		// to hang indefinitely if the OS does not acknowledge termination.
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
		}
	}
}

// start starts llama-server with the model at path, and its vision projector unless "", on port,
// or on a free one when 0.
func (s *Server) start(path, vision string, port int) (*process, error) {
	if port == 0 {
		var err error
		if port, err = freePort(); err != nil {
			return nil, err
		}
	}
	// No prompt cache: it grew the footprint to 7 GB over a few hundred lines, at no gain in speed.
	args := []string{"-m", path, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"-c", "4096", "-ngl", "99", "--cache-ram", "0", "-ctxcp", "1"}
	if vision != "" {
		args = append(args, "--mmproj", vision)
	}
	cmd := exec.Command(s.bin, args...)
	resource.Hide(cmd)
	cmd.Stdout, cmd.Stderr = s.logs, s.logs
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &process{cmd: cmd, model: path, vision: vision, port: port, url: fmt.Sprintf("http://127.0.0.1:%d", port),
		done: make(chan struct{}), warmed: make(chan struct{})}
	go func() { cmd.Wait(); close(p.done) }()
	return p, nil
}

func (p *process) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// wait waits until the server has loaded the model and answers, and checks that the server
// answering is p's, with p's model: another program may have taken the port meanwhile.
func (p *process) wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// A replaced or stopped process must also interrupt an in-flight HTTP request.
	go func() {
		select {
		case <-p.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return errors.New("models: llama-server exited")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		r, err := serverRequest(ctx, client, p.url+"/health")
		if err != nil {
			continue
		}
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			continue // loading the model
		}
		r, err = serverRequest(ctx, client, p.url+"/v1/models")
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		r.Body.Close()
		if err != nil {
			return err
		}
		if r.StatusCode != http.StatusOK {
			return fmt.Errorf("models: model identity request: HTTP %d", r.StatusCode)
		}
		if !strings.Contains(string(b), filepath.Base(p.model)) {
			return errors.New("models: another server answers on llama-server's port")
		}
		return nil
	}
}

// warm has the server translate a sentence once, as the app's requests look, and read a picture
// with it when it has its vision projector, for the first translation not to wait. The first time a
// GPU runs them, Vulkan builds its programs, in 10 to 40 seconds, which its driver keeps on disk;
// whether it has is not to be known from here, so it warms each time, which then takes as long as a
// sentence. The request has its own context: it finishes when the one waiting gives up, for the next
// not to build them again. A failure only leaves the first translation slow. custom, when not nil,
// is the request instead.
func (p *process) warm(custom func(context.Context, string) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	go func() {
		select {
		case <-p.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	if custom != nil {
		return custom(ctx, p.url)
	}
	content := any(warmText)
	if p.vision != "" {
		content = []map[string]any{
			{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + warmPicture()}},
			{"type": "text", "text": warmText},
		}
	}
	body, err := json.Marshal(map[string]any{
		"messages":   []map[string]any{{"role": "user", "content": content}},
		"max_tokens": 8, "temperature": 0, "cache_prompt": false,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", r.StatusCode)
	}
	return nil
}

// warmText is what warm translates: about as long as a subtitle's request.
const warmText = "把下面的文本翻译成中文，只输出译文，不要解释。\n\nI never thought I would see you again, not after everything that happened at the harbor that night."

// warmPicture is the picture warm has read: lines of light strokes on dark, as a game's dialogue,
// about as many tokens as a selection's (some 380), in base64.
var warmPicture = sync.OnceValue(func() string {
	const w, h = 768, 384
	im := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(24)
			if y%64 > 24 && y%64 < 48 && x%14 < 8 && x > 32 && x < w-32 {
				v = 230
			}
			im.Pix[y*im.Stride+x] = v
		}
	}
	var b bytes.Buffer
	png.Encode(&b, im)
	return base64.StdEncoding.EncodeToString(b.Bytes())
})

// serverRequest binds both connection establishment and body reads to cancellation
// and to a short per-request timeout, within the overall model-loading deadline.
func serverRequest(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// freePort is a port nothing listens on now.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
