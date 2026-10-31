#!/bin/sh
# Static checks, with no tests and no builds: the license inventory, gofmt, go vet for macOS and
# Windows, the app's layering and design rules, and a type check of the Swift engine (macOS only).
set -eu

go=0
swift=0
for arg in "$@"; do
    case "$arg" in
        go) go=1 ;;
        swift) swift=1 ;;
        --help)
            echo "Usage: scripts/check.sh [go] [swift]"
            echo "go: licenses, gofmt, go vet for macOS and Windows, layering and design rules."
            echo "swift: type-checks the Swift engine (macOS)."
            echo "With neither, both, swift only on macOS."
            exit 0 ;;
        *) echo "Unknown option: $arg" >&2; exit 2 ;;
    esac
done
if [ "$go$swift" = 00 ]; then
    go=1
    [ "$(uname -s)" = Darwin ] && swift=1
fi
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$go" = 1 ]; then
    if ! command -v x86_64-w64-mingw32-gcc >/dev/null; then
        echo "Vetting the engines for Windows needs x86_64-w64-mingw32-gcc (brew install mingw-w64; on Debian/Ubuntu: apt install gcc-mingw-w64-x86-64)." >&2
        exit 2
    fi
    "$root/scripts/package-licenses.sh" --check
    unformatted=$(cd "$root" && gofmt -l app engine/asr engine/windows)
    if [ -n "$unformatted" ]; then
        echo "gofmt needed:" >&2
        echo "$unformatted" >&2
        exit 1
    fi
    (
        # The app needs no cgo: it vets for both platforms anywhere.
        cd "$root/app"
        GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./...
        GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
        go test -count=1 layers_test.go design_test.go
    )
    for m in engine/asr engine/windows; do
        (
            cd "$root/$m"
            go vet ./...
            CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go vet ./...
        )
    done
    echo "Go checks passed."
fi

if [ "$swift" = 1 ]; then
    if [ "$(uname -s)" != Darwin ]; then
        echo "The Swift engine is checked on macOS 26 with Xcode 26." >&2
        exit 2
    fi
    cache=$(mktemp -d "${TMPDIR:-/tmp}/tomoru-check.XXXXXX")
    trap 'rm -rf "$cache"' EXIT HUP INT TERM
    export CLANG_MODULE_CACHE_PATH="$cache"
    swiftc -typecheck -swift-version 5 -target "$(uname -m)-apple-macos26.0" -module-cache-path "$cache" \
        "$root"/engine/mac/Sources/tomoru-engine/*.swift
    echo "Swift checks passed."
fi
