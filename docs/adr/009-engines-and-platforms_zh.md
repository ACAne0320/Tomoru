# ADR 009：引擎与平台

[English](009-engines-and-platforms.md)

日期：2026-11-21。状态：采用。

## 背景

这些决定原本写在代码注释里，代码不再带注释后移到这里。范围是各平台引擎（`engine/mac`、`engine/windows`、`engine/linux`）和 Go 引擎共用的代码（`engine/shared`）；`engine/asr` 的决定文档里都已写明。[引擎协议](../engine-protocol_zh.md)、[本地识别](../local-asr_zh.md)、[架构](../architecture_zh.md)和[图片翻译](../image-translation_zh.md)已经写明的不再重复。

## 决策

### Go 引擎共用

1. `engine_linux.h` 和 `engine_windows.h` 把对 PulseAudio、GTK 和 COM 的调用放在 C 里，因为这些 API 回调的是 C 函数，COM 的接口是要用 C 调用的 vtable。
2. `selectArea`（`linux/select_linux.go`、`windows/select_windows.go`）、`run`（`windows/capture_windows.go`）和 `speak`（`windows/speak_windows.go`）在整个调用期间持有 `runtime.LockOSThread`：GTK、COM 对象、Win32 窗口和 SAPI 语音都属于创建它们的系统线程，否则 goroutine 可能换线程。
3. `FillSilence`（`shared/hear.go`）和 macOS 引擎的 `Hearer`（`Hear.swift`）一样，只在音源已经 0.3 秒没有给出任何东西时才发静音。音源在出声时，它的样本就是时间线，即使它的时钟和墙上时间有漂移，因为 tomoru-asr 的停顿和时钟只来自音频。睡眠或挂起之后最多发 2 秒静音，足够把句子收尾，然后重设时钟，而不是把整段空白当 PCM 重放。
4. `decodeWAV`（`shared/file.go`）按 RIFF 的要求，跳过奇数长度块后面的填充字节。
5. `resample`（`shared/file.go`）降到 16 kHz 时对每个输出样本覆盖的样本取平均，升采样用线性插值，不用正规滤波器：对语音够用。
6. `newSilero`（`shared/voice.go`）的判定和 macOS 引擎的 Core ML 检测器一致。阈值是 0.5，因为 sherpa-onnx 把结束阈值定在开始阈值下 0.15，正好是 macOS 引擎的 0.35。`MinSpeechDuration` 和 `MinSilenceDuration` 少一个窗口，因为 sherpa-onnx 从第一个窗口结束时开始计数。

### Linux

7. `appID`（`linux/select_linux.go`）和 `session`（`linux/select_linux.c`）用 MyGo 给应用的 ID（启动它的桌面项，否则是父进程的程序名）向 `org.freedesktop.host.portal.Registry` 注册引擎，让 portal 把它的截图和权限算作应用的。Register 失败就忽略：旧的 portal 没有注册表，按 cgroup 识别应用。
8. portal 回答 2 时，`portal_file`（`linux/select_linux.c`）去读权限存储，因为 GNOME 和 KDE 把用户的拒绝回答成 2，即一般失败，而不是 1。
9. `tc_select`（`linux/select_linux.c`）在 Wayland 下用 portal 截图，因为 XWayland 只看得到 X11 窗口。在 X11 下用 X 服务器的时间显示第一个窗口，窗口管理器就当作用户的操作并给它焦点。
10. `stream_read`（`linux/capture_linux.c`）把空洞（`pa_stream_peek` 没给数据却有长度）当静音送出再丢弃，而不是跳过，让音频时间保持连续。
11. `open_locked`（`linux/capture_linux.c`）用 `PA_STREAM_DONT_MOVE` 打开监听源：监听源消失时流就结束，引擎再打开新的默认输出，而不是让服务器把它移到麦克风上。

### Windows

