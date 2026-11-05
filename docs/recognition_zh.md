# 识别适配层

[English](recognition.md)

状态：2026-10-31 设计，待定问题已于当天决定。六步都已完成，每步都用快速集实测过：第 1 步（底座）、第 2 步（整段运行器）、第 3 步（由识别引擎断句）、第 4 步（Qwen3-ASR）、第 5 步（OpenAI 转写）和第 6 步（OpenAI Realtime）；“现在的做法”描述的是它们之前的代码。以 [local-asr_zh.md](local-asr_zh.md) 为基础，那里记录了现在的识别做法和原因。

## 目标

以后接入一个识别引擎，只需要写一个适配器、加一条目录项，会话、断句器和界面都不用改。在这个底座上接入本地 Qwen3-ASR 1.7B、OpenAI 的 Realtime 转写和 `/v1/audio/transcriptions` 接口（以及兼容它的服务），并且由识别引擎决定句子在哪里结束，不再用我们自己的规则。

## 为什么

在 Windows 上录的一段只狼对白（157 s，狼和九郎；2026-10-31）暴露了现在的识别在游戏语音上的问题：

- SenseVoice 听错名字和古风说法（葦名 → 葦シ名，葦の葉笛 → 足の歯ブエ，还凭空多出「ぎ院？」），句子也切错了地方：结束一段的 0.25 s 停顿是在视频上调出来的，视频里的人说话几乎不停；而游戏角色会在一句话中间停顿，句与句之间反而停得很短。`その後大事はないかその / 後とはあの / 世のことを覚えておらぬのかはい。`
- Qwen 云端（`qwen-audio-3.1-asr-flash-streaming`）一句一条，标点正确，名字也对：它自己断的句比我们的规则好。
- llama.cpp 上的 Qwen3-ASR 1.7B Q8_0 标点和云端一样好，名字错得多一些（アシナ、一神様）。bf16 结果相同，不是量化的问题。

| Qwen3-ASR 1.7B Q8_0 | M5，24 GB | RTX 5060，8 GB |
|---|---|---|
| 内存 | 常驻 2.9–3.1 GB（统一内存） | 显存多占 2.5 GB（只狼运行时共 6.6 / 8 GB） |
| 解码 | 每 10 s 语音 0.7 s | 预热后每 10 s 0.3 s；前两次请求 3–5 s |

## 需求

1. 所有识别引擎共用一个接口，分两类（见下），由适配器实现。
2. "把语音切成段、逐段解码"只实现一份，所有整段解码的引擎共用：SenseVoice、Qwen3-ASR、OpenAI 转写接口。
3. 句子在识别引擎结束它的地方结束（它的标点、它判断的一句话说完）。断句器不再保留我们自己的规则，只留一个防止句子过长的保险。
4. 新的识别引擎：Qwen3-ASR 1.7B（本地，llama.cpp，需要 GPU）；OpenAI Realtime 转写；OpenAI `/v1/audio/transcriptions`，可填自己的 base URL，以支持兼容服务。
5. 引擎目录和设置页从适配器的条目生成识别引擎列表，就像翻译列出用户添加的 API 一样。
6. 不能悄悄变差：每一步都用评测和现在的表现对比，包括对白类的用例。

## 不做

- 独占全屏游戏：字幕层盖不上去，是另一个问题。
- 插件系统：适配器编译进程序，和引擎的决定一致（local-asr_zh.md）。
- 下文列出的之外，不改翻译、字幕层和采集引擎。

## 现在的做法

```
引擎 listen ──result/clock/voice────────────────────────────► live.Session ─► segment ─► translate
            （Apple：识别在引擎里）
引擎 listen --asr none ──audio/voice──► live.Recognizer ──result/clock/voice──┘
                                         ├ tomoru-asr（SenseVoice；自带 VAD 和流式）
                                         └ cloudasr（Qwen，WebSocket）
```

