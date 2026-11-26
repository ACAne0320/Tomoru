# ADR 003: Recognition

[中文](003-recognition_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
recognition adapters (`asr/apple`, `asr/batch`, `asr/llama`, `asr/openai`, `asr/qwencloud`), and
record only what [recognition](../recognition.md), [local ASR](../local-asr.md) and the
[engine protocol](../engine-protocol.md) leave out.

## Decision

1. `still` (`app/internal/asr/apple/apple.go`): a hypothesis unchanged for 1.3 s is a pause,
   because SpeechTranscriber's hypothesis changes only about once a second while speech goes on;
   a shorter wait would finalize mid-speech.
2. `clausePause` (`apple.go`): finalizing at 0.2 s voice pauses is for Chinese only. It kept
   Chinese words, but in English it dropped some ("Today we are …"), in Japanese it moved a
   greeting's halves apart, and in Korean it garbled words at the cuts (the benchmark, 2026-10-20,
   `bench/README.md`).
3. `behind` (`app/internal/asr/batch/batch.go`): decoding lags when 3 inputs are waiting; volatile
   decodes are skipped until it catches up.
4. `Runner.put` (`batch.go`): when the input queue (256 inputs, about 25 s of audio) is full, it
   doesn't wait. It stops the decoder and the runner ends with `ErrBehind`: waiting would hold the
   engine's events, and the subtitles, as long as the decode, and forever for one that hangs.
5. `forced` (`batch.go`): `Stop` waits `finishing` (5 s) plus 1 s for the runner to end, then
   stops the decoder, ending the decode in progress, so a llama-server that hangs can't hold the
   session, or the app quitting, forever.
6. `Runner.early` (`batch.go`) cuts an early final halfway between the sentence's end mark and the
   next piece: a piece's time is about its middle, and cutting there lost the start of the next
   word.
7. `tokensPerSecond` and `tokensMore` (`app/internal/asr/llama/llama.go`): Qwen3-ASR's limit of 12
   tokens a second of audio plus 16 is measured against speech, which took 3.6 tokens a second, 7
   at the 99th percentile, over 1,400 decodes of an interview in English and Chinese. An answer
   that long is the model going on by itself, as over cheering ("Congratulations!" 250 times for
   3 s).
8. `Decoder.ask` (`llama.go`) always sends a system message, empty with no context: without one
   the model took noise for speech more often. It also asks for `reasoning_format: none`: the
   chat template of the Qwen3-ASR files uploaded on 2026-11-21 ends reasoning at `<asr_text>`, and
   llama-server then put `language Japanese` in `reasoning_content`, which left every answer
   without its language and so empty. With the flag, old and new files gave the same text on 20
   stretches in four languages, told the language and Auto-detect alike.
9. `TestErrors` (`app/internal/asr/openai/openai_test.go`) accepts a measured round trip of 0:
   Windows' clock is coarse enough for a local one to read as 0.
10. `Realtime.volatile` (`app/internal/asr/openai/realtime.go`) sends streamed text only up to the
    first committed turn whose text hasn't come. A short turn's text comes a second late, and the
    next turn's sentence, committed from its volatile text, went before it (Sekiro's 「狼よ、」
    after the line it starts).
11. `MaxVocabulary` and `weight` (`app/internal/asr/qwencloud/qwencloud.go`): at most 100 hot
    words, though the service takes 2000, at weight 4 on its scale of 1 to 5 (50 is for lists of
    50 words at most).
12. `loud` (`qwencloud.go`): sound that keeps the connection is a peak of 100 of 32768, about
    -50 dBFS. A paused video or nothing playing is digital silence, 0, and -50 dBFS is quieter than
    any music's quiet passage.
13. `Session.run` (`qwencloud.go`) makes the first call as the session starts, not at the first
    voice, so a refused key shows as translating starts.

## Consequences

A batch decoder that can't keep up ends the session with a reason instead of freezing it, and a
hung one ends with `Stop`. Chinese on Apple's recognition commits sooner than other languages.
Qwen cloud connects, and bills, from the start of each session until it first hangs up on silence.

## Evidence

- `app/internal/asr/apple/apple_test.go`: `TestStill`, `TestClausePause`.
- `app/internal/asr/batch/batch_test.go`: `TestBehind`, `TestStopStuck`, `TestEarly`.
- `app/internal/asr/llama/llama_test.go`: `TestBound`, `TestDecode`.
- `app/internal/asr/openai/realtime_test.go`: `TestRealtimeOrder`.
- `app/internal/asr/qwencloud/qwencloud_test.go`: `TestIdleSound`, `TestAuth`.
