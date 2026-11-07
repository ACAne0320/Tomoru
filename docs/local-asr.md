# Speech recognition: local models and the cloud

[中文](local-asr_zh.md)

Goal: recognition can also use models downloaded to the computer, with one implementation shared
by all platforms, or a cloud recognizer. On macOS the default is the system's recognition (Apple's
SpeechTranscriber, in the engine); Windows has no system recognition, so it uses a local model or
the cloud.

## Decisions

| Item | Decision |
|---|---|
| Runtime | sherpa-onnx (C API + Go bindings, pinned at v1.13.8). It also supports Qwen3-ASR, Fun-ASR-Nano, Whisper and more; after evaluation only SenseVoice is used (see "Other models" below) |
| Process | Recognition runs in its own process, `tomoru-asr` (Go + cgo), started and stopped by the app like llama-server: a crash doesn't take the UI down, stopping gives the memory back to the system, and the app itself needs no cgo |
| Models | Only SenseVoice Small for now, downloaded when the user asks for it, not bundled with the app (230 MB, which every update would download again, and which users of cloud recognition never need) |

## Architecture

```
platform engine tomoru-engine        app: asr/batch (since 2026-10-31)          tomoru-asr (all platforms)
  listen --asr none: hearing only      stretches by the engine's voice  ── decode ──►  SenseVoice
  ── audio + voice ──────────────────► simulated streaming decoding   ◄─ decoded ──
                                       ── result / voice / clock ──► segmenter
```

- Platform engines take `--asr none`: no recognition, only `audio` events, with silence filled
  in when nothing plays, so audio time stays continuous, and `voice` events of their Silero VAD.
- The app's batch runner (`app/internal/asr/batch`) cuts speech into stretches by those `voice`
  events and has `tomoru-asr serve` decode them; it reports in the engine protocol's own events
  (`result`, `voice`, `clock`), so the segmenter, translation and replay tests need no change.
  Until 2026-10-31 tomoru-asr did all of it itself, with a VAD of its own; it moved to the app so
  that other models share it ([recognition](recognition.md)).
- The Windows engine (`engine/windows`) implements only hearing and `--asr none`, and reuses
  `tomoru-asr` for recognition; a Linux engine would do the same.

## How a local model streams

SenseVoice is an offline model, so streaming is simulated with "VAD segments + periodic
re-decoding", in the app's batch runner:

- The engine's Silero VAD decides whether someone is speaking, in 32 ms windows. A stretch starts
  once voice has lasted 0.25 s (shorter is a click or a cough); an engine that sends no `voice`
  within its first second detects none, and all it hears is one stretch, made final every 12 s.
- While speech goes on, the current segment (with 0.4 s more before it, as VAD starts late) is
  decoded again every 0.4 s, as the volatile result. Only what two consecutive decodes agree on
  from the start is shown, and what's shown never shrinks: with a little more audio, earlier words
  get read as something else and back again ("과학 소설 속", "과학 소 속"), and readers would see
  words they've read change. Benchmark (`bench/`, 29 cases): rewrites went from 24% to 2% (Apple:
  5%), the error rate stayed the same, and words show about 0.5 s later. Keeping the agreed part
  and showing the newest after it as usual only got to 19%; hiding the last word as well didn't
  help.
- When VAD ends the segment (0.25 s of silence; at 0.35 s the pauses between TTS sentences didn't
  split, and a sentence waited for the next to finish), it's decoded once more in full, as the
  final result, with each character's time. Since the batch runner took the engine's detector
  (2026-10-31) it is 0.5 s: on macOS the engine's Silero v6 (Core ML) hears pauses under music that
  tomoru-asr's `silero_vad.onnx` didn't, and at 0.25 s a variety show was cut into 28 stretches of
  2 s where it had been 5, losing words at the cuts (Korean 18.5 → 26.0 % errors on the quick
  set). 0.5 s gave back English and Chinese and most of Korean (22.4 %); 0.8 s did no better.
- A segment is at most 12 s; beyond that it's finalized, but the last character is left out and
  decoded again with the next segment, so a word isn't cut in half.
- The app's `finalize` is ignored: the segmenter takes volatile results that stop changing for a
  pause, but volatile results lag the sound. Finalizing then cut into the next sentence's first
  words and lost them (reproduced three times out of three with Korean TTS). VAD finalizes at each
  pause itself, more precisely.