- `live.Recognizer`（`app/internal/live/live.go`）有 `Audio`、`Finalize`、`Stop`、`Done`、`Err`；识别器报告的是 `engine.Event`，即引擎协议（`docs/engine-protocol_zh.md`），所以断句器对所有识别器一视同仁。这部分好用，保留。
- `provider.Recognizer`（`app/internal/provider/provider.go`）按字符串分支：`apple`、`local`、`qwen-cloud`。`core.engineCatalog`（`app/internal/core/catalog.go`）把列表写死：macOS 上的 Apple、`models.All` 里的识别模型、`QwenCloud`。Qwen 的设置是单独的结构 `prefs.ASRCloud`。
- `tomoru-asr`（`engine/asr/stream.go`）自己做人声检测（sherpa-onnx 跑 Silero，静音 0.25 s 结束一段），每 0.4 s 重新解码正在变长的这段作为临时结果，只显示两次解码一致的部分且不往回缩，12 s 强制定稿。
- `segment`（`app/internal/segment/`）大部分句子边界由自己决定：定稿字之后的停顿（`FinalPause` 0.6 s）、临时结果不变的停顿（`VolatilePause` 1.3 s）、中文按人声停顿切分句（`VoiceCuts`）、超过长度时在分句标点和语言的连接处切开，并在停顿处要求识别器 `finalize`。
- `models.Server`（`app/internal/models/server.go`）一次只用一个模型跑一个 llama-server。

## 设计

### 两类识别引擎

| 类别 | 谁来断句 | 识别引擎 |
|---|---|---|
| 流式 | 服务边听边断：先给临时结果，在它判断一句话说完时给定稿 | Apple（在引擎里）、Qwen 云端、OpenAI Realtime |
| 整段 | app 按人声检测把语音切成段，模型给每段加标点 | SenseVoice、Qwen3-ASR（llama.cpp）、OpenAI 转写 |

流式适配器每个服务写一个。整段识别只需要实现 `Decoder`（给一段音频，返回文本），由共用的 `batch.Runner` 变成识别器。接入新模型变简单就在这里：大多数新模型都是整段解码器。

### 接口

放在新包 `app/internal/asr`；`live` 改为导入 `asr`，不再自己声明。

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

识别器还接收引擎的 `voice` 事件（`Voice(engine.Event)`）：会话转给有这个方法的识别器，三个都有。语言随工厂函数给出，要留意的内容在 `Env.Scene` 里（见“提示”）。

`Finalize` 已从接口里去掉：断句器不再要求定稿，识别器自己决定结果在哪里定稿。Apple 识别在 macOS 引擎里运行时，会话仍按 `Spec.Hint` 向引擎发 `finalize`（见下）。

### 条目

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

`Env` 是 app 提供的东西：tomoru-asr 的路径、模型文件、识别用的 llama-server、所选云端服务及其地址、模型和密钥（`Service`），以及场景（`Scene`）。`core.engineCatalog` 列出 `asr.Specs`；`provider` 不再构造识别器。

### 整段识别的运行器

`app/internal/asr/batch` 把 `Decoder` 变成 `Recognizer`。它就是现在 `tomoru-asr` 的流式做法（local-asr_zh.md），挪进 app 并做成通用的：

- **切段**：用引擎发来的 `voice` 事件（两个平台都是 Silero），不再自带检测。一段从人声开始前 0.4 s 开始（检测偏晚），没有人声持续 `Gap` 后结束；超过 `Longest` 后，在引擎检测到的下一个停顿处结束，停顿再短也算。
- **临时结果**：这段变长期间，每隔 `Volatile` 解码一次；只显示两次解码一致的部分，不往回缩，去掉模型每次都补的句号（都和现在一样）。
- **定稿**：这段结束时再解码一次。没有时间的字按字数在这段里均匀分配时间，用于延迟统计。
- **提前出句**：模型已经结束的一句，后面还有文字，并且两次解码一致，就立刻定稿（发 `final: true` 的 `result`），翻译不用等一长段说完。有时间的解码器丢掉这句的音频，从它的结尾往后解码；没有时间的，在下一个停顿处结束这段，以后的解码就不必在文本里找这句。
- **负载**：同时只有一个解码在跑。解码跟不上时跳过临时解码，不排队；定稿解码一定执行。
- **噪声**：模型标为没有语言的定稿（Qwen3-ASR 的 `language None`）丢掉，只有标点的文字也丢掉。别的文字照样保留：日语、韩语对白里有真的英文词（"OK"、"Yes"），要丢掉它们得先做评测。

