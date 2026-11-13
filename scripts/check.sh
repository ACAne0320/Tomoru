#!/bin/sh
# Static checks, with no tests and no builds: the license inventory, gofmt, go vet, the linters of
# .golangci.yml and unreachable functions for macOS and Windows, the app's layering and design
# rules, and a type check of the Swift engine (macOS only).
set -eu

go=0
swift=0
for arg in "$@"; do
    case "$arg" in
        go) go=1 ;;
        swift) swift=1 ;;
        --help)
            echo "Usage: scripts/check.sh [go] [swift]"
            echo "go: licenses, gofmt, go vet, linters and unreachable functions for macOS and Windows,"
            echo "    layering and design rules. The linters are built once into ~/.cache/tomoru-tools."
            echo "swift: type-checks the Swift engine (macOS), warnings as errors."
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

# tool <name> <package> <version> prints the path of a pinned Go tool, built the first time.
tool() {
    dir="${TOMORU_TOOLS:-$HOME/.cache/tomoru-tools}/$1-$3"
    if [ ! -x "$dir/$1" ] && [ ! -x "$dir/$1.exe" ]; then
        (cd "$root/app" && GOBIN="$dir" go install "$2@$3") >&2
    fi
    echo "$dir/$1"
}

# analyze checks the module it runs in, for the platform the environment names.
analyze() {
    go vet ./...
    "$lint" run ./...
    # Only tests reach core/coretest, the desktop's test adapter.
    unreachable=$("$deadcode" ./...)
    unreachable=$(echo "$unreachable" | grep -v '^internal/core/coretest/' || true)
    if [ -n "$unreachable" ]; then
        echo "Nothing reaches these${GOOS:+ on $GOOS}: remove them, or move a test's helper to a _test.go file." >&2
        echo "$unreachable" >&2
        exit 1
    fi
}

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
    lint=$(tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint v2.15.0)
    deadcode=$(tool deadcode golang.org/x/tools/cmd/deadcode v0.52.0)
    (
        # The app needs no cgo: it is checked for both platforms anywhere.
        cd "$root/app"
        for target in darwin/arm64 windows/amd64; do
            export GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0
            analyze
        done
        unset GOOS GOARCH CGO_ENABLED
        go test -count=1 layers_test.go design_test.go
    )
    for m in engine/asr engine/windows; do
        (
            cd "$root/$m"
            analyze
            export CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
            analyze
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
    swiftc -typecheck -warnings-as-errors -swift-version 5 -target "$(uname -m)-apple-macos26.0" -module-cache-path "$cache" \
        "$root"/engine/mac/Sources/tomoru-engine/*.swift
    echo "Swift checks passed."
fi
