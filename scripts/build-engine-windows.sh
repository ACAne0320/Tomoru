#!/bin/sh
# Builds Tomoru's Windows programs on any computer, with mingw-w64's cross compiler
# (brew install mingw-w64), into app/resources/windows-amd64/bin/:
# - tomoru-engine.exe (engine/windows): hearing, speaking, selecting a part of the screen;
# - tomoru-asr.exe (engine/asr): recognition with downloaded models;
# - the sherpa-onnx libraries both load from beside them, which come with its Go module,
#   and its license in licenses/.
# The voice detector they load, vad/silero_vad.onnx, comes from scripts/fetch-vad.sh.
set -e
cd "$(dirname "$0")/.."
res=$PWD/app/resources/windows-amd64
mkdir -p "$res/bin" "$res/licenses"
export CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
(cd engine/windows && go build -trimpath -o "$res/bin/tomoru-engine.exe" .)
(cd engine/asr && go build -trimpath -o "$res/bin/tomoru-asr.exe" .)

mod=$(cd engine/asr && go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-windows)
for lib in sherpa-onnx-c-api.dll onnxruntime.dll; do
	rm -f "$res/bin/$lib" # the module cache's files are read-only, and so are their copies
	cp "$mod/lib/x86_64-pc-windows-gnu/$lib" "$res/bin/"
	chmod 644 "$res/bin/$lib"
done
scripts/package-licenses.sh windows-amd64
echo "built $res/bin/tomoru-engine.exe and tomoru-asr.exe"
