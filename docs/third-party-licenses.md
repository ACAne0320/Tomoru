# Third-party licenses and release notices

[中文](third-party-licenses_zh.md)

Tomoru's GPL-3.0-or-later license covers its own code. Its dependencies, icons and
downloaded models keep their own terms. The checked-in [notice inventory](../third_party/README.md)
contains the original license texts, their [sources and versions](../third_party/SOURCES.tsv),
and checksums. [Outstanding provenance requirements](../third_party/REVIEW.md)
must be resolved before publishing the affected binary.

| Component | Version/source used | Collected notices |
| --- | --- | --- |
| Go runtime and app dependencies | Go 1.27.3; versions in the three `go.mod` files | Go BSD, websocket ISC, purego Apache, typesetting and HarfBuzz notices, `x/image`, `x/net` (Windows), `x/sys`, `x/text` licenses and patent grants |
| MyGo | `v0.7.3` | MIT; Apache notice for its Ebitengine-derived shader code |
| Lucide / Feather | lucide-static 0.469.0, SVG subset | Original ISC notice and Feather MIT notice |
| sherpa-onnx | v1.13.8 | Apache-2.0 plus identified native component notices, including GPL eSpeak NG |
| ONNX Runtime | 1.28.2, verified against both bundled libraries and sherpa's build files | MIT and the complete upstream `ThirdPartyNotices.txt` |
| Silero VAD | Core ML model based on v6; ONNX asset identified by its pinned hash | Silero MIT |
| FluidInference Core ML conversion | `b419383c55c110e2c9271fa6ee0ea83d03c70d96` | Exact model card carrying its MIT declaration and attribution |
| llama.cpp / ggml | `b11960` (v0.8.0) and platform archive hashes | MIT and identified vendored component notices; upstream binary provenance review remains necessary |
| LLVM OpenMP (Windows) | 20.1.8, `libomp.dll` in llama.cpp's Windows archive | Apache-2.0 with LLVM exception |

The notice set is copied into both platforms' `licenses/` directories. It can
include optional upstream features not used by Tomoru. macOS system frameworks
and Windows system-installed DLLs are not bundled by these scripts. In
particular, a system DLL and an extra runtime copied into the app must not be
treated as the same distribution case.

Run these commands from the repository root:

```sh
scripts/package-licenses.sh --check
scripts/package-licenses.sh darwin-arm64
scripts/package-licenses.sh windows-amd64
scripts/package-licenses.sh --release-check darwin-arm64
scripts/package-licenses.sh --release-check windows-amd64
```

The first command checks the tracked license files, source inventory and
dependency declarations. Copying is offline; the resource build/fetch scripts
also run it automatically. The release check verifies the `app/resources/`
notice bundle against those sources and rejects known blockers. It does not
inspect the final installer or certify every upstream binary's licensing.

What remains to review before publishing a binary is in [the provenance review](../third_party/REVIEW.md).

When changing a dependency, read the new version's license and native build
dependencies, update the notice files and `SOURCES.tsv`, and review the open
items. Then regenerate `third_party/SHA256SUMS` for the files in
`third_party/licenses/` and `third_party/DEPENDENCIES.sha256` for the same listed
dependency declaration files. Repackage the notices and verify the built app
contains them before uploading.

SenseVoice, Qwen3-ASR, Index-Translate and Hy-MT2 (Apache-2.0) weights are downloaded after installation. Their
model licenses do not become GPL by being used with Tomoru. Check their model
cards before mirroring or redistributing weights; benchmark videos and audio
also retain their original terms.
