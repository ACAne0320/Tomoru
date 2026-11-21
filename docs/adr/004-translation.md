# ADR 004: Translation

[中文](004-translation_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
translation engines offered (`core/cloud.go`) and how `translate` asks them, and record only what
[scenes](../scenes.md) and [image translation](../image-translation.md) leave out.

## Decision

1. `CloudSizes` (`app/internal/core/cloud.go`): the Index-Translate cloud offers 2B and 9B beside
   the default 35B-A3B, because, measured, they translated closer to the human subtitles and sooner
   (`bench/README.md`).
2. `MTServices` (`cloud.go`): TokenHub's Hy-MT2 defaults to Plus 7B, as good as Pro 30B-A3B,
   quicker and seldom refused; Pro takes 60 requests a minute. Lite 1.8B is offered too.
3. `Index` and `HyMT` (`app/internal/translate/translate.go`) decode greedily (temperature 0), as
   their authors advise; general chat models (`Chat`) keep 0.2.
4. `hymtPrompt` (`translate.go`) doesn't ask Hy-MT2 for colloquial subtitles, as Index-Translate is
   asked: asked, the 7B put 呗 and 啦 in lectures, and chrF on the full set fell from 40.5 to 38.1
   (1.8B) and from 40.4 to 36.1 (7B).
5. `Endpoint.request` (`translate.go`) sends llama.cpp's own parameters, `cache_prompt` and
   `chat_template_kwargs`, only to the app's llama-server (`Endpoint.Llama`): other servers may
   refuse them, and the Index cloud, on vLLM, answers 400 to `cache_prompt` (2026-11-10).
6. `Endpoint.request` asks other chat models not to think in each vendor's form at once:
   `thinking: {type: disabled}` (DeepSeek, GLM, Doubao), `reasoning_effort: none` (DeepSeek,
   OpenAI, Gemini), `chat_template_kwargs.enable_thinking` (vLLM, SGLang) and `enable_thinking`
   (Qwen's cloud). A line's translation would wait for the thinking and is no better for it.
7. `Endpoint.body` (`translate.go`) sets `max_tokens` to 128 plus the line's length in runes:
   enough for a long line's translation, and bounded by the line, so that a model repeating itself
   stops.
8. `unfinished` (`translate.go`) ends a translation the token limit cut with "…", so that it
   doesn't read as finished; its start is still worth showing.
9. `Cloud` (`translate.go`): the Index-Translate cloud asks again after 2 s without first words
   (`FirstWait`), gives up after a 2 s gap or 8 s in all, runs 3 at once and warms a connection
   idle for 30 s (`WarmIdle`). Measured 2026-10-26: first words in 0.5 s typically, 1.2 s at most
   with 4 requests at a time, or now and then (half the requests, for some hours) after 6 s if at
   all, once after 30 s; a whole line within 1.5 s; it closes a connection unused for 40 to 60 s,
   and a new one takes 0.4 to 0.5 s more.
10. `Endpoint.do` (`translate.go`) sends `User-Agent: Index-Translate-Client/1.0` with every
    request: the public endpoint's firewall wants it.
11. `ErrEmpty` (`translate.go`): a reply with no text is an error, not an empty translation. The
    Index-Translate cloud answered so while it was down (2026-10-20), and an empty line looked
    translated.
12. `LimitedPause` (`translate.go`): after a service says too many requests (429), drafts pause
    for a minute (`ErrDraftPaused`) and the requests left are the lines'. A draft can go without,
    and Hy-MT2 Pro on Tencent Cloud takes 60 a minute, which a fast talk's drafts alone go past.
13. `imageTimeout` (`app/internal/translate/image.go`): a picture's request has 2 minutes in all,
    unless `Endpoint.Client` sets its own: started a minute late, a long translation streams in for
    half a minute more.
14. The handlers of `TestImageSlow` (`image_test.go`) and `TestFirstWait` (`translate_test.go`) read
    the request body first: Go's HTTP server notices the client hanging up only once the body has
    been read, and without it the tests hang.

## Consequences

A rate-limited service keeps translating lines but shows no drafts for a minute. Each new chat
API gets every vendor's thinking switch; one that refuses them costs a single retry, as
[scenes](../scenes.md) describes. A cut translation shows as unfinished rather than missing.

## Evidence

- `app/internal/translate/translate_test.go`: `TestNoThinking`, `TestTokenLimit`, `TestLimited`,
  `TestFirstWait`, `TestGap`, `TestWarm`, `TestCloud`, `TestNoTranslation`, `TestEndpoint`.
- `app/internal/translate/image_test.go`: `TestImageSlow`.
- `app/internal/core/engines_test.go`: `TestCloudSizes`.
