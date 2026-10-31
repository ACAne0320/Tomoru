#!/bin/sh
# Repository checks; no model downloads, credentials, microphone or system audio needed.
set -eu

mode=full
race=
for arg in "$@"; do
    case "$arg" in
        --portable) mode=portable ;;
        --full) mode=full ;;
        --race) race=-race ;;
        --help)
            echo "Usage: scripts/check.sh [--full|--portable] [--race]"
            echo "Full (default): notices, the Go modules, UI and architecture checks, Windows builds, Swift contracts and build."
            echo "Portable: notices, headless Go packages, the Go modules, Windows builds and static architecture/design checks."
            exit 0 ;;
        *) echo "Unknown option: $arg" >&2; exit 2 ;;
    esac
done
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ "$mode" = full ] && [ "$(uname -s)" != Darwin ]; then
    echo "Full checks require macOS 26 and Xcode 26; use --portable for headless Go checks." >&2
    exit 2
fi
if ! command -v x86_64-w64-mingw32-gcc >/dev/null; then
    echo "Windows build checks require x86_64-w64-mingw32-gcc (brew install mingw-w64; on Debian/Ubuntu: apt install gcc-mingw-w64-x86-64)." >&2
    exit 2
fi
check_tmp=$(mktemp -d "${TMPDIR:-/tmp}/tomoru-check.XXXXXX")
trap 'rm -rf "$check_tmp"' EXIT HUP INT TERM
mkdir "$check_tmp/windows"
"$root/scripts/package-licenses.sh" --check

(
    cd "$root/app"
    if [ "$mode" = full ]; then
        go test -count=1 $race ./...
    else
        # Exclude the UI and its desktop test adapter, not domain packages that
        # happen to be imported by them. The benchmark is a headless entrypoint.
        packages=$(go list ./internal/... ./cmd/... | awk '!/^tomoru\/internal\/ui\// && !/^tomoru\/internal\/core\/coretest($|\/)/')
        go test -count=1 $race $packages
        go test -count=1 layers_test.go design_test.go
    fi
)
(
    cd "$root/engine/asr"
    go test -count=1 $race ./...
)
(
    # The Windows engine's hearing, files and protocol run anywhere; its calls into Windows are
    # checked below.
    cd "$root/engine/windows"
    go test -count=1 $race ./...
)

# Formatting, and the Windows build: the app needs no cgo for it, the engines a cross compiler.
unformatted=$(cd "$root" && gofmt -l app engine/asr engine/windows)
if [ -n "$unformatted" ]; then
    echo "gofmt needed:" >&2
    echo "$unformatted" >&2
    exit 1
fi
(
    cd "$root/app"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$check_tmp/windows/Tomoru.exe" .
)
for m in engine/windows engine/asr; do
    (
        cd "$root/$m"
        export CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
        go vet ./...
        go build -trimpath -o "$check_tmp/windows/$(basename "$m").exe" .
    )
done
echo "Windows app and engine builds passed."

if [ "$mode" = full ]; then
    export CLANG_MODULE_CACHE_PATH="$check_tmp/module-cache"
    export SWIFTPM_MODULECACHE_OVERRIDE="$check_tmp/module-cache"
    swiftc -module-cache-path "$check_tmp/module-cache" \
        "$root/engine/mac/Sources/tomoru-engine/Protocol.swift" \
        "$root/engine/mac/Tests/ProtocolContract.swift" -o "$check_tmp/protocol-contract"
    "$check_tmp/protocol-contract" "$root/protocol/fixtures.json"
    swiftc -target "$(uname -m)-apple-macos26.0" -module-cache-path "$check_tmp/module-cache" \
        "$root/engine/mac/Sources/tomoru-engine/Protocol.swift" \
        "$root/engine/mac/Sources/tomoru-engine/Hear.swift" \
        "$root/engine/mac/Sources/tomoru-engine/Voice.swift" \
        "$root/engine/mac/Sources/tomoru-engine/Capture.swift" \
        "$root/engine/mac/Sources/tomoru-engine/File.swift" \
        "$root/engine/mac/Tests/HearerContract.swift" -o "$check_tmp/hearer-contract"
    "$check_tmp/hearer-contract"
    swift build --package-path "$root/engine/mac" --scratch-path "$check_tmp/swift-build" --disable-sandbox
fi

echo "Repository checks passed ($mode${race:+, race})."
