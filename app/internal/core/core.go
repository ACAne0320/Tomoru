// Package core is Tomoru's application layer: the settings, translating, and what the system
// says (the audio permission, recognition per language, the apps to hear). It knows no window:
// the UI reads its state, calls its actions, and redraws when it says it changed.
//
// A Core is used on the main thread only. Its work in the background (asking the engine, the
// session's reports) comes back through Deps.Post, which runs a function there.
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

// Session is a running translating session.
type Session interface {
	Stop()
	Wait()
}

// LoginItem opens the app at login.
type LoginItem interface {
	OpenAtLogin() bool
	SetOpenAtLogin(on bool) error
}

// Deps are what a Core uses of the world. Those left nil are the real ones, but Post.
type Deps struct {
	// Context is the application lifetime; nil creates one owned by Core.
	Context context.Context
	// Post runs fn on the main thread, in the order of the calls, without waiting.
	Post func(fn func())
	// PrefsPath is where the settings are kept; "" keeps them nowhere.
	PrefsPath string
	// ScenesPath is where the translation scenes are kept; "" keeps them nowhere.
	ScenesPath string
	// HistoryDir is where the subtitles are kept when the user asks; "" keeps them nowhere.
	HistoryDir string
	// StatsPath is where the numbers of every day are kept (Usage); "" keeps them nowhere.
	StatsPath string
	// Now is the time; nil is time.Now. Tests set it.
	Now func() time.Time
	// LogPath is the app's log file, "" for none.
	LogPath string
	// EnginePath finds the engine, ASRPath tomoru-asr.
	EnginePath, ASRPath func() (string, error)
	// StartSession starts a translating session.
	StartSession func(live.Config, func(live.State)) (Session, error)
	// RunEngine runs an engine command that answers and exits.
	RunEngine func(ctx context.Context, bin string, args ...string) ([]engine.Event, error)
	// Translator, when set, is the translator of every session, whatever the engine: for tests.
	Translator translate.Translator
	// ImageEndpoint translates every picture, whatever the engine; nil translates each with the
	// engine chosen. Tests set it.
	ImageEndpoint *translate.Endpoint
	// Select has the user select a part of the screen, and gives it as a PNG, nil if cancelled,
	// with where it is in the screens' pixels, if known: capture.Selector. Without it, pictures
	// come from the clipboard only.
	Select func(ctx context.Context) ([]byte, image.Rectangle, error)
	Login  LoginItem
	// Models downloads and keeps the local models; Server runs the translation models, and
	// ASRServer, apart, the recognition models llama-server runs. Secrets keeps the API keys of the
	// user's endpoints. Without them, there are no local models and no keys.
	Models    Models
	Server    Server
	ASRServer Server
	Secrets   Secrets
	// GPU is the memory of the largest GPU here llama-server runs models on, in bytes: 0 for none,
	// or not known yet. Without it, none.
	GPU func() uint64
	// Proxy has the cloud reached as the settings say: directly with off, else through addr, or
	// the system's proxy when "" (package proxy). SystemProxy is the system's, as Preferences
	// shows it: "" for none. Without them, as Go's transport does.
	Proxy       func(off bool, addr string)
	SystemProxy func() string
	// Network tells whether the computer is online, and on a metered connection; without it, online
	// and not metered.
	Network func() (online, metered bool)
}

// Models downloads and keeps the local models: a *models.Manager.
type Models interface {
	Status(id string) models.Status
	Path(id string) string
	Download(id string)
	Cancel(id string)
	Delete(id string) error
}

// Server runs a local model: a *models.Server. It is called from the main thread and from the
// background, and must be safe for concurrent use.
type Server interface {
	Ensure(ctx context.Context, path string) (url string, err error)
	// EnsureVision is Ensure, the model reading pictures too.
	EnsureVision(ctx context.Context, path string) (url string, err error)
	Stop()
}

