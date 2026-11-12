# Engine protocol (v1)

[中文](engine-protocol_zh.md)

The engine is the platform-specific half of Tomoru: it hears the computer's audio and recognizes
speech. Everything after recognition (segmenting, translating, showing) is the app's, in Go, on
every platform. The app runs the engine as a child process and talks to it in JSON lines.

- **stdout** carries the protocol only: one JSON object per line, flushed per line.
- **stderr** is free-form logging. The app copies it to its log file and never parses it.
- **stdin** carries commands from the app, one JSON object per line. When stdin closes, the
  engine exits, so it never outlives the app, even if the app crashes.
- Every object has a `type`. Unknown types and unknown fields are ignored by both sides, so either
  side can add fields without breaking the other.

The macOS engine is `engine/mac` (Swift: Core Audio tap + SpeechAnalyzer), built as
`tomoru-engine` into `app/resources/darwin-arm64/bin/`. The Windows engine is `engine/windows`
(Go, its calls into Windows in C: WASAPI's endpoint and process loopback, SAPI, and a window of its own to select
a part of the screen), built as `tomoru-engine.exe` into `app/resources/windows-amd64/bin/` by
`scripts/build-engine-windows.sh`. Windows has no speech recognition for apps, so it only hears:
`listen` needs `--asr none`, and `prepare` fails with `unsupported-locale`.

Recognition with a downloaded model is not the platform's: the app cuts the audio a platform
engine hears with `listen --asr none` into stretches of speech by the engine's `voice` events, and
`tomoru-asr` (`engine/asr`, Go + sherpa-onnx, the same on every platform) decodes each (see
"tomoru-asr" below, and [recognition](recognition.md)). The app reports what it recognizes in the
same events as an engine that recognizes itself. The cloud's recognition (`app/internal/asr/qwencloud`, Qwen ASR on
Alibaba Cloud Model Studio) takes the same `audio` in the app, and reports in the same events.

## Commands

```
tomoru-engine listen --locale <bcp47> [--app <id> | --file <audio>] [--vad <model> | none] [--asr none]
                                                     recognize until stdin closes; with --asr none, only hear
tomoru-engine locales                                the locales recognition hears
tomoru-engine prepare --locale <bcp47>               get recognition for a language ready
tomoru-engine speak --locale <bcp47> -- <sentence>…  say sentences aloud with the system's voice
tomoru-engine access                                 may the engine hear the system?
tomoru-engine request-access                         have the system ask the user
tomoru-engine screen-access                          may the app capture the screen (Screen Recording)?
tomoru-engine request-screen-access                  have the system ask the user for it
tomoru-engine apps                                   the apps that can be heard on their own
tomoru-engine select --out <png>                     have the user select a part of the screen
```

`--app` hears one app only; its ID is the platform's: a bundle ID on macOS, the program's file name
on Windows (`chrome.exe`), whose process tree is heard, helpers included. An app that isn't
running yet is heard once it starts. Without it the engine hears everything the system plays
but itself. `--file` hears a recording instead, played at its own pace and then silence, as if the
system played it: benchmarks (`bench/`) and tests use it, with no speakers and no permission.

Every command first prints `hello`. A command that fails prints `error` and exits with a
non-zero status. One that succeeds exits with 0.

## Engine → app

### `hello`

```json
{"type":"hello","protocol":1,"engine":"mac","version":"0.1.0"}
```

This is the first line of every command. The app refuses a `protocol` it doesn't know.
Empty stdout is also an invalid handshake. Malformed JSON or an `error` event terminates the
reader and the child, so a failed protocol stream cannot leave a process blocked on stdout.
The app first closes stdin when stopping a child, then kills it if it has not exited after one
second. A blocked audio write does not prevent this shutdown.

### `state`

```json
{"type":"state","state":"downloading"}
```

Sent by `listen`, `prepare` and `speak` as they go:

- `downloading`: recognition assets for the language are being fetched. A new language took
  13–21 s on macOS, and no progress was reported.
- `ready`: recognition is ready (`prepare` ends here).
- `listening`: capture has started, and results will follow.
- `speaking`: `speak` has started saying its sentences, one after another with a pause between
  them, through the default output, where `listen` hears them as it hears a video. It exits once
  they are said. The app's first-run guide tries translating with them.

