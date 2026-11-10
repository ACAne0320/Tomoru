package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// TestImageEngineUpgrade sets the engine for pictures of settings from before it, once: the
// subtitles' engine where it reads pictures, else the cloud. One set stays as it is.
func TestImageEngineUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name, mt, image, want string
		vision                prefs.Vision
	}{
		{"cloud", "cloud", "", "cloud", 0},
		{"local model", "local-9b", "", "local-9b", 0},
		{"API not tested", "custom:a", "", "custom:a", prefs.VisionUnknown},
		{"API reading pictures", "custom:a", "", "custom:a", prefs.VisionYes},
		{"API reading none", "custom:a", "", "cloud", prefs.VisionNo},
		{"API gone", "custom:gone", "", "cloud", 0},
		{"chosen before", "local-9b", "cloud", "cloud", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := prefs.Default("zh-Hans")
			p.Onboarded, p.MT, p.ImageMT = true, tc.mt, tc.image
			p.Endpoints = []prefs.Endpoint{{ID: "a", Name: "A", URL: "http://a", Model: "m", Vision: tc.vision}}
			c := New(p, Deps{Post: func(func()) {}})
			if c.Prefs.ImageMT != tc.want {
				t.Errorf("pictures with %q, want %q", c.Prefs.ImageMT, tc.want)
			}
			c.SetEngine("cloud")
			if c.Prefs.ImageMT != tc.want {
				t.Errorf("after choosing the subtitles' engine: pictures with %q, want %q", c.Prefs.ImageMT, tc.want)
			}
		})
	}
}

// TestImageEngineInGuide has the guide's choice of a translation engine translate pictures too,
// where it reads them: its one choice of an engine.
func TestImageEngineInGuide(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	m.ready["local-2b"] = true
	c.Choose(MT, "local-2b")
	if c.Prefs.ImageMT != "local-2b" {
		t.Errorf("in the guide: pictures with %q", c.Prefs.ImageMT)
	}
	c.Choose(MT, "cloud")
	c.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded = true })
	c.Choose(MT, "local-2b")
	if c.Prefs.ImageMT != "cloud" {
		t.Errorf("after the guide: pictures with %q", c.Prefs.ImageMT)
	}
}

// TestImageEngineMenu lists the engines that can read pictures now: the cloud first, the local
// models downloaded but Hy-MT2, which reads none, and the user's APIs but those a test found to
// read none, or prompted as Hy-MT2.
func TestImageEngineMenu(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	c.Prefs.Endpoints = []prefs.Endpoint{
		{ID: "new", Name: "Not tested"},
		{ID: "sees", Name: "Reads", Vision: prefs.VisionYes},
		{ID: "text", Name: "Text only", Vision: prefs.VisionNo},
		{ID: "hymt", Name: "Hy-MT2", Format: "hy-mt"},
	}
	ids := func() []string {
		var out []string
		for _, e := range c.ImageEngines() {
			out = append(out, e.ID)
		}
		return out
	}
	if got, want := ids(), []string{"cloud-2b", "cloud-9b", "cloud", "custom:new", "custom:sees"}; !slices.Equal(got, want) {
		t.Errorf("nothing downloaded: %q, want %q", got, want)
	}
	m.ready["local-2b"], m.ready["sensevoice-small"], m.ready["hy-mt2-1.8b"] = true, true, true
	if got, want := ids(), []string{"local-2b", "cloud-2b", "cloud-9b", "cloud", "custom:new", "custom:sees"}; !slices.Equal(got, want) {
		t.Errorf("2B downloaded: %q, want %q", got, want)
	}
	c.SetImageEngine("custom:sees")
	if c.Prefs.ImageMT != "custom:sees" {
		t.Errorf("chosen %q", c.Prefs.ImageMT)
	}
}