`tomoru-asr` 变成解码器：`tomoru-asr serve --model <dir>` 读 `{"type":"decode","id":7,"lang":"ja","final":true,"pcm":"…"}`，回 `{"type":"decoded","id":7,"tokens":[…]}`。它的人声检测和流式逻辑去掉；sherpa-onnx 保留，用于 SenseVoice，以及 Windows 引擎里的人声检测。

### 断句

断句器改为跟着识别引擎走：

1. 定稿文本在识别引擎结束句子的地方切开（它的标点，按 `segment/sentence.go` 里各语言的规则），每个完整的句子提交。
2. 最后一个句末之后的部分：如果识别引擎的定稿表示一句话说完（`FinalsEnd`：Apple、Qwen 云端，它们在听到说话人停下时才定稿），就随定稿一起提交；否则（整段解码器的一段，可能被人声检测切在句中）留作待定，和下一次定稿拼在一起。
3. 定稿之后 `TailWait`（0.6 s，实测见下）内没有新的人声，或者会话停止，就提交待定的尾巴。
4. 临时结果只有在适配器说它的句末靠得住时才提交句子：Qwen 云端靠 `steady`（同现在），整段运行器把这种句子直接作为定稿发出（见上）。
5. 保留一个保险：句子宽度超过两行（90，中日韩字算 2，和评测的"过长 %"一致），不管是待定的还是已经结束的，都在两行以内的最后一个分句标点处切开，没有就在最后两个词之间切开，需要几次切几次。已经结束的长句来自 OpenAI：它的一段一次性定稿，一句 120 个字母整句提交（第 5 步）。

从断句器里去掉：`FinalPause`、`VolatilePause`、`ClausePause`、`MinClause`、`VoiceCuts`（挪进 Apple 适配器）、`Joint`，以及要求 `finalize`。流式适配器的"一句话说完"阈值按实测设定，而不是设成服务允许的最短（Qwen 云端的 `silence` 现在是 300 ms，是为我们的停顿规则选的）。

第 3 步完成时的实测（2026-10-31，快速集，每种语言 2 分钟；改动前 → 后）：

| | 错误率 % | 过长 % | 碎句 % | 见到译文 p50 / p90 | chrF |
|---|---|---|---|---|---|
| Apple | 16.4 → 15.8 | 1 → 1 | 10 → 8 | 1.53 → 1.24 / 2.91 → 2.58 s | 24.5 → 23.1 |
| Qwen 云端 | 16.7 → 14.3 | 1 → 1 | 8 → 4 | 2.72 → 3.16 / 5.38 → 7.43 s | 21.8 → 24.3 |
| SenseVoice | 13.1 → 15.0 | 0 → 0 | 6 → 7 | 1.86 → 1.66 / 3.57 → 3.50 s | 23.1 → 23.4 |

- SenseVoice 每次解码末尾都补句号，不管这段在哪里结束；定稿时去掉它（`batch.Caps.Stops`），否则句中的停顿会结束一行。它的错误率是第 2 步换检测器时升上去的（那时 14.7）。
- Qwen 云端改为等整句，之前每三四秒就在逗号处切开：译文更好，最慢的晚了 2 秒，这一点已接受。
- `TailWait`：1.0 s 能让慢节奏的游戏对白连成一句（只狼：「ほら、昔聞かせてくれたであろう」），但在快速集上译文 p50 晚 0.25 s，韩语最慢的晚 4 s，过长行 4%（中文 10%）：连续讲话里把前后两句拼到了一起。停顿长度分不开这两种情况（TED 演讲中位数 0.6 s，只狼 0.7 s），音源也分不开，而且不该让用户来选。所以用 0.6 s；更好的答案交给识别引擎：对标点好的解码器（Qwen3-ASR，第 4 步），语音段越过短停顿继续，由听到前后两边的模型来结束句子，提前出句保证不慢。SenseVoice 的标点撑不起这个做法（只狼那段它把几句连在一起没有标点）。

