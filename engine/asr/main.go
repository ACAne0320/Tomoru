package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
)

const version = "0.1.0"

var out = struct {
	sync.Mutex
	w *bufio.Writer
}{w: bufio.NewWriter(os.Stdout)}

func send(e Event) {
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("protocol encoding: %v", err)
		os.Exit(1)
	}
	out.Lock()
	defer out.Unlock()
	if _, err := out.w.Write(append(b, '\n')); err != nil {
		os.Exit(1)
	}
	if err := out.w.Flush(); err != nil {
		os.Exit(1)
	}
}

func fail(code, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	send(Event{Type: "error", Code: code, Message: msg})
	log.Printf("error %s: %s", code, msg)
	os.Exit(1)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("[asr] ")
	send(Event{Type: "hello", Protocol: 1, Engine: "asr", Version: version})
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: tomoru-asr serve --model <dir> --lang <auto|zh|en|ja|ko|yue> [--threads <n>]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	model := fs.String("model", "", "the model's directory")
	lang := fs.String("lang", "auto", `the language spoken, or "auto" to tell it stretch by stretch`)
	threads := fs.Int("threads", 2, "threads to decode with")
	fs.Parse(os.Args[2:])
	rec, err := newSenseVoice(*model, *lang, *threads)
	if err != nil {
		fail("model-failed", "%v", err)
	}
	log.Printf("recognizing %s with %s, %d threads", *lang, *model, *threads)
	err = readCommands(os.Stdin, func(c Command) error {
		if c.Type != "decode" {
			return nil
		}
		ev, err := decode(rec, *lang, c)
		if err != nil {
			return err
		}
		send(ev)
		return nil
	})
	if err != nil {
		fail("internal", "%v", err)
	}
	log.Print("stdin closed")
}
