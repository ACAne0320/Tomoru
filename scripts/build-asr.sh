#!/bin/sh
# Builds tomoru-asr (engine/asr), recognition with downloaded models, into the app's resources:
# app/resources/darwin-arm64/bin/tomoru-asr and the sherpa-onnx libraries it loads from beside it
# (@executable_path), with sherpa-onnx's license in app/resources/darwin-arm64/licenses/.
#
# sherpa-onnx is pinned in engine/asr/go.mod; its libraries come with its Go module.
set -e
cd "$(dirname "$0")/../engine/asr"
res=../../app/resources/darwin-arm64
mkdir -p "$res/bin" "$res/licenses"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -o "$res/bin/tomoru-asr" .

mod=$(go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-macos)
for lib in libsherpa-onnx-c-api.dylib libonnxruntime.dylib; do
	rm -f "$res/bin/$lib" # the module cache's files are read-only, and so are their copies
	cp "$mod/lib/aarch64-apple-darwin/$lib" "$res/bin/"
	chmod 755 "$res/bin/$lib"
done
# cgo points the binary at the module cache: point it beside itself instead.
bin="$res/bin/tomoru-asr"
for p in $(otool -l "$bin" | awk '/LC_RPATH/{getline; getline; print $2}'); do
	install_name_tool -delete_rpath "$p" "$bin"
done
install_name_tool -add_rpath @executable_path "$bin" 2>/dev/null
codesign -f -s - "$bin" 2>/dev/null # install_name_tool broke the linker's signature
../../scripts/package-licenses.sh darwin-arm64
echo "built $bin"