第 4 步 Qwen3-ASR 1.7B，2026-10-31 完成（快速集，一次一段：多个会话共用一块 GPU 会互相拖慢）：

| | 错误率 % | 漏字 % | 上屏 p50 | 见到译文 p50 / p90 | chrF |
|---|---|---|---|---|---|
| M5，SenseVoice（每段 120 s） | 15.0 | 9.0 | 1.25 s | 1.65 / 3.53 s | 23.4 |
| M5，Qwen3-ASR | 13.9 | 5.1 | 1.80 s | 2.71 / 6.22 s | 24.4 |
| RTX 5060，SenseVoice（每段 160 s） | 16.7 | 10.1 | 1.35 s | 1.70 / 4.27 s | 22.4 |
| RTX 5060，Qwen3-ASR | 12.7 | 4.2 | 1.46 s | 2.64 / 5.89 s | 23.4 |

- 它不给字的时间：一段进行中它结束的句子，从临时结果提交（`VolatileSentences`）；它结束一句之后，这段在说话人的下一个停顿处结束，停顿再短也算（`batch.Caps.Untimed`），这样段短、解码快。不这样做时段长到 15 s，原文晚 0.8 s 出现。
- 临时解码仍要两次一致：每次直接显示（`Steady`）时，它会给说了一半的句子加句号，这些半句被提交后又再出现一次：错误率 45%。
- 语音段越过 1 s 以内的停顿继续（`Gap`）：只狼的「ほら昔聞かせてくれたであろう」能连成一句。每 0.5 s 解码一次，超过 15 s 在下一个停顿处定稿。
- `llama-server --list-devices` 显示有 GPU 且内存够（3 GB）时，引擎页推荐它；默认仍是 SenseVoice。

第 5 步 OpenAI 转写，2026-10-31 完成（M5 上的快速集，4 段同时跑：云端服务互不影响）：

| | 错误率 % | 漏字 % | 上屏 p50 / p90 | 见到译文 p50 / p90 | chrF | 过长 % |
|---|---|---|---|---|---|---|
| Longest 8 s | 11.4 | 6.3 | 5.88 / 11.47 s | 4.65 / 10.80 s | 24.7 | 13 |
| Longest 5 s | 11.5 | 6.1 | 5.04 / 9.01 s | 4.12 / 7.79 s | 24.6 | 8 |
| Longest 3 s | 12.9 | 6.8 | 3.82 / 6.55 s | 3.18 / 5.64 s | 22.9 | 7 |
| Longest 5 s，加下面两处修正 | 11.2 | 6.0 | 5.47 / 9.98 s | 5.19 / 9.59 s | 25.2 | 0 |
| Qwen3-ASR（第 4 步） | 13.9 | 5.1 | 1.80 / 6.06 s | 2.71 / 6.22 s | 24.4 | 2 |
| Qwen 云端（第 3 步） | 14.3 | 4.7 | 1.84 / 3.70 s | 3.16 / 7.43 s | 24.3 | 1 |

- 目前最准的识别（英语 1.3%，韩语 14%，Qwen3-ASR 是 21%），也最慢：一段结束、请求往返之后文字才出来，2–5 s 的音频往返 1–1.8 s，10 s 的 2.5 s。往返时间不稳定，每次跑相差 0.5 s 左右：第四次在日语上也慢了，而两处修正碰不到日语。要低延迟得靠 Realtime（第 6 步）。
- `Longest` 取 5 s：准确率和 8 s 一样，译文 p90 早 2–3 s；3 s 丢了上下文：错误率 11.5% → 12.9%。
- 两处修正对所有识别引擎都生效：空格分词的语言，每段文字开头补一个空格（`batch.Spaced`：「…tell」和「Us to do」拼成了「tellUs」）；保险切分也切已经结束的句子（见上）。修正后 SenseVoice、Apple 和 Qwen3-ASR 的快速集结果不变，只是 Qwen3-ASR 的过长行从 2% 降到 0。

