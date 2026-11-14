# 开发与验证

[English](development.md)

仓库包含三个 Go module：`app`、`engine/asr` 和 `engine/windows`，以及 `engine/mac` Swift package。先读 [架构说明](architecture_zh.md) 了解功能所有权，产品和界面规范见 [设计系统](design-system_zh.md)。

## 工具与启动

桌面开发需要 Apple silicon、macOS 26、Go 1.27.3（以各 `go.mod` 为准）及 Xcode 26 或它的命令行工具（Swift 6.2 或更新版本）。Go 下载依赖和首次准备随包资源需要联网。

从仓库根目录执行：

```sh
scripts/setup.sh              # 首次运行，改了 engine/ 之后再运行
cd app
go tool mygo dev              # 改代码后自动重新构建并重启
go test ./...
go tool mygo build -sign -    # 本机测试用的包，在 app/build/darwin-arm64/
```

`scripts/setup.sh` 构建引擎和 tomoru-asr，下载人声检测和 llama-server，都放进 `app/resources/<os>-<arch>/`。`go tool mygo dev` 也监视这个目录，所以在 app 运行时执行它（或只执行 `scripts/build-engine-mac.sh`），app 会带着新引擎重启。开发版叫 `Tomoru Dev`（`dev.acane.tomoru.dev`），权限和设置与已安装的 Tomoru 分开。`go run . --guide` 再次进入引导。`go tool mygo dev -background` 启动和每次重建后都不把 app 带到前台，保存代码时焦点留在编辑器里。

