package shared

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
)

type Platform struct {
	Engine string
	System string
	App    string

	Source       func(app string) Source
	HearsOneApp  bool
	Apps         func()
	Speak        func(path string)
	Select       func(path string) (image.Rectangle, error)
	ScreenAccess func(request bool) Access
}

var ErrCancelled = errors.New("cancelled")

func noScreenPermission(bool) Access { return AccessGranted }

func Run(p Platform) {
	log.SetFlags(0)
	log.SetPrefix("[engine] ")
	Send(Event{Type: "hello", Protocol: 1, Engine: p.Engine, Version: version})
	usage := func() {
		fmt.Fprintf(os.Stderr, "usage: tomoru-engine listen --locale <bcp47> --asr none [--app %s | --file <wav>] [--vad <model> | none] | speak --file <wav> | select --out <png> | access | request-access | screen-access | request-screen-access | apps\n", p.App)
		os.Exit(2)
	}
	if len(os.Args) < 2 {
		usage()
	}
	noASR := p.System + " has no speech recognition for apps"
	if p.ScreenAccess == nil {
		p.ScreenAccess = noScreenPermission
	}
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "screen-access", "request-screen-access":
		Send(Event{Type: "access", State: string(p.ScreenAccess(cmd == "request-screen-access"))})

	case "access", "request-access":
		Send(Event{Type: "access", State: string(AccessGranted)})

	case "apps":
		if p.Apps != nil {
			p.Apps()
		}

	case "prepare":
		Fail("unsupported-locale", "%s", noASR)

	case "speak":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		file := fs.String("file", "", "the WAV recording to play")
		fs.Parse(args)
		if *file == "" {
			usage()
		}
		if p.Speak == nil {
			Fail("internal", "speaking is not supported on %s", p.System)
		}
		p.Speak(*file)

	case "select":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		path := fs.String("out", "", "where the picture of the selection goes, as PNG")
		fs.Parse(args)
		if *path == "" {
			usage()
		}
		at, err := p.Select(*path)
		switch {
		case errors.Is(err, ErrCancelled):
		case err != nil:
			Fail("capture-failed", "%v", err)
		default:
			Send(Event{Type: "area", X: at.Min.X, Y: at.Min.Y, Width: at.Dx(), Height: at.Dy()})
		}

	case "listen":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		fs.String("locale", "", "the language spoken, BCP 47: the recognizer's business")
		asr := fs.String("asr", "", "none: hear only")
		app := fs.String("app", "", "the app to hear alone, "+p.App)
		file := fs.String("file", "", "a WAV recording to hear instead of the system")
		vad := fs.String("vad", defaultVAD(), "Silero VAD's ONNX model, or none")
		fs.Parse(args)
		if *asr != "none" {
			Fail("unsupported-locale", "%s: listen --asr none", noASR)
		}
		var src Source = &FileSource{path: *file}
		switch {
		case *file != "":
		case *app != "" && !p.HearsOneApp:
			Fail("capture-failed", "hearing one app alone is not supported on %s yet", p.System)
		default:
			src = p.Source(*app)
		}
		listen(src, *vad)
		select {}

	default:
		usage()
	}
}

func listen(src Source, vad string) {
	go func() {
		if err := readCommands(os.Stdin, func(Command) {}); err != nil {
			Fail("internal", "stdin: %v", err)
		}
		log.Print("stdin closed")
		os.Exit(0)
	}()
	var voice func([]float32)
	if vad != "none" {
		if v, err := newSilero(vad); err != nil {
			log.Printf("voice detection off: %v", err)
		} else {
			voice = Feeder(NewDetector(v, func(on bool, t float64) { Send(Event{Type: "voice", On: on, T: t}) }))
			Send(Event{Type: "voice", On: false, T: 0})
		}
	}
	if err := NewHearer(Send, voice).Start(src); err != nil {
		Fail("capture-failed", "%v", err)
	}
}
