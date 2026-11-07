# 发布来源审核

[English](REVIEW.md)

此目录的声明覆盖已识别的组件。发布二进制前，还需针对该文件的确切来源处理下列事项。
复制许可的脚本不能代替这项审核。

## Windows LLVM OpenMP 运行库：已解决

llama.cpp `b9840` 从 Visual Studio 的 `debug_nonredist` 目录复制 `libomp140.x86_64.dll`，
微软不允许再分发这个文件。自 [ggml-org/llama.cpp#26678](https://github.com/ggml-org/llama.cpp/pull/26678)
起，它的 Windows 发布包改用 LLVM 20.1.8 官方 Windows 安装包里的 `libomp.dll`，并附上该版本的
许可 `LICENSE-LLVM-OpenMP`。Tomoru 固定的 `b11960`（llama.cpp v0.8.0）已包含这项修改，许可收为
`llvm-openmp.txt`。发布检查仍会拒绝 `libomp140.x86_64.dll`。

## MyGo 源码：已解决

`app/go.mod` 已把 MyGo 固定到公开的正式版本 `v0.7.3`。`v0.7.2` 是第一个包含 Tomoru 需要的
Windows 修复和 `.icns` 图标支持的版本；`v0.7.3` 还修复了 Windows 上缩放窗口时画面被放大的问题。
它的许可文件和依赖与 `v0.7.2` 相同。发布检查仍会拒绝对 MyGo 的本地 `replace`。

## 需要保留的模型和原生库来源

- **ONNX Runtime：** sherpa-onnx v1.13.8 提供的两个平台动态库都标识为 1.28.2，与其固定的
  [macOS](https://github.com/k2-fsa/sherpa-onnx/blob/11afbd009a7f8c08f4bcf2fc1b265d0df4670fbf/cmake/onnxruntime-osx-arm64.cmake)
  和 [Windows](https://github.com/k2-fsa/sherpa-onnx/blob/11afbd009a7f8c08f4bcf2fc1b265d0df4670fbf/cmake/onnxruntime-win-x64.cmake)
  构建文件一致。清单保存了 1.28.2 完整的上游第三方声明。
- **Silero ONNX：** 用 `scripts/fetch-vad.sh` 的 SHA-256 标识该文件；sherpa-onnx
  `asr-models` 的发布资产元数据也给出这一哈希。不要因为另一份 Core ML 模型基于 v6，就把此
  ONNX 文件标为 v6。Silero v5.1.2 和 v6.0 的 MIT 正文逐字节相同。
- **FluidInference：** 下载的确切模型修订在模型卡中声明 MIT，并注明 Silero 和 FluidAudio。
  清单保留完整模型卡；该修订没有独立 LICENSE 文件。转换项目当前源码的 Apache-2.0
  许可不替代模型的 MIT 声明；如需额外的转换版权声明，应向模型发布者确认。
- **sherpa-onnx：** 虽然 Tomoru 只使用识别和 VAD，预编译 C API 库仍含 eSpeak NG 等 TTS
  组件。已收集 eSpeak NG 的 GPL、piper-phonemize、uni-algo、Kaldi、OpenFst、
  simple-sentencepiece、Darts、KISS FFT、Eigen 和 hclust-cpp 的声明。需要保留实际
  二进制对应的源码、构建说明和上游补丁；只附包装层的 Apache 许可不够。
- **llama.cpp：** 记录 `b11960`（v0.8.0）、commit `8642541b255bfb5dc39cd351a178be09013a55d6`
  以及 `scripts/fetch-llama.sh` 的平台归档哈希。清单包含已识别的 JSON、HTTP、图片、
  子进程、哈希（xxHash、rotate-bits；SHA-1 和 SHA-256 属公有领域）和 BoringSSL 组件声明，
  以及 Windows 上 LLVM 的 OpenMP 运行库。仍需检查上游发布包内嵌的网页 UI 及其他启用功能是否有
  附加声明；本清单不是该上游二进制的完整 SBOM。
- **Lucide：** SVG 来自 `lucide-static` 0.469.0（每个文件头部都写着）；该版本的上游
  LICENSE 放在 SVG 旁边和 `lucide.txt` 中，另附 Feather MIT 声明。更新图标时换成同一版本的声明。

这些运行时声明不授予再分发下载的 SenseVoice、Index-Translate、Hy-MT2 权重或外部基准媒体的权利。
分发此类文件前应检查各自的模型卡和许可。普通模型下载和本地基准输入不属于发布归档。
