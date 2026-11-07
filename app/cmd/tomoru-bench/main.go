// Command tomoru-bench runs the benchmark's cases (bench/cases.tsv, bench/README.md) through Tomoru
// as the app runs them: the engine hears each recording at its pace (listen --file), and the
// session recognizes, segments, drafts and translates it. It records when each change of the
// subtitles came, for bench/score.py to score against the references.
//
//	go run ./cmd/tomoru-bench -asr apple -mt cloud [-j 10] [-max 120] [-case id,...] [-lang ja] [-scene talk] [-all]
//	go run ./cmd/tomoru-bench -mt cloud -mt-only    translate the references' sentences only
//
// -asr is apple, a recognition model's ID (sensevoice, qwen3-asr-0.6b…), qwen-cloud[:model] with
// DASHSCOPE_API_KEY and DASHSCOPE_API_HOST, or openai[:model] (live transcription, or the
// transcription API for another model than gpt-live-transcribe) with OPENAI_API_KEY and
// OPENAI_BASE_URL;
// -mt is cloud[:model], a translation model's ID (local-2b…), or chat with -mt-url, -mt-model and
// the key in $MT_API_KEY.
// A run goes to bench/runs/<name>/: run.json, and <case>.json for each case. Another run of the same
// name and engines adds its cases to it: a long run goes in batches.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/scene"
)

