#!/bin/sh
# Builds the macOS engine (engine/mac) into the app's resources, where `go run`, `go test` and
# `go tool mygo build` find it: app/resources/darwin-arm64/bin/tomoru-engine.
set -e
cd "$(dirname "$0")/../engine/mac"
swift build -c release --arch arm64
dest=../../app/resources/darwin-arm64/bin
mkdir -p "$dest"
# A new file, not one written over: macOS kills a program started from a file written over while
# it ran (the app's engine, say), its signature cached as the old one's.
rm -f "$dest/tomoru-engine"
cp "$(swift build -c release --arch arm64 --show-bin-path)/tomoru-engine" "$dest/"
../../scripts/package-licenses.sh darwin-arm64
echo "built $dest/tomoru-engine"