- `asr/openai`：调用 `POST {base}/v1/audio/transcriptions` 的整段解码器，把这段作为 WAV 发送，带上 `model`、`language`（ISO-639-1：繁体中文发 `zh`；粤语和菲律宾语没有两字母代码，不发，由模型自己判断）和 `prompt`（上一段末尾约 120 个字，让人名和句子前后一致）。连接或服务端失败（5xx、429）的解码重试一次；key 被拒（401）是 `openai.ErrAuth`。
- 不给时间，也不做临时解码（`Volatile: 0`，`batch` 视为不做）：每次解码按发送的音频计费。一段在 0.5 s 的停顿处结束，或超过 5 s 后的第一个停顿处结束，结束后往返一次，文字才出来。
- 2026-10-31 查阅 OpenAI 文档：转写模型有 `gpt-transcribe`（这里的默认），以及已弃用、2027-02-26 停用的 `gpt-4o-transcribe`、`gpt-4o-mini-transcribe` 和 `whisper-1`。只有 `gpt-transcribe` 接受 `languages`（多个可能的语言）和 `keywords`（预期会出现的词），以后术语表可以填它。实时转写有 `gpt-live-transcribe`（第 6 步）。
- 设置：`prefs.ASRAPIs` 是设置好的云端服务（`qwen-cloud`、`openai`），各自记地址和模型（空为服务默认）以及是否存了 key；key 在系统凭据里，名为 `asr-key:` 加服务 ID，和 Qwen 原来的一样，所以读取设置时把 `prefs.ASRCloud` 并进列表，不用挪 key。引擎页云端一侧列出 Qwen 和 OpenAI；OpenAI 的对话框可以填兼容服务的地址和模型名。

第 6 步 OpenAI Realtime，2026-10-31 完成（M5 上的快速集，4 段同时跑）：

| | 错误率 % | 漏字 % | 上屏 p50 / p90 | 见到译文 p50 / p90 | chrF |
|---|---|---|---|---|---|
| Realtime，模型默认的 delay | 13.1 | 5.9 | 1.49 / 2.18 s | 1.82 / 2.99 s | 23.8 |
| Realtime，`delay: low` | 14.1 | 6.1 | 1.24 / 1.94 s | 1.81 / 3.09 s | 23.1 |
| Realtime，`delay: high` | 14.7 | 7.2 | 1.81 / 4.79 s | 2.12 / 5.19 s | 23.5 |
| 转写接口（第 5 步） | 11.2 | 6.0 | 5.47 / 9.98 s | 5.19 / 9.59 s | 25.2 |
| Qwen3-ASR | 13.9 | 5.1 | 1.63 / 6.12 s | 2.61 / 6.17 s | 24.4 |
| SenseVoice | 15.0 | 9.0 | 1.25 / 4.33 s | 1.67 / 3.49 s | 24.1 |

