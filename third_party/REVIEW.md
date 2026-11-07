# Release provenance review

[中文](REVIEW_zh.md)

The notices in this directory cover identified components. Before distributing
a binary, resolve the following items against that binary's exact provenance.
The notice packaging script does not replace this review.

## Windows LLVM OpenMP runtime: resolved

llama.cpp `b9840` copied `libomp140.x86_64.dll` from Visual Studio's
`debug_nonredist` directory, which Microsoft does not allow to be redistributed.
Since [ggml-org/llama.cpp#26678](https://github.com/ggml-org/llama.cpp/pull/26678)
its Windows release ships `libomp.dll` from the official LLVM 20.1.8 Windows
installer, with that release's license as `LICENSE-LLVM-OpenMP`. Tomoru pins
`b11960` (llama.cpp v0.8.0), which has it, and collects the license as
`llvm-openmp.txt`. The release check still rejects `libomp140.x86_64.dll`.

## MyGo source: resolved

`app/go.mod` pins MyGo to the public release `v0.7.3`. `v0.7.2` was the first
with the Windows fixes and the `.icns` icon Tomoru needs; `v0.7.3` also stops
magnifying a window's frames while it is resized on Windows. Its license files
and dependencies are those of `v0.7.2`. The release check still
rejects a local `replace` of MyGo.

## Model and native-library provenance to retain

- **ONNX Runtime:** both supplied sherpa-onnx v1.13.8 libraries identify version
  1.28.2. This agrees with the pinned sherpa-onnx
  [macOS](https://github.com/k2-fsa/sherpa-onnx/blob/11afbd009a7f8c08f4bcf2fc1b265d0df4670fbf/cmake/onnxruntime-osx-arm64.cmake)
  and [Windows](https://github.com/k2-fsa/sherpa-onnx/blob/11afbd009a7f8c08f4bcf2fc1b265d0df4670fbf/cmake/onnxruntime-win-x64.cmake)
  build files. The full 1.28.2 upstream notice set is included.
- **Silero ONNX:** identify it by the SHA-256 in `scripts/fetch-vad.sh`, also
  published in the sherpa-onnx `asr-models` release asset metadata. Do not label
  this asset v6 merely because the separate Core ML model uses v6. The Silero
  v5.1.2 and v6.0 MIT license files are byte-identical.
- **FluidInference:** the exact downloaded model revision declares MIT in its
  model card and credits Silero and FluidAudio. That complete card is preserved;
  the revision has no standalone LICENSE file. The conversion projects' current
  Apache-2.0 source license is not substituted for the model's declaration.
  Obtain any additional conversion-specific notice from its publisher if needed.
- **sherpa-onnx:** its prebuilt C API library contains TTS components, including
  eSpeak NG, even though Tomoru uses it for recognition and VAD. The collected
  native notices include eSpeak NG's GPL, piper-phonemize, uni-algo, Kaldi,
  OpenFst, simple-sentencepiece, Darts, KISS FFT, Eigen and hclust-cpp. Preserve
  corresponding source and build instructions for the actual binary, including
  any upstream patches; an Apache notice for the wrapper alone is insufficient.
- **llama.cpp:** record release `b11960` (v0.8.0), commit
  `8642541b255bfb5dc39cd351a178be09013a55d6`, and the platform archive hashes in
  `scripts/fetch-llama.sh`. Notices for identified vendored JSON, HTTP, image,
  subprocess, hashing (xxHash, rotate-bits; SHA-1 and SHA-256 are public domain)
  and BoringSSL code are included, and on Windows LLVM's OpenMP runtime. Review the upstream release's
  embedded web UI and other enabled build features for additional notices;
  this inventory is not an exhaustive SBOM of that upstream binary.
- **Lucide:** the SVG files are from `lucide-static` 0.469.0, as each file's
  header says; its upstream LICENSE sits beside them and in `lucide.txt`, with
  the Feather MIT notice. Use the same version's notice when refreshing icons.

The runtime notices do not grant rights to redistribute downloaded SenseVoice,
Index-Translate or Hy-MT2 weights, or externally sourced benchmark media. Review their
own model cards and licenses before redistributing those files. Ordinary model
downloads and local benchmark inputs remain separate from the release archive.
