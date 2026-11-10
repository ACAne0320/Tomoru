package core

import (
	"context"
	"image"
	"log"
	"strings"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

// Translating a picture, as the picture's shortcut asks (docs/image-translation.md): a part of the screen
// the user selects, when the app may capture the screen, else the one on the clipboard. Neither it
// nor its translation is kept.

// imageCloud is Index-Translate's cloud, which translates pictures, in the size chosen for them.
// Tests replace it.
var imageCloud = translate.Cloud

// readsPictures tells whether translation engine id reads pictures: the cloud's sizes, a local model with
// its vision projector, an endpoint of a format that reads them not found to read none.
func readsPictures(id string, endpoints []prefs.Endpoint) bool {
	if m, ok := models.ByID(id); ok {
		return !m.ASR && m.Vision != ""
	}
	if eid, ok := strings.CutPrefix(id, prefs.CustomPrefix); ok {
		for _, e := range endpoints {
			if e.ID == eid {
				return e.Vision != prefs.VisionNo && formatOf(e).ReadsPictures()
			}
		}
		return false
	}
	_, cloud := CloudSizeByID(id)
	return cloud
}

// imageEngineFrom is the engine for pictures of one who translates the subtitles with engine id:
// the same where it reads pictures, else the cloud.
func imageEngineFrom(id string, endpoints []prefs.Endpoint) string {
	if readsPictures(id, endpoints) {
		return id
	}
	return "cloud"
}

// ImageEngines are the engines to choose for pictures, those that can read them now, in the order
// of Engines: the local models downloaded, the cloud's sizes, and the user's APIs but those found
// to read none, or prompted as a model that reads none.
func (c *Core) ImageEngines() []Engine {
	var es []Engine
	for _, e := range c.Engines(MT) {
		if readsPictures(e.ID, c.Prefs.Endpoints) && (e.Model == nil || c.ModelStatus(e.ID).State == models.Ready) {
			es = append(es, e)
		}
	}
	return es
}

// SetImageEngine translates pictures with engine id from now on, one of ImageEngines.
func (c *Core) SetImageEngine(id string) {
	if id == c.Prefs.ImageMT {
		return
	}
	c.Prefs.ImageMT = id
	c.settingsChanged()
}

// fitImageEngine has the cloud translate pictures in place of an endpoint gone, or found to read
// none.
func (c *Core) fitImageEngine() {
	if strings.HasPrefix(c.Prefs.ImageMT, prefs.CustomPrefix) && !readsPictures(c.Prefs.ImageMT, c.Prefs.Endpoints) {
		c.Prefs.ImageMT = "cloud"
	}
}

// serving is the local model llama-server runs for the subtitles, "" for none.
func (c *Core) serving() string {
	if _, local := models.ByID(c.Prefs.MT); local && c.Running {
		return c.Prefs.MT
	}
	return ""
}

// ImageView is the picture translated last, as its card shows it.
type ImageView struct {
	PNG     []byte          // the picture; nil when the clipboard had none
	Area    image.Rectangle // where it was selected, in the screens' pixels; empty when not known
	Text    string          // the translation so far
	Running bool
	Err     error  // why it could not be translated
	Via     string // the engine translating the picture, as Preferences.ImageMT names it
	run     int    // the translation's: what an older one sends is dropped
}

// ID tells pictures apart: it changes with each one translated.
func (v ImageView) ID() int { return v.run }

// ImageService owns picture translation and selection lifetimes. It receives
// immutable language/glossary inputs, never the application's mutable settings.
type ImageService struct {
	Image         ImageView
	imageCancel   context.CancelFunc
	selecting     bool
	work          *tasks
	endpoint      *translate.Endpoint // translates every picture, whatever the engine: for tests
	selectPicture func(context.Context) ([]byte, image.Rectangle, error)
	changed       func()
	translated    func(via string) // a picture's translation came, for the numbers
	release       func()           // the card closed after a local model translated
}

// imageRoute is how a picture is translated: the endpoint, which open gets ready, of engine via.
type imageRoute struct {
	via  string
	open func(context.Context) (*translate.Endpoint, error)
}

// TranslateImage translates with a snapshot of the picture's language, and the scene's glossary when
// it is the subtitles': its names are in that language.
func (c *Core) TranslateImage(pic []byte) {
	tgt := c.ImageTgt()
	var terms []translate.Term
	if tgt == c.Prefs.Tgt {
		for _, t := range c.Scene().Terms {
			terms = append(terms, translate.Term{Src: t.Src, Tgt: t.Tgt})
		}
	}
	var r imageRoute
	if e := c.ImageService.endpoint; e != nil {
		r.via, r.open = c.Prefs.ImageMT, func(context.Context) (*translate.Endpoint, error) { return e, nil }
	} else {
		r.open, r.via = c.EngineService.imageEndpoint(c.Prefs.ImageMT, c.serving(), c.Prefs.Endpoints)
	}
	c.ImageService.translate(pic, tgt, terms, r)
}

// releaseServer stops llama-server, which a picture had started, unless the subtitles use it.
func (c *Core) releaseServer() {
	if c.serving() == "" && c.deps.Server != nil {
		c.deps.Server.Stop()
	}
}

// ImageTgt is the language pictures are translated into: the one chosen on their card, else the
// subtitles'.
func (c *Core) ImageTgt() string {
	if c.Prefs.ImageTgt != "" {
		return c.Prefs.ImageTgt
	}
	return c.Prefs.Tgt
}

// SetImageSize keeps the size the user resized the picture's card to, w by h, for the next.
func (c *Core) SetImageSize(w, h int) {
	if w == c.Prefs.ImageW && h == c.Prefs.ImageH {
		return
	}
	c.Prefs.ImageW, c.Prefs.ImageH = w, h
	c.SavePrefs()
}

// SetImageTgt translates pictures into l from now on, the one shown again, where it was; the
// subtitles' language stays. Chosen again, the subtitles' language is followed as it changes.
func (c *Core) SetImageTgt(l string) {
	if l == c.ImageTgt() {
		return
	}
	c.Prefs.ImageTgt = l
	if l == c.Prefs.Tgt {
		c.Prefs.ImageTgt = ""
	}
	c.settingsChanged()
	if pic := c.Image.PNG; pic != nil {
		at := c.Image.Area
		c.TranslateImage(pic)
		c.Image.Area = at
	}
}

func (s *ImageService) translate(pic []byte, tgt string, terms []translate.Term, r imageRoute) {
	if s.work.ctx.Err() != nil {
		return
	}
	s.stopImage()
	run := s.Image.run + 1
	pic = append([]byte(nil), pic...)
	s.Image = ImageView{PNG: pic, Running: pic != nil, Via: r.via, run: run}
	defer s.changed()
	if pic == nil {
		return
	}
	ctx, cancel := context.WithCancel(s.work.ctx)
	s.imageCancel = cancel
	s.work.Go(func() {
		defer cancel()
		stream := func(text string) {
			s.work.Post(func() {
				if s.Image.run == run && s.Image.Running && ctx.Err() == nil {
					s.Image.Text = text
					s.changed()
				}
			})
		}
		via := r.via
		e, err := r.open(ctx)
		var text string
		if err == nil {
			text, err = e.TranslateImage(ctx, pic, tgt, terms, stream)
		}
		cancelled := ctx.Err() != nil
		s.work.Post(func() {
			if s.Image.run != run || cancelled {
				return
			}
			s.Image.Text, s.Image.Err, s.Image.Running, s.Image.Via = text, err, false, via
			s.imageCancel = nil
			if err != nil {
				log.Println("image:", via, err)
			}
			if err == nil && s.translated != nil {
				s.translated(via)
			}
			s.changed()
		})
	})
}

// CloseImage cancels the operation and invalidates every queued result.
func (s *ImageService) CloseImage() {
	s.stopImage()
	if _, local := models.ByID(s.Image.Via); local && s.release != nil {
		s.release()
	}
	s.Image = ImageView{run: s.Image.run + 1}
	s.changed()
}

func (s *ImageService) stopImage() {
	if s.imageCancel != nil {
		s.imageCancel()
		s.imageCancel = nil
	}
}

func (s *ImageService) selectImage(done func([]byte, image.Rectangle, error)) {
	if s.selecting || s.selectPicture == nil || s.work.ctx.Err() != nil {
		return
	}
	s.selecting = true
	s.work.Go(func() {
		pic, at, err := s.selectPicture(s.work.ctx)
		s.work.Post(func() { s.selecting = false; done(pic, at, err) })
	})
}

// CanSelect tells whether the user can select a part of the screen to translate: the app may
// capture it.
func (c *Core) CanSelect() bool { return c.Screen == "granted" && c.ImageService.selectPicture != nil }

// SelectImage has the user select a part of the screen, and translates it; shown runs once its
// translation starts. Cancelled, nothing is translated. Failing, as when Screen Recording was
// turned off since, it checks the permission again.
func (c *Core) SelectImage(shown func()) {
	c.ImageService.selectImage(func(pic []byte, at image.Rectangle, err error) {
		switch {
		case err != nil:
			log.Println(err)
			c.screenCheckedAt = time.Time{}
			c.CheckScreen()
		case pic != nil:
			c.TranslateImage(pic)
			c.Image.Area = at
			shown()
		}
	})
}

// CheckScreen checks whether the app may capture the screen, unless it did in the last few
// seconds: views that show it check while they show, as it may be turned on or off in System
// Settings meanwhile.
func (c *Core) CheckScreen() {
	if c.ScreenAsking || time.Since(c.screenCheckedAt) < ListEvery {
		return
	}
	c.screenCheckedAt = time.Now()
	c.ask(10*time.Second, c.setScreen, "screen-access")
}

// RequestScreen has the system ask the user for Screen Recording, once; after, it is turned on in
// System Settings.
func (c *Core) RequestScreen() {
	if c.ScreenAsking {
		return
	}
	c.ScreenAsking = true
	c.changed()
	c.ask(2*time.Minute, func(evs []engine.Event) { c.ScreenAsking = false; c.setScreen(evs) }, "request-screen-access")
}

func (c *Core) setScreen(evs []engine.Event) {
	c.screenCheckedAt = time.Now()
	if len(evs) > 0 {
		c.Screen = evs[len(evs)-1].State
	}
}