- `asr/openai` 的 `Realtime`：WebSocket 连 `{base}/v1/realtime?intent=transcription`，用 `session.update` 配置（`type: transcription`、24 kHz PCM、`gpt-live-transcribe`、`languages` 里放语言、`turn_detection: null`），等 `session.updated` 确认后再用。2026-10-31 查阅文档并实际试过：服务至少要 24 kHz（16 kHz 被拒）；配置被拒时会话不会断，而是沿用默认配置，自己判断说话轮次却不转写任何东西，所以收到这个错误就结束我们的会话；`gpt-live-transcribe` 不支持服务端的轮次检测。
- 说话轮次由引擎的语音检测切分，和整段运行器的语音段一样（至少 0.25 s，带上前 0.4 s，0.5 s 的停顿结束；模型结束了一句之后或超过 8 s 后，第一个停顿就结束）；只发送这些轮次，重采样到 24 kHz，每轮在停顿处提交。
- 文字边听边出，逐词到达，比音频晚约 1 s：作为临时结果，句末可信（`VolatileSentences`），所以句子在这一轮结束前就提交。每轮的定稿在提交后约 0.5 s 到达；不同轮次的定稿可能乱序到达，按条目 ID 以提交顺序给出。
- 到译文的 p90 是所有识别里最快的（3.0 s）；英语、韩语、中文都比 Qwen3-ASR 和 SenseVoice 准，日语落后（25%）。
- `delay` 交给模型默认：`high` 更晚也更不准，因为轮次本来就在说话人停顿时提交；`low` 到译文并不更快。
- 连接断开时丢弃它：欠定稿的轮次用已流出的文字作为定稿，下一轮重新连接。一轮的定稿迟到 2 s 也这样处理：为了顺序，后面的轮次要等它，起初等 10 s，丢一个定稿就冻住后面所有字幕。
- 网络慢时，怎么优化都做不成实时字幕，所以改为提示用户。在 Windows 上经一条不稳定的代理实测（一次请求 20 s 超时，TLS 握手 7.5 s），定稿迟到 10 s 以上，发送音频卡住 5 s。
- 所有云端识别用同一套方式判断网络慢。每个适配器只测自己能测的，作为 `lag` 事件发出（`batch.Took`、`batch.Stalled`）：OpenAI 实时转写测一轮提交后多久收到定稿（正常 0.5 s）；转写接口测每次解码（2–5 s 音频 1–1.8 s），由整段运行器对 `Remote` 解码器计时；Qwen 云端每 3 s ping 一次 WebSocket（正常中位数 0.37 s，最慢 2.6 s），因为它的定稿要等它听到静音，正常网络下句末之后中位数 1.4 s，有音乐时到 7.4 s；另外都测发送音频是否卡住。`asr` 给每个云端识别包一层（`watched`）统一判断：最近五次测量有三次 ≥4 s，或发送卡住 4 s，字幕处提示网络不稳定，请检查网络或代理（不点名服务）；最近三次都在 2 s 内时提示消失。以后加新的云端识别，只要报测量值，不用再写判断。
- 设置里只有一个 OpenAI：模型留空或填 `gpt-live-transcribe` 是实时转写；填别的模型（`gpt-transcribe` 或兼容服务的模型）走转写接口（第 5 步），更准，文字出得晚。设置页在模型栏下面说明这一点，不替用户做选择。

### 提示

场景的术语表在识别之后纠正名字，并把名字交给翻译（见[场景](scenes_zh.md)）。现在会话开始时也把它们告诉识别：`asr.Env.Scene`，即名字的原文（用户自己加的在前）和场景描述。这不需要任何设置：每个适配器把它转成自己模型能接收的形式，不接收提示的照常识别。

| 适配器 | 告诉它的 |
|---|---|
| OpenAI 实时转写 | `keywords` 放名字（去掉它不接受的 `<`、`>` 和换行）；`prompt` 放场景描述 |
| OpenAI 转写接口 | `gpt-transcribe` 用 `keywords`；其他模型把名字放在 `prompt` 开头，`whisper-1` 就是这样接收关键词的 |
| Qwen 云端 | `vocabulary` 放名字，权重 4（最高 5）；`input.context` 放描述和名字，最多 400 字 |
| Qwen3-ASR | 名字放在 system 消息里，最多 300 字；不放描述：描述是用户的语言，日语语音配中文描述时，模型会用中文回答（「アシナ」写成「阿信」） |
| Apple、SenseVoice | 不告诉：Apple 的 SpeechTranscriber 忽略 `contextualStrings`（2026-10-31 又试了一次：转写结果一字不差），SenseVoice 没有热词 |

2026-10-31 在只狼那段对话上实测，场景里有七个名字，其中两个真的出现了（一心、葦名）：OpenAI 实时转写不加提示写成アシュナ，加了写成葦名；转写接口从アシナ变成葦名；Qwen 云端不加提示就都对；Qwen3-ASR 加不加都是アシナ。没有任何引擎写出没说过的名字。

会话开始时读取场景：识别引擎接收提示时（`Spec.Hints`），切换到名字不同的场景会重新开始会话；会话中途学到的名字从下一次会话开始生效。

### 适配器

