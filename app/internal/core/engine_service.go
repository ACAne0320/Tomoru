package core

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"tomoru/internal/asr"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/provider"
	"tomoru/internal/translate"
)

type TestResult struct {
	Running bool
	Text    string
	Err     error
	Took    time.Duration
	Vision  prefs.Vision
	ToCloud bool
}

const TestSample = "Good evening, everyone. Today's stream will run a bit longer than usual."

type EngineService struct {
	Tests     map[string]TestResult
	pending   map[Kind]string
	tests     map[string]*engineTest
	work      *tasks
	models    Models
	server    Server
	asrServer Server
	secrets   Secrets
	changed   func()
	visions   map[api]prefs.Vision
}

type api struct{ url, model, format string }

func apiOf(e prefs.Endpoint) api {
	return api{strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(e.URL), "/"), "/v1"), strings.TrimSpace(e.Model), formatOf(e).ID}
}

func formatOf(e prefs.Endpoint) *translate.Format {
	if f, ok := translate.FormatByID(e.Format); ok {
		return f
	}
	return translate.Chat
}

func modelFormat(m models.Model) *translate.Format {
	f, _ := translate.FormatByID(m.Format)
	return f
}

type engineTest struct{ cancel context.CancelFunc }

func newEngineService(work *tasks, models Models, server, asrServer Server, secrets Secrets, changed func()) *EngineService {
	return &EngineService{Tests: map[string]TestResult{}, pending: map[Kind]string{}, tests: map[string]*engineTest{}, visions: map[api]prefs.Vision{}, work: work, models: models, server: server, asrServer: asrServer, secrets: secrets, changed: changed}
}

func (s *EngineService) notify() {
	if s.changed != nil {
		s.changed()
	}
}

func (s *EngineService) startTest(test string, timeout time.Duration, run func(context.Context) TestResult, done func(TestResult)) {
	if s.work.ctx.Err() != nil {
		return
	}
	s.forgetTest(test)
	ctx, cancel := context.WithTimeout(s.work.ctx, timeout)
	op := &engineTest{cancel: cancel}
	s.tests[test] = op
	s.Tests[test] = TestResult{Running: true}
	s.notify()
	s.work.Go(func() {
		defer cancel()
		result := run(ctx)
		if ctx.Err() != nil {
			result.Err = ctx.Err()
		}
		result.Running = false
		s.work.Post(func() {
			if s.tests[test] != op {
				return
			}
			delete(s.tests, test)
			s.Tests[test] = result
			if done != nil {
				done(result)
			}
			s.notify()
		})
	})
}

func (s *EngineService) forgetTest(test string) {
	if op := s.tests[test]; op != nil {
		op.cancel()
		delete(s.tests, test)
	}
	delete(s.Tests, test)
}

func (s *EngineService) ForgetTest(test string) { s.forgetTest(test); s.notify() }

func (s *EngineService) stopTests() {
	for id, op := range s.tests {
		op.cancel()
		delete(s.tests, id)
		r := s.Tests[id]
		r.Running = false
		r.Err = context.Canceled
		s.Tests[id] = r
	}
}

func (s *EngineService) testEndpoint(test string, e prefs.Endpoint, key, tgt string, found func(prefs.Vision) bool) {
	e.URL = apiOf(e).url
	s.startTest(test, 30*time.Second, func(ctx context.Context) TestResult {
		ep := s.endpoint(e, key)
		pictureCtx, stop := context.WithCancel(ctx)
		defer stop()
		read := make(chan error, 1)
		go func() {
			_, err := ep.TranslateImage(pictureCtx, testPicture(), tgt, nil, nil)
			read <- err
		}()
		start := time.Now()
		out, err := ep.Translate(ctx, translate.Request{Src: "en", Tgt: tgt, Text: TestSample}, nil)
		r := TestResult{Text: out, Err: err, Took: time.Since(start)}
		if err != nil {
			stop()
		}
		if pictureErr := <-read; err == nil {
			r.Vision = visionOf(pictureErr)
		}
		return r
	}, func(r TestResult) {
		if r.Err == nil && r.Vision != prefs.VisionUnknown {
			s.visions[apiOf(e)] = r.Vision
			if found(r.Vision) {
				r.ToCloud = true
				s.Tests[test] = r
			}
		}
	})
}