`-sign -` 使用 ad-hoc 临时签名，供本机测试，产物在 `app/build/darwin-arm64/`。它只为这次命令覆盖 `macos.signingIdentity`，贡献者无需持有发布证书，也无需修改 `app/mygo.json`。`go tool mygo dev` 默认也使用临时签名。若要用固定身份测试权限，可以传入自己的证书：`-sign "你的证书名称"`，见 MyGo 的 [构建参数](https://github.com/egoist/mygo/blob/main/docs/cli.md#mygo-build)。正式发布仍使用现有证书和更新密钥，见 [发布指南](release_zh.md)。

Windows 版在 Windows 上用 MSYS2 的 MINGW64 终端构建，步骤见 [README](../README_zh.md#windows)；`go tool mygo dev` 在那里和在 Mac 上一样用。引擎和 tomoru-asr 从自己旁边加载 sherpa-onnx 的库，所以它们的测试要把这些库放进 `PATH`：

```sh
export PATH="$PATH:/c/Program Files/Go/bin:$PWD/app/resources/windows-amd64/bin"
(cd engine/asr && go test ./...)
(cd engine/windows && go test ./...)
scripts/check.sh go && scripts/test.sh go     # 它们要的编译器就是 MSYS2 的 MinGW-w64
pacman -S diffutils                            # 装一次，package-licenses.sh --release-check 要用
```

也可以在 Mac 上交叉编译，需要 mingw-w64（`brew install mingw-w64`，引擎和 tomoru-asr 用 cgo 调 sherpa-onnx）：

```sh
scripts/setup.sh windows-amd64   # tomoru-engine.exe、tomoru-asr.exe、人声检测和 llama-server
cd app && go tool mygo build -platform windows/amd64
```

产物在 `app/build/windows-amd64/`（`Tomoru.exe` 与旁边的 `bin/`、`vad/`、`licenses/`），另有安装包 `tomoru-setup-<version>-amd64.exe`：在 Windows 上 MyGo 第一次会自己下载 NSIS；在 Mac 上要先装 NSIS（`brew install makensis`）。Windows 支持范围为 Windows 11 x64。「全部系统声音」用设备环回捕获默认播放设备，默认设备变化后会重新连接；单独输出到其他设备的应用不在捕获范围内。指定应用仍用进程环回，可能漏掉通信音频；遇到这种情况，把应用输出设为系统默认播放设备，并选择「全部系统声音」。进程环回 API 要求 [Windows build 20348 或更新](https://learn.microsoft.com/en-us/windows/win32/api/audioclientactivationparams/ns-audioclientactivationparams-audioclient_activation_params)，因此不支持 Windows 10。

构建脚本把程序放到 `app/resources/<os>-<arch>/bin/`。用户下载的模型放在用户数据目录，或用户在设置里选的文件夹中的「Tomoru Models」文件夹（选了就把模型移过去）；无需为了单元测试下载它们。目录及协议分别见 [架构说明](architecture_zh.md) 和 [引擎协议](engine-protocol_zh.md)。

## 检查和测试

在仓库根目录运行：

```sh
scripts/check.sh            # 静态检查：不跑测试，不构建
scripts/test.sh             # 测试
scripts/test.sh --race      # 开启 Go 的 race detector
```

两个脚本都可以指定要跑的部分；不指定时跑本平台能跑的全部（`ui` 和 `swift` 需要 macOS 26 和 Xcode 26）：

| | 部分 | 运行内容 |
|---|---|---|
| `check.sh` | `go` | 许可证清单、`gofmt`；app 针对 macOS 和 Windows、两个引擎针对本平台和 Windows 的 `go vet`、`.golangci.yml` 里的 linter 和没有调用方的函数（`deadcode`）（需要 mingw-w64：`brew install mingw-w64`，`apt install gcc-mingw-w64-x86-64`）；以及 app 的分层、平台和设计规则。linter 的版本写在脚本里，第一次运行时构建到 `~/.cache/tomoru-tools` |
| | `swift` | Swift 引擎的类型检查，警告视为错误 |
| `test.sh` | `go` | 三个 Go module 的测试，含 benchmark，不含 UI 和 `core/coretest` 桌面适配器 |
| | `ui` | app 的 UI 测试 |
| | `swift` | Swift 协议验证器（对照 `protocol/fixtures.json`），以及 `HearerContract.swift`：向生产收听器输入合成 PCM，验证静音限额和时间连续性，不启动采集或模型 |

两者都不构建要发布的包，那是发布时的事（`go tool mygo build`，见 [发布指南](release_zh.md)）。测试不使用缓存。本地 ASR 使用 cgo，`test.sh go` 需要本机 C 编译器。

CI 在 Checks 和 Tests 两个 workflow 里运行同样的脚本，触发于推送到 `main` 和 pull request。每个 job 只在它关心的路径有改动时运行（`.github/paths.yml`），只改文档时一个都不跑；被跳过的 job 算作通过。Go 在 Linux 上跑，UI 和 Swift 在 `macos-26` 上跑（[官方镜像列表](https://github.com/actions/runner-images/blob/main/README.md)）。

仅针对修改运行测试时：

```sh
cd app
go test ./internal/live ./internal/core
go test ./internal/ui/settings
cd ../engine/asr
go test ./...
```

如果执行环境禁止 loopback 监听，涉及 `httptest` 的测试需要在允许本地端口的环境运行。测试只使用本地模拟服务，默认不需要外部 API。

## 验证层级

| 层级 | 作用 | 执行时机 |
| --- | --- | --- |
| 受影响包测试 | 验证当前行为、失败与取消路径 | 每次功能变更 |
| `check.sh` | 格式、针对 macOS 和 Windows 的 vet 和 lint、无用代码、包边界和平台边界、设计规则、许可证、Swift 引擎的类型 | 每次代码变更、CI |
| `test.sh --race` | 业务规则、持久化、子进程、并发、协议实现、UI、Swift 契约 | 每次代码变更、CI、发布门槛 |
| 人工录音与桌面验收 | 权限、真实录音、快捷键、跨屏字幕、退出、打包资源 | 发布前及平台行为变更 |
| Windows 11 x64 实机 | 在 MSYS2 里运行 `scripts/check.sh go` 和 `scripts/test.sh go`（或用 `GOOS=windows GOARCH=amd64 go test -c` 交叉编译测试程序，在包目录运行）、引导全流程、采集、截图、托盘 | Windows 相关变更、发布前 |
| 选配真实评测 | 指定音频、模型或云端服务的质量与延迟 | 识别/断句/翻译效果变更 |

真实评测的输入、参数及评分见 [benchmark 指南](../bench/README_zh.md)。`TOMORU_E2E` 等变量启用真实端到端测试；`TOMORU_DOWNLOAD` 启用下载测试；普通检查保持这些变量未设置，避免自动下载或云端调用。UI gallery 通过 `TOMORU_GALLERY` 输出截图，是人工布局验收的补充。

## 修改的落点

- UI 控件编辑副本；持久化、词表更新、会话切换放到应用命令。新增菜单需验证回调在绘制结束后仍能提交。
- 新增后台任务必须说明谁取消它、何时等待、如何拒绝旧结果。新子进程需测试无响应退出。
- 新增句子记录消费者订阅 `OnLine`，不能读取十行显示快照来猜完成状态。
- 新增包族更新分层规则及规则测试；不要为了让测试通过放开整个上层依赖。
- 协议变更同时修改共享 fixtures、四个实现（app、tomoru-asr、Windows 引擎、Swift）的契约检查和协议文档。
- 平台差异放在边界（引擎、`capture`、`resource.Hide`、`shell` 的托盘图标）；界面上只有说法不同时用 `shared.OnWindows`，不在领域包里判断平台。提到平台的文件要加入 `app/layers_test.go` 的 `platformFiles`。

发布步骤与签名要求见 [发布指南](release_zh.md)。贡献流程见 [贡献指南](../CONTRIBUTING_zh.md)。
