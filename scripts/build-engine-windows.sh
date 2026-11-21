#!/bin/sh
set -e
cd "$(dirname "$0")/.."
res=$PWD/app/resources/windows-amd64
mkdir -p "$res/bin" "$res/licenses"
export CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
(cd engine/windows && go build -trimpath -o "$res/bin/tomoru-engine.exe" .)
(cd engine/asr && go build -trimpath -o "$res/bin/tomoru-asr.exe" .)

mod=$(cd engine/asr && go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-windows)
for lib in sherpa-onnx-c-api.dll onnxruntime.dll; do
	rm -f "$res/bin/$lib"
	cp "$mod/lib/x86_64-pc-windows-gnu/$lib" "$res/bin/"
	chmod 644 "$res/bin/$lib"
done
scripts/package-licenses.sh windows-amd64
echo "built $res/bin/tomoru-engine.exe and tomoru-asr.exe"
