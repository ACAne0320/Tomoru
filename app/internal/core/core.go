package core

import (
	"context"
	"errors"
	"image"
	"log"
	"time"

	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
	"tomoru/internal/usage"
)

type Permission string

const (
	PermissionUnchecked Permission = ""
	PermissionGranted   Permission = Permission(engine.AccessGranted)
	PermissionDenied    Permission = Permission(engine.AccessDenied)
	PermissionUnknown   Permission = Permission(engine.AccessUnknown)
)

type ASRState string

const (
	ASRUnprepared ASRState = ""
	ASRPreparing  ASRState = "preparing"
	ASRReady      ASRState = "ready"
	ASRFailed     ASRState = "error"
)

type Session interface {
	Close()
}

type LoginItem interface {
	OpenAtLogin() bool
	SetOpenAtLogin(on bool) error
}

type Deps struct {
	Context             context.Context
	Post                func(fn func())
	PrefsPath           string
	ScenesPath          string
	HistoryDir          string
	StatsPath           string
	Now                 func() time.Time
	LogPath             string
	LicensesDir         string
	Version, OS         string
	EnginePath, ASRPath func() (string, error)
	StartSession        func(live.Config, func(live.State)) (Session, error)
	RunEngine           func(ctx context.Context, bin string, args ...string) ([]engine.Event, error)
	Translator          translate.Translator
	ImageEndpoint       *translate.Endpoint
	Select              func(ctx context.Context) ([]byte, image.Rectangle, error)
	Login               LoginItem
	Models              Models
	Server              Server
	ASRServer           Server
	Secrets             Secrets
	GPU, Memory         func() (bytes uint64, known bool)
	ImageCloud          func() *translate.Endpoint
	Proxy               func(off bool, addr string)
	SystemProxy         func() string
	Network             func() (online, metered bool)
}

type Models interface {
	Status(id string) models.Status
	Path(id string) string
	Download(id string)
	Cancel(id string)
	Delete(id string) error
	Dir() string
	Moving() models.Move
	Move(to string, done func(error))
}

type Server interface {
	Ensure(ctx context.Context, path string) (url string, err error)
	EnsureProjector(ctx context.Context, path string) (url string, err error)
	Stop()
}

type Secrets interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

type Core struct {
	*PreferencesService
	*SceneService
	SaveErr     error
	SystemProxy string
	proxyAsked  time.Time

	*LearningService
	*ImageService
	*SessionController
	*EngineService
	Pressed map[Shortcut]time.Time
	held    map[Shortcut]bool

	Screen          Permission
	ScreenAsking    bool
	screenCheckedAt time.Time

	*HistoryService
	finished finishedLog

	work     *tasks
	closed   bool
	deps     Deps
	onChange []func()

	Access    Permission
	Checking  bool
	Asking    bool
	CheckedAt time.Time
	ASR       map[string]ASRState
	sysLangs  []string
	Apps      []App
	listedAt  time.Time
	AtLogin   bool
	LoginErr  string

	Speaking bool
	SpeakErr error
	speak    context.CancelFunc
	speakRun int

	runSince   time.Time
	runEngines usage.Engines
	ledger     usage.Ledger
}

type App struct {
	Name, ID string
	Playing  bool
}

func New(p prefs.Prefs, d Deps) *Core {
	if d.EnginePath == nil {
		d.EnginePath = func() (string, error) { return "", errors.New("engine path not configured") }
	}
	if d.ASRPath == nil {
		d.ASRPath = func() (string, error) { return "", errors.New("ASR path not configured") }
	}
	if d.StartSession == nil {
		d.StartSession = func(cfg live.Config, f func(live.State)) (Session, error) { return live.Start(cfg, f) }
	}
	if d.RunEngine == nil {
		d.RunEngine = engine.Run
	}
	c := &Core{PreferencesService: &PreferencesService{Prefs: p.Clone(), saved: p.Clone()}, SceneService: &SceneService{}, deps: d, ASR: map[string]ASRState{},
		Pressed: map[Shortcut]time.Time{}, held: map[Shortcut]bool{}}
	c.work = newTasks(d.Context, d.Post)
	c.EngineService = newEngineService(c.work, d.Models, d.Server, d.ASRServer, d.Secrets, c.changed)
	if d.ImageCloud != nil {
		c.EngineService.imageCloud = d.ImageCloud
	}
	c.HistoryService = &HistoryService{work: c.work, changed: c.changed, finished: &c.finished}
	c.ImageService = &ImageService{work: c.work, endpoint: d.ImageEndpoint, selectPicture: d.Select, changed: c.changed, translated: c.imageTranslated, release: c.releaseServer}
	c.LearningService = &LearningService{work: c.work, changed: c.changed, rewritten: func() {
		c.histChanged()
		if c.History.Shown != "" {
			c.ShowHistory(c.History.Shown)
		}
	}}
	c.SessionController = &SessionController{work: c.work, startSession: d.StartSession, changed: c.changed, hideAfter: c.HideAfter, now: c.now,
		onState: func(st live.State) {
			c.HistoryService.drain()
			if st.Phase == live.PhaseListening {
				c.ASR[c.Prefs.Src] = ASRReady
			}
		},
		onStopped: func() {
			c.HistoryService.drain()
			c.closeHistory()
			c.endRun()
		}}
	c.loadScenes()
	c.syncHistoryPolicy()
	c.loadStats()
	c.useProxy()
	c.saved = c.Prefs.Clone()
	c.chooseImageEngineOnce()
	c.retireModels()
	if d.Login != nil {
		c.AtLogin = d.Login.OpenAtLogin()
	}
	return c
}

func (c *Core) LogPath() string { return c.deps.LogPath }

func (c *Core) LicensesDir() string { return c.deps.LicensesDir }

func (c *Core) Network() (online, metered bool) {
	if c.deps.Network == nil {
		return true, false
	}
	return c.deps.Network()
}

func (c *Core) Changed() {
	c.usePending()
	c.noteFinished()
	c.changed()
}

func (c *Core) OnChange(fn func()) { c.onChange = append(c.onChange, fn) }

func (c *Core) changed() {
	for _, fn := range c.onChange {
		fn()
	}
}

func (c *Core) SavePrefs() error {
	c.SaveErr = errors.Join(c.PreferencesService.save(c.deps.PrefsPath), c.SceneService.save(c.deps.ScenesPath))
	if c.SaveErr != nil {
		log.Println("settings:", c.SaveErr)
	}
	return c.SaveErr
}

func (c *Core) settingsChanged() {
	c.useProxy()
	c.useGlossary()
	c.syncHistoryPolicy()
	c.SavePrefs()
	c.changed()
}

func (c *Core) HideAfter() time.Duration {
	if c.Prefs.KeepShown {
		return 0
	}
	return time.Duration(c.Prefs.HideAfterSeconds * float64(time.Second))
}

func (c *Core) Context() context.Context { return c.work.ctx }

func (c *Core) useProxy() {
	if c.deps.Proxy != nil {
		c.deps.Proxy(c.Prefs.NoProxy, c.Prefs.ProxyURL)
	}
}

func (c *Core) RefreshSystemProxy() {
	ask := c.deps.SystemProxy
	if ask == nil || !c.proxyAsked.IsZero() && c.now().Sub(c.proxyAsked) < RefreshEvery {
		return
	}
	c.proxyAsked = c.now()
	c.work.Go(func() {
		s := ask()
		c.work.Post(func() {
			if s != c.SystemProxy {
				c.SystemProxy = s
				c.changed()
			}
		})
	})
}