- The period at the end of volatile results is removed: SenseVoice adds one to every decode, and
  the segmenter would commit each volatile result as a sentence. Since 2026-10-31 it is left out
  of final results too, where the stretch ends (a pause mid-sentence ended lines with it); the
  segmenter commits such an unfinished tail after 0.6 s without voice, or joins it to the next.
- A sentence that two volatile decodes agree SenseVoice ended, with speech after it, is made final
  at once, and the stretch goes on from between its full stop and the next word (2026-10-31): long
  stretches don't hold translation back ([recognition](recognition.md)).
- Extra spaces between characters are removed for Japanese and Chinese (SenseVoice inserts spaces
  in Japanese).

Measured (M5, 2 CPU threads): 8 s of audio decodes in about 120 ms; the CoreML execution provider
wasn't faster, so the CPU is used.

## Models

| ID | Model | Files | Size | Sources |
|---|---|---|---|---|
| `sensevoice-small` | SenseVoice Small int8 (`2024-07-17`) | `model.int8.onnx`, `tokens.txt` | 228 MB | Hugging Face `csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17` (sherpa-onnx's own); ModelScope `pengzhendong/sherpa-onnx-sense-voice-zh-en-ja-ko-yue`, with the same SHA-256 for every file |

- ⚠️ The `int8-2025-09-09` version can't be used: with sherpa-onnx 1.13.8 every result is labeled
  Cantonese, and Japanese and Korean come out garbled.
- A model is a list of files, each checked against its SHA-256 and resumable. Recognition models
  go in their own `models/<id>/` directory.
- Silero VAD (`silero_vad.onnx`, MIT, 630 KB) isn't downloaded by users: it ships with the app in
  `vad/`, beside the Mac engine's Core ML version.

## Packaging

- `scripts/build-asr.sh` puts `tomoru-asr` and sherpa-onnx's two dynamic libraries
  (`libsherpa-onnx-c-api`, `libonnxruntime`) in `app/resources/<platform>/bin/`, with the rpath set
  to `@executable_path`.
- A self-signed certificate has no Team ID, so `tomoru-asr`, like llama-server, needs
  `disable-library-validation` (`app/entitlements/bundled-libraries.plist`).
- The Windows build is cross-compiled by `scripts/build-engine-windows.sh`, with the DLLs beside
  `tomoru-asr.exe`.

## Cloud recognition

Ruled out when choosing: `qwen-audio-3.1-asr-flash` over HTTP (0.9–11.7 s of latency, and only
final results, unfit for live use); `paraformer-realtime-8k-v2` (usable for Chinese only).

- Qwen ASR's streaming models on Alibaba Cloud Model Studio: `qwen-audio-3.1-asr-flash-streaming`
  (default) or `3.0`, over WebSocket (`/api-ws/v1/inference`: run-task → binary PCM →
  result-generated → finish-task). With the user's own API key, kept in the system keychain; the
  host can be a workspace's own domain.
- Implemented in Go in the app (`internal/asr/qwencloud`), with no tomoru-asr and no cgo. The engine
  only hears, with `--asr none`; the app sends the audio straight to the service, and the engine's
  `voice` events too. Intermediate results (`sentence_end: false`) map to volatile results, final
  ones (with word times) to final results, so the segmenter needs no change.
- `max_sentence_silence` is 300 ms (at the default 1300 ms, 12 s of TTS was finalized only once,
  at the end). 200 ms and `multi_threshold_mode_enabled` didn't split finer: TTS pauses between
  sentences are shorter. So sentences are mostly committed on periods in intermediate results;
  with TTS the first sentence commits about 2 s after it's spoken; real speech remains to be
  measured.
- English words from the service carry their space sometimes before (" chat"), sometimes after
  ("chat. "). A trailing space is moved to the front of the next word, or the segmenter would
  commit the last word twice.
- `finalize` is ignored, as by the batch runner.
- Silence is billed too: with 15 s of silence followed by 12 s of speech, `usage.duration`
  reported 25 and 27 s, the whole task's audio. So the engine runs Silero VAD and sends `voice`
  even with `--asr none`, and after 10 s of silence (no voice, and the audio itself nearly zero)
  the connection is closed; on voice or sound it reconnects, first sending the 0.5 s of audio
  before the voice (VAD starts late, and `voice` events arrive after the audio), keeping only the
  last 2 s while disconnected. Measured (Japanese TTS with a 12 s gap): disconnected after 10 s of
  silence, reconnected as the second part began, which showed about 0.4 s later than the first.
  Watching only for voice hurt music: on a Billie Eilish song in the benchmark, voice detection
  never found a voice from start to end, hung up after 10 s, and the error rate was 98.5%; with
  "don't hang up while there's sound", 57%.
- Volatile results are "stable text + a preview at the end", and the preview often guesses wildly,
  with periods of its own ("잘 놀다.다는 게." became "잘 놀다 간다는 게 회사" the next time). The
  segmenter commits on a period, so it committed the wild preview, and then the same speech again
  once the service corrected it. So a period in a volatile result is kept only if it and the
  character after it were already the same in the previous volatile result, and in English and
  Korean only if a space follows; Han characters in Korean and English previews are removed too
  (each sentence's preview often began with "不不" or "没差"). Benchmark (`bench/`, 29 cases): error
  rate 16.9% → 13.6%, Korean 20.9% → 13.0%, chrF 31.7 → 35.7; translations showed about 0.3–0.5 s
  later for English, Japanese and Korean. Chinese doesn't have this problem and keeps its periods.
- Each connection first sends 0.2 s of silence: otherwise, when speech starts right away, the
  service drops the first syllable (Japanese 「皆さん」 → 「なさん」; fine with 0.1 s of silence
  before).
- Known: 3.0 once got a whole Japanese passage wrong.

## Other models (evaluated 2026-10-23)

Qwen3-ASR 1.7B has been used since 2026-10-31, on llama-server with a GPU: on a game's dialogue it
punctuated and named far better than SenseVoice ([recognition](recognition.md)).

Only SenseVoice: Macs use the system's recognition by default, and SenseVoice is mainly for
Windows and Linux.

The evaluation used 8 benchmark cases (Chinese, English, Japanese, Korean; a talk and a podcast or
variety show each), the first 120 s of each. Offline: audio fed to the recognizer as fast as
possible, accuracy only, not speed. Live: `tomoru-bench -j 2`, on an M5. GPU: llama.cpp b9840 (Metal
on the Mac, Vulkan on Windows), on the segments SenseVoice's VAD cut (not widened: widening by
0.2–0.3 s made neighboring segments each recognize the words on the boundary).

| Model | Offline error rate zh / en / ja / ko | Mean | Live (CPU) | Per segment (GPU) | Download |
|---|---|---|---|---|---|
| SenseVoice Small int8 | 15.9 / 16.7 / 16.0 / 13.7 | 15.6% | Keeps up: shown in 1.15 s, 24 CPU s/min | — | 239 MB |
| Qwen3-ASR 1.7B Q8_0 (GGUF) | 13.9 / 13.0 / 16.8 / 13.3 | 14.2% | — | M5 median 450 ms; RTX 5060 300–390 ms per 10 s | 2.4 GB |
| Qwen3-ASR 0.6B Q8_0 (GGUF) | 14.5 / 12.5 / 19.8 / 24.5 | 17.8% | — | M5 median 180 ms; RTX 5060 150–220 ms | 970 MB |
| Qwen3-ASR 0.6B int8 (sherpa-onnx) | 15.6 / 13.9 / 20.6 / 15.7 | 16.4% | Falls behind: shown in 14 s, 130 CPU s/min | — | 983 MB |
| Fun-ASR-Nano-2512 int8 | 16.3 / 15.0 / 19.1 / unsupported | — | Falls behind: shown in 6.4 s | — | 994 MB |
| Whisper large-v3-turbo int8 | 30.4 / 14.6 / 20.0 / 32.0 | 24.2% | Falls behind: shown in 50 s | — | 1036 MB |
| Dolphin small int8 | 28.9 / unsupported / 31.9 / 49.3 | — | Keeps up: shown in 1.2 s | — | 250 MB |

- The references are human subtitles, not verbatim transcripts, so podcasts are 20–30% off to
  begin with; with only 8 cases, differences of 1–2 points aren't reliable.
- Models with an LLM decoder (Qwen3-ASR, Fun-ASR, Whisper) can't keep up live on the CPU; nearly
  all their errors in the live test are words missed.
- Only Qwen3-ASR 1.7B beats SenseVoice, by 1.4 points on average, mostly on Korean and Japanese
  talks; but it needs a GPU and 2.4 GB, gives only text, without times or volatile results, and
  would compete with local translation models for the GPU. Not enough gain, so not now.
- Fun-ASR-Nano makes things up for Korean (error rate 290–460%). Dolphin doesn't do English.

## Later

- With a second local recognition model, mark one "recommended" by memory, cores and GPU.
- Test hotwords (Qwen3-ASR and Fun-ASR have them, SenseVoice doesn't) on names.
