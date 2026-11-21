#!/bin/sh
set -e
cd "$(dirname "$0")"
case $(uname -s) in
MINGW* | MSYS*) platform=${1:-windows-amd64} ;;
Linux) platform=${1:-linux-amd64} ;;
*) platform=${1:-darwin-arm64} ;;
esac
case $platform in
darwin-arm64)
	./build-engine-mac.sh
	./build-asr.sh ;;
windows-amd64)
	./build-engine-windows.sh ;;
linux-amd64)
	./build-engine-linux.sh ;;
*)
	echo "unknown platform $platform" >&2
	exit 2 ;;
esac
./fetch-vad.sh
./fetch-llama.sh "$platform"
