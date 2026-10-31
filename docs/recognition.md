# Recognition adapters

[中文](recognition_zh.md)

Status: designed 2026-10-31, its open questions decided the same day. All six steps are done:
1 (the base), 2 (the batch runner), 3 (sentences by the recognizer), 4 (Qwen3-ASR), 5 (OpenAI
transcriptions) and 6 (OpenAI Realtime), each measured on the quick set; "How it works now"
describes the code before them. It builds on
[local-asr.md](local-asr.md), which records how recognition works today and why.

## Objective

Adding a recognizer should take one adapter and one catalog entry, with nothing changed in the
session, the segmenter or the UI. On that base, add a local Qwen3-ASR 1.7B, OpenAI's Realtime
transcription and its `/v1/audio/transcriptions` API (and the services compatible with it), and let
the recognizer decide where sentences end instead of rules of ours.

## Why

A Sekiro dialogue recorded on Windows (157 s, Wolf and Kuro; 2026-10-31) showed what a game's
speech does to recognition as it is:

- SenseVoice misheard names and old-fashioned speech (葦名 → 葦シ名, 葦の葉笛 → 足の歯ブエ, made-up
  「ぎ院？」), and its sentences were cut in the wrong places: the 0.25 s pause that ends a stretch
  was tuned on videos, where people talk without stopping, while a character pauses inside a
  sentence and barely between two. `その後大事はないかその / 後とはあの / 世のことを覚えておらぬのかはい。`
- Qwen's cloud (`qwen-audio-3.1-asr-flash-streaming`) gave one sentence a line, punctuated, with the
  names right: its own sentence ends were better than ours.
- Qwen3-ASR 1.7B Q8_0 on llama.cpp punctuated as well as the cloud, with more mistakes in names
  (アシナ, 一神様). bf16 gave the same text, so quantization isn't the cause.

| Qwen3-ASR 1.7B Q8_0 | M5, 24 GB | RTX 5060, 8 GB |
|---|---|---|
| Memory | 2.9–3.1 GB resident (unified memory) | +2.5 GB of VRAM (6.6 of 8 GB with Sekiro running) |
| Decoding | 0.7 s per 10 s of speech | 0.3 s per 10 s, once warm; the first two requests took 3–5 s |

## Requirements

1. One interface for every recognizer, of two families (below), and adapters that implement it.
2. One implementation of "cut speech into stretches and decode each" shared by every recognizer
   that decodes whole stretches: SenseVoice, Qwen3-ASR, OpenAI's transcriptions API.
3. Sentences end where the recognizer ends them (its punctuation, its end of utterance). The
   segmenter keeps no rule of ours but one safety cut for overlong sentences.
4. New recognizers: Qwen3-ASR 1.7B (local, llama.cpp, GPU); OpenAI Realtime transcription; OpenAI
   `/v1/audio/transcriptions` with a base URL of the user's, for compatible services.
5. The engine catalog and the settings list recognizers from the adapters' entries, as translation
   lists APIs the user added.
6. Nothing gets worse unnoticed: every step is measured with the benchmark against the current
   behavior, including dialogue cases.

## Non-goals

- Full-screen exclusive games: the overlay can't show over them; that's a separate problem.
- A plugin system: adapters are compiled in, as decided for engines (local-asr.md).
- Changing translation, the overlay, or the capture engines beyond what's listed below.

## How it works now

```
engine listen ──result/clock/voice──────────────────────────► live.Session ─► segment ─► translate
              (Apple: recognition in the engine)
engine listen --asr none ──audio/voice──► live.Recognizer ──result/clock/voice──┘
                                           ├ tomoru-asr (SenseVoice; own VAD and streaming)
                                           └ cloudasr (Qwen, WebSocket)
```

- `live.Recognizer` (`app/internal/live/live.go`) is `Audio`, `Finalize`, `Stop`, `Done`, `Err`;
  recognizers report `engine.Event`s, the engine protocol (`docs/engine-protocol.md`), so the
  segmenter takes every recognizer alike. That part works and stays.