func visionOf(err error) prefs.Vision {
	var status *translate.StatusError
	switch {
	case err == nil:
		return prefs.VisionYes
	case errors.Is(err, translate.ErrNoPictures):
		return prefs.VisionNo
	case errors.As(err, &status):
		if s := status.Status; s == http.StatusInternalServerError || s >= 400 && s < 500 && s != http.StatusTooManyRequests {
			return prefs.VisionNo
		}
	}
	return prefs.VisionUnknown
}

var testPicture = sync.OnceValue(func() []byte {
	const text = "Good evening, everyone."
	face := basicfont.Face7x13
	m := image.NewGray(image.Rect(0, 0, 7*len(text)+16, 29))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	d := font.Drawer{Dst: m, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(8, 19)}
	d.DrawString(text)
	var b bytes.Buffer
	png.Encode(&b, m)
	return b.Bytes()
})

func (s *EngineService) testAPI(test string, a prefs.Service, key string) {
	spec, ok := asr.ByID(a.ID)
	if !ok || spec.Test == nil {
		return
	}
	s.startTest(test, 15*time.Second, func(ctx context.Context) TestResult {
		took, err := spec.Test(ctx, s.cloudConfig(a, key))
		return TestResult{Err: err, Took: took}
	}, nil)
}

func (s *EngineService) translator(id string, endpoints []prefs.Endpoint, apis []prefs.Service, keepServer bool) (func(context.Context) (translate.Translator, error), bool, error) {
	if m, ok := models.ByID(id); ok {
		if s.models != nil && s.models.Moving().Active {
			return nil, true, ErrModelsMoving
		}
		if s.models == nil || s.server == nil || s.models.Status(id).State != models.Ready {
			return nil, true, ErrModelMissing
		}
		path, format := s.models.Path(id), modelFormat(m)
		return func(ctx context.Context) (translate.Translator, error) {
			tr, err := provider.Local(ctx, s.server, path, format)
			if err != nil {
				return nil, LocalError{err}
			}
			return tr, nil
		}, true, nil
	}
	stop := func() { s.freeServer(keepServer) }
	if eid, ok := strings.CutPrefix(id, prefs.CustomPrefix); ok {
		for _, e := range endpoints {
			if e.ID == eid {
				return func(context.Context) (translate.Translator, error) { stop(); return s.endpoint(e, ""), nil }, false, nil
			}
		}
		return nil, false, ErrNoEndpoint
	}
	if svc, ok := MTServiceByID(id); ok {
		if !hasKey(id, apis) {
			return nil, false, ErrMTCloudKey
		}
		i := slices.IndexFunc(apis, func(a prefs.Service) bool { return a.ID == id })
		e := svc.endpoint(apis[i])
		return func(context.Context) (translate.Translator, error) {
			stop()
			return provider.Endpoint(e.URL, e.Model, string(s.readKey(apiKey(id))), svc.Format), nil
		}, false, nil
	}
	tr := translate.Cloud()
	if size, ok := CloudSizeByID(id); ok {
		tr.Model = size.Model
	}
	return func(context.Context) (translate.Translator, error) { stop(); return tr, nil }, false, nil
}

func (s *EngineService) noTranslator(keepServer bool) func(context.Context) (translate.Translator, error) {
	return func(context.Context) (translate.Translator, error) {
		s.freeServer(keepServer)
		return nil, nil
	}
}

func (s *EngineService) freeServer(keep bool) {
	if s.server != nil && !keep {
		s.server.Stop()
	}
}

