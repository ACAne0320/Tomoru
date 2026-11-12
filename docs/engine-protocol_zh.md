# 引擎协议（v1）

[English](engine-protocol.md)

引擎是 Tomoru 里和平台相关的那一半：听电脑播放的声音，识别语音。识别之后的一切（断句、翻译、显示）都由 app 完成，用 Go 写，各平台相同。app 把引擎作为子进程运行，用 JSON lines 和它通信。

- **stdout** 只承载协议：每行一个 JSON 对象，每行刷新一次。
- **stderr** 是自由格式的日志。app 把它抄进日志文件，从不解析。
- **stdin** 承载 app 发来的命令，每行一个 JSON 对象。stdin 关闭时引擎退出，所以即使 app 崩溃，引擎也不会比它活得久。
- 每个对象都有 `type`。双方都忽略不认识的类型和字段，所以任何一方都可以加字段而不破坏另一方。

macOS 引擎是 `engine/mac`（Swift：Core Audio tap + SpeechAnalyzer），构建为 `tomoru-engine`，放进 `app/resources/darwin-arm64/bin/`。Windows 引擎是 `engine/windows`（Go，调用 Windows 的部分用 C：WASAPI 设备与进程环回、SAPI，以及自己的框选窗口），由 `scripts/build-engine-windows.sh` 构建为 `tomoru-engine.exe`，放进 `app/resources/windows-amd64/bin/`。Windows 没有给应用用的语音识别，所以它只收听：`listen` 必须带 `--asr none`，`prepare` 以 `unsupported-locale` 失败。

用下载的模型识别不属于平台：app 按引擎的 `voice` 事件，把平台引擎用 `listen --asr none` 听到的音频切成一段段语音，由 `tomoru-asr`（`engine/asr`，Go + sherpa-onnx，各平台相同）逐段解码（见下文「tomoru-asr」和[识别适配层](recognition_zh.md)）。app 用和自己会识别的引擎相同的事件报告识别结果。云端识别（`app/internal/asr/qwencloud`，阿里云百炼的 Qwen ASR）在 app 里接收同样的 `audio`，用同样的事件报告。

## 命令

```
tomoru-engine listen --locale <bcp47> [--app <id> | --file <audio>] [--vad <model> | none] [--asr none]
                                                     识别直到 stdin 关闭；带 --asr none 时只收听
tomoru-engine locales                                识别能听的 locale
tomoru-engine prepare --locale <bcp47>               准备好一种语言的识别
tomoru-engine speak --locale <bcp47> -- <sentence>…  用系统的声音朗读句子
tomoru-engine access                                 引擎能听系统的声音吗？
tomoru-engine request-access                         让系统询问用户
tomoru-engine screen-access                          app 能截屏吗（屏幕录制）？
tomoru-engine request-screen-access                  让系统向用户申请这个权限
tomoru-engine apps                                   可以单独收听的应用
tomoru-engine select --out <png>                     让用户框选屏幕的一部分
```

`--app` 只听一个应用；它的 ID 按平台而定：macOS 上是 bundle ID，Windows 上是程序的文件名（`chrome.exe`），收听它的整个进程树，包括辅助进程。还没运行的应用，启动后开始收听。不带它时，引擎收听系统播放的一切声音，除了它自己。`--file` 改为收听一个录音，按它自己的速度播放，之后是静音，就像系统在播放它一样：评测（`bench/`）和测试用它，不需要扬声器，也不需要权限。

每个命令先输出 `hello`。失败的命令输出 `error`，以非零状态退出；成功的以 0 退出。

## 引擎 → app

### `hello`

```json
{"type":"hello","protocol":1,"engine":"mac","version":"0.1.0"}
```

每个命令的第一行。app 拒绝不认识的 `protocol`。stdout 为空也是无效的握手。JSON 格式错误或 `error` 事件会终止读取方和子进程，所以出错的协议流不会让进程卡在 stdout 上。app 停止子进程时先关闭 stdin，一秒后还没退出就杀掉它。音频写入阻塞也不妨碍这样关闭。

### `state`

```json
{"type":"state","state":"downloading"}
```

`listen`、`prepare` 和 `speak` 在进行中发送：

- `downloading`：正在下载这种语言的识别资源。macOS 上一种新语言用了 13–21 s，不报告进度。
- `ready`：识别准备好了（`prepare` 到此结束）。
- `listening`：采集已经开始，之后会有识别结果。
- `speaking`：`speak` 开始朗读它的句子，一句接一句，中间有停顿，从默认输出播放，`listen` 像听视频一样听到它们。读完就退出。app 的首次引导用它来试翻译。

