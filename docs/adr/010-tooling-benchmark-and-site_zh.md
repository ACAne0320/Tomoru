# ADR 010：工具、评测与网站

[English](010-tooling-benchmark-and-site.md)

日期：2026-11-21。状态：采用。

## 背景

这些决定原本写在代码注释里，代码不再带注释后移到这里。范围是 `.github/` 里的工作流、`.golangci.yml`、`scripts/`、仓库自己的测试（`app/design_test.go`、`app/locales/catalog_test.go`）、评测（`bench/`、`app/cmd/tomoru-bench`）和网站（`site/`）。[开发](../development_zh.md)、[发布](../release_zh.md)、[本地识别](../local-asr_zh.md)、[设计语言](../design-system_zh.md)、`bench/README.md` 和 `site/README.md` 已经写明的不再重复。

## 决策

### CI 与 lint

1. `checks.yml` 和 `tests.yml` 申请 `pull-requests: read`：`dorny/paths-filter` 通过 GitHub API 读取拉取请求改动的文件，没有这个权限会在拉取请求上失败。
2. `.golangci.yml` 在 `internal/ui/` 下排除 unparam 的 “`ui.Element`) is never used”：和 kit 的其他部分一样，每个构建函数都返回自己的元素，不管用不用。`_test.go` 也关掉 unparam：测试辅助函数接收 `t`，返回值有些测试不用。

### 发布与网站部署

3. `release.yml` 让各平台的任务依次运行，不管上一个成败都接着跑（`!cancelled()`）：`go tool mygo build -upload` 会把发布里的 `SHA256SUMS` 和自己的合并，两个同时上传会互相覆盖。
4. `release.yml` 在预发布的说明里写 `sh install.sh <tarball>`，而不是 `curl … | sh` 那行，因为 `install.sh` 下载的是 `releases/latest`，预发布不算。
5. `release.yml` 用 `tr -- '-+' '..'` 拼出 Debian 包的文件名：MyGo 用 `~` 写预发布的 Debian 版本号，让它排在正式版之前，而 GitHub 会把文件名里的 `~` 换成 `.`（0.2.0-beta.1 的是 `tomoru_0.2.0.beta.1_amd64.deb`）。
6. `release.yml` 给 MSYS2 设 `path-type: inherit`，让它的 shell 找到 setup-go 装的 Go 和 runner 上的 `gh`，默认设置会把它们藏起来。
7. `site.yml` 用仓库密钥 `CLOUDFLARE_API_TOKEN`（用 Cloudflare 的 “Edit Cloudflare Workers” 模板生成的令牌）和 `CLOUDFLARE_ACCOUNT_ID` 部署。

### 构建脚本

8. `build-asr.sh`、`build-engine-linux.sh` 和 `build-engine-windows.sh` 先删除库，再复制，再 `chmod`：Go 的模块缓存是只读的，`cp` 会保留权限，下一次复制就写不进去。
9. `build-engine-linux.sh` 用 `patchelf` 把 rpath 设为 `$ORIGIN`，和 `build-asr.sh` 在 macOS 上用 `@executable_path` 一样：cgo 让程序指向模块缓存。
10. `build-asr.sh` 在 `install_name_tool` 之后重新做 ad hoc 签名，因为它破坏了链接器的签名；Apple 芯片不运行没有有效签名的二进制。
11. `build-engine-mac.sh` 先删除旧引擎再复制新的：一个程序运行期间它的文件被原地覆盖，macOS 会按缓存的旧签名把它杀掉。
12. `fetch-llama.sh` 把 `TAG` 固定在测量本地翻译时用的 llama.cpp 版本。Windows 和 Linux 用 Vulkan 版，任何厂商的 GPU 都能跑，没有 GPU 时用 CPU。
13. `fetch-llama.sh` 复制 llama-server 和它需要的库，以及这些库再需要的（Linux 用 `readelf`，Windows 用 mingw-w64 的 `objdump`），每个都按被需要时的名字复制（`cp -L` 穿过链接）。ggml 的 CPU 和 Vulkan 后端按名字加入，因为 ggml 启动时才加载它们，任何导入表里都没有。
14. `fetch-llama.sh` 先把 `readelf` 和 `objdump` 的输出写进文件，再在循环里读，因为 `for` 列表里的命令替换失败时，`set -e` 不会停下。
15. `package-licenses.sh` 的复制模式只更新它管理的声明，不删除任何东西：未知文件留给 `--release-check` 报告，由人来审查。

### 检查与测试

16. `test.sh` 的 `go` 部分不包括 `internal/ui/...` 和 `core/coretest`，因为它们需要桌面会话；评测不需要界面，保留。
17. `unstyled`（`app/design_test.go`）只减不增：列表里的文件不存在了，或者已经改用 kit 的 token，测试就失败，也不能有新文件加入。
18. `TestDependencies`（`app/design_test.go`）保留 `golang.org/x/sys`，用于模型所需的剩余磁盘空间，以及 Windows 上的内存和任务栏主题。
19. 目录缺少某条消息时 `TestComplete`（`app/locales/catalog_test.go`）失败，因为 `mygo i18n check` 只警告，应用会显示英文。

### 评测

20. `cpuTimes`（`app/cmd/tomoru-bench/cpu_windows.go`）在 Windows 上把子进程的 CPU 记为 0，因为 Windows 不保存已结束子进程的总量。Windows 运行的 CPU s/min 只算评测本身。
21. `fetch`（`bench/fetch.py`）下载完整音频，再用 ffmpeg 截取片段：让 yt-dlp 截取时，一小时讲座里的一段花了超过 15 分钟，因为 YouTube 会限速跳转请求。

### 网站

22. `tween`（`site/public/studio.js`）用 `requestAnimationFrame` 逐帧移动光标和被拖动的东西，新的移动打断旧的。先把终点写进样式再从起点做动画，有时会先闪到终点一帧，看着像抽搐。
23. `.cam`（`site/public/styles.css`）把缩放声明两次。能用长度除以长度的浏览器用第二行 `calc(100cqw / (var(--w) * 1px))`；不能的丢掉它，用第一行 `tan(atan2(…))`，同样是 a/b。顺序很重要：Safari 26 在 `atan2` 里算错 `cqw`，但能做除法。没有脚本时靠它缩放；演示播放时写上自己的 transform 覆盖它。
24. `icons.html`（`site/src/partials/`）是 Lucide 图标（ISC）的 sprite，和应用用的是同一套。

## 影响与取舍

发布任务依次运行会更慢，前一个失败后一个仍会运行；“Re-run failed jobs” 只重建失败的部分。好几个脚本步骤只是为了应对模块缓存、macOS 签名和 ggml 加载的行为；去掉其中一个，就会在某个平台上构建失败。

## 证据

- `app/design_test.go`：`TestDesignTokens`（`unstyled` 列表）、`TestDesignScale`、`TestDependencies`。
- `app/locales/catalog_test.go`：`TestComplete`。
- `scripts/check.sh` 和 `scripts/test.sh`，CI 也这样运行。