12. `formats`（`windows/capture_windows.go`）先向 WASAPI 要 16 kHz 单声道，不行就要音频引擎一定给的 48 kHz 立体声，在 Go 里转换。
13. `onTaskbar`（`windows/apps_windows.c`）排除被隐藏（cloaked）的窗口：挂起的商店应用保留可见但被隐藏的窗口。
14. `activationParams`（`windows/capture_windows.c`）自己声明进程环回的参数，因为 mingw-w64 没有 `audioclientactivationparams.h`。
15. `handler`（`windows/capture_windows.c`）响应 `IAgileObject`：`ActivateAudioInterfaceAsync` 要求完成处理程序是 agile 的，并从它自己的线程调用。
16. `speak_windows.c` 先在 OneCore 的类别里找语音，再找 SAPI 自己的，因为 Windows 随语言安装的语音属于 OneCore，SAPI 把它们分开列出。
17. `tc_select`（`windows/select_windows.c`）让进程按显示器感知 DPI（v2），每块屏幕按自己的缩放以像素截取，不按系统缩放拉伸。它把 Esc 注册为热键，因为 Windows 可能不把键盘交给没有接收用户最近一次输入的进程。

### macOS

18. `Capture.swift` 通过 Core Audio 进程 tap 收音（macOS 14.2+），因为 tap 只需要“系统录音”权限，不需要录屏。
19. `request-access`（`main.swift`）启动一个 tap 并等待回答，最多一分钟：没有请求权限的调用，系统在 tap 第一次录音时询问。被拒绝时得到的是静音，不是错误，所以 `TapCapture` 每 30 秒记录一次收到的内容。
20. `start`（`Capture.swift`）处理 `--app` 时，tap 所有 bundle ID 等于该应用、或以它加一个点开头、或责任进程是该应用的进程，因为应用会通过有自己 bundle ID 的辅助进程播放（比如浏览器的音频进程）。
21. `responsiblePID` 和 `tccPreflight`（`Capture.swift`）用 `dlsym` 加载私有的 `responsibility_get_pid_responsible_for_pid` 和 `TCCAccessPreflight`，因为没有公开 API 能查辅助进程所属的应用，或检查“系统录音”权限。它们不存在时，分别退回进程本身和 `unknown`。
22. `audioAccess`（`Capture.swift`）把录屏权限也算作 tap 的权限。
23. `requestScreenAccess`（`Capture.swift`）在状态为 `unknown` 时调用一次 `CGRequestScreenCaptureAccess`，最多等一分钟回答；系统只提供打开系统设置。应用每次都在新启动的引擎进程里重新检查，所以权限无需重启应用（`core/system.go` 的 `ask`）。
24. `installAssets`（`Listen.swift`）总是发出资源安装请求，即使该语言已经安装：请求会为应用保留这个语言（最多五个），已安装时不花时间。新语言用了 13–21 秒，没有进度报告。
25. `quietTail`（`Listen.swift`）：静音满 2 秒后，引擎不再把音频交给 SpeechAnalyzer，它会在静音里听出东西（什么都没播放时 tap 给出的零值里，第 6 秒出现置信度 0.91 的“あ”）。下一段声音带着自己的时间进来，时间线仍然算上了这段静音。
26. `copyBuffer`（`Listen.swift`）复制 tap 的缓冲区，因为回调返回后 tap 会重用这块内存。
27. `Voice.swift` 以 `.cpuOnly` 运行 Core ML 检测器：一个每秒运行 31 次的小模型，调度的开销比运行本身还大。
28. `selecting`（`Select.swift`）把引擎收到的 SIGTERM 转给 `screencapture`，它在自己的进程组里，否则会把十字准星留在屏幕上。

## 影响与取舍

macOS 引擎依赖两个私有符号和未写入文档的行为（静默的拒绝、语言保留）；苹果改动它们时，退路能让引擎继续工作，只是能做的少一些。Go 引擎的 C 代码和线程锁定只在各自的平台上测试，按 AGENTS.md 的要求。

## 证据

- `engine/shared/engine_test.go`：`TestHearer`、`TestHearerResumesWithoutSilenceBacklog`（挂起后最多 2 秒）、`TestDecodeWAV`、`TestDetector`。
- `engine/shared/engine_test.go` 和 `engine/asr/protocol_test.go`：`TestSharedProtocolFixtures`，对照 `protocol/fixtures.json`。
