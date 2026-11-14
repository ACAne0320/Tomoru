# 第三方许可与发布声明

[English](third-party-licenses.md)

Tomoru 的 GPL-3.0-or-later 许可覆盖自身代码。依赖、图标和下载的模型保留各自的条款。
仓库中的[许可清单](../third_party/README.md)保存许可正文、[来源和版本](../third_party/SOURCES.tsv)
以及校验值。发布相应平台的程序前，需要处理[尚未解决的来源与分发事项](../third_party/REVIEW.md)。

| 组件 | 使用的版本或来源 | 已收集的声明 |
| --- | --- | --- |
| Go 运行时和 app 依赖 | Go 1.27.3；三个 `go.mod` 中的版本 | Go BSD、websocket ISC、purego Apache、typesetting 和 HarfBuzz 声明，以及 `x/image`、`x/net`（Windows）、`x/sys`、`x/text` 的许可和专利授权 |
| MyGo | `v0.9.0` | MIT；其 Ebitengine 派生 shader 代码的 Apache 许可；其权限引导参照 PermissionFlow，附其 MIT 声明 |
| Lucide / Feather | lucide-static 0.469.0，仓库中的 SVG 子集 | 原有 ISC 声明和 Feather MIT 声明 |
| sherpa-onnx | v1.13.8 | Apache-2.0 及已识别的原生依赖声明，包括采用 GPL 的 eSpeak NG |
| ONNX Runtime | 1.28.2，已与两平台动态库及 sherpa 构建配置核对 | MIT 和完整的上游 `ThirdPartyNotices.txt` |
| Silero VAD | Core ML 模型基于 v6；ONNX 文件以固定哈希标识 | Silero MIT |
| FluidInference Core ML 转换 | `b419383c55c110e2c9271fa6ee0ea83d03c70d96` | 该修订的完整模型卡，含 MIT 声明和作者信息 |
| llama.cpp / ggml | `b11960`（v0.8.0）和各平台归档哈希 | MIT 及已识别的内嵌组件许可；仍需审核上游二进制的完整来源 |
| LLVM OpenMP（Windows） | 20.1.8，llama.cpp Windows 归档中的 `libomp.dll` | Apache-2.0 附 LLVM 例外 |

同一份许可集会复制到两个平台的 `licenses/` 目录，其中可能包含 Tomoru 未使用的上游可选功能。
脚本没有捆绑 macOS 系统框架或 Windows 系统已安装的 DLL。系统组件和额外复制进 app 的运行库，
分发条件可能不同。

在仓库根目录执行：

```sh
scripts/package-licenses.sh --check
scripts/package-licenses.sh darwin-arm64
scripts/package-licenses.sh windows-amd64
scripts/package-licenses.sh --release-check darwin-arm64
scripts/package-licenses.sh --release-check windows-amd64
```

第一条检查已保存的许可文件、来源清单和依赖声明。复制过程不联网；资源构建和下载脚本也会自动复制。
发布检查会将 `app/resources/` 中的许可包与源码核对，并拒绝已知的发布阻断项。
它不检查最终安装包，也不代表所有上游二进制的许可审核已经完成。

发布二进制前还需审核的事项见[来源审核记录](../third_party/REVIEW.md)。

升级依赖时，先审阅新版本许可和原生依赖，更新正文、`SOURCES.tsv` 以及待办事项，再为
`third_party/licenses/` 的文件重新生成 `third_party/SHA256SUMS`，并针对原有依赖声明文件列表
更新 `third_party/DEPENDENCIES.sha256`。重新复制许可包，确认构建后的 app 中也包含这些文件，
然后再上传。

SenseVoice、Qwen3-ASR、Index-Translate 和 Hy-MT2（Apache-2.0）权重在安装后下载，不会因为与 Tomoru 一起使用就改为 GPL。
镜像或再分发权重前需要检查模型卡；基准视频与音频也保留原有条款。
