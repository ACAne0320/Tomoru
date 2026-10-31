# 第三方许可声明

[English](README.md)

Tomoru 的代码采用 GPL-3.0-or-later。第三方组件保留各自的许可条款。
此目录收集的许可正文会随 app 放在 `licenses/` 目录中。

`SOURCES.tsv` 记录各份声明对应的组件、版本、原始位置和摘录方式。
`SHA256SUMS` 校验这些文件。`DEPENDENCIES.sha256` 记录本次清单对应的依赖声明和资源下载脚本。
升级依赖时，应审查并更新许可清单；仅重新计算哈希不等于完成许可审核。

清单涵盖 Go 运行时、app 的 Go 依赖、MyGo 中源自 Ebitengine 的 shader 代码、Lucide 和 Feather
图标、sherpa-onnx 及已识别的原生依赖、ONNX Runtime 完整的上游第三方声明、Silero VAD、
FluidInference 模型声明、llama.cpp 中已识别的内嵌库，以及它在 Windows 上附带的 LLVM OpenMP 运行库。
部分上游声明可能涵盖当前构建未启用的可选功能。

两个平台携带同一套声明，不根据链接器可能保留的对象文件猜测哪些版权信息可以省略。
系统框架和系统已安装的 DLL 不会复制进此目录。安装后下载的模型有自己的模型卡和条款，
不由 Tomoru 的 GPL 或本清单授权。

尚未解决的来源与分发要求见 `REVIEW_zh.md`。存在某份许可文件，不代表相关二进制的所有
再分发要求均已满足。
