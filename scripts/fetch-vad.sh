#!/bin/sh
set -e
REV=b419383c55c110e2c9271fa6ee0ea83d03c70d96
BASE=https://huggingface.co/FluidInference/silero-vad-coreml/resolve/$REV/silero-vad-unified-v6.0.0.mlmodelc

cd "$(dirname "$0")/.."
dest=app/resources/darwin-arm64/vad/silero-vad.mlmodelc
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

while read -r sum file; do
	mkdir -p "$tmp/m/$(dirname "$file")"
	curl -fsSL -o "$tmp/m/$file" "$BASE/$file"
	echo "$sum  $tmp/m/$file" | sha256sum --strict -c - >/dev/null
done <<SUMS
2141be60ea0adf7acb1232fbcfaffb2be308ae02e6672d3762aedf36611ea9fd analytics/coremldata.bin
f460dcdf796b19c04bc38ab6e69601831f634e5e74d499487f0c8fe17ca12f0f coremldata.bin
282b1b788b5d4d66d6a70d468464a7ada71d2469a8705526b741fd9ab841295a metadata.json
79eea23c368f3e7edf85af798a953f5b0910e4b08ff48e55fff8545dd05fd047 model.mil
853cf34740d3f5061f977ebe2976f7c921b064261c9c4753b3a1196f2dba42b4 weights/weight.bin
SUMS

rm -rf "$dest"
mkdir -p "$(dirname "$dest")"
mv "$tmp/m" "$dest"
echo "put $dest"

onnx=$(dirname "$dest")/silero_vad.onnx
curl -fsSL -o "$tmp/silero_vad.onnx" https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx
echo "9e2449e1087496d8d4caba907f23e0bd3f78d91fa552479bb9c23ac09cbb1fd6  $tmp/silero_vad.onnx" | sha256sum --strict -c - >/dev/null
mv "$tmp/silero_vad.onnx" "$onnx"
echo "put $onnx"
for platform in windows-amd64 linux-amd64; do
	mkdir -p app/resources/$platform/vad
	cp "$onnx" app/resources/$platform/vad/
	echo "put app/resources/$platform/vad/silero_vad.onnx"
done
for platform in darwin-arm64 windows-amd64 linux-amd64; do
	scripts/package-licenses.sh $platform
done
