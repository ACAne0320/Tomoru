# 发布

[English](release.md)

怎样为 macOS 和 Windows 11 x64 构建并发布 Tomoru 的一个版本，让已安装的 app 更新到它。

贡献者在本机打包时，使用 [开发签名方式](development_zh.md#工具与启动)。下文的证书和更新密钥用于正式发布。

## 一次性准备

- **签名证书**：登录钥匙串里的「Tomoru Self-Signed」，备份（`.p12` 和它的密码）放在仓库以外。每个版本都必须用同一张证书签名：macOS 把「系统录音」权限绑定在证书上，换了证书就会再向用户要一次权限。
- **更新密钥**：`mygo-update.key`，用 `go tool mygo keygen` 生成，放在仓库以外（下面的命令假定在 `~/.config/tomoru/update-keys/`）。已安装的 app 只接受用它签名的更新，所以要在密码管理器里留一份：丢了它，所有已安装的 app 就再也收不到更新。它的公钥是 `app/mygo.json` 里的 `updates.publicKey`。
- **GitHub**：`gh` 已登录，并有权限在 `updates.github` 指定的仓库发布 release。

## 每次发布

1. 在 `app/mygo.json` 里设版本号，在 `app/CHANGELOG.md` 写这个版本的一节（`## 0.1.1`）。更新窗口把这一节显示为更新说明。
2. 获取并构建 app 随包带的东西：

   ```sh
   scripts/setup.sh
   scripts/package-licenses.sh --check
   scripts/package-licenses.sh --release-check darwin-arm64
   ```

   许可证检查验证源许可证包，以及打包输入 `app/resources/<platform>/licenses`，并阻止存在未解决再分发问题的版本发布。上传公开二进制前，请查看 [第三方许可证清单](third-party-licenses_zh.md)，解决每个失败项。

3. 在仓库根目录运行 `scripts/check.sh` 和 `scripts/test.sh --race`，两者都必须通过（需要安装 mingw-w64）。见[验证层级](development_zh.md#验证层级)。然后试用构建出的 app：首次运行的权限、用 Apple 和本地识别听真实的声音、换语言和场景、快捷键、字幕位置、慢翻译之后的字幕记录、重开后的设置，以及工作中途退出。确认打包后的 app 能找到所有随包的引擎和库，且 `licenses/` 包含它们的 [许可证正文和声明](third-party-licenses_zh.md)。自动检查不覆盖签名、权限和付费的云服务。
4. 构建、签名，并上传一个草稿 release：

   ```sh
   cd app
   MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
   ```

   它写出 `build/darwin-arm64/`：app、`tomoru-<version>-arm64.dmg`、更新包、相对之前几个版本的增量包、`update-darwin-arm64.json` 和 `install.sh`，并把它们上传到标签为 `v<version>` 的草稿 release。
5. 在 GitHub 上检查草稿并发布。已安装的 app 在启动后不久检查一次，之后每天一次，那时就会发现它。

## Windows

在 Windows 11 x64 电脑上用 MSYS2 的 MINGW64 终端构建，工具见 [README](../README_zh.md#windows)，许可证检查另需 `diffutils`（`pacman -S diffutils`）。更新密钥也要从密码管理器复制到那台电脑的 `~/.config/tomoru/update-keys/`，`gh` 也要在那里登录。安装包用的 NSIS 由 MyGo 第一次构建时自己下载：

```sh
export PATH="$PATH:/c/Program Files/Go/bin:$PWD/app/resources/windows-amd64/bin"
scripts/setup.sh
scripts/package-licenses.sh --check
scripts/package-licenses.sh --release-check windows-amd64
scripts/check.sh go
scripts/test.sh go ui
cd app
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

也可以在 Mac 上交叉编译，需要 mingw-w64（`brew install mingw-w64`）和 NSIS（`brew install makensis`）：先 `scripts/setup.sh windows-amd64`，再 `go tool mygo build -platform windows/amd64 -upload`。

它写出 `build/windows-amd64/`：`Tomoru.exe` 和旁边的 `bin/`、`vad/`、`licenses/`，以及 `tomoru-setup-<version>-amd64.exe`。先在 Windows 11 x64 电脑上试一下：从头走一遍引导（SenseVoice 下载到 `%LocalAppData%\Tomoru\models`）、收听全部声音和单个应用、托盘的面板、快捷键、框选翻译、本地翻译模型，以及翻译中途退出。app 没有签名：SmartScreen 会在第一次启动时警告（点「更多信息」，再点「仍要运行」）。

许可证发布检查针对打包前的平台资源，不检查最终安装包。仍需确认成品中包含对应的许可证正文和声明。

## 用户第一次要做的

app 有签名但没有公证：macOS 会在第一次启动时拦下它。在下载页上写明：

> 打开 Tomoru。macOS 提示无法验证它时，打开 系统设置 › 隐私与安全性，点 Tomoru 旁边的「仍要打开」。

更新不需要用户做什么：更新程序检查它们的签名，app 的证书不变，权限也就保留。
