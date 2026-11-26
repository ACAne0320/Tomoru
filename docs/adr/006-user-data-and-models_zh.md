# ADR 006：用户数据与模型

[English](006-user-data-and-models.md)

日期：2026-11-21。状态：采用。

## 背景

这些决策原先写在代码注释里，直到代码不再保留注释。它们涉及 Tomoru 在用户电脑上保存的数据（`prefs`、`atomicfile`、`applog`、`history`、`usage`）和本地模型（`models`）：放在哪里、如何下载和移动、发布哪些版本、llama-server 如何运行。其余内容[架构](../architecture_zh.md)、[场景](../scenes_zh.md)和模型相关文档已经写明。

## 决策

1. `Prefs.GuideStep`（`app/internal/prefs/prefs.go`）记住首次引导所在的页面，以便回到那里：系统设置在授予权限时会退出应用。
2. `atomicfile.Write`（`app/internal/atomicfile/atomicfile.go`）不创建目录：父目录由调用方负责，缺失时报告错误，而不是悄悄建出来。
3. `File.And`（`app/internal/applog/applog.go`）同时写日志和控制台，只报告日志的错误：从 Windows 资源管理器启动的应用没有控制台，每次写入都会失败，日志照常继续。
4. `Writer.create`（`app/internal/history/history.go`）从不让新会话使用已删除会话（`.deleted`）的名字，否则撤销会把旧会话恢复到它上面；它改用下一个空闲的名字。
5. `count`（`history.go`）按换行符计数会话的行，不逐行解析，因为列表每次都要读所有会话。应用退出时被截断的最后一行没有换行，不计入；读取时跳过无法解析的行。
6. `rename`（`history.go`）重命名失败时最多再试五次，每次多等 20 毫秒：在 Windows 上，搜索索引或杀毒软件短暂打开的文件在此期间无法替换或移动。
7. `usage` 包（`app/internal/usage/usage.go`）只保存数字，从不保存说了什么。
8. `nextLocalDay`（`usage.go`）搜索第一个本地日期改变的时刻，而不是直接取 `time.Date` 的下一个 00:00：有些时区在午夜跳过或重复时钟，甚至跳过一整天（Pacific/Apia，2011-12-30），只用 `time.Date` 会取到前一天或第二个午夜。
9. `app/internal/usage/interval_test.go` 导入 `time/tzdata`，让系统没有时区数据库时（如 Windows）也能加载测试用到的时区。
10. `models.Dir`（`app/internal/models/models.go`）把模型放在用户的本地数据目录（`~/Library/Application Support`、`%LocalAppData%`、`$XDG_DATA_HOME` 或 `~/.local/share`）下的 `Tomoru/models`。不放在应用里，因为更新会整体替换它；不放在缓存里，因为清理工具会清空它；也不放在 MyGo 的用户数据目录，它在 Windows 上是漫游配置文件，在 Linux 上是 `~/.config`。
11. `Model.URLs`（`models.go`）对中国用户先试 ModelScope，因为那里 Hugging Face 慢或无法访问；其他地区先试 Hugging Face；两者都留在列表中。每个源都固定在含有 `models.All` 所列文件的提交上（`Rev`、`MSRev`），从不用分支：2026 年 11 月 ggml-org 上传了新的 Qwen3-ASR 文件，ModelScope 的副本随之更新，它的 `master` 给出的文件大小和 SHA-256 都和清单不同，而清单在 Hugging Face 上仍固定在旧的上传（#169）。换到新的上传时，固定的提交和清单一起改；只改了元数据时（如 Qwen3-ASR），旧文件留在 `Model.Older` 里，仍算已下载，没人需要再下一遍同样的权重。
12. `Manager.fetchFile` 和 `fetch`（`app/internal/models/download.go`）：不是最后一个的下载源，若前 10 秒平均低于 1 MB/s，就让给下一个，下一个从同一字节续传。任何源在整整 10 秒没有进展（包括等待响应头）后放弃；最后一个源只要有进展，再慢也继续，整个下载没有总期限。`fetchFile` 返回前关闭 `.part` 文件：Windows 不能重命名或删除打开着的文件。响应表明文件大小不同的源，在写入任何内容前就让给下一个；发来的字节比文件多的源会清空 `.part`：那些字节属于另一个文件，否则下一个源会被要求给出超过文件末尾的范围。从 `.part` 续传前，`startMatches` 先向源要前 64 KiB，和 `.part` 的开头不同就从头下载：在大小相同或更小的旧上传上开始的 `.part`（如 0.2.3 的 Qwen3-ASR），否则要等整个下载完才发现损坏。这些字节也算进展，慢的最后一个源照样能读完它们。
13. `moveFiles`（`app/internal/models/move.go`）：目标是换了名字的同一个目录（例如 macOS 上大小写不同）时，什么也不移动，也不把其中的文件当作已移走而删除。
14. `models.All`（`models.go`）：Index-Translate 以 Q4_K_M 发布，在我们的测试中它与 Q8_0 犯同样的错误，体积不到三分之二，视觉投影器为 Q8_0。Hy-MT2 同样是 Q4_K_M，在这里的 llama-server 参数下，占用内存与同尺寸的 Index-Translate 相当（M5 上 Metal 内存：1.8B 1380 MiB，2B 1389；7B 5043，9B 5257）。Hy-MT3 也是 Q4_K_M，占用稍多（1.8B 1586 MiB，7B 5540），需要的内存不变（[ADR 011](011-hy-mt3-on-device_zh.md)）。
15. `Server.start`（`app/internal/models/server.go`）以 `--cache-ram 0` 运行 llama-server：提示缓存在几百行内让占用涨到 7 GB，速度却没有提升。
16. `Server.ensure` 和 `process.wait`（`server.go`）：为加载视觉投影器而在同一端口重启的服务器，若拿不回该端口，就换一个端口启动。`wait` 通过 `/v1/models` 确认应答的是我们的服务器和我们的模型：其间可能有别的程序占用了该端口。
17. `process.warm`（`server.go`）每次启动都用一个与应用请求相同形式的请求预热服务器（一句话；有投影器时再加一张图片）。GPU 第一次运行时，Vulkan 要花 10 到 40 秒构建程序，驱动会把它们存在磁盘上，但应用无从得知是否已经存过。该请求有自己的 2 分钟 context，调用方放弃后仍会完成，让下次启动不必重新构建；失败只会让第一次翻译变慢。

## 影响与取舍

历史和模型文件在所有平台上都按 Windows 的文件锁定规则打开、关闭和重命名。慢而稳定的镜像无论多久都能完成下载。每次启动 llama-server 多一个预热请求，驱动存好程序后很快。

## 证据

- `app/internal/applog/applog_test.go`：`TestAnd`。
- `app/internal/history/history_test.go`：`TestUndo`、`TestDeleteWhileRecording`、`TestRemovedWhileRecording`。
- `app/internal/usage/interval_test.go`：跳过或重复午夜的时区，以及被跳过的日期。
- `app/internal/models/models_test.go`：`TestSlowSource`、`TestStalledSource`、`TestLastSourceKeepsSlowProgress`、`TestMirrorWithAnotherFile`、`TestResumeAnotherUpload`、`TestSlowStartCompared`、`TestPinnedRevisions`、`TestOlderFiles`、`TestServerVision`。
- `app/internal/models/move_test.go`：`TestMoveSameDir`。
- `app/internal/models/server_test.go`：`TestWarm`、`TestWarmExit`。