| 适配器 | 类别 | 说明 |
|---|---|---|
| `apple` | 流式 | 在引擎里。`asr/apple` 的 `Hint` 告诉会话何时让引擎 `finalize`：临时结果 1.3 s 不变时；中文的分句结束得晚，在 0.2 s 的停顿处也定稿（原来断句器 `VoiceCuts` 做的事） |
| `qwen-cloud` | 流式 | `cloudasr` 挪到 `asr/qwencloud`；静音挂断挪到 `asr`，所有计费的适配器共用 |
| `sensevoice-small` | 整段 | `tomoru-asr serve`；有时间；临时解码间隔 0.4 s；Gap 和 Longest 按实测（现在 0.5 s、12 s；改用引擎的检测器前是 0.25 s，见 local-asr_zh.md） |
| `qwen3-asr-1.7b` | 整段 | llama-server 加载模型的音频投影器，用 chat 接口把这段作为 `input_audio` WAV 发过去；回复是 `language Japanese<asr_text>…`；用 assistant 预填 `language Japanese<asr_text>` 指定语言；场景里的名字放在 system 消息里；没有时间。只用 GPU：CPU 跟不上 |
| `openai` | 流式 | 默认实时转写：WebSocket 会话，24 kHz，轮次由引擎的语音检测切分，每轮提交；`…transcription.delta` 作为临时结果，`…completed` 作为定稿，按条目提交的顺序，因为定稿可能乱序到达；计费 |
| `openai` 填了别的模型 | 整段 | `POST {base}/v1/audio/transcriptions`，multipart WAV；`model`、`language`、`prompt`（上一段）；不做临时解码（每次解码都花钱且要往返）；Gap 0.5 s，Longest 5 s，降低延迟 |

### 两个 llama-server

`models.Server` 只跑一个模型；本地识别加本地翻译的会话需要两个。app 为每个用途各保留一个服务，各用一个端口，选中时启动并预热（识别用一秒静音请求一次）。模型文件来自 `ggml-org/Qwen3-ASR-1.7B-GGUF`，Hugging Face 和 ModelScope 上同名，两边的 SHA-256 一致，也和上面实测用的文件一致（`Qwen3-ASR-1.7B-Q8_0.gguf` 2065 MB，`mmproj-Qwen3-ASR-1.7B-Q8_0.gguf` 339 MB）。`models.Model` 加 `Runtime`（`sherpa` 或 `llama`）和实测的 `Needs`：Qwen3-ASR 1.7B Q8_0 在 Mac 上 3.1 GB 内存，在 Windows 上 2.6 GB 显存；引擎页已经会给翻译模型显示这些。两个本地模型一起用时，8 GB 显存留给游戏的就不多了，引擎页要说明。

### 设置

- `prefs.ASR` 仍是所选识别引擎的 ID。
- 用户为识别设置的 API 和翻译的一样保存（`prefs.ASRAPIs`，密钥在 `mygo.Secrets`），记下类型（`qwen-cloud`、`openai`）、base URL 和模型。Qwen 云端已并进来：读取设置时把 `prefs.ASRCloud` 迁进列表，旧字段删除（第 5 步）。
- 默认仍是 SenseVoice（小，任何电脑都能跑）。满足 Qwen3-ASR 的 `Needs`（有足够显存的 GPU，或内存足够的 Mac）时，引擎页推荐 Qwen3-ASR。
- 引擎页云端一侧列出 Qwen 和 OpenAI，OpenAI 可以填用户自己的地址来用兼容服务；"添加 API…"（同时用多个兼容服务）等有人需要再做。本地一侧列出 SenseVoice 和 Qwen3-ASR 1.7B，并标明需要什么。

## 涉及的代码

- 新增：`app/internal/asr/`（接口、条目、静音挂断、网络慢）、`asr/apple`、`asr/batch`、`asr/qwencloud`（来自 `cloudasr`）、`asr/llama`、`asr/openai`、`asr/sensevoice`。
- 修改：`live`（接收 `asr.Recognizer`；只对引擎里的 Apple 识别 finalize）、`segment`（上面的规则）、`provider`（识别器的构造移到 `asr`）、`core/catalog.go` 和 `core/session.go`、`models`（Runtime、Qwen3-ASR 条目、每个用途一个服务）、`prefs`（识别 API）、设置的引擎页、`cmd/tomoru-bench/providers.go`。
- `engine/asr`：用 `serve` 代替流式；`docs/engine-protocol.md` 和 `protocol/fixtures.json` 加 `decode`/`decoded`。
- `app/layers_test.go`：`asr` 在 `live` 和 `core` 之下、`engine` 之上；不导入界面。
- 文档：本文；local-asr 中它不再负责的流式部分指向这里；architecture 的目录表。

