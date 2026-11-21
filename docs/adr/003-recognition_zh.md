# ADR 003：识别

[English](003-recognition.md)

日期：2026-11-21。状态：采用。

## 背景

这些决策原先写在代码注释里，直到代码不再保留注释。它们涉及各识别适配器（`asr/apple`、`asr/batch`、`asr/llama`、`asr/openai`、`asr/qwencloud`），只记录[识别](../recognition_zh.md)、[本地识别](../local-asr_zh.md)和[引擎协议](../engine-protocol_zh.md)没有写到的部分。

## 决策

1. `still`（`app/internal/asr/apple/apple.go`）：假设 1.3 秒不变即视为停顿，因为说话期间 SpeechTranscriber 的假设大约每秒才变一次；等得更短会在说话中途 finalize。
2. `clausePause`（`apple.go`）：按 0.2 秒的人声停顿 finalize 只用于中文。中文因此保住了词，但英文会丢词（"Today we are …"），日文会把一句问候拆成两半，韩文会在切点处乱字（评测，2026-10-20，`bench/README.md`）。
3. `behind`（`app/internal/asr/batch/batch.go`）：有 3 个输入在等待即视为解码落后；在追上之前跳过 volatile 解码。
4. `Runner.put`（`batch.go`）：输入队列（256 个输入，约 25 秒音频）满时不等待，而是停掉解码器，runner 以 `ErrBehind` 结束：等待会让引擎的事件和字幕停住，持续整个解码的时间，解码卡死则永远停住。
5. `forced`（`batch.go`）：`Stop` 等待 runner 结束 `finishing`（5 秒）再加 1 秒，然后停掉解码器、结束进行中的解码，使卡死的 llama-server 不会让会话或正在退出的应用永远等下去。
6. `Runner.early`（`batch.go`）把提前定稿切在句末标点与下一片之间的中点：一片的时间约在其中间，切在那里会丢掉下一个词的开头。
7. `tokensPerSecond` 和 `tokensMore`（`app/internal/asr/llama/llama.go`）：Qwen3-ASR 每秒音频 12 个 token 再加 16 的上限，是对照真实语音定的：一段英文和中文访谈的 1,400 次解码中，语音每秒 3.6 个 token，第 99 百分位为 7。达到这个长度的回答是模型自说自话，例如在欢呼声上（3 秒欢呼写出 250 次 "Congratulations!"）。
8. `Decoder.ask`（`llama.go`）总是发送 system 消息，没有上下文时为空：没有它，模型更常把噪声当成语音。
9. `TestErrors`（`app/internal/asr/openai/openai_test.go`）接受测得为 0 的往返时间：Windows 的时钟精度粗，本地往返可能读成 0。
10. `Realtime.volatile`（`app/internal/asr/openai/realtime.go`）只发送到第一个已提交但文本未到的轮次为止的流式文本。短轮次的文本会晚到一秒，下一轮从 volatile 文本提交的句子曾排到它前面（《只狼》的「狼よ、」出现在它开头的那句之后）。
11. `MaxVocabulary` 和 `weight`（`app/internal/asr/qwencloud/qwencloud.go`）：热词最多 100 个（服务允许 2000 个），权重为其 1 到 5 级中的 4（50 用于至多 50 个词的列表）。
12. `loud`（`qwencloud.go`）：保持连接的"有声"是峰值达到 32768 中的 100，约 -50 dBFS。暂停的视频或没有播放时是数字静音 0，而 -50 dBFS 比任何音乐的安静段落都轻。
13. `Session.run`（`qwencloud.go`）在会话开始时就发起第一次连接，而不是等到第一次人声，让被拒绝的密钥在开始翻译时就显示出来。

## 影响与取舍

跟不上的批量解码器会带着原因结束会话而不是让它冻结，卡死的解码器随 `Stop` 结束。在 Apple 识别下，中文比其他语言提交得早。Qwen 云端从每个会话开始就连接并计费，直到第一次因静音挂断。

## 证据

- `app/internal/asr/apple/apple_test.go`：`TestStill`、`TestClausePause`。
- `app/internal/asr/batch/batch_test.go`：`TestBehind`、`TestStopStuck`、`TestEarly`。
- `app/internal/asr/llama/llama_test.go`：`TestBound`。
- `app/internal/asr/openai/realtime_test.go`：`TestRealtimeOrder`。
- `app/internal/asr/qwencloud/qwencloud_test.go`：`TestIdleSound`、`TestAuth`。