### `result`

一个识别结果：从 `start` 到 `end` 这段音频的假设。

```json
{"type":"result","final":false,"start":12.30,"end":13.10,
 "tokens":[{"text":"今日は","start":12.30,"end":12.80},{"text":"いい天気","start":12.80,"end":13.10,"conf":0.91}]}
```

- `final: false`：**临时**结果。它是上一个定稿结果之后的实时假设，整个替换上一个临时结果。之后的结果可能修改它。
- `final: true`：这些 token 确定了，不会再变。下一个临时结果从它们之后开始。
- **文本**是各 token 的 `text` 连起来，包括其中的空格和标点。
- **token 的时间**：临时结果里识别器还没给词计时时，可以没有 `start` 和 `end`。
- **`conf`** 是识别器给出的 0 到 1 的置信度（如果它给的话）。macOS 上只有定稿结果有。

### `clock`

```json
{"type":"clock","t":13.42}
```

音频时钟，收听期间每 100 ms 发一次。断句器按它判断停顿，从不读墙上时间，所以回放结果是确定的。

### 时间

所有时间都是从 `listening` 起算的音频秒数，精确到毫秒。静音时采集不再送来缓冲区，时钟就按墙上时间继续走，让停顿照样过去。

### `voice`

```json
{"type":"voice","on":false,"t":7.456}
```

说话人的声音在 `t` 开始（`on: true`）或停止，由人声检测判断（Silero VAD：macOS 上是 app 随包放在引擎 `bin/` 旁边 `vad/` 里的 Core ML 模型，Windows 上是那里的 ONNX 模型，经 sherpa-onnx 运行）。检测以 32 ms 为一步：超过阈值 64 ms 算开始，低于阈值 96 ms 算停止，停止时刻是这 96 ms 的开头。没有检测器的引擎不发（`listen --vad none` 关掉它）。带 `listen --asr none` 时，它在发出的音频上运行，并且先发一个 `{"on":false,"t":0}`，表示它开着：app 的云端识别在没人说话时挂断，本地模型的语音段也由它切分。

### `audio`

```json
{"type":"audio","t":12.300,"pcm":"AAD//wEA…"}
```

`listen --asr none` 用它代替 `result` 和 `clock`：引擎听到的声音，16 kHz 单声道 16 位小端 PCM，base64 编码，大约每 100 ms 一次。`t` 是第一个采样的时间。音频没有空隙：采集什么都没送来时，引擎发静音，让时间跟着音频走。

### `error`

```json
{"type":"error","code":"no-permission","message":"System Audio Recording is off for Tomoru"}
```

`message` 写进日志，app 按 `code` 显示自己的文字：

| code | 含义 |
|---|---|
| `no-permission` | 系统不让引擎听它的声音 |
| `unsupported-locale` | 识别不支持这种语言 |
| `assets-failed` | 识别资源没下载下来 |
| `capture-failed` | 采集停止了，或者没能开始 |
| `no-voice` | 系统没有朗读这种语言的声音（`speak`） |
| `model-failed` | 识别模型加载不了（`tomoru-asr`） |
| `internal` | 其他一切 |

出错后 `listen` 退出，所以采集或识别器失败就结束这次运行。是否重新开始由 app 决定。

### 一次性命令的回答

```json
{"type":"access","state":"granted"}
{"type":"app","id":"com.apple.Safari","name":"Safari","playing":true}
{"type":"locale","id":"ja-JP"}
```

