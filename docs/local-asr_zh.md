# 语音识别：本地模型和云端

[English](local-asr.md)

目标：识别也能用下载到本机的模型，各平台共用一份实现；也能用云端识别。macOS 默认用系统识别（Apple SpeechTranscriber，在引擎里）；Windows 没有系统识别，用本地模型或云端。

## 设计决定

| 项 | 决定 |
|---|---|
| 运行时 | sherpa-onnx（C API + Go 绑定，锁定 v1.13.8）。它还支持 Qwen3-ASR、Fun-ASR-Nano、Whisper 等，评测后只用 SenseVoice（见下文“其他模型”） |
| 进程 | 识别放在独立进程 `tomoru-asr`（Go + cgo），由 app 启动和停止，和 llama-server 一样：崩溃不影响界面，停止后内存还给系统，app 本身不需要 cgo |
| 模型 | 目前只有 SenseVoice Small，用户选它时才下载，不随 app 打包（230 MB，每次更新都要重新下载，选云端识别的用户也用不上） |

## 架构

```
平台引擎 tomoru-engine             app：asr/batch（2026-10-31 起）          tomoru-asr（各平台共用）
  listen --asr none：只收听          按引擎的人声切段          ── decode ──►   SenseVoice
  ── audio + voice ────────────────► 模拟流式解码              ◄─ decoded ──
                                     ── result / voice / clock ──► 断句器
```

- 平台引擎加 `--asr none`：不做识别，只发 `audio` 事件，没有声音时补静音，保证音频时间连续；另外发它的 Silero VAD 的 `voice` 事件。
- app 的整段识别运行器（`app/internal/asr/batch`）按这些 `voice` 事件把语音切成段，交给 `tomoru-asr serve` 解码；它用引擎协议自己的事件（`result`、`voice`、`clock`）报告，所以断句器、翻译、回放测试都不用改。2026-10-31 之前这些都由 tomoru-asr 自己做，用它自己的 VAD；挪到 app 是为了让其他模型共用（[识别适配层](recognition_zh.md)）。
- Windows 引擎（`engine/windows`）只实现收听和 `--asr none`，识别直接复用 `tomoru-asr`；以后的 Linux 引擎也一样。

## 本地模型的流式做法

SenseVoice 是离线模型，所以用"VAD 切段 + 周期重解码"模拟流式：

- 引擎的 Silero VAD 判断有没有人说话，每 32 ms 一个窗口。人声持续 0.25 s 才开始一段（更短的是咔哒声或咳嗽）；引擎在第一秒内没发任何 `voice`，就当它没有检测器，听到的全部算一段，每 12 s 定稿一次。
- 说话期间，每 0.4 s 把当前这段（往前多带 0.4 s，VAD 的起点偏晚）重新解码一次，作为临时结果。只显示连续两次解码从开头起一致的部分，而且显示的部分不往回缩：多听一点音频，前面的词会被重新读成别的又读回来（“과학 소설 속”“과학 소 속”），读者会看到已经读过的字在变。评测（`bench/`，29 段）：改写从 24% 降到 2%（Apple 5%），错误率不变，字晚约 0.5 s 出现。保留一致的部分、后面照常显示最新的，只降到 19%；再藏起最后一个词也没用。
- VAD 判定这段结束（静音 0.25 s；0.35 s 时 TTS 的句间停顿切不开，前一句要等后一句说完）后，再完整解码一次，作为定稿结果，带每个字的时间。运行器改用引擎的检测器后（2026-10-31）改为 0.5 s：macOS 引擎的 Silero v6（Core ML）能在音乐里听出 tomoru-asr 的 `silero_vad.onnx` 听不出的停顿，0.25 s 时一段综艺被切成 28 段、每段 2 s（原来是 5 段），切口处丢字（快速集上韩语错误率 18.5 → 26.0%）。0.5 s 让英语、中文回到原来的水平，韩语回来大半（22.4%）；0.8 s 没有更好。
- 一段最长 12 s，超过就强制定稿，但留下最后一个字，并到下一段里重新解码，避免把词从中间切开。
- 忽略 app 发来的 `finalize`：断句器在临时结果停止变化时以为是停顿，但临时结果比声音慢一拍。实测在这时定稿，会切进下一句的头几个字，把它们丢掉（韩语 TTS 三次都复现了）。VAD 自己在每个停顿处定稿，更准。
- 临时结果去掉末尾的句号：SenseVoice 每次解码都会在末尾补一个句号，断句器会把每段临时结果都当成一句话提交。2026-10-31 起定稿也去掉这段末尾的句号（句中的停顿曾因此结束一行）；断句器在 0.6 s 没有人声后提交这种没说完的尾巴，或把它接到下一段。
- 两次临时解码一致认为 SenseVoice 已经结束、后面还有内容的句子，立刻定稿，这段从它的句号和下一个字之间继续（2026-10-31）：长段不会拖住翻译（[识别适配层](recognition_zh.md)）。
- 日语、中文去掉字与字之间多余的空格（SenseVoice 在日语里会插空格）。