### `result`

A recognition result: the hypothesis for the audio from `start` to `end`.

```json
{"type":"result","final":false,"start":12.30,"end":13.10,
 "tokens":[{"text":"今日は","start":12.30,"end":12.80},{"text":"いい天気","start":12.80,"end":13.10,"conf":0.91}]}
```

- `final: false`: a **volatile** result. It is the live hypothesis since the last final one, and
  it replaces the previous volatile result entirely. A later result may revise it.
- `final: true`: the tokens are fixed and will not change. The next volatile result starts after
  them.
- **The text** is the tokens' `text` concatenated, including their spaces and punctuation.
- **Token times**: `start` and `end` may be missing on volatile results, when the recognizer has
  not timed the words yet.
- **`conf`** is the recognizer's confidence from 0 to 1, where it gives one. On macOS that is
  final results only.

### `clock`

```json
{"type":"clock","t":13.42}
```

This is the audio clock, sent every 100 ms while listening. The segmenter decides pauses by it,
so it never reads the wall clock and replays deterministically.

### Times

All times are seconds of audio since `listening`, with millisecond precision. When capture stops
delivering buffers in silence, the clock goes on with wall time, so that pauses still pass.

### `voice`

```json
{"type":"voice","on":false,"t":7.456}
```

The speaker's voice started (`on: true`) or stopped at `t`, by voice activity detection (Silero VAD:
on macOS a Core ML model the app ships in `vad/` beside the engine's `bin/`, on Windows the ONNX
model there, through sherpa-onnx). The detector decides in steps of 32 ms: voice starts after 64 ms
over its threshold, and stops after 96 ms under it, at the start of those 96 ms. An engine without a detector sends none (`listen --vad none` turns it off).
With `listen --asr none` it runs on the audio sent, and sends `{"on":false,"t":0}` first, to say that
it is on: the app's cloud recognizer hangs up while nobody speaks, and a local model's stretches of
speech are cut by it.

### `audio`

```json
{"type":"audio","t":12.300,"pcm":"AAD//wEA…"}
```

Sent by `listen --asr none` instead of `result` and `clock`: what the engine hears, as
16 kHz mono 16-bit little-endian PCM in base64, about every 100 ms. `t` is the time of its first
sample. The audio has no gaps: while capture delivers nothing, the engine sends silence, so that
time goes on as the audio does.

### `error`

```json
{"type":"error","code":"no-permission","message":"System Audio Recording is off for Tomoru"}
```

`message` is for the log, and the app shows its own text for each `code`:

| code | meaning |
|---|---|
| `no-permission` | the system doesn't let the engine hear its audio |
| `unsupported-locale` | recognition doesn't support the language |
| `assets-failed` | the recognition assets didn't download |
| `capture-failed` | capture stopped, or didn't start |
| `no-voice` | the system has no voice to say the language (`speak`) |
| `model-failed` | the recognition model could not be loaded (`tomoru-asr`) |
| `internal` | anything else |

After an error, `listen` exits, so a capture or recognizer failure ends the run. The app decides
whether to start again.

### Answers of the one-shot commands

```json
{"type":"access","state":"granted"}
{"type":"app","id":"com.apple.Safari","name":"Safari","playing":true}
{"type":"locale","id":"ja-JP"}
```

- `access` and `request-access` answer with one `access` object. Its `state` is `granted`,
  `denied`, or `unknown` (the user hasn't been asked yet). `screen-access` and
  `request-screen-access` answer the same way for Screen Recording, which selecting a part of the
  screen to translate takes; the system counts the engine's permissions as the app's.
- `locales` sends one `locale` object per locale the system's recognition hears (macOS:
  `SpeechTranscriber.supportedLocales`), by BCP 47 tag, then exits; the app offers those languages
  to speak. Apple's `zh-HK` is Cantonese. `supportedLocale(equivalentTo:)` is no test of a locale:
  it gave `th-TH`, which is not among them. The Windows engine has no recognition, and the app
  doesn't ask it.