- `provider.Recognizer` (`app/internal/provider/provider.go`) switches on a kind string: `apple`,
  `local`, `qwen-cloud`. `core.engineCatalog` (`app/internal/core/catalog.go`) hard-codes the list:
  Apple on macOS, the ASR models of `models.All`, then `QwenCloud`. Its settings are their own
  struct, `prefs.ASRCloud`.
- `tomoru-asr` (`engine/asr/stream.go`) does voice detection (Silero through sherpa-onnx, 0.25 s
  of silence ends a stretch), re-decodes a growing stretch every 0.4 s as volatile results, shows
  only what two decodes agree on and never shrinks it, finalizes at 12 s.
- `segment` (`app/internal/segment/`) decides most sentence ends itself: pauses after final tokens
  (`FinalPause` 0.6 s) or of an unchanged hypothesis (`VolatilePause` 1.3 s), clause cuts at voice
  pauses for Chinese (`VoiceCuts`), cuts at clause marks and at joints of the language over a
  length, and asks the recognizer to `finalize` at pauses.
- `models.Server` (`app/internal/models/server.go`) runs one llama-server with one model at a time.

## Design

### Two families

| Family | Who ends sentences | Recognizers |
|---|---|---|
| Stream | The service, as it hears: volatile results, then final ones at its end of utterance | Apple (in the engine), Qwen cloud, OpenAI Realtime |
| Batch | The app cuts speech into stretches by voice detection; the model punctuates each stretch | SenseVoice, Qwen3-ASR (llama.cpp), OpenAI transcriptions |

A stream adapter is written per service. A batch recognizer is a `Decoder`, "a stretch of audio
in, text out", and the shared `batch.Runner` makes it a recognizer. This is where adding a model
becomes small: most new models are decoders.

### Interfaces

A new package, `app/internal/asr`, owns them; `live` imports `asr` instead of declaring its own.

```go
// Recognizer recognizes the audio the engine hears, reporting as the engine does
// (docs/engine-protocol.md): a *batch.Runner of a decoder, a *qwencloud.Session or an
// *openai.Realtime. It decides itself where results are final.
type Recognizer interface {
	Audio(engine.Event) error
	Stop()
	Done() <-chan struct{}
	Err() error
}

// Factory starts a session's recognizer for the language lang, reporting its events to on.
type Factory func(lang string, on func(engine.Event)) (Recognizer, error)

// Decoder decodes a stretch of speech, 16 kHz mono 16-bit PCM (asr/batch).
type Decoder interface {
	Decode(ctx context.Context, pcm []byte) ([]Piece, error) // Piece: Text, At from the stretch's start
	Stop()
	Done() <-chan struct{}
	Err() error
}

// Caps are how a decoder is run, in seconds.
type Caps struct {
	Volatile float64 // between volatile decodes of a stretch going on; 0 for none
	Gap      float64 // without voice that end a stretch
	Longest  float64 // a stretch is at most
	Stops    bool    // every decode ends with a full stop, left out
	Untimed  bool    // no piece is timed: the runner spreads the stretch over them
	Steady   bool    // each volatile decode shows as it is
	Remote   bool    // decodes over the network: each is timed, to tell a slow network
}
```

The recognizers also take the engine's `voice` events (`Voice(engine.Event)`): the session hands
them on to those that have the method, as all three do. The language comes with the factory, and
what to expect with `Env.Scene` (see "Hints").

`Finalize` left the interface: the segmenter no longer asks for a final, and the recognizers end
results themselves. The session still sends `finalize` to the macOS engine, when Apple's
recognition runs in it, as `Spec.Hint` says (below).

### The entries

```go
// Spec is a recognizer to choose. asr.Specs lists them; the catalog and the settings read it.
type Spec struct {
	ID                string        // as prefs.ASR names it
	Family            Family        // Stream or Batch
	Cloud             bool          // what is heard leaves the computer
	Billed            bool          // the service bills the audio sent: hang up while nobody speaks
	URL               string        // a cloud's address unless the user gives one
	Models            []string      // a cloud's models, the default first
	Langs             []string      // what it recognizes, in the order of lang.All; nil: the system's
	Model             *models.Model // to download first, or nil
	Key               bool          // needs an API key
	Hints             bool          // is told what to expect (Env.Scene)
	VolatileSentences bool          // the sentences its volatile text ends hold
	FinalsEnd         bool          // its final results end utterances
	Hint              func(lang string) func(engine.Event) (through float64, ok bool) // Apple's, in the engine
	New               func(env Env) (Factory, error) // nil factory: the engine recognizes
	Test              func(ctx context.Context, svc Service) (time.Duration, error) // a cloud's connection
}
```