实测（M5，2 线程 CPU）：8 s 音频解码约 120 ms；CoreML 执行器没有更快，所以用 CPU。

## 模型

| ID | 模型 | 文件 | 大小 | 来源 |
|---|---|---|---|---|
| `sensevoice-small` | SenseVoice Small int8（`2024-07-17`） | `model.int8.onnx`、`tokens.txt` | 228 MB | Hugging Face `csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17`（sherpa-onnx 官方）；ModelScope `pengzhendong/sherpa-onnx-sense-voice-zh-en-ja-ko-yue`，两边文件的 SHA-256 一致 |

- ⚠️ 不能用 `int8-2025-09-09` 版：配 sherpa-onnx 1.13.8 时所有结果都会标成粤语，日语、韩语输出乱码。
- 模型清单从"一个模型一个文件"改成"一个模型多个文件"，每个文件单独校验 SHA-256、断点续传。识别模型放在 `models/<id>/` 子目录里。
- Silero VAD（`silero_vad.onnx`，MIT，630 KB）不需要用户下载，随 app 一起打包，放在 `vad/` 目录下，和 Mac 引擎的 Core ML 版放在一起。

## 打包

- `scripts/build-asr.sh` 把 `tomoru-asr` 和 sherpa-onnx 的两个动态库（`libsherpa-onnx-c-api`、`libonnxruntime`）放进 `app/resources/<平台>/bin/`。rpath 改成 `@executable_path`。
- 自签名证书没有 Team ID，所以 `tomoru-asr` 和 llama-server 一样需要 `disable-library-validation`（`app/entitlements/bundled-libraries.plist`）。
- Windows 版由 `scripts/build-engine-windows.sh` 交叉编译，DLL 放在 `tomoru-asr.exe` 旁边。

## 云端识别

选型时排除了：`qwen-audio-3.1-asr-flash` 的 HTTP 接口（延迟 0.9–11.7 s，且只返回最终结果，不适合实时）；`paraformer-realtime-8k-v2`（只有中文能用）。

- 用阿里云百炼的 Qwen ASR 流式模型：`qwen-audio-3.1-asr-flash-streaming`（默认）或 `3.0`，WebSocket（`/api-ws/v1/inference`，run-task → 二进制 PCM → result-generated → finish-task）。用户自己的 API key，存在系统钥匙串里；服务地址可以填业务空间域名。
- 在 app 里用 Go 实现（`internal/asr/qwencloud`），不需要 tomoru-asr，也不需要 cgo。引擎用 `--asr none` 只收听，app 把音频直接发给服务，引擎的 `voice` 也交给它。中间结果（`sentence_end: false`）对应临时结果，定稿（带逐词时间）对应定稿结果，断句器不用改。
- `max_sentence_silence` 设成 300 ms（默认 1300 ms 时，12 s 的 TTS 只在最后定稿一次）。200 ms 和 `multi_threshold_mode_enabled` 都没有切得更细：TTS 句间停顿更短。所以句子主要靠中间结果里出现的句号提交，TTS 上第一句约在说完后 2 s 提交，真人语音待测。
- 服务端的英文词，空格有时在前（" chat"）有时在后（"chat. "）。词尾的空格会挪到下一个词前面，否则断句器会把最后一个词重复提交一次。
- 和整段识别运行器一样忽略 `finalize`。
- 静音也计费：先发 15 s 静音再说 12 s 话，`usage.duration` 报的是 25、27 s，是整个任务的音频时长。所以引擎在 `--asr none` 时也跑 Silero VAD、发 `voice`，静音满 10 s（没人声，音频也几乎为零）就断开连接；有人声或有声音时重新连接，先补发人声前 0.5 s 的音频（VAD 起点偏晚，`voice` 事件也比音频晚到），断开期间的音频只缓存最近 2 s。实测（日语 TTS，中间空 12 s）：静音 10 s 后断开，第二段开口时重连，比第一段晚约 0.4 s 上屏。只看人声检测会误伤音乐：评测里 Billie Eilish 的歌，人声检测从头到尾没判出人声，10 s 后挂断，错误率 98.5%；加上“有声音就不挂”后 57%。
- 临时结果是“已经稳定的文字 + 末尾一段预览”，预览常常乱猜，还自带句号（“잘 놀다.다는 게.”下一次变成“잘 놀다 간다는 게 회사”）。断句器见句号就提交，提交了乱猜的预览，服务端改正后同一段话又提交一次。所以临时结果里的句号，只有它和后面一个字在上一次临时结果里已经一样时才保留，英语、韩语还要求句号后面是空格；韩语、英语预览里的汉字也去掉（每句开头的预览常是“不不”“没差”）。评测（`bench/`，29 段）：错误率 16.9% → 13.6%，韩语 20.9% → 13.0%，chrF 31.7 → 35.7；英日韩看到译文晚了约 0.3–0.5 s。中文没有这个问题，照旧保留句号。
- 每次连接先发 0.2 s 静音：否则一开头就有人声时，服务端会丢第一个音节（日语「皆さん」→「なさん」；前面有 0.1 s 静音就正常）。
- 已知：3.0 偶发过一次日语全部识别错。

