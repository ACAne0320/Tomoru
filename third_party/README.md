# Third-party notices

[中文](README_zh.md)

Tomoru's code is GPL-3.0-or-later. Third-party components retain their own terms.
The full texts collected here accompany the app in its `licenses/` directory.

`SOURCES.tsv` records each notice's component, version, original location and any
extraction. `SHA256SUMS` verifies those exact files. `DEPENDENCIES.sha256` records
the dependency declarations and resource download scripts this inventory was
prepared for. Changing a dependency requires reviewing its notices and updating
that inventory; recalculating a checksum alone is not a license review.

The inventory includes the Go runtime, the app's Go dependencies, MyGo's
Ebitengine-derived shader code, Lucide and Feather icons, sherpa-onnx and its
identified native dependencies, ONNX Runtime's complete upstream third-party
notices, Silero VAD, the FluidInference model declaration, llama.cpp's
identified bundled libraries, and the LLVM OpenMP runtime it ships on Windows. Optional upstream components may appear in these
notice sets even when the selected build does not use them.

The same notice set accompanies both platforms, so source attribution remains
available without guessing which upstream object files a linker retained.
System frameworks and system-installed DLLs are not copied into this directory.
Models downloaded after installation have their own model cards and terms;
they are not licensed by Tomoru's GPL or by this inventory.

See `REVIEW.md` for unresolved provenance and distribution requirements. The
presence of a license file does not clear every binary for redistribution.
