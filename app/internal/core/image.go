package core

import (
	"context"
	"image"
	"log"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
)

func readsPictures(id string, endpoints []prefs.Endpoint) bool {
	if m, ok := models.ByID(id); ok {
		return !m.ASR && m.Projector != ""
	}
	if eid, ok := prefs.EndpointOf(id); ok {
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

func imageEngineFrom(id string, endpoints []prefs.Endpoint) string {
	if readsPictures(id, endpoints) {
		return id
	}
	return "cloud"
}

func (c *Core) ImageEngines() []Engine {
	var es []Engine
	for _, e := range c.Engines(MT) {
		if readsPictures(e.ID, c.Prefs.Endpoints) && (e.Model == nil || c.ModelStatus(e.ID).State == models.Ready) {
			es = append(es, e)
		}
	}
	return es
}

func (c *Core) SetImageEngine(id string) {
	if id == c.Prefs.ImageMT {
		return
	}
	c.Prefs.ImageMT = id
	c.settingsChanged()
}

func (c *Core) serving() string {
	if _, local := models.ByID(c.Prefs.MT); local && c.Running && c.Prefs.Translates() {
		return c.Prefs.MT
	}
	return ""
}

type ImageView struct {
	PNG     []byte
	Area    image.Rectangle
	Text    string
	Running bool
	Err     error
	Via     string
	run     int
}

func (v ImageView) ID() int { return v.run }

type ImageService struct {
	Image         ImageView
	imageCancel   context.CancelFunc
	selectCancel  context.CancelFunc
	selectNext    func([]byte, image.Rectangle, error)
	work          *tasks
	endpoint      *translate.Endpoint
	selectPicture func(context.Context) ([]byte, image.Rectangle, error)
	changed       func()
	translated    func(via string)
	release       func()
}

type imageRoute struct {
	via  string
	open func(context.Context) (*translate.Endpoint, error)
}

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

func (c *Core) releaseServer() {
	if c.serving() == "" && c.deps.Server != nil {
		c.deps.Server.Stop()
	}
}

func (c *Core) ImageTgt() string {
	if c.Prefs.ImageTgt != "" {
		return c.Prefs.ImageTgt
	}
	return c.Prefs.Tgt
}

func (c *Core) SetImageSize(w, h int) {
	if w == c.Prefs.ImageW && h == c.Prefs.ImageH {
		return
	}
	c.Prefs.ImageW, c.Prefs.ImageH = w, h
	c.SavePrefs()
}

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

func (s *ImageService) CloseImage() {
	s.stopImage()
	if _, local := models.ByID(s.Image.Via); local && s.release != nil {
		s.release()
	}
	s.Image = ImageView{run: s.Image.run + 1}
	s.changed()
}

func (s *ImageService) usesServer() bool {
	_, local := models.ByID(s.Image.Via)
	return local && s.Image.PNG != nil
}

func (s *ImageService) stopImage() {
	if s.imageCancel != nil {
		s.imageCancel()
		s.imageCancel = nil
	}
}

func (s *ImageService) selectImage(done func([]byte, image.Rectangle, error)) {
	if s.selectPicture == nil || s.work.ctx.Err() != nil {
		return
	}
	if s.selectCancel != nil {
		s.selectNext = done
		s.selectCancel()
		return
	}
	ctx, cancel := context.WithCancel(s.work.ctx)
	s.selectCancel = cancel
	s.work.Go(func() {
		pic, at, err := s.selectPicture(ctx)
		s.work.Post(func() {
			cancel()
			s.selectCancel = nil
			if next := s.selectNext; next != nil {
				s.selectNext = nil
				s.selectImage(next)
				return
			}
			done(pic, at, err)
		})
	})
}

func (c *Core) CanSelect() bool {
	return c.Screen == PermissionGranted && c.ImageService.selectPicture != nil
}

func (c *Core) chooseImageEngineOnce() {
	if c.Prefs.ImageMT == "" {
		c.Prefs.ImageMT = imageEngineFrom(c.Prefs.MT, c.Prefs.Endpoints)
	}
}

const (
	screenDialogWait = time.Hour
	accessDialogWait = 2 * time.Minute
	prepareWait      = 2 * time.Minute
)

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

func (c *Core) CheckScreen() {
	if c.ScreenAsking || time.Since(c.screenCheckedAt) < ListEvery {
		return
	}
	c.screenCheckedAt = time.Now()
	c.ask(10*time.Second, c.setScreen, "screen-access")
}

func (c *Core) RequestScreen() {
	if c.ScreenAsking {
		return
	}
	c.ScreenAsking = true
	c.changed()
	c.ask(screenDialogWait, func(evs []engine.Event) { c.ScreenAsking = false; c.setScreen(evs) }, "request-screen-access")
}

func (c *Core) setScreen(evs []engine.Event) {
	c.screenCheckedAt = time.Now()
	if len(evs) > 0 {
		c.Screen = Permission(evs[len(evs)-1].State)
	}
}