`Env` carries what the app provides: tomoru-asr's path, the model's files, the llama-server for
recognition, the cloud service chosen with its URL, model and key (`Service`), and the scene
(`Scene`). `core.engineCatalog` lists `asr.Specs`; `provider` no longer builds recognizers.

### The batch runner

`app/internal/asr/batch` turns a `Decoder` into a `Recognizer`. It is today's `tomoru-asr`
streaming (local-asr.md), moved into the app and made general:

- **Stretches**: from the engine's `voice` events (Silero on both platforms), not a detector of its
  own. A stretch starts 0.4 s before voice (detection is late) and ends after `Gap` without voice,
  or, past `Longest`, at the next pause the engine detects, however short.
- **Volatile**: every `Volatile` while the stretch grows, a decode; shown is only what two decodes
  agree on, never shrinking, without the period the model ends every decode with (all as now).
- **Final**: one more decode when the stretch ends. Untimed tokens get times spread evenly by
  letters over the stretch, for latency statistics.
- **Early sentences**: a sentence the model ended, with text after it, that two decodes agree on,
  is final at once (a `result` with `final: true`), so translation doesn't wait for a long stretch to
  end. A timed decoder drops its audio and decodes from its end; an untimed one ends the stretch
  at the next pause, so that later decodes don't have to find it in their text.
- **Load**: one decode in flight. When decoding can't keep up, volatile decodes are skipped, never
  queued; finals always run.
