#!/bin/sh
# Puts everything the app runs beside it into app/resources/<platform>/: the engine, local
# recognition, the voice detector and llama-server. Run it once, and again after changing engine/;
# go tool mygo dev restarts the app when it finishes.
#
#	scripts/setup.sh [darwin-arm64 | windows-amd64]   (default: this computer's)
set -e
cd "$(dirname "$0")"
case $(uname -s) in
MINGW* | MSYS*) platform=${1:-windows-amd64} ;;
*) platform=${1:-darwin-arm64} ;;
esac
case $platform in
darwin-arm64)
	./build-engine-mac.sh
	./build-asr.sh ;;
windows-amd64)
	./build-engine-windows.sh ;;
*)
	echo "unknown platform $platform" >&2
	exit 2 ;;
esac
./fetch-vad.sh
./fetch-llama.sh "$platform"