// Secrets keeps secrets: mygo.Secrets. It is called from the background too.
type Secrets interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// Core is the application's state and actions.
type Core struct {
	*PreferencesService
	*SceneService
	SaveErr error // the last settings/scene persistence error; dirty data remains retryable
	// SystemProxy is the system's proxy, "host:port", as RefreshSystemProxy last found it: "" for
	// none, or not asked yet.
	SystemProxy string
	proxyAsked  time.Time

	*LearningService
	*ImageService
	*SessionController
	*EngineService
	// The global shortcuts: when each was last pressed, and those held, that do nothing else.
	Pressed map[Shortcut]time.Time
	Held    map[Shortcut]bool

	// Screen Recording, which selecting a part of the screen takes: "granted", "denied",
	// "unknown" (not asked), or "" until checked; ScreenAsking while the system asks.
	Screen          string
	ScreenAsking    bool
	screenCheckedAt time.Time

	*HistoryService

	work     *tasks
	closed   bool
	deps     Deps
	onChange []func()

	// What the system says.
	Access    string            // System Audio Recording: "granted", "denied", "unknown" (not asked), or "" until checked
	Checking  bool              // a permission check is running
	Asking    bool              // the system is asking the user for it
	CheckedAt time.Time         // when the last check ended
	ASR       map[string]string // recognition for a language: "preparing", "ready" or "error"
	sysLangs  []string          // the languages the system's recognition hears, nil until it said
	Apps      []App             // the apps to hear, as last listed
	listedAt  time.Time
	AtLogin   bool   // the app opens at login
	LoginErr  string // why opening at login could not be turned on

	// Saying the samples aloud.
	Speaking bool
	SpeakErr error // why they could not be said
	speak    context.CancelFunc
	speakRun int

	runSince   time.Time     // when translating last started, while it goes on
	runEngines usage.Engines // the engines it started with
	ledger     usage.Ledger  // the numbers of every day, of the sessions ended (stats.go)
}

// App is an app whose sound can be heard alone.
type App struct {
	Name, ID string
	Playing  bool // it plays sound now
}

// New makes a Core with the settings p.
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
	c := &Core{PreferencesService: &PreferencesService{Prefs: p.Clone(), saved: p.Clone()}, SceneService: &SceneService{}, deps: d, ASR: map[string]string{},
		Pressed: map[Shortcut]time.Time{}, Held: map[Shortcut]bool{}}
	c.work = newTasks(d.Context, d.Post)
	c.EngineService = newEngineService(c.work, d.Models, d.Server, d.ASRServer, d.Secrets, c.changed)
	c.HistoryService = &HistoryService{work: c.work, changed: c.changed}
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
			if st.Phase == "listening" {
				c.ASR[c.Prefs.Src] = "ready"
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
	if d.Login != nil {
		c.AtLogin = d.Login.OpenAtLogin()
	}
	return c
}

// LogPath is the app's log file, or "".
func (c *Core) LogPath() string { return c.deps.LogPath }

// Network tells whether the computer is online, and whether the connection is metered.
func (c *Core) Network() (online, metered bool) {
	if c.deps.Network == nil {
		return true, false
	}
	return c.deps.Network()
}

// Changed tells the UI that the state changed, as the models' downloads do: a model chosen that
// came in is used.
func (c *Core) Changed() {
	c.usePending()
	c.changed()
}

// OnChange calls fn after an application action changes the state shown by the UI.
func (c *Core) OnChange(fn func()) { c.onChange = append(c.onChange, fn) }

func (c *Core) changed() {
	for _, fn := range c.onChange {
		fn()
	}
}

// SavePrefs retries dirty settings and scenes. Saving never refreshes business state.
func (c *Core) SavePrefs() error {
	c.SaveErr = errors.Join(c.PreferencesService.save(c.deps.PrefsPath), c.SceneService.save(c.deps.ScenesPath))
	if c.SaveErr != nil {
		log.Println("settings:", c.SaveErr)
	}
	return c.SaveErr
}

// settingsChanged commits an application edit, independently of any window.
func (c *Core) settingsChanged() {
	c.useProxy()
	c.useGlossary()
	c.syncHistoryPolicy()
	c.SavePrefs()
	c.changed()
}

// HideAfter is how long the subtitles stay when nobody speaks.
func (c *Core) HideAfter() time.Duration {
	return time.Duration(c.Prefs.HideAfter * float64(time.Second))
}

// Context is cancelled when the application shuts down. Platform UI workers use it too.
func (c *Core) Context() context.Context { return c.work.ctx }

// useProxy has the cloud reached as the settings say.
func (c *Core) useProxy() {
	if c.deps.Proxy != nil {
		c.deps.Proxy(c.Prefs.NoProxy, c.Prefs.ProxyURL)
	}
}

// RefreshSystemProxy asks the system's proxy again, in the background, for Preferences to show it
// in SystemProxy: once in a few seconds at most, as the system's settings change.
func (c *Core) RefreshSystemProxy() {
	ask := c.deps.SystemProxy
	if ask == nil || !c.proxyAsked.IsZero() && c.now().Sub(c.proxyAsked) < 3*time.Second {
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