- **Noise**: a final with no letters of the language, or that the model labels with no language
  (Qwen3-ASR's `language None`), is dropped.

`tomoru-asr` becomes a decoder: `tomoru-asr serve --model <dir>` reads
`{"type":"decode","id":7,"lang":"ja","final":true,"pcm":"…"}` and answers
`{"type":"decoded","id":7,"tokens":[…]}`. Its voice detection and streaming go; sherpa-onnx stays
for SenseVoice, and for voice detection in the Windows engine.

### Sentences

The segmenter becomes the recognizer's follower:

1. A final result's text is split where the recognizer ended sentences (its punctuation, by the
   language's rules in `segment/sentence.go`); each complete sentence is committed.
2. What follows the last end: where the recognizer's finals end utterances (`FinalsEnd`: Apple,
   Qwen cloud, whose finals come where they heard the speaker stop) it is committed with them;
   else (a batch decoder's stretch, which voice detection may have cut mid-sentence) it stays
   pending, and the next final joins it.
3. The pending tail is committed when no voice has come for `TailWait` (0.6 s, measured below)
   after the final, or when the session stops.
4. Volatile text commits a sentence only where the adapter says its volatile sentence ends hold:
   Qwen cloud through `steady` (as now), the batch runner by sending them as finals (above).
5. One safety cut stays: a sentence wider than two lines (90, a CJK character two wide, as the
   benchmark's Long % counts), pending or ended, is cut within them, at its last clause mark, else
   between its last two words, as often as it takes. Ended ones came with OpenAI's stretches,
   final at once, a sentence of 120 letters whole (step 5).

Removed from the segmenter: `FinalPause`, `VolatilePause`, `ClausePause`, `MinClause`, `VoiceCuts`
(into the Apple adapter), `Joint`, and asking for `finalize`. Stream adapters set their service's own end of utterance by measurement, not to the
shortest it takes (Qwen cloud's `silence` is 300 ms now, chosen for our pause rules).

Measured when step 3 was done (2026-10-31, the quick set, 2 minutes a language; before → after):

| | Errors % | Long % | Fragments % | Translation seen p50 / p90 | chrF |
|---|---|---|---|---|---|
| Apple | 16.4 → 15.8 | 1 → 1 | 10 → 8 | 1.53 → 1.24 / 2.91 → 2.58 s | 24.5 → 23.1 |
| Qwen cloud | 16.7 → 14.3 | 1 → 1 | 8 → 4 | 2.72 → 3.16 / 5.38 → 7.43 s | 21.8 → 24.3 |
| SenseVoice | 13.1 → 15.0 | 0 → 0 | 6 → 7 | 1.86 → 1.66 / 3.57 → 3.50 s | 23.1 → 23.4 |

- SenseVoice ends every decode with a full stop, wherever its stretch ends; it is left out of
  final results (`batch.Caps.Stops`), or a pause mid-sentence ended a line. Its errors rose with
  step 2's detector (14.7 there).
- Qwen cloud waits for whole sentences where commas cut it every 3–4 s before: its translations
  are better and its slowest come 2 s later, which was accepted.
- `TailWait`: 1.0 s kept slow game dialogue together (Sekiro: 「ほら、昔聞かせてくれたであろう」),
  but on the quick set had translations 0.25 s later at p50, Korean's slowest 4 s later, and long
  lines 4 in 100 (Chinese 10), joining sentences across pauses in steady speech. Pauses don't
  tell the two apart (a TED talk's median 0.6 s, Sekiro's 0.7 s), nor does the audio source, and
  the user should not have to choose. So 0.6 s, and the better answer is the recognizer's: for
  decoders that punctuate well (Qwen3-ASR, step 4), stretches go on through short pauses and the
  model, hearing both sides, ends sentences, early sentences keeping it quick. SenseVoice's
  punctuation can't carry that (it ran Sekiro's sentences together unpunctuated).

Step 4, Qwen3-ASR 1.7B, done 2026-10-31 (the quick set, one case at a time: models sharing a GPU
slow each other):

| | Errors % | Missed % | Shown p50 | Translation seen p50 / p90 | chrF |
|---|---|---|---|---|---|
| M5, SenseVoice (120 s a case) | 15.0 | 9.0 | 1.25 s | 1.65 / 3.53 s | 23.4 |
| M5, Qwen3-ASR | 13.9 | 5.1 | 1.80 s | 2.71 / 6.22 s | 24.4 |
| RTX 5060, SenseVoice (160 s a case) | 16.7 | 10.1 | 1.35 s | 1.70 / 4.27 s | 22.4 |
| RTX 5060, Qwen3-ASR | 12.7 | 4.2 | 1.46 s | 2.64 / 5.89 s | 23.4 |

- It times no word: the sentences it ends while a stretch goes on commit from its volatile text
  (`VolatileSentences`), and once it has ended one, the stretch ends at the speaker's next pause
  however short (`batch.Caps.Untimed`), which keeps stretches short and decodes quick. Without
  that, stretches ran to 15 s and showed text 0.8 s later.
- Volatile decodes still need two that agree: shown as decoded (`Steady`), it put full stops at
  half-said sentences, which committed and came again: 45 % errors.
- Stretches go on through pauses of up to 1 s (`Gap`): Sekiro's 「ほら昔聞かせてくれたであろう」
  holds together. A decode every 0.5 s, made final at the next pause past 15 s.
- The engines page recommends it where `llama-server --list-devices` shows a GPU with its
  memory (3 GB); SenseVoice stays the default.

Step 5, OpenAI transcriptions, done 2026-10-31 (the quick set on the M5, 4 cases at once: a
cloud service shares nothing):

| | Errors % | Missed % | Shown p50 / p90 | Translation seen p50 / p90 | chrF | Long % |
|---|---|---|---|---|---|---|
| Longest 8 s | 11.4 | 6.3 | 5.88 / 11.47 s | 4.65 / 10.80 s | 24.7 | 13 |
| Longest 5 s | 11.5 | 6.1 | 5.04 / 9.01 s | 4.12 / 7.79 s | 24.6 | 8 |
| Longest 3 s | 12.9 | 6.8 | 3.82 / 6.55 s | 3.18 / 5.64 s | 22.9 | 7 |
| Longest 5 s, the two fixes below | 11.2 | 6.0 | 5.47 / 9.98 s | 5.19 / 9.59 s | 25.2 | 0 |
| Qwen3-ASR (step 4) | 13.9 | 5.1 | 1.80 / 6.06 s | 2.71 / 6.22 s | 24.4 | 2 |
| Qwen cloud (step 3) | 14.3 | 4.7 | 1.84 / 3.70 s | 3.16 / 7.43 s | 24.3 | 1 |

- The most accurate recognizer yet (English 1.3 %, Korean 14 % to Qwen3-ASR's 21 %), and the
  slowest: its text shows only once a stretch has ended and come back, a round trip of 1–1.8 s
  for 2–5 s of audio, 2.5 s for 10 s. Runs vary by about 0.5 s with the round trips: the fourth
  run was slower in Japanese too, which the fixes do not touch. Lower latency is for Realtime
  (step 6).
- `Longest` 5 s: as accurate as 8 s, and its translations 2–3 s earlier at p90; 3 s loses
  context: errors 11.5 → 12.9 %.
- Two fixes, which apply to every recognizer: a stretch's text starts with a space where its
  script spaces words (`batch.Spaced`: "…tell" and "Us to do" ran together as "tellUs"); and the
  safety cut cuts ended sentences too (above). SenseVoice's, Apple's and Qwen3-ASR's quick sets came
  out the same after them, but for Qwen3-ASR's long lines, 2 % → 0.

- `asr/openai`: a batch decoder of `POST {base}/v1/audio/transcriptions`, the stretch as a WAV,
  with `model`, `language` (ISO-639-1: `zh` for Traditional Chinese, none for Cantonese and
  Filipino, which the model tells itself) and `prompt`, the end of the stretch before (about 120
  characters), for names and sentences to go on alike. A decode the connection or the server
  failed (5xx, 429) is tried once more; a refused key (401) is `openai.ErrAuth`.
- Run untimed, with no volatile decode (`Volatile: 0`, which `batch` takes as none): each decode
  is billed by the audio sent. A stretch ends at a pause of 0.5 s, or the first pause past 5 s,
  and its text shows then, after a round trip.
- OpenAI's documentation, read 2026-10-31: the transcription models are `gpt-transcribe` (the
  default here), and `gpt-4o-transcribe`, `gpt-4o-mini-transcribe` and `whisper-1`, deprecated,
  to shut down 2027-02-26. Only `gpt-transcribe` takes `languages` (several possible) and
  `keywords` (words to expect), which a glossary could fill later. Realtime transcription has
  `gpt-live-transcribe` (step 6).
- Settings: `prefs.ASRAPIs`, the cloud services set up (`qwen-cloud`, `openai`), each
  with its address and model ("" for the service's) and whether a key is kept; the key is in the
  credential store as `asr-key:` and the service's ID, as Qwen's was, so moving `prefs.ASRCloud`
  into the list as settings load needs no key moved. The engines page lists Qwen and OpenAI on
  the cloud side; OpenAI's dialog takes an address and a model name for a compatible service.

Step 6, OpenAI Realtime, done 2026-10-31 (the quick set on the M5, 4 cases at once):

| | Errors % | Missed % | Shown p50 / p90 | Translation seen p50 / p90 | chrF |
|---|---|---|---|---|---|
| Realtime, the model's default delay | 13.1 | 5.9 | 1.49 / 2.18 s | 1.82 / 2.99 s | 23.8 |
| Realtime, `delay: low` | 14.1 | 6.1 | 1.24 / 1.94 s | 1.81 / 3.09 s | 23.1 |
| Realtime, `delay: high` | 14.7 | 7.2 | 1.81 / 4.79 s | 2.12 / 5.19 s | 23.5 |
| Transcriptions (step 5) | 11.2 | 6.0 | 5.47 / 9.98 s | 5.19 / 9.59 s | 25.2 |
| Qwen3-ASR | 13.9 | 5.1 | 1.63 / 6.12 s | 2.61 / 6.17 s | 24.4 |
| SenseVoice | 15.0 | 9.0 | 1.25 / 4.33 s | 1.67 / 3.49 s | 24.1 |

- `asr/openai`'s `Realtime`: a WebSocket to `{base}/v1/realtime?intent=transcription`, configured
  with `session.update` (`type: transcription`, PCM at 24 kHz, `gpt-live-transcribe`, the
  language in `languages`, `turn_detection: null`) and used once `session.updated` says so. Read
  2026-10-31, and tried: the service takes 24 kHz at least (16 kHz was refused, and a refused
  configuration leaves the session on its defaults, hearing turns itself and transcribing
  nothing: its error ends ours); `gpt-live-transcribe` takes no turn detection of the service's.
- Turns are the engine's voice detection's, as the batch runner's stretches (minimum 0.25 s, 0.4 s
  before, a pause of 0.5 s, or the first pause once the model ended a sentence or past 8 s); only
  they are sent, resampled to 24 kHz, and each is committed at its pause.
- Its text streams as it hears, word by word, about a second behind the audio: volatile results,
  whose sentence ends hold (`VolatileSentences`), so sentences commit before their turn ends. A
  turn's final text comes about half a second after its commit; completions may come out of
  order, and are given in the order of the commits, by item ID.
- The fastest recognizer to a translation at p90 (3.0 s), and more accurate than Qwen3-ASR and
  SenseVoice in English, Korean and Chinese; behind them in Japanese (25 %).
- `delay` is left to the model: `high` was later and less accurate, the turns being committed at
  the speaker's pauses anyway; `low` no quicker to a translation.
- A connection that ends drops: the turns it owed final text have their streamed text, and the
  next turn connects again. A turn's final text 2 s late is given up the same way: the turns after
  it wait for it, for their order, and 10 s, at first, held every subtitle after a final text lost.
- Where the network is slow, nothing makes live subtitles of it: the session says so instead.
  Tried on Windows through a proxy that lost connections (a request 20 s, a TLS handshake 7.5 s),
  final texts came 10 s late and more, and sending audio stalled 5 s.
- Every cloud recognizer is told slow the same way. Each measures what it can of its network and
  sends it as an event `lag` (`batch.Took`, `batch.Stalled`): OpenAI's live transcription, how
  long a turn's final text took after its commit (0.5 s on a good connection); its transcription
  API, each decode (1–1.8 s for 2–5 s of audio), as the batch runner times a `Remote` decoder;
  Qwen cloud, WebSocket pings every 3 s (0.37 s at the median, 2.6 s at worst), as its final
  results come at its silences, 1.4 s after a sentence at the median and 7.4 s under music on a
  good connection; and each, sending audio that stalled. `asr` wraps every cloud recognizer
  (`watched`): once three of its last five measures took 4 s or more, or sending stalled 4 s, the
  session's subtitles say the network is unstable, to check it or the proxy, naming no service;
  the notice goes once its last three took under 2 s. A recognizer added measures, and needs no
  more.
- One OpenAI in the settings: its model empty, or `gpt-live-transcribe`, is live transcription;
  another, `gpt-transcribe` or a compatible service's, goes to the transcription API (step 5),
  more accurate, its text later. The settings show that under the model field, and choose
  nothing for the user.

### Hints

The scene's glossary corrects names after recognition, and gives them to translation
([scenes](scenes.md)). Recognition is told them too, as a session starts: `asr.Env.Scene`, the
names as said (the user's own first) and the scene's description. There is no setting for it:
each adapter gives its model what it takes, and those that take nothing hear without.

| Adapter | Told |
|---|---|
| OpenAI, live transcription | `keywords`, the names (no `<`, `>` or line breaks, which it refuses); `prompt`, the description |
| OpenAI, transcription API | `keywords` with `gpt-transcribe`; another model has the names before its `prompt`, as `whisper-1` takes keywords |
| Qwen cloud | `vocabulary`, the names at weight 4 of 5; `input.context`, the description and the names, 400 characters at most |
| Qwen3-ASR | the names in a system message, 300 characters at most; not the description: in the reader's language, Chinese for Japanese speech, it had the model answer in it ("アシナ" as "阿信") |
| Apple, SenseVoice | nothing: Apple's SpeechTranscriber ignores `contextualStrings` (tried again 2026-10-31: the same transcript to the letter), SenseVoice has no hot words |

Measured 2026-10-31 on the Sekiro case, with a scene of seven names, two of them said (一心, 葦名):
OpenAI's live transcription wrote アシュナ without, 葦名 with; its transcription API アシナ, then
葦名; Qwen cloud had both right without; Qwen3-ASR wrote アシナ either way. No engine wrote a
name that was not said.

A session takes the scene as it starts: switching to a scene of other names starts a new one for
a recognizer that takes hints (`Spec.Hints`); a name learned meanwhile is heard from the next.

### Adapters

| Adapter | Family | Notes |
|---|---|---|
| `apple` | Stream | In the engine. `asr/apple`'s `Hint` tells the session when to have the engine `finalize`: once the hypothesis has stood 1.3 s, and in Chinese, whose clauses it ends late, at 0.2 s pauses too (what the segmenter's `VoiceCuts` did) |
| `qwen-cloud` | Stream | `cloudasr` moved to `asr/qwencloud`; the hang-up on silence moves to `asr` for every billed adapter |
| `sensevoice-small` | Batch | `tomoru-asr serve`; Timed; Volatile 0.4 s; Gap and Longest by measurement (now 0.5 s, 12 s; 0.25 s before the engine's detector, see local-asr.md) |
| `qwen3-asr-1.7b` | Batch | llama-server with the model's audio projector, chat API with the stretch as `input_audio` WAV; the reply is `language Japanese<asr_text>…`; the language is forced by an assistant prefill `language Japanese<asr_text>`; the scene's names as a system message; untimed. GPU only: on the CPU it falls behind |
| `openai` | Stream | Realtime transcription by default: a WebSocket session, 24 kHz, turns by the engine's voice detection, each committed; `…transcription.delta` as volatile, `…completed` as final, in the commit order of items, as completions may come out of order; billed |
| `openai` with another model | Batch | `POST {base}/v1/audio/transcriptions`, multipart WAV; `model`, `language`, `prompt` (the stretch before); Volatile 0 (each decode costs money and a round trip); Gap 0.5 s, Longest 5 s, for latency |

### Two llama-servers

`models.Server` runs one model; a session with local recognition and local translation needs two.
The app keeps a server per role, each on its own port, started and warmed (one request with a
second of silence for recognition) when chosen. Its files come from `ggml-org/Qwen3-ASR-1.7B-GGUF`,
by that name on both Hugging Face and ModelScope, the same SHA-256 on both and as the files measured
above (`Qwen3-ASR-1.7B-Q8_0.gguf` 2065 MB, `mmproj-Qwen3-ASR-1.7B-Q8_0.gguf` 339 MB). `models.Model`
gets `Runtime` (`sherpa` or `llama`) and its measured `Needs`: 3.1 GB of memory on a Mac, 2.6 GB of VRAM on Windows for
Qwen3-ASR 1.7B Q8_0, which the engines page already shows for translation models. Both local
models together leave little of 8 GB of VRAM to a game; the page says so.

### Settings

- `prefs.ASR` stays the chosen recognizer's ID.
- APIs the user sets up for recognition are kept as translation's are (`prefs.ASRAPIs`, keys in
  `mygo.Secrets`), with what they are (`qwen-cloud`, `openai`),
  base URL and model. Qwen's cloud joined them: `prefs.ASRCloud` is moved into the list when
  settings are read, and the old field goes (step 5).
- The default stays SenseVoice (small, any computer). Where Qwen3-ASR's `Needs` are met (a GPU with
  the VRAM, or a Mac with the memory), the engines page recommends Qwen3-ASR.
- On the engines page, the cloud side lists Qwen and OpenAI, OpenAI's at an address of the user's
  for a compatible service; "Add API…", for more than one compatible service, waits until someone
  needs it. The local side lists SenseVoice and Qwen3-ASR 1.7B with what it needs.

## Affected code

- New: `app/internal/asr/` (interfaces, entries, hang-up on silence, slow network), `asr/apple`,
  `asr/batch`, `asr/qwencloud` (from `cloudasr`), `asr/llama`, `asr/openai`, `asr/sensevoice`.
- Changed: `live` (takes `asr.Recognizer`; finalize only for Apple's, in the engine), `segment` (the rules above), `provider`
  (recognition construction goes to `asr`), `core/catalog.go` and `core/session.go`, `models`
  (Runtime, Qwen3-ASR's entry, a server per role), `prefs` (recognition APIs), settings' engines
  page, `cmd/tomoru-bench/providers.go`.
- `engine/asr`: `serve` instead of streaming; `docs/engine-protocol.md` and
  `protocol/fixtures.json` for `decode`/`decoded`.
- `app/layers_test.go`: `asr` below `live` and `core`, above `engine`; nothing imports UI.
- Docs: this file; local-asr.md points here for the streaming it no longer does; architecture.md's
  table.

## Edge cases

- Decoding slower than speech: volatile decodes are skipped; if finals fall behind too, the session
  says so (as a recognizer error), and the engines page recommends a lighter model.
- llama-server crashes or fails to start: the session stops with the error, like `tomoru-asr` now.
- Music and sound effects recognized as words (the cloud's 「Do」, 「多河」 in the Sekiro case):
  dropped when they have no letters of the language; the rest the benchmark's Noise chars/min shows.
- A stretch with nothing recognized: no result.
- The cloud: no key or a refused key stops the session with an error the engines page explains;
  a slow network shows a notice (above). A lost connection ends Qwen's session with its error;
  OpenAI's Realtime drops it and connects again for the next turn.
- Changing recognizer, language or source mid-session: the session restarts, as now.
- Replays: the runner keeps to the audio clock, so it replays the same way with a fake decoder;
  real decoders on a GPU may differ by a word between runs.

## Test plan

- `asr/batch` with a fake decoder: stretches by voice events, `Gap`, `Longest` ending at the next
  pause, agreement and never shrinking, early sentences (timed and untimed), skipping under load,
  stopping mid-decode.
- `segment`: complete sentences commit, tails join the next final, `TailWait`, the safety cut;
  the current tests that describe removed rules go with them.
- Protocol: `decode`/`decoded` in `protocol/fixtures.json`, read by the Go and `engine/asr` sides.
- Adapters: OpenAI transcriptions against an `httptest` server; Realtime against a fake WebSocket,
  completions out of order; Qwen3-ASR's reply parsing and prefill; the hang-up on silence moved,
  with its tests.
- Benchmark, before and after each step: error rate, Shown and Final p50, Rewrite %, Long %,
  Fragment %, Noise, on the 29 cases plus dialogue cases. A local case kind for recordings that
  aren't on YouTube (the Sekiro recording stays out of the repository), with a reference written
  by hand.
- By hand, on Windows: Sekiro in windowed mode with SenseVoice, Qwen3-ASR and the clouds, and the
  overlay looked at on screen.

## Steps

Each step lands on its own and is measured before the next.

1. **Base**: `asr` package and interface; `cloudasr` moved; entries and catalog from `asr.Specs`;
   `Finalize` still called. Behavior unchanged; the benchmark equal.
2. **Batch runner**: `asr/batch`, `tomoru-asr serve`, SenseVoice through the runner with today's
   numbers (Gap 0.25 s, Longest 12 s). The benchmark equal. Done 2026-10-31: the engine's detector
   on macOS cut more often, so Gap became 0.5 s; on the quick set English and Chinese are equal,
   Japanese and Korean 1.5 and 3.9 points worse, left to step 3.
3. **Sentences by the recognizer**: the new segmenter rules and early sentences; Gap, `TailWait`
   and the services' end of utterance chosen by measurement, dialogue cases included. Old rules
   deleted.
4. **Qwen3-ASR 1.7B**: `asr/llama`, a server per role, the model's entry, Needs, the engines page.
   Measured on both machines.
5. **OpenAI transcriptions**: `asr/openai`, recognition APIs in settings, Qwen's cloud migrated into them.
6. **OpenAI Realtime**.

## Decisions

Taken 2026-10-31, on the questions this design left open:

1. Qwen's cloud settings join the recognition APIs, its key migrated (step 5).
2. SenseVoice stays the default; the engines page recommends Qwen3-ASR where a GPU can run it.
3. Qwen3-ASR downloads from `ggml-org/Qwen3-ASR-1.7B-GGUF`, on Hugging Face and ModelScope alike.
4. Apple's recognizer is not like the others: the handling that speeds its Chinese up stays, inside
   its adapter, not in the segmenter.
5. The safety cut for overlong sentences stays, for now.
