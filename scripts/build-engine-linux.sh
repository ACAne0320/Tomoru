#!/bin/sh
set -e
cd "$(dirname "$0")/.."
res=$PWD/app/resources/linux-amd64
mkdir -p "$res/bin" "$res/licenses"
export CGO_ENABLED=1 GOOS=linux GOARCH=amd64
(cd engine/linux && go build -trimpath -o "$res/bin/tomoru-engine" .)
(cd engine/asr && go build -trimpath -o "$res/bin/tomoru-asr" .)

mod=$(cd engine/asr && go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-linux)
for lib in libsherpa-onnx-c-api.so libonnxruntime.so; do
	rm -f "$res/bin/$lib"
	cp "$mod/lib/x86_64-unknown-linux-gnu/$lib" "$res/bin/"
	chmod 755 "$res/bin/$lib"
done
for bin in tomoru-engine tomoru-asr; do
	patchelf --set-rpath '$ORIGIN' "$res/bin/$bin"
done
scripts/package-licenses.sh linux-amd64
echo "built $res/bin/tomoru-engine and tomoru-asr"