- `access` 和 `request-access` 用一个 `access` 对象回答。它的 `state` 是 `granted`、`denied` 或 `unknown`（还没问过用户）。`screen-access` 和 `request-screen-access` 对屏幕录制做同样的回答，框选翻译需要它；系统把引擎的权限算作 app 的。
- `locales` 对系统识别能听的每个 locale 发一个 `locale` 对象（macOS：`SpeechTranscriber.supportedLocales`），用 BCP 47 标签，然后退出；app 只让用户选这些语言作为说的语言。Apple 的 `zh-HK` 是粤语。`supportedLocale(equivalentTo:)` 不能用来判断支不支持：它对 `th-TH` 也有返回，而泰语并不在列表里。Windows 引擎没有识别，app 不会问它。
- `apps` 对每个可以单独收听声音的应用发一个 `app` 对象：在程序坞里有图标的运行中应用（Windows 上是在任务栏上有窗口的），以及任何正在播放声音的应用（`playing`），正在播放的排在前面。然后退出。
- Windows 收听播放的声音和截屏都不需要权限：几个权限命令都回答 `granted`。
- `select` 让用户框选屏幕的一部分，写成 PNG 存到 `--out`，发送它的位置，以 0 退出：`{"type":"area","x":1298,"y":654,"width":328,"height":106}`，单位是像素：显示器的点（DIP）乘以它的缩放比例，从主显示器左上角算起，所以 `x` 和 `y` 可以是负数。app 把译文显示在它旁边。按 Esc 取消：什么都不写，也不发。Windows 上引擎在所有屏幕上方显示各屏幕变暗的画面，用户在上面拖动（右键也能取消），结束时让 app 拿到键盘。macOS 上它运行系统的 `screencapture -i`，后者不告诉选区在哪：引擎在它运行时看鼠标左键和指针。单击（按空格之后，在那里是选一个窗口）不发送位置：译文显示在菜单栏图标下。

## app → 引擎

### `finalize`

```json
{"type":"finalize","through":13.10}
```

要求识别器现在就把 `through` 之前的一切定稿，而不是等它自己的长停顿。app 只为 Apple 识别向 macOS 引擎发送它（`app/internal/asr/apple`），而且只在停顿中发送：临时结果不再变化时，以及中文在说话人停顿时。说话途中强制定稿会让识别器在那里结束这句话，插进标点，并丢掉或写坏词。

识别器做不到这一点的引擎忽略它。

## tomoru-asr

```
tomoru-asr serve --model <dir> --lang <zh|en|ja|ko|yue> [--threads <n>]   解码一段段语音
```

`<dir>` 是一个下载好的模型（SenseVoice：`model.int8.onnx` 和 `tokens.txt`）。app 把语音切成段，每段在变长时和结束时请求解码（`app/internal/asr/batch`，[识别适配层](recognition_zh.md)）；tomoru-asr 只负责解码。

- **stdin** 承载 `decode` 命令，每条一段语音，16 kHz 单声道 16 位小端 PCM，base64 编码；其他命令忽略。stdin 关闭时退出。

  ```json
  {"type":"decode","id":"7","pcm":"AAD//wEA/38="}
  ```

- **stdout** 承载 `hello`（`"engine":"asr"`），然后每个 `decode` 按顺序回一个 `decoded`，`id` 相同：这段的 token，每个带从这段开头算起的时间，没有 `end`。SenseVoice 在中文、粤语、日语词之间插的空格已去掉。

  ```json
  {"type":"decoded","id":"7","tokens":[{"text":"Hello","start":0},{"text":".","start":0.48}]}
  ```

- 它的 `error` code 是 `model-failed` 和 `internal`（无效的 JSON、格式错误的 PCM 或读输入出错），出错即退出。

## 回放

一次 `listen` 运行的引擎 stdout，是 app 看到的一切的完整记录：结果和时钟，按顺序。存成文件后，可以在 Go 测试里通过断句器回放，不需要音频和引擎。断句器的改动就是这样做回归测试的（`app/internal/segment/testdata/`）。

## 版本

只有旧版 app 会读错的改动才提高 `protocol`。加字段或加类型不改它。

## 契约检查

`protocol/fixtures.json` 是共享的 v1 契约：合法的生产者记录、握手失败、可选的 token 时间、空的定稿结果、音频时钟、时间零点的 voice-off、向前兼容的字段和事件、出错终止，以及 stdin 命令。两个 Go module 在协议测试里加载它。ASR 的测试还验证 EOF 时的冲刷和格式错误的输入；app 的测试启动真实的子进程，验证正常 EOF、强制关闭、写入阻塞和协议失败。

Windows 引擎的测试（`engine/windows`，在任何电脑上都能运行）用同样的 fixtures 检查它的生产者和命令读取。

Swift 验证器把生产代码的 `Protocol.swift` 和 `engine/mac/Tests/ProtocolContract.swift` 一起编译。它用同样的 fixtures 检查有类型的生产者构造函数，并经过生产代码的命令解码器测试 stdin EOF。它只需要 Foundation，不需要音频权限或下载的模型。仓库的检查命令运行这个验证器，并构建完整的 macOS 引擎，从而检查所有协议调用点。

资源查找不在这个协议客户端里。桌面和评测的入口提供显式的资源根目录；客户端和契约检查不依赖 UI 框架。
