// tomoru-engine: Tomoru's Windows engine. It hears what the computer plays with WASAPI's process
// loopback, and speaks docs/engine-protocol.md on stdin and stdout. It hears only: Windows has no
// recognizer for apps to use, so tomoru-asr or the cloud recognizes what it hears.
//
//	tomoru-engine listen --locale <bcp47> --asr none [--app <exe> | --file <wav>] [--vad <silero_vad.onnx> | none]
//	tomoru-engine speak --locale <bcp47> -- <sentence>...
//	tomoru-engine select --out <png>
//	tomoru-engine access | request-access | screen-access | request-screen-access | apps
//
// An app is its program's file name, as "chrome.exe": its helper processes are of the same file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tomoru-engine listen --locale <bcp47> --asr none [--app <exe> | --file <wav>] [--vad <model> | none] | speak --locale <bcp47> -- <sentence>... | select --out <png> | access | request-access | screen-access | request-screen-access | apps")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("[engine] ")
	send(Event{Type: "hello", Protocol: 1, Engine: "windows", Version: version})
	if len(os.Args) < 2 {
		usage()
	}
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "access", "request-access", "screen-access", "request-screen-access":
		// Windows asks no permission to hear what plays, nor to capture the screen.
		send(Event{Type: "access", State: "granted"})

	case "apps":
		listApps()

	case "prepare":
		fail("unsupported-locale", "Windows has no speech recognition for apps")

	case "speak":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		locale := fs.String("locale", "", "the language to speak, BCP 47")
		fs.Parse(args)
		lines := fs.Args()
		if i := slices.Index(args, "--"); i >= 0 {
			lines = args[i+1:]
		}
		if *locale == "" || len(lines) == 0 {
			usage()
		}
		speak(*locale, lines)

	case "select":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		path := fs.String("out", "", "where the picture of the selection goes, as PNG")
		fs.Parse(args)
		if *path == "" {
			usage()
		}
		at, err := selectArea(*path)
		if err != nil {
			fail("capture-failed", "%v", err)
		}
		if at != nil {
			send(Event{Type: "area", X: at.Min.X, Y: at.Min.Y, Width: at.Dx(), Height: at.Dy()})
		}

	case "listen":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		fs.String("locale", "", "the language spoken, BCP 47: the recognizer's business")
		asr := fs.String("asr", "", "none: hear only")
		app := fs.String("app", "", "the program to hear alone, as chrome.exe")
		file := fs.String("file", "", "a WAV recording to hear instead of the system")
		vad := fs.String("vad", defaultVAD(), "Silero VAD's ONNX model, or none")
		fs.Parse(args)
		if *asr != "none" {
			fail("unsupported-locale", "Windows has no speech recognition for apps: listen --asr none")
		}
		listen(*app, *file, *vad)
		select {} // until stdin closes

	default:
		usage()
	}
}

// listen hears the system, or app alone, or file, and sends what it hears until stdin closes.
func listen(app, file, vad string) {
	// Commands come on stdin; when it closes, the app has gone, and so does the engine.
	go func() {
		if err := readCommands(os.Stdin, func(Command) {}); err != nil {
			fail("internal", "stdin: %v", err)
		}
		log.Print("stdin closed")
		os.Exit(0)
	}()
	var voice func([]float32)
	if vad != "none" {
		if v, err := newSilero(vad); err != nil {
			log.Printf("voice detection off: %v", err)
		} else {
			voice = Feeder(NewDetector(v, func(on bool, t float64) { send(Event{Type: "voice", On: on, T: t}) }))
			// Voice detection is on: nobody speaks yet.
			send(Event{Type: "voice", On: false, T: 0})
		}
	}
	var src Source = &FileSource{path: file}
	if file == "" {
		src = systemSource(app)
	}
	if err := NewHearer(send, voice).Start(src); err != nil {
		fail("capture-failed", "%v", err)
	}
}