// TestImageEngineOnly sends a picture to the engine chosen for pictures and to no other: not the
// subtitles', not the cloud when the one chosen fails. A local model translating the subtitles
// keeps llama-server: another chosen for pictures says so, and closing the card leaves it.
func TestImageEngineOnly(t *testing.T) {
	var cloudAsked atomic.Int32
	var model atomic.Value // the cloud's size asked
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cloudAsked.Add(1)
		var body struct{ Model string }
		json.NewDecoder(r.Body).Decode(&body)
		model.Store(body.Model)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"云端\"}}]}\n\n")
	}))
	defer cloud.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer api.Close()
	defer func(f func() *translate.Endpoint) { imageCloud = f }(imageCloud)
	imageCloud = func() *translate.Endpoint {
		return &translate.Endpoint{URL: cloud.URL, Model: "m", Format: translate.Index}
	}
	c, f, m, srv, _ := engineCore(t)
	c.Prefs.Onboarded = true
	pic := picture()
	done := func() {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); c.Image.Running; {
			if time.Now().After(deadline) {
				t.Fatalf("still translating: %+v", c.Image)
			}
			f.flush(t, 1)
		}
	}

	m.ready["local-2b"], m.ready["local-9b"] = true, true
	c.SetEngine("local-9b")
	c.TranslateImage(pic)
	done()
	if c.Image.Via != "cloud" || c.Image.Text != "云端" || len(srv.ensured) != 0 {
		t.Errorf("the subtitles' engine is not the pictures': %+v, served %q", c.Image, srv.ensured)
	}
	if model.Load() != translate.CloudModel {
		t.Errorf("the cloud asked for %v", model.Load())
	}
	c.SetImageEngine("cloud-9b") // a smaller size of the cloud's reads pictures too
	c.TranslateImage(pic)
	done()
	if c.Image.Via != "cloud-9b" || model.Load() != "Index-Translate-9B" {
		t.Errorf("pictures with the cloud's 9B: %+v, asked for %v", c.Image, model.Load())
	}
	c.SetImageEngine("cloud")

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL, Model: "m"}, "key")
	c.SetImageEngine(prefs.CustomPrefix + id)
	asked := cloudAsked.Load()
	c.TranslateImage(pic)
	done()
	if c.Image.Err == nil || c.Image.Via != prefs.CustomPrefix+id || cloudAsked.Load() != asked {
		t.Errorf("the API failed: %+v; the cloud asked %d times more", c.Image, cloudAsked.Load()-asked)
	}

	c.Start()
	f.flush(t, 1)
	c.SetImageEngine("local-2b")
	ensured := len(srv.ensured)
	c.TranslateImage(pic)
	done()
	if !errors.Is(c.Image.Err, ErrModelBusy) || len(srv.ensured) != ensured {
		t.Errorf("9B translating the subtitles: %+v, served %q", c.Image, srv.ensured)
	}
	stops := srv.stops
	c.CloseImage()
	if srv.stops != stops {
		t.Errorf("closing the card stopped the subtitles' llama-server")
	}
}

// TestImageEngineDeleted has the cloud translate pictures once the engine chosen for them is
// deleted, a model or an API; undoing the API's deletion chooses it again.
func TestImageEngineDeleted(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	c.Prefs.Onboarded = true
	m.ready["local-2b"] = true
	c.SetImageEngine("local-2b")
	c.DeleteModel("local-2b")
	if c.Prefs.ImageMT != "cloud" {
		t.Errorf("model deleted: pictures with %q", c.Prefs.ImageMT)
	}

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: "http://127.0.0.1:1", Model: "m"}, "")
	c.SetImageEngine(prefs.CustomPrefix + id)
	undo := c.DeleteEndpoint(id)
	if c.Prefs.ImageMT != "cloud" {
		t.Errorf("API deleted: pictures with %q", c.Prefs.ImageMT)
	}
	undo()
	if c.Prefs.ImageMT != prefs.CustomPrefix+id {
		t.Errorf("deleting undone: pictures with %q", c.Prefs.ImageMT)
	}
}

