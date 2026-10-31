package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/prefs"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

// TestTranslateImage translates a picture into the language translating goes to, with the scene's
// names, as it streams in; into another chosen on its card, again, without them, the subtitles'
// language kept; with no picture, it says so; closed meanwhile, it drops what comes. Each translation
// got is counted, under Index-Translate in the cloud; none is in the guide.
func TestTranslateImage(t *testing.T) {
	var prompt string
	asked, done := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		prompt = string(b)
		if strings.Contains(prompt, "slow") {
			close(asked)
			<-r.Context().Done() // until the card closes
			close(done)
			return
		}
		for _, piece := range []string{"克劳德：", "走吧。"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()
	c, f := scenesCore(t)
	c.ImageService.endpoint = &translate.Endpoint{URL: srv.URL, Model: "m", Style: translate.Index}
	c.Prefs.Onboarded = true
	id := c.AddScene("FF7")
	c.AddTerm(id, scene.Term{Src: "クラウド", Tgt: "克劳德"})
	c.SetScene(id)

	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	c.TranslateImage(b.Bytes())
	if !c.Image.Running || c.Image.PNG == nil {
		t.Fatalf("translating: %+v", c.Image)
	}
	f.flush(t, 3) // two pieces, then the end
	if c.Image.Running || c.Image.Err != nil || c.Image.Text != "克劳德：走吧。" {
		t.Errorf("translated: %+v", c.Image)
	}
	if !strings.Contains(prompt, "翻译为中文") || !strings.Contains(prompt, "クラウド→克劳德") {
		t.Errorf("prompt %s", prompt)
	}

	tgt := c.Prefs.Tgt
	c.Image.Area = image.Rect(10, 20, 30, 40)
	c.SetImageTgt("en")
	if !c.Image.Running || c.Image.PNG == nil || c.Image.Area.Empty() || c.ImageTgt() != "en" || c.Prefs.Tgt != tgt {
		t.Fatalf("into English: tgt %q, image's %q, %+v", c.Prefs.Tgt, c.ImageTgt(), c.Image)
	}
	f.flush(t, 3)
	if strings.Contains(prompt, "翻译为中文") || strings.Contains(prompt, "克劳德") {
		t.Errorf("English prompt %s", prompt)
	}
	c.SetImageTgt(tgt) // the subtitles' again: followed
	f.flush(t, 3)
	if c.Prefs.ImageTgt != "" || c.ImageTgt() != tgt {
		t.Errorf("following %q: %q", tgt, c.Prefs.ImageTgt)
	}

	c.TranslateImage(nil)
	if c.Image.Running || c.Image.PNG != nil {
		t.Errorf("no picture: %+v", c.Image)
	}

	c.ImageService.endpoint.Model = "slow"
	c.TranslateImage(b.Bytes())
	<-asked
	c.CloseImage()
	<-done
	f.flush(t, 1)
	if c.Image.Running || c.Image.PNG != nil || c.Image.Text != "" || c.Image.Err != nil {
		t.Errorf("closed: %+v", c.Image)
	}
	var body map[string]any
	json.Unmarshal([]byte(prompt), &body)
	if body["model"] != "slow" {
		t.Errorf("the slow one did not go: %v", body["model"])
	}
	images := func() (n int) {
		for _, d := range c.Usage().Days {
			for _, u := range d.Uses {
				if u.MT == "cloud" {
					n += u.Images
				}
			}
		}
		return n
	}
	if n := images(); n != 3 {
		t.Errorf("%d pictures counted, want the three translated", n)
	}
	c.Prefs.Onboarded = false
	c.ImageService.endpoint.Model = "m"
	c.TranslateImage(b.Bytes())
	f.flush(t, 3)
	if n := images(); n != 3 || c.Image.Text == "" {
		t.Errorf("%d pictures counted with the guide's, translated %q", n, c.Image.Text)
	}
}

// TestSelectImage selects a part of the screen once the app may capture it, and translates it;
// cancelled, nothing; failing, it checks the permission again.
func TestSelectImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
	}))
	defer srv.Close()
	c, f := newCore(t)
	c.ImageService.endpoint = &translate.Endpoint{URL: srv.URL, Model: "m", Style: translate.Index}
	f.answers["screen-access"] = []engine.Event{{Type: "access", State: "unknown"}}
	c.CheckScreen()
	f.flush(t, 1)
	if c.Screen != "unknown" || c.CanSelect() {
		t.Fatalf("not asked: %q, can select %v", c.Screen, c.CanSelect())
	}
	f.answers["request-screen-access"] = []engine.Event{{Type: "access", State: "granted"}}
	c.RequestScreen()
	if !c.ScreenAsking {
		t.Error("not asking")
	}
	f.flush(t, 1)
	if c.Screen != "granted" || c.ScreenAsking || c.CanSelect() {
		t.Fatalf("granted: %q, asking %v; can select %v without Select", c.Screen, c.ScreenAsking, c.CanSelect())
	}

	var pic []byte
	var err error
	at := image.Rect(100, 200, 400, 260)
	c.ImageService.selectPicture = func(context.Context) ([]byte, image.Rectangle, error) { return pic, at, err }
	shown := 0
	c.SelectImage(func() { shown++ }) // cancelled
	f.flush(t, 1)
	if shown != 0 || c.Image.PNG != nil {
		t.Errorf("cancelled: shown %d, %+v", shown, c.Image)
	}

	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	pic = b.Bytes()
	c.SelectImage(func() { shown++ })
	f.flush(t, 1)
	if shown != 1 || !c.Image.Running || c.Image.Area != at {
		t.Fatalf("selected: shown %d, %+v", shown, c.Image)
	}
	f.flush(t, 2)
	if c.Image.Text != "好" {
		t.Errorf("translated: %+v", c.Image)
	}

	// Turned off in System Settings since: the capture fails, and the permission is checked again.
	pic, err = nil, errors.New("capture: exit status 1: could not create image from rect")
	f.answers["screen-access"] = []engine.Event{{Type: "access", State: "denied"}}
	c.SelectImage(func() { shown++ })
	f.flush(t, 1)
	f.flush(t, 1)
	if shown != 1 || c.Screen != "denied" || c.CanSelect() {
		t.Errorf("failed: shown %d, screen %q", shown, c.Screen)
	}
}