func main() {
	log.SetFlags(0)
	resourceDir := flag.String("resources", "", "bundled resources root (default: <bench>/../app/resources)")
	benchDir := flag.String("bench", "", "the bench directory (default: ../bench or ./bench)")
	var hear, local []string
	for _, m := range models.All {
		switch {
		case m.ID == "sensevoice-small":
			hear = append(hear, "sensevoice")
		case m.ASR:
			hear = append(hear, m.ID)
		default:
			local = append(local, m.ID)
		}
	}
	asrName := flag.String("asr", "apple", "recognition: apple, "+strings.Join(hear, ", ")+", qwen-cloud[:model], openai[:model]")
	mt := flag.String("mt", "cloud", "translation: cloud[:model], "+strings.Join(local, ", ")+", chat")
	mtURL := flag.String("mt-url", "", "with -mt chat: the API's base URL, without /v1")
	mtModel := flag.String("mt-model", "", "with -mt chat: the model")
	draft := flag.Bool("draft", true, "translate sentences while they are spoken, as the app does by default")
	mtOnly := flag.Bool("mt-only", false, "translate the references' sentences, with no recognition")
	jobs := flag.Int("j", 1, "cases at once; more makes it faster and the latencies of local engines worse")
	maxSec := flag.Float64("max", 0, "seconds of each case at most, from its start; 0 for all")
	only := flag.String("case", "", "the cases to run, by ID, comma-separated")
	lang := flag.String("lang", "", "only the cases of this language")
	kind := flag.String("scene", "", "only the cases of this scene")
	everyCase := flag.Bool("all", false, "the cases with no reference yet too: their latencies have nothing to go by")
	tail := flag.Duration("tail", 10*time.Second, "how long to wait after the recording, for its last sentence")
	name := flag.String("name", "", "the run's name (default: from -asr, -mt and the time)")
	glossary := flag.String("glossary", "", "a directory of glossaries, <case>.json each, the terms of a scene (internal/scene) to translate the case with")
	about := flag.String("about", "", "a directory of scene descriptions, <case>.txt each, to recognize and translate the case with")
	flag.Parse()

	dir := *benchDir
	if dir == "" {
		for _, d := range []string{"../bench", "bench"} {
			if _, err := os.Stat(filepath.Join(d, "cases.tsv")); err == nil {
				dir = d
				break
			}
		}
	}
	all, err := readCases(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		log.Fatal(err)
	}
	var cases []Case
	ids := strings.FieldsFunc(*only, func(r rune) bool { return r == ',' })
	for _, c := range all {
		if (len(ids) == 0 || slices.Contains(ids, c.ID)) && (*lang == "" || c.Lang == *lang) && (*kind == "" || c.Scene == *kind) &&
			(c.HasRef() || *everyCase && !*mtOnly) {
			cases = append(cases, c)
		}
	}
	if len(cases) == 0 {
		log.Fatal("no cases")
	}

	roots := []string{*resourceDir}
	if roots[0] == "" {
		roots[0] = filepath.Join(dir, "..", "app", "resources")
	}
	tr, stopMT, err := translator(*mt, *mtURL, *mtModel, roots...)
	if err != nil {
		log.Fatal(err)
	}
	var spec asr.Spec
	var env asr.Env
	if !*mtOnly {
		var stop func()
		if spec, env, stop, err = recognizer(*asrName, roots...); err != nil {
			log.Fatal(err)
		}
		defer stop()
	}
	bin, err := engine.Path(roots...)
	if err != nil && !*mtOnly {
		log.Fatal(err)
	}

	run := Run{ASR: *asrName, MT: *mt, Draft: *draft, MTOnly: *mtOnly, Glossary: *glossary, About: *about, Jobs: *jobs, Max: *maxSec, Started: time.Now()}
	if *mtOnly {
		run.ASR = "reference"
	}
	run.Name = *name
	if run.Name == "" {
		run.Name = fmt.Sprintf("%s_%s_%s", strings.ReplaceAll(run.ASR, ":", "-"), *mt, run.Started.Format("0102-1504"))
	}
	out := filepath.Join(dir, "runs", run.Name)
	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}
	media := filepath.Join(dir, "media")
	tmp, err := os.MkdirTemp("", "tomoru-bench")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	engine.Log = io.Discard

	var mu sync.Mutex
	sem := make(chan struct{}, max(1, *jobs))
	var wg sync.WaitGroup
	for _, c := range cases {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			t0 := time.Now()
			var r Result
			desc, err := readAbout(*about, c.ID)
			if err != nil {
				log.Fatal(err)
			}
			if *mtOnly {
				r = translateRefs(c, media, tr, desc)
			} else {
				// The case's scene: its glossary's names and its description, which recognition is
				// told to expect, and translation is given.
				var g *scene.Glossary
				if *glossary != "" {
					if g, err = readGlossary(filepath.Join(*glossary, c.ID+".json")); err != nil {
						log.Fatal(err)
					}
				}
				env := env
				env.Scene = asr.Scene{Names: g.Names(), About: desc}
				recognize, err := spec.New(env)
				if err != nil {
					log.Fatal(err)
				}
				cfg := live.Config{Engine: bin, Recognize: recognize, VolatileSentences: spec.VolatileSentences, FinalsEnd: spec.FinalsEnd, Translator: tr, Draft: *draft, About: func() string { return desc }}
				if spec.Hint != nil {
					cfg.Hint = spec.Hint(c.Lang)
				}
				if g != nil {
					cfg.Terms = g.Apply
				}
				r = runCase(c, media, tmp, *maxSec, *tail, cfg)
			}
			b, _ := json.MarshalIndent(r, "", " ")
			if err := os.WriteFile(filepath.Join(out, c.ID+".json"), b, 0o644); err != nil {
				log.Fatal(err)
			}
			mu.Lock()
			run.Audio += r.Audio
			run.Cases = append(run.Cases, c.ID)
			mu.Unlock()
			status := "ok"
			if r.Err != "" {
				status = "FAILED: " + r.Err
			}
			log.Printf("%-24s %5.0fs audio in %5.0fs: %s", c.ID, r.Audio, time.Since(t0).Seconds(), status)
		}()
	}
	wg.Wait()
	stopMT()
	run.Wall = time.Since(run.Started).Seconds()
	run.CPUChild, run.CPUSelf = cpuTimes()
	// A run of that name already there gets these cases too, as a run in batches does.
	if b, err := os.ReadFile(filepath.Join(out, "run.json")); err == nil {
		var before Run
		if json.Unmarshal(b, &before) == nil && before.ASR == run.ASR && before.MT == run.MT && before.Draft == run.Draft {
			for _, id := range before.Cases {
				if !slices.Contains(run.Cases, id) {
					run.Cases = append(run.Cases, id)
				}
			}
			run.Started = before.Started
			run.Wall += before.Wall
			run.CPUChild += before.CPUChild
			run.CPUSelf += before.CPUSelf
			run.Jobs = max(run.Jobs, before.Jobs)
		}
	}
	slices.Sort(run.Cases)
	// The audio of the cases as they are now: a case run again counts once.
	run.Audio = 0
	for _, id := range run.Cases {
		var r Result
		if b, err := os.ReadFile(filepath.Join(out, id+".json")); err == nil && json.Unmarshal(b, &r) == nil {
			run.Audio += r.Audio
		}
	}
	b, _ := json.MarshalIndent(run, "", " ")
	if err := os.WriteFile(filepath.Join(out, "run.json"), b, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: %d cases, %.0f s of audio in %.0f s; score with: uv run bench/score.py %s", run.Name, len(run.Cases), run.Audio, run.Wall, run.Name)
}
