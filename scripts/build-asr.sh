#!/bin/sh
set -e
cd "$(dirname "$0")/../engine/asr"
res=../../app/resources/darwin-arm64
mkdir -p "$res/bin" "$res/licenses"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -o "$res/bin/tomoru-asr" .

mod=$(go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-macos)
for lib in libsherpa-onnx-c-api.dylib libonnxruntime.dylib; do
	rm -f "$res/bin/$lib"
	cp "$mod/lib/aarch64-apple-darwin/$lib" "$res/bin/"
	chmod 755 "$res/bin/$lib"
done
bin="$res/bin/tomoru-asr"
for p in $(otool -l "$bin" | awk '/LC_RPATH/{getline; getline; print $2}'); do
	install_name_tool -delete_rpath "$p" "$bin"
done
install_name_tool -add_rpath @executable_path "$bin" 2>/dev/null
codesign -f -s - "$bin" 2>/dev/null
../../scripts/package-licenses.sh darwin-arm64
echo "built $bin"
