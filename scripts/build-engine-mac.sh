#!/bin/sh
set -e
cd "$(dirname "$0")/../engine/mac"
swift build -c release --arch arm64
dest=../../app/resources/darwin-arm64/bin
mkdir -p "$dest"
rm -f "$dest/tomoru-engine"
cp "$(swift build -c release --arch arm64 --show-bin-path)/tomoru-engine" "$dest/"
../../scripts/package-licenses.sh darwin-arm64
echo "built $dest/tomoru-engine"
