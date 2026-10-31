// Tomoru shows live translated subtitles over whatever the computer plays. See docs/architecture.md.
//
// The layers, each depending only on those after it:
//
//	internal/ui/shell     the windows and the menu bar icon, and going between them
//	internal/ui/...       the surfaces: overlay, panel, settings, guide; and what they share
//	internal/core         the application: settings, translating, what the system says
//	internal/live         a session: engine → segmenter → translator
//	internal/engine, segment, translate, models, prefs, resource, applog
//
//	go run . [--start] [--guide]
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"tomoru/internal/applog"
	"tomoru/internal/asr/llama"
	"tomoru/internal/capture"
	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/proxy"
	"tomoru/internal/resource"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shell"
)

func main() {
	start := flag.Bool("start", false, "start translating at launch, whatever the settings say")
	guide := flag.Bool("guide", false, "run the first-run guide again, as on a first launch")
	flag.Parse()
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running one opens its settings
	}
	// Updates: checked soon after launch, then once a day, offered in a window of their own, in the
	// app's language.
	mygo.Use(native.New(updater.Options{Icon: assets.AppIconPNG}))

	mygo.App.WhenReady(func() {
		// The logs: the app's and the engine's in tomoru.log, llama-server's, which says much, apart.
		var logPath string
		var llamaLog io.Writer
		if dir, err := mygo.App.Path(mygo.PathLogs); err == nil {
			if f, err := applog.Open(dir, "tomoru"); err == nil {
				logPath = f.Path()
				log.SetOutput(f.And(os.Stderr))
				engine.Log = f.And(os.Stderr)
			}
			if f, err := applog.Open(dir, "llama-server"); err == nil {
				llamaLog = f
			}
		}
		log.Printf("Tomoru %s, %s", mygo.App.Version(), mygo.App.Locale())

		path, scenesPath, historyDir, statsPath := "", "", "", ""
		if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
			path, scenesPath, historyDir, statsPath = filepath.Join(dir, "settings.json"), filepath.Join(dir, "scenes.json"), filepath.Join(dir, "history"), filepath.Join(dir, "stats.json")
		} else {
			log.Println("settings:", err)
		}
		p, err := prefs.Load(path, mygo.App.Locale())
		if err != nil {
			log.Println("settings:", err)
		}
		if *guide {
			p.Onboarded, p.GuideStep = false, 0
		}
		// Before the first window, which shows in it.
		mygo.Theme.SetSource(kit.ThemeSource(p.Appearance))
		ctx, cancel := context.WithCancel(context.Background())
		post := shell.Post(ctx)
		roots := resourceRoots()
		var k *core.Core
		// The cloud is reached as the settings say, from the first request: the models' downloads
		// resume as the manager starts.
		proxy.Install()
		proxy.Set(p.NoProxy, p.ProxyURL)
		deps := core.Deps{Context: ctx, Post: post,
			Proxy: proxy.Set, SystemProxy: func() string { return proxy.System("https://api.openai.com") },
			EnginePath: func() (string, error) { return engine.Path(roots...) },
			ASRPath:    func() (string, error) { return engine.ASRPath(roots...) }, PrefsPath: path, ScenesPath: scenesPath, HistoryDir: historyDir, StatsPath: statsPath, LogPath: logPath, Login: mygo.App, Secrets: mygo.Secrets,
			Network: func() (bool, bool) { s := mygo.Network.Status(); return s.Online, s.Metered },
			Select:  capture.Selector(func() (string, error) { return engine.Path(roots...) })}
		// The local models: downloaded into the user's data, run by the llama-server the app ships,
		// with a model's vision projector once a picture had it downloaded.
		var manager *models.Manager
		if dir, err := models.Dir(); err == nil {
			china := strings.HasPrefix(mygo.App.Locale(), "zh")
			manager = models.NewManagerContext(ctx, dir, china, func() { post(func() { k.Changed() }) })
			deps.Models = manager
		} else {
			log.Println("models:", err)
		}
		if bin, err := resource.Bin("llama-server", roots...); err == nil {
			// Translation's, and recognition's apart: both may run at once.
			server, hearing := models.NewServer(bin, llamaLog), models.NewServer(bin, llamaLog)
			hearing.Warm = llama.Warm
			if manager != nil {
				server.Vision, hearing.Vision = manager.VisionOf, manager.VisionOf
			}
			deps.Server, deps.ASRServer = server, hearing
			// The GPUs, for the engines to recommend: once, in the background (some 50 ms).
			var gpu atomic.Uint64
			deps.GPU = gpu.Load
			go func() {
				ds, err := models.Devices(ctx, bin)
				if err != nil {
					log.Println("models: devices:", err)
				}
				for _, d := range ds {
					gpu.Store(max(gpu.Load(), d.Memory))
				}
				post(func() { k.Changed() })
			}()
		} else {
			log.Println("models:", err)
		}
		k = core.New(p, deps)

		// A menu bar app. Until the first-run guide is done, a regular one from the start: become one
		// later, once in front, and the system keeps it out of the apps it goes back to, and its
		// permission prompt, answered, gives the front to the app before, often System Settings.
		if p.Onboarded {
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
		} else {
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		}
		mygo.App.OnWindowAllClosed(func() {}) // a menu bar app keeps running
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { k.Shutdown(); cancel() })
		ui := shell.New(k)
		// Starting the app again, from Finder or the launcher, opens its settings: the menu bar icon
		// may be out of sight.
		mygo.App.OnSecondInstance(func([]string, string) { ui.ShowSettings() })
		ui.Launch(*start)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// resourceRoots is desktop composition policy. Runtime packages receive these
// paths explicitly and never consult the UI framework or working directory.
func resourceRoots() []string {
	var roots []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		roots = append(roots, dir)
	}
	if dir, err := os.Getwd(); err == nil {
		for {
			if _, err := os.Stat(filepath.Join(dir, "mygo.json")); err == nil {
				roots = append(roots, filepath.Join(dir, "resources"))
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return roots
}