// TestImageEngines translates pictures with the engine chosen: a local model, whose llama-server,
// started for it, stops when the card closes; the user's API, which fails alone, the cloud not
// asked.
func TestImageEngines(t *testing.T) {
	answer := func(text string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if b, _ := io.ReadAll(r.Body); strings.Contains(string(b), `"model":"bad"`) {
				http.Error(w, "this model reads no images", http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", text)
		}))
	}
	cloud, local, api := answer("云端"), answer("本机"), answer("接口")
	defer cloud.Close()
	defer local.Close()
	defer api.Close()
	defer func(f func() *translate.Endpoint) { imageCloud = f }(imageCloud)
	imageCloud = func() *translate.Endpoint {
		return &translate.Endpoint{URL: cloud.URL, Model: "m", Style: translate.Index}
	}
	c, f, m, srv, sec := engineCore(t)
	srv.url = local.URL
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	translated := func(want string) {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); c.Image.Running; {
			if time.Now().After(deadline) {
				t.Fatalf("still translating: %+v", c.Image)
			}
			f.flush(t, 1)
		}
		if c.Image.Text != want || c.Image.Err != nil {
			t.Fatalf("translated %q, want %q: %+v", c.Image.Text, want, c.Image)
		}
	}

	c.TranslateImage(b.Bytes())
	if c.Image.Via != "cloud" {
		t.Fatalf("the cloud: via %q", c.Image.Via)
	}
	translated("云端")
	m.ready["local-2b"] = true
	c.SetEngine("local-2b")
	c.TranslateImage(b.Bytes())
	if c.Image.Via != "local-2b" {
		t.Fatalf("local: via %q", c.Image.Via)
	}
	translated("本机")
	if len(srv.ensured) != 1 || srv.ensured[0] != m.Path("local-2b") {
		t.Errorf("served %q", srv.ensured)
	}
	stops := srv.stops
	c.CloseImage()
	if srv.stops != stops+1 {
		t.Errorf("closed: %d stops, want %d", srv.stops, stops+1)
	}

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL, Model: "m"}, "key")
	c.SetEngine(prefs.CustomPrefix + id)
	c.TranslateImage(b.Bytes())
	translated("接口")
	if c.Image.Via != prefs.CustomPrefix+id || string(sec[keyName(id)]) != "key" {
		t.Errorf("the API: via %q", c.Image.Via)
	}
	e, _ := c.Prefs.Endpoint(id)
	e.Model = "bad"
	c.SaveEndpoint(*e, "")
	// The API fails, as one that reads no pictures: the picture goes nowhere else.
	c.TranslateImage(b.Bytes())
	for deadline := time.Now().Add(2 * time.Second); c.Image.Running; {
		if time.Now().After(deadline) {
			t.Fatalf("still translating: %+v", c.Image)
		}
		f.flush(t, 1)
	}
	if c.Image.Err == nil || c.Image.Text != "" || c.Image.Via != prefs.CustomPrefix+id {
		t.Errorf("the API failed: %+v", c.Image)
	}
}