// TestEndpointReadsPictures tests an API with a picture too, alongside the sample: the endpoint
// saved keeps whether it read it; one found to read none no longer translates pictures; a new one
// keeps what its test found once saved, and one whose model changed, nothing.
func TestEndpointReadsPictures(t *testing.T) {
	var sent atomic.Value
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "image_url") {
			sent.Store(string(b))
			if strings.Contains(string(b), `"model":"text`) {
				http.Error(w, `{"error":"unknown variant image_url"}`, http.StatusBadRequest)
				return
			}
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家晚上好。\"}}]}\n\n")
	}))
	defer api.Close()
	c, f, _, _, _ := engineCore(t)
	c.Prefs.Onboarded = true
	test := func(e prefs.Endpoint) TestResult {
		t.Helper()
		c.TestEndpoint("dialog", e, "")
		f.flush(t, 1)
		return c.Tests["dialog"]
	}

	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL, Model: "text"}, "")
	c.SetImageEngine(prefs.CustomPrefix + id)
	saved, _ := c.Prefs.Endpoint(id)
	e := *saved
	if r := test(e); r.Err != nil || r.Vision != prefs.VisionNo || !r.ToCloud {
		t.Fatalf("text only: %+v", r)
	}
	if body, _ := sent.Load().(string); !strings.Contains(body, "data:image/png;base64,") {
		t.Errorf("no picture sent: %.200s", body)
	}
	if e, _ := c.Prefs.Endpoint(id); e.Vision != prefs.VisionNo || c.Prefs.ImageMT != "cloud" {
		t.Errorf("found text only: kept %v, pictures with %q", e.Vision, c.Prefs.ImageMT)
	}

	if r := test(e); r.ToCloud {
		t.Error("tested again: says the cloud took over again")
	}

	e.Model = "sees"
	if r := test(e); r.Vision != prefs.VisionYes || r.ToCloud {
		t.Fatalf("reads pictures: %+v", r)
	}
	if e, _ := c.Prefs.Endpoint(id); e.Vision != prefs.VisionNo {
		t.Errorf("another model tested, the one saved now %v", e.Vision)
	}
	c.SaveEndpoint(e, "")
	if e, _ := c.Prefs.Endpoint(id); e.Vision != prefs.VisionYes {
		t.Errorf("saved as tested: %v", e.Vision)
	}

	added := prefs.Endpoint{Name: "New", URL: api.URL + "/v1", Model: "text-2"}
	test(added)
	added.ID = c.SaveEndpoint(added, "")
	if e, _ := c.Prefs.Endpoint(added.ID); e.Vision != prefs.VisionNo {
		t.Errorf("a new one tested, then saved: %v", e.Vision)
	}
	added.Model = "other"
	c.SaveEndpoint(added, "")
	if e, _ := c.Prefs.Endpoint(added.ID); e.Vision != prefs.VisionUnknown {
		t.Errorf("its model changed: %v", e.Vision)
	}

	// The sample failing, nothing is known of pictures.
	e.URL = "http://127.0.0.1:1"
	if r := test(e); r.Err == nil || r.Vision != prefs.VisionUnknown {
		t.Errorf("unreachable: %+v", r)
	}
}

// TestEndpointNoPictures tests an API prompted as Hy-MT2 is without a picture: it reads none.
// Saved so while chosen for pictures, it gives them to the cloud.
func TestEndpointNoPictures(t *testing.T) {
	var pictures atomic.Int32
	var line atomic.Value
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), "image_url") {
			pictures.Add(1)
		} else {
			line.Store(string(b))
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"大家晚上好。\"}}]}\n\n")
	}))
	defer api.Close()
	c, f, _, _, _ := engineCore(t)
	c.Prefs.Onboarded = true
	id := c.SaveEndpoint(prefs.Endpoint{Name: "Mine", URL: api.URL, Model: "m"}, "")
	c.SetImageEngine(prefs.CustomPrefix + id)
	saved, _ := c.Prefs.Endpoint(id)
	e := *saved
	e.Format = "hy-mt"

	c.TestEndpoint("dialog", e, "")
	f.flush(t, 1)
	if r := c.Tests["dialog"]; r.Err != nil || r.Text != "大家晚上好。" || r.Vision != prefs.VisionNo || pictures.Load() != 0 {
		t.Fatalf("test %+v, %d pictures sent", r, pictures.Load())
	}
	if body, _ := line.Load().(string); !strings.Contains(body, "【待翻译文本】") {
		t.Errorf("not asked as Hy-MT2: %.300s", body)
	}
	c.SaveEndpoint(e, "")
	if c.Prefs.ImageMT != "cloud" {
		t.Errorf("saved as Hy-MT2: pictures with %q", c.Prefs.ImageMT)
	}
}

func picture() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	return b.Bytes()
}
