package translate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// picture is a PNG of w by h pixels.
func picture(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestTranslateImage sends a picture with the prompt in one turn, the glossary whole in it, and
// reads the translation as it streams in, a line repeated once.
func TestTranslateImage(t *testing.T) {
	var body struct {
		Messages []struct {
			Content []struct {
				Type     string            `json:"type"`
				Text     string            `json:"text"`
				ImageURL map[string]string `json:"image_url"`
			} `json:"content"`
		} `json:"messages"`
		MaxTokens int `json:"max_tokens"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		for _, piece := range []string{"艾露娜：克劳德。", "\nHP 120", "\nHP 120", "\n完"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, Llama: true}
	pic := picture(t, 1200, 300) // sent as it is
	var last string
	out, err := e.TranslateImage(context.Background(), pic, "zh", []Term{{"クロード", "克劳德"}, {"エルナ", "艾露娜"}}, func(s string) { last = s })
	if want := "艾露娜：克劳德。\nHP 120\n完"; err != nil || out != want || last != want {
		t.Fatalf("%q %v, streamed %q", out, err, last)
	}
	if len(body.Messages) != 1 || len(body.Messages[0].Content) != 2 {
		t.Fatalf("messages %+v", body.Messages)
	}
	img, text := body.Messages[0].Content[0], body.Messages[0].Content[1]
	sent, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(img.ImageURL["url"], "data:image/png;base64,"))
	if img.Type != "image_url" || !bytes.Equal(sent, pic) {
		t.Errorf("picture part %q, %d bytes", img.Type, len(sent))
	}
	if text.Type != "text" || !strings.Contains(text.Text, "请将图片中的文本翻译为中文") || !strings.Contains(text.Text, "【硬性要求】专名/术语对照: クロード→克劳德、エルナ→艾露娜") {
		t.Errorf("prompt %q", text.Text)
	}
	if body.MaxTokens < 1024 {
		t.Errorf("max_tokens %d: a page's text would be cut off", body.MaxTokens)
	}
}

// TestNoPictures sends nothing to a model whose format reads no pictures, and says so.
func TestNoPictures(t *testing.T) {
	var asked int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: HyMT}
	if out, err := e.TranslateImage(context.Background(), picture(t, 1200, 300), "zh", nil, nil); !errors.Is(err, ErrNoPictures) || out != "" || asked != 0 {
		t.Errorf("%q %v, %d requests", out, err, asked)
	}
	if HyMT.ReadsPictures() || !Index.ReadsPictures() || !Chat.ReadsPictures() {
		t.Error("which formats read pictures")
	}
}

// TestImagePrompt asks plainly without a glossary, and a chat model in English.
func TestImagePrompt(t *testing.T) {
	if p := indexImagePrompt("en", nil); p != "请将图片中的文本翻译为英语，直接输出翻译结果。" {
		t.Errorf("Index prompt %q", p)
	}
	if p := chatImagePrompt("ja", []Term{{"Cloud", "クラウド"}}); !strings.Contains(p, "text in the image into Japanese") || !strings.Contains(p, "Cloud → クラウド") {
		t.Errorf("chat prompt %q", p)
	}
}

// TestFit scales a picture larger than MaxSide down to it, keeping its shape, one smaller than
// MinSide up two or three times, no larger than the most a model takes, and sends one between as it
// is; what is not a PNG is an error.
func TestFit(t *testing.T) {
	mid := picture(t, 1200, 300)
	if got, err := fit(mid, MaxSide); err != nil || !bytes.Equal(got, mid) {
		t.Errorf("between: %d bytes, %v", len(got), err)
	}
	for _, c := range []struct{ w, h, max, wantW, wantH int }{
		{4096, 1024, MaxSide, 2048, 512},
		{348, 126, MaxSide, 1044, 378}, // a selection on a screen at 100%: three times
		{520, 180, MaxSide, 1040, 360}, // twice
		{120, 600, MaxSide, 240, 1200}, // the long side counts, either way
		{900, 300, 1280, 1280, 426},    // as far as a local model's most
	} {
		got, err := fit(picture(t, c.w, c.h), c.max)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _ := png.DecodeConfig(bytes.NewReader(got))
		if cfg.Width != c.wantW || cfg.Height != c.wantH {
			t.Errorf("%dx%d scaled to %dx%d, want %dx%d", c.w, c.h, cfg.Width, cfg.Height, c.wantW, c.wantH)
		}
	}
	if _, err := fit([]byte("not a picture"), MaxSide); err == nil {
		t.Error("no error for a non-PNG")
	}
}

func TestDedupe(t *testing.T) {
	for in, want := range map[string]string{
		"a\nb\nb\nc":  "a\nb\nc",
		"a\n\na\nb":   "a\n\nb", // the blank line between keeps its place
		"a\nb\na":     "a\nb\na",
		"HP 1\nHP 1 ": "HP 1",
		"":            "",
	} {
		if got := Dedupe(in); got != want {
			t.Errorf("Dedupe(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestImageLoop ends a translation whose model says a line over and over, keeping what came before;
// a picture is scaled to the endpoint's ImageSide.
func TestImageLoop(t *testing.T) {
	var sent []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content []struct {
					ImageURL map[string]string `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		sent, _ = base64.StdEncoding.DecodeString(strings.TrimPrefix(body.Messages[0].Content[0].ImageURL["url"], "data:image/png;base64,"))
		for i := range 1000 {
			piece := "Elenna:\n"
			if i == 0 {
				piece = "Claude?\n"
			}
			if _, err := fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()
	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, ImageSide: 100}
	out, err := e.TranslateImage(context.Background(), picture(t, 400, 200), "en", nil, nil)
	if err != nil || out != "Claude?\nElenna:" {
		t.Errorf("%q %v", out, err)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(sent)); err != nil || cfg.Width != 100 || cfg.Height != 50 {
		t.Errorf("sent %+v %v", cfg, err)
	}
}

func TestLooping(t *testing.T) {
	for in, want := range map[string]bool{
		"a\na\na\n":       true,
		"a\na\n\na\n":     true,  // blank lines between
		"a\na\na":         false, // the third not ended yet
		"a\na\nb\n":       false,
		"b\na\na\na\nx":   true,
		"a\nb\na\nb\na\n": false,
		"":                false,
	} {
		if got := looping(in); got != want {
			t.Errorf("looping(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestImageStalled asks a picture whose first words have not come within ImageWait again, once,
// on a new connection: over HTTP/2 the request would go on the one that answers nothing.
func TestImageStalled(t *testing.T) {
	var mu sync.Mutex
	var conns []string
	answer := 1 // the request that answers, from 0; others answer nothing
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n, a := len(conns), answer
		conns = append(conns, r.RemoteAddr)
		mu.Unlock()
		if n != a {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"完\"}}]}\n\ndata: [DONE]\n\n")
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	// stalled handlers may still run after a call returns, so the test reads and sets under mu too
	seen := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(conns)
	}
	reset := func(c []string, a int) {
		mu.Lock()
		defer mu.Unlock()
		conns, answer = c, a
	}

	e := &Endpoint{URL: srv.URL, Model: "m", Format: Index, Client: srv.Client(), ImageWait: 50 * time.Millisecond}
	out, err := e.TranslateImage(context.Background(), picture(t, 1200, 300), "zh", nil, nil)
	if err != nil || out != "完" {
		t.Fatalf("%q %v", out, err)
	}
	c := seen()
	if len(c) != 2 || c[0] == c[1] {
		t.Errorf("asked on %q, not on a new connection", c)
	}
	reset(c, 2) // the next picture goes on a connection other than the one stalled
	_, err = e.TranslateImage(context.Background(), picture(t, 1200, 300), "zh", nil, nil)
	if c = seen(); err != nil || len(c) != 3 || c[2] == c[0] {
		t.Errorf("%v, asked on %q", err, c)
	}

	reset(nil, -1)
	_, err = e.TranslateImage(context.Background(), picture(t, 1200, 300), "zh", nil, nil)
	if c = seen(); !errors.Is(err, ErrStalled) || len(c) != 2 {
		t.Errorf("%v, asked %d times", err, len(c))
	}
}