## 其他模型（2026-10-23 评测）

Qwen3-ASR 1.7B 从 2026-10-31 起采用，由 llama-server 在 GPU 上运行：在游戏对白上，它的标点和名字都比 SenseVoice 好得多（[识别适配层](recognition_zh.md)）。

只用 SenseVoice：Mac 上默认用系统识别，SenseVoice 主要给 Windows / Linux 用。

评测用 `bench/` 的 8 段（中英日韩，每种语言一段演讲、一段播客或综艺），每段取前 120 s。离线：音频尽快送进识别，只看准确率，不看快慢。实时：`tomoru-bench -j 2`，M5。GPU：llama.cpp b9840（Mac 上用 Metal，Windows 上用 Vulkan），用 SenseVoice 的 VAD 切出的段（不往外扩边界，扩 0.2–0.3 s 时相邻两段会把边界上的词各认一次）。

| 模型 | 离线错误率 中 / 英 / 日 / 韩 | 平均 | 实时（CPU） | 每段耗时（GPU） | 下载 |
|---|---|---|---|---|---|
| SenseVoice Small int8 | 15.9 / 16.7 / 16.0 / 13.7 | 15.6% | 跟得上，上屏 1.15 s，24 CPU 秒/分 | — | 239 MB |
| Qwen3-ASR 1.7B Q8_0（GGUF） | 13.9 / 13.0 / 16.8 / 13.3 | 14.2% | — | M5 中位 450 ms；RTX 5060 10 s 一段 300–390 ms | 2.4 GB |
| Qwen3-ASR 0.6B Q8_0（GGUF） | 14.5 / 12.5 / 19.8 / 24.5 | 17.8% | — | M5 中位 180 ms；RTX 5060 150–220 ms | 970 MB |
| Qwen3-ASR 0.6B int8（sherpa-onnx） | 15.6 / 13.9 / 20.6 / 15.7 | 16.4% | 跟不上：上屏 14 s，130 CPU 秒/分 | — | 983 MB |
| Fun-ASR-Nano-2512 int8 | 16.3 / 15.0 / 19.1 / 不支持 | — | 跟不上：上屏 6.4 s | — | 994 MB |
| Whisper large-v3-turbo int8 | 30.4 / 14.6 / 20.0 / 32.0 | 24.2% | 跟不上：上屏 50 s | — | 1036 MB |
| Dolphin small int8 | 28.9 / 不支持 / 31.9 / 49.3 | — | 跟得上，上屏 1.2 s | — | 250 MB |

- 参考答案是人工字幕，不是逐字稿，播客类本身就差 20–30%；只有 8 段，1–2 个点的差别不可靠。
- 带 LLM 解码器的模型（Qwen3-ASR、Fun-ASR、Whisper）在 CPU 上跟不上实时，实时测里的错误几乎都是漏掉的。
- 只有 Qwen3-ASR 1.7B 比 SenseVoice 好，平均低 1.4 个点，主要在韩语和日语演讲，但要 GPU，2.4 GB，只出文字，没有时间，也没有临时结果，还要和本地翻译模型抢 GPU。收益不够，先不做。
- Fun-ASR-Nano 认韩语时会胡编（错误率 290–460%）。Dolphin 不认英语。

## 以后

- 有了第二个本地识别模型时，再按内存、核数、显卡打“推荐”标签。
- 测一下热词（Qwen3-ASR、Fun-ASR 有，SenseVoice 没有）对人名的效果。
