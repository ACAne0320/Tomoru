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
	useDesktop()
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.Use(native.New(updater.Options{Icon: assets.AppIconPNG}))

	mygo.App.WhenReady(func() {
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
		mygo.Theme.SetSource(kit.ThemeSource(p.Appearance))
		ctx, cancel := context.WithCancel(context.Background())
		post := shell.Post(ctx)
		roots := resourceRoots()
		var k *core.Core
		proxy.Install()
		proxy.Set(p.NoProxy, p.ProxyURL)
		deps := core.Deps{Context: ctx, Post: post, NoVoice: noVoice,
			Proxy: proxy.Set, SystemProxy: func() string { return proxy.System("https://api.openai.com") },
			EnginePath: func() (string, error) { return engine.Path(roots...) },
			ASRPath:    func() (string, error) { return engine.ASRPath(roots...) }, PrefsPath: path, ScenesPath: scenesPath, HistoryDir: historyDir, StatsPath: statsPath, LogPath: logPath, LicensesDir: resource.Licenses(roots...), Version: mygo.App.Version(), OS: osVersion(), Login: mygo.App, Secrets: mygo.Secrets,
			Network: func() (bool, bool) { s := mygo.Network.Status(); return s.Online, s.Metered },
			Select:  capture.Selector(func() (string, error) { return engine.Path(roots...) })}
		var manager *models.Manager
		dir, err := models.Dir()
		if p.ModelsDir != "" {
			dir, err = p.ModelsDir, nil
		}
		if err == nil {
			mirrors := models.HuggingFaceFirst
			if strings.HasPrefix(mygo.App.Locale(), "zh") {
				mirrors = models.ModelScopeFirst
			}
			manager = models.NewManagerContext(ctx, dir, mirrors, func() { post(func() { k.Changed() }) })
			deps.Models = manager
		} else {
			log.Println("models:", err)
		}
		if bin, err := resource.Bin("llama-server", roots...); err == nil {
			var projectors []models.ServerOption
			if manager != nil {
				projectors = append(projectors, models.WithProjectors(manager.ProjectorOf))
			}
			server := models.NewServer(bin, llamaLog, projectors...)
			hearing := models.NewServer(bin, llamaLog, append(projectors, models.WithWarm(llama.Warm))...)
			deps.Server, deps.ASRServer = server, hearing
			var gpu atomic.Uint64
			var gpuKnown atomic.Bool
			deps.GPU = func() (uint64, bool) { return gpu.Load(), gpuKnown.Load() }
			deps.Memory = func() (uint64, bool) { m := models.Memory(); return m, m > 0 }
			go func() {
				ds, err := models.Devices(ctx, bin)
				if err != nil {
					log.Println("models: devices:", err)
				}
				for _, d := range ds {
					gpu.Store(max(gpu.Load(), d.Budget))
				}
				gpuKnown.Store(err == nil)
				post(func() { k.Changed() })
			}()
		} else {
			log.Println("models:", err)
		}
		k = core.New(p, deps)

		if p.Onboarded {
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
		} else {
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		}
		mygo.App.OnWindowAllClosed(func() {})
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { k.Shutdown(); cancel() })
		ui := shell.New(k)
		mygo.App.OnSecondInstance(func([]string, string) { ui.ShowSettings() })
		mygo.App.OnActivate(func(bool) { ui.ShowSettings() })
		ui.Launch(*start)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

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