- `apps` sends one `app` object per app whose sound can be heard on its own: the running apps with
  a Dock icon (on Windows, a window on the taskbar), and any app playing sound (`playing`), those
  playing first. Then it exits.
- Windows asks no permission to hear what plays or to capture the screen: the access commands
  answer `granted`.
- `select` has the user select a part of the screen, which it writes to `--out` as PNG, sends where
  it is, and exits with 0: `{"type":"area","x":1298,"y":654,"width":328,"height":106}`, in pixels:
  a display's points (DIPs) times its scale, from the top-left of the main display, so `x` and `y`
  may be negative. The app shows the translation next to it. Escape cancels: it writes and sends
  nothing. On Windows the engine shows a picture of every screen, dimmed, over them, on which the
  user drags (the right button cancels too), and lets the app take the keyboard as it ends. On
  macOS it runs the system's `screencapture -i`, which does not tell where the part was: the engine
  watches the left button and the pointer while it runs. A click, which picks a window there
  (after Space), sends no area: the translation shows under the menu bar icon.

## App → engine

### `finalize`

```json
{"type":"finalize","through":13.10}
```

This asks the recognizer to make everything up to `through` final now, rather than after its own
long pause. The app sends it to the macOS engine for Apple's recognition only (`app/internal/asr/apple`),
and only inside a pause: when the hypothesis has stopped changing, and in Chinese at the speaker's
pauses. Forcing a final mid-speech makes the recognizer end the utterance there, which inserts
punctuation and drops or garbles words.

An engine whose recognizer can't do this ignores it.

## tomoru-asr

```
tomoru-asr serve --model <dir> --lang <zh|en|ja|ko|yue> [--threads <n>]   decode stretches of speech
```

`<dir>` is a downloaded model (SenseVoice: `model.int8.onnx` and `tokens.txt`). The app cuts speech
into stretches and asks for each to be decoded as it grows and once more when it ends
(`app/internal/asr/batch`, [recognition](recognition.md)); tomoru-asr only decodes.

- **stdin** carries `decode` commands, a stretch each, as 16 kHz mono 16-bit little-endian PCM in
  base64; other commands are ignored. When stdin closes, it exits.

  ```json
  {"type":"decode","id":"7","pcm":"AAD//wEA/38="}
  ```

- **stdout** carries `hello` (`"engine":"asr"`), then a `decoded` for each `decode`, in order, with
  the same `id`: the stretch's tokens, each with its time from the stretch's start, and no `end`.
  The spaces SenseVoice puts between Chinese, Cantonese and Japanese words are gone.

  ```json
  {"type":"decoded","id":"7","tokens":[{"text":"Hello","start":0},{"text":".","start":0.48}]}
  ```

- Its `error` codes are `model-failed` and `internal` (invalid JSON, malformed PCM or an input read
  error); each ends it.

## Replay

An engine's stdout from a `listen` run is a complete record of what the app saw: results and
clock, in order. Saved to a file, it replays through the segmenter in Go tests without audio or an
engine. That is how segmenter changes are regression-tested (`app/internal/segment/testdata/`).

## Versioning

`protocol` goes up only for changes an older app would misread. Adding a field or a type doesn't
change it.

## Contract checks

`protocol/fixtures.json` is the shared v1 contract: valid producer records, handshake failures,
optional token times, empty final results, audio clocks, voice-off at time zero, forward-compatible
fields/events, error termination, and stdin commands. Both Go modules load it in their protocol
tests. The ASR tests also verify EOF flushing and malformed input; the app tests start actual
child processes to verify graceful EOF, forced shutdown, blocked writes and protocol failure.

The Windows engine's tests (`engine/windows`, which run on any computer) check its producers and
command reader against the same fixtures.

The Swift verifier compiles the production `Protocol.swift` with
`engine/mac/Tests/ProtocolContract.swift`. It checks the typed producer constructors against
the same fixtures and exercises stdin EOF through the production command decoder. It needs
Foundation only, without audio permissions or downloaded models. The repository check command
runs this verifier and builds the full macOS engine, which checks all protocol call sites.

Resource discovery is outside this protocol client. Desktop and benchmark entry points supply
explicit resource roots; the client and contract checks have no dependency on the UI framework.
