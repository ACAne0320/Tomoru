# 发布

[English](release.md)

怎样为 macOS 和 Windows 11 x64 构建并发布 Tomoru 的一个版本，让已安装的 app 更新到它。

贡献者在本机打包时，使用 [开发签名方式](development_zh.md#工具与启动)。下文的证书和更新密钥用于正式发布。

## 一次性准备

- **签名证书**：登录钥匙串里的「Tomoru Self-Signed」，备份（`.p12` 和它的密码）放在仓库以外。每个版本都必须用同一张证书签名：macOS 把「系统录音」权限绑定在证书上，换了证书就会再向用户要一次权限。
- **更新密钥**：`mygo-update.key`，用 `go tool mygo keygen` 生成，放在仓库以外（下面的命令假定在 `~/.config/tomoru/update-keys/`）。已安装的 app 只接受用它签名的更新，所以要在密码管理器里留一份：丢了它，所有已安装的 app 就再也收不到更新。它的公钥是 `app/mygo.json` 里的 `updates.publicKey`。
- **GitHub secret**，发布 workflow（`.github/workflows/release.yml`）用它们签名：

  ```sh
  base64 -i tomoru-signing.p12 | gh secret set MACOS_CERTIFICATE
  gh secret set MACOS_CERTIFICATE_PASSWORD   # .p12 的密码
  gh secret set MYGO_UPDATER_PRIVATE_KEY < ~/.config/tomoru/update-keys/mygo-update.key
  ```

## 每次发布

1. 在 `app/mygo.json` 里设版本号，在 `app/CHANGELOG.md` 写这个版本的一节（`## 0.1.1`）。这一节就是这个版本的更新说明，GitHub 的 release 页面和 app 的更新窗口都显示它。每一条末尾链接到它的 PR，没有 PR 就链接到它的提交：`([6a25ce5](https://github.com/ACAne0320/Tomoru/commit/6a25ce5))`。
2. 提交到 `main`，等 Checks 和 Tests 通过。
3. 推送这个版本的 tag：

   ```sh
   git tag v0.1.1
   git push origin v0.1.1
   ```

   Release workflow 检查 tag 和版本号一致、changelog 里有这一节，用这一节加上安装包的下载链接（GitHub 的，以及给中国大陆用的官网链接）作说明创建草稿 release，然后同时在 macOS 和 Windows（MSYS2）上构建：`scripts/setup.sh`、许可证发布检查，再 `go tool mygo build -upload`，签名 app 和更新包并上传到草稿。带连字符的 tag（`v0.2.0-beta.1`）发成预发布版。某个平台失败时，点「Re-run failed jobs」重新构建它，上传到同一个草稿。
4. 发布前先试用草稿里的文件。在 Mac 上用 `tomoru-<version>-arm64.dmg`：首次运行的权限、用 Apple 和本地识别听真实的声音、换语言和场景、快捷键、字幕位置、慢翻译之后的字幕记录、重开后的设置，以及工作中途退出。在 Windows 11 x64 上用 `tomoru-setup-<version>-amd64.exe`：从头走一遍引导、收听全部声音和单个应用、托盘的面板、快捷键、框选翻译、本地翻译模型，以及翻译中途退出。确认两边都能找到随包的引擎和库，且 `licenses/` 里有它们的 [许可证正文和声明](third-party-licenses_zh.md)：许可证检查只看打包前的资源，不看安装包。
5. 在 GitHub 上发布草稿。已安装的 app 在启动后不久检查一次，之后每天一次，那时就会发现它。半小时内官网会把它的安装包复制到 R2，下载按钮随之给出新版（[site/README_zh.md](../site/README_zh.md)）。

## 手动构建

不用 workflow 时，每个平台在 `gh` 已登录、更新密钥放在 `~/.config/tomoru/update-keys/` 的电脑上构建；`go tool mygo build -upload` 上传到这个版本的草稿 release，没有就创建一个。在 Mac 上（证书在登录钥匙串里）：

```sh
scripts/setup.sh
scripts/package-licenses.sh --release-check darwin-arm64
cd app
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

在 Windows 11 x64 上，用 MSYS2 的 MINGW64 终端，工具见 [README](../README_zh.md#windows)，另需 `diffutils`（`pacman -S diffutils`）；安装包用的 NSIS 由 MyGo 第一次构建时自己下载：

```sh
export PATH="$PATH:/c/Program Files/Go/bin"
scripts/setup.sh
scripts/package-licenses.sh --release-check windows-amd64
cd app
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

之后 `build/darwin-arm64/` 里有 app、磁盘映像、更新包、相对之前版本的增量包、`update-darwin-arm64.json` 和 `install.sh`；`build/windows-amd64/` 里有 `Tomoru.exe` 和旁边的 `bin/`、`vad/`、`licenses/`，以及安装包。Windows 版没有签名：SmartScreen 会在第一次启动时警告（点「更多信息」，再点「仍要运行」）。

## 用户第一次要做的

app 有签名但没有公证：macOS 会在第一次启动时拦下它。在下载页上写明：

> 打开 Tomoru。macOS 提示无法验证它时，打开 系统设置 › 隐私与安全性，点 Tomoru 旁边的「仍要打开」。

更新不需要用户做什么：更新程序检查它们的签名，app 的证书不变，权限也就保留。
