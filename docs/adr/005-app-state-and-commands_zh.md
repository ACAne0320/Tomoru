# ADR 005：应用状态与命令

[English](005-app-state-and-commands.md)

日期：2026-11-21。状态：采用。

## 背景

这些决策原先写在代码注释里，直到代码不再保留注释。它们涉及应用层 `core`：选择引擎、密钥、会话的生命周期、历史写入、图片选区和反馈。[架构](../architecture_zh.md)和 ADR 001 已写明通用规则；这里只记录它们没有写到的部分。

## 决策

1. `Core.Shortfall`（`app/internal/core/catalog.go`）以整 GB（`gb` 四舍五入）比较内存、显存与模型的需求，与电脑标称的容量一致：8 GB 的显卡算作 8，尽管驱动会占用一部分。
2. `apiKey`（`app/internal/core/cloud.go`）在凭据存储中把翻译服务的密钥命名为 `mt-key:<id>`，与识别服务的 `asr-key:<id>` 并列。
3. `Core.DeleteEndpoint`（`app/internal/core/engines.go`）删除 API 后，字幕仍选着它：翻译以 `ErrNoEndpoint` 停止，而不是换用另一个引擎继续；`prefs.Load`（`app/internal/prefs/prefs.go`）也不会重置指向已不存在的 API 的 `MT`。图片也仍选着它：之后的图片以 `ErrNoEndpoint` 失败，发给测出不能读图片的 API 的图片以 `ErrNoVision` 失败，都不会改发云端（#128）。撤销会恢复该 API 和它的密钥，并在删除使翻译停止时重新开始翻译。
4. `Core.Start`（`app/internal/core/session.go`）在会话的后台 `Prepare` 中、与翻译器一起读取云端识别的密钥，而不在主线程：凭据存储可能阻塞。
5. `Core.freeModels`（`session.go`）：停止时停掉本地模型的 llama-server，归还内存（Qwen3-ASR 1.7B 与 Index-Translate 9B 合计 8.6 GB），但图片翻译正在使用的那个除外；再次开始时从系统文件缓存加载只需 1 到 3 秒。重启（例如切换语言时）保留它们。
6. `SessionController.apply`（`app/internal/core/session_controller.go`）把没有错误就结束、或以 `*exec.ExitError` 结束（崩溃、被系统杀掉）的引擎报告为"引擎已退出"（`IsExited`）：退出码对用户没有意义。
7. `SessionController.apply` 只在显示内容变化时重绘（`sameShown`）：引擎的时钟每秒报告数次，却没有新东西可显示。
8. `HistoryService.drain`（`app/internal/core/history_service.go`）按从旧到新写入等待重试的运行，每个写空后才写下一个，最后才写当前正在记录的会话，使重启不会丢掉上一次运行写入失败的句子，也不会把它们排到更新的句子之后。
9. `ImageService.selectImage`（`app/internal/core/image.go`）：选区打开时再按图片快捷键，会结束那个选区，并在它结束后重新选择，而不是忽略这次按键：选区可能卡在看不见的地方（#62）。
10. `Core.Feedback` 和 `Core.IssueURL`（`app/internal/core/feedback.go`）说明应用自身的情况（版本、系统、引擎、语言、去掉凭据和查询参数的 API 地址、只说类型的代理），供用户电脑上的 AI 代理或 issue 表单使用。其中不含 API 密钥、字幕、录音或所听内容的名称。
11. `maxIssueURL`（`feedback.go`）：issue 表单的地址最长 4000 字节，用户写的文字按此截断。GitHub 在约 6 KB 时出错（2026-11），并把未登录的人带去登录，地址又被转义一次。

## 影响与取舍

删除正在使用的 API 会让字幕停止，直到用户选择另一个引擎或撤销。停止释放内存，代价是下次开始要多几秒加载。从应用提交的报告带有配置，但不含用户所看的内容。

## 证据

- `app/internal/core/engines_test.go`：`TestShortfall`、`TestEndpoints`（删除与撤销）、`TestStopFreesModels`。
- `app/internal/prefs/prefs_test.go`：`TestLoad`（已删除的 API 保持选中）。
- `app/internal/core/lifetime_test.go`：`TestCloudCredentialPreparationDoesNotBlockApplicationThread`。
- `app/internal/core/core_test.go`：`TestClockRedrawsNothing`。
- `app/internal/core/history_retry_test.go`：`TestHistoryRetriesFailedWritesAcrossSessionRestart`。
- `app/internal/core/image_test.go`：`TestSelectImageAgain`。
- `app/internal/core/feedback_test.go`：`TestFeedbackSaysTheSetupWithoutSecrets`、`TestIssueURLStartsTheBugForm`。