func (s *EngineService) imageEndpoint(id, serving string, endpoints []prefs.Endpoint) (open func(context.Context) (*translate.Endpoint, error), via string) {
	fail := func(err error) func(context.Context) (*translate.Endpoint, error) {
		return func(context.Context) (*translate.Endpoint, error) { return nil, err }
	}
	if m, ok := models.ByID(id); ok {
		switch {
		case s.models != nil && s.models.Moving().Active:
			return fail(ErrModelsMoving), id
		case serving != "" && serving != id:
			return fail(ErrModelBusy), id
		}
		if m.Vision == "" || s.models == nil || s.server == nil || s.models.Status(id).State != models.Ready {
			return fail(ErrModelMissing), id
		}
		path, format := s.models.Path(id), modelFormat(m)
		return func(ctx context.Context) (*translate.Endpoint, error) {
			e, err := provider.LocalVision(ctx, s.server, path, format)
			if err != nil {
				return nil, LocalError{err}
			}
			return e, nil
		}, id
	}
	if eid, ok := strings.CutPrefix(id, prefs.CustomPrefix); ok {
		for _, e := range endpoints {
			if e.ID == eid {
				return func(context.Context) (*translate.Endpoint, error) { return s.endpoint(e, ""), nil }, id
			}
		}
		return fail(ErrNoEndpoint), id
	}
	size, ok := CloudSizeByID(id)
	if !ok {
		size, _ = CloudSizeByID("cloud")
	}
	return func(context.Context) (*translate.Endpoint, error) {
		e := imageCloud()
		e.Model = size.Model
		return e, nil
	}, size.ID
}

func (s *EngineService) endpoint(e prefs.Endpoint, key string) *translate.Endpoint {
	if key == "" {
		key = string(s.readKey(keyName(e.ID)))
	}
	return provider.Endpoint(e.URL, e.Model, key, formatOf(e))
}

func (s *EngineService) cloudConfig(a prefs.Service, key string) asr.Service {
	if key == "" {
		key = string(s.readKey(apiKey(a.ID)))
	}
	return asr.Service{URL: a.URL, Model: a.Model, Key: key}
}

func (s *EngineService) readKey(name string) []byte {
	if s.secrets == nil {
		return nil
	}
	b, _ := s.secrets.Get(name)
	return b
}

func (s *EngineService) saveKey(name string, key []byte) bool {
	return s.secrets != nil && s.secrets.Set(name, key) == nil
}
func (s *EngineService) deleteKey(name string) {
	if s.secrets != nil {
		s.secrets.Delete(name)
	}
}

func (s *EngineService) ModelStatus(id string) models.Status {
	if s.models == nil {
		return models.Status{}
	}
	return s.models.Status(id)
}

func (s *EngineService) ModelsDir() string {
	if s.models == nil {
		return ""
	}
	return s.models.Dir()
}

func (s *EngineService) ModelsMoving() models.Move {
	if s.models == nil {
		return models.Move{}
	}
	return s.models.Moving()
}

func (s *EngineService) Download(id string) {
	if s.models != nil {
		s.models.Download(id)
	}
}

func (s *EngineService) CancelDownload(id string) {
	if s.models != nil {
		s.models.Cancel(id)
	}
}

func (s *EngineService) deleteModel(id string) {
	for kind, pending := range s.pending {
		if pending == id {
			delete(s.pending, kind)
		}
	}
	switch m, _ := models.ByID(id); {
	case !m.ASR && s.server != nil:
		s.server.Stop()
	case m.Served() && s.asrServer != nil:
		s.asrServer.Stop()
	}
	if s.models != nil {
		s.models.Delete(id)
	}
}

func (s *EngineService) chosen(kind Kind, current string) string {
	if id, ok := s.pending[kind]; ok {
		return id
	}
	return current
}
func (s *EngineService) cancelPending(kind Kind) { delete(s.pending, kind) }
func (s *EngineService) usable(e Engine, apis []prefs.Service) bool {
	if !hasKey(e.ID, apis) {
		return false
	}
	return e.Model == nil || s.ModelStatus(e.ID).State == models.Ready
}
func (s *EngineService) choose(e Engine, apis []prefs.Service) bool {
	if !s.usable(e, apis) {
		s.pending[e.Kind] = e.ID
		return false
	}
	delete(s.pending, e.Kind)
	return true
}

func (s *EngineService) readyPending(endpoints []prefs.Endpoint, apis []prefs.Service) []Engine {
	var ready []Engine
	for kind, id := range s.pending {
		found := false
		for _, e := range engineCatalog(kind, endpoints) {
			if e.ID != id {
				continue
			}
			found = true
			if s.usable(e, apis) {
				delete(s.pending, kind)
				ready = append(ready, e)
			}
			break
		}
		if !found {
			delete(s.pending, kind)
		}
	}
	return ready
}
