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

type Server struct {
	bin    string
	Vision func(model string) string
	Warm   func(ctx context.Context, url string) error

	mu   sync.Mutex
	cur  *process
	logs io.Writer
}

type process struct {
	cmd    *exec.Cmd
	model  string
	vision string
	port   int
	url    string
	done   chan struct{}

	warming sync.Once
	warmed  chan struct{}
}

func NewServer(bin string, logs io.Writer) *Server { return &Server{bin: bin, logs: logs} }

func (s *Server) Ensure(ctx context.Context, path string) (string, error) {
	return s.ensure(ctx, path, false)
}

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
			port = p.port
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
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
		}
	}
}

func (s *Server) start(path, vision string, port int) (*process, error) {
	if port == 0 {
		var err error
		if port, err = freePort(); err != nil {
			return nil, err
		}
	}
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

func (p *process) wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
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
			continue
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

const warmText = "把下面的文本翻译成中文，只输出译文，不要解释。\n\nI never thought I would see you again, not after everything that happened at the harbor that night."

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

func serverRequest(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