## 边界情况

- 解码比说话慢：跳过临时解码。如果一次解码迟迟不返回，后面积压了约 25 s 的音频（定稿太慢，或 llama-server、网络卡住），整段运行器就停掉解码器，会话带着 `batch.ErrBehind` 停止，提示换一个更轻的模型或检查网络。
- llama-server 崩溃或启动失败：会话带着错误停止，和现在的 `tomoru-asr` 一样。
- 音乐和音效被识别成词（只狼那段里云端的「Do」「多河」）：照样显示成字幕，只丢掉 Qwen3-ASR 的 `language None` 和只有标点的文字；有多少看评测里的"噪声字/分钟"。
- 一段什么都没识别出来：不发结果。
- 云端：没有密钥或密钥被拒时，会话带着错误停止，引擎页说明原因；网络慢时显示提示（见上）。连接断开时 Qwen 的会话带着错误结束；OpenAI 实时转写丢弃这条连接，下一轮重新连接。
- 会话中途换识别引擎、语言或音源：会话重启，和现在一样。
- 回放：运行器按音频时钟走，用假的解码器能完全复现；GPU 上的真实解码器两次运行可能差一个词。

## 测试计划

- `asr/batch` 用假解码器测：按人声事件切段、`Gap`、超过 `Longest` 后在下一个停顿结束、两次一致且不回缩、提前出句（有时间和没时间两种）、负载下跳过、解码中途停止。
- `segment`：完整句子提交、尾巴和下一次定稿拼接、`TailWait`、过长保险；描述已删规则的现有测试一并删除。
- 协议：`protocol/fixtures.json` 加 `decode`/`decoded`，Go 和 `engine/asr` 两侧都读。
- 适配器：OpenAI 转写对 `httptest` 服务测；Realtime 对假 WebSocket 测，包括 completed 乱序；Qwen3-ASR 的回复解析和预填；静音挂断挪位置后带着它的测试。
- 评测：每一步前后对比错误率、出字 p50、定稿 p50、改写 %、过长 %、碎句 %、噪声，覆盖 29 段加对白用例。加一种本地用例，用于不在 YouTube 上的录音（只狼录音不进仓库），参考文本手写。
- 手测（Windows）：只狼窗口模式下分别用 SenseVoice、Qwen3-ASR 和云端，并在屏幕上实际看字幕层。

## 步骤

每一步单独合入，测完再做下一步。

1. **底座**：`asr` 包和接口；挪 `cloudasr`；条目和目录改由 `asr.Specs` 生成；暂时仍调用 `Finalize`。行为不变，评测持平。
2. **整段运行器**：`asr/batch`、`tomoru-asr serve`，SenseVoice 走运行器，参数沿用现在的（Gap 0.25 s，Longest 12 s）。评测持平。2026-10-31 完成：macOS 引擎的检测器切得更碎，Gap 改为 0.5 s；快速集上英语、中文持平，日语、韩语差 1.5 和 3.9 个点，留给第 3 步。
3. **由识别引擎断句**：新的断句规则和提前出句；Gap、`TailWait` 和各服务的"一句话说完"阈值按实测选，包括对白用例。删除旧规则。
4. **Qwen3-ASR 1.7B**：`asr/llama`、每个用途一个服务、模型条目、Needs、引擎页。两台机器上实测。
5. **OpenAI 转写**：`asr/openai`，设置里的识别 API，Qwen 云端迁移进来。
6. **OpenAI Realtime**。

## 决定

2026-10-31，对本设计留下的问题：

1. Qwen 云端的设置并入识别 API，迁移它的密钥（第 5 步）。
2. SenseVoice 仍是默认；有 GPU 能跑 Qwen3-ASR 时，引擎页推荐它。
3. Qwen3-ASR 从 `ggml-org/Qwen3-ASR-1.7B-GGUF` 下载，Hugging Face 和 ModelScope 都有。
4. Apple 识别和其他识别不一样：加快它中文出句的处理保留，但放在它自己的适配器里，不放在断句器里。
5. 过长句子的保险暂时保留。
