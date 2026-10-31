#!/bin/sh
# Tests, uncached: no model downloads, credentials, microphone or system audio needed.
set -eu

go=0
ui=0
swift=0
race=
for arg in "$@"; do
    case "$arg" in
        go) go=1 ;;
        ui) ui=1 ;;
        swift) swift=1 ;;
        --race) race=-race ;;
        --help)
            echo "Usage: scripts/test.sh [go] [ui] [swift] [--race]"
            echo "go: the Go modules' tests but the UI's, anywhere."
            echo "ui: the app's UI tests (macOS and Windows)."
            echo "swift: the Swift engine's protocol and hearer contracts (macOS)."
            echo "With none, all, ui and swift on macOS only. --race turns on Go's race detector."
            exit 0 ;;
        *) echo "Unknown option: $arg" >&2; exit 2 ;;
    esac
done
if [ "$go$ui$swift" = 000 ]; then
    go=1
    if [ "$(uname -s)" = Darwin ]; then
        ui=1
        swift=1
    fi
fi
if [ "$swift" = 1 ] && [ "$(uname -s)" != Darwin ]; then
    echo "The Swift tests run on macOS 26 with Xcode 26." >&2
    exit 2
fi
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$go" = 1 ]; then
    (
        cd "$root/app"
        # The UI and its desktop test adapter need a desktop; the benchmark is headless.
        packages=$(go list ./internal/... ./cmd/... | awk '!/^tomoru\/internal\/ui\// && !/^tomoru\/internal\/core\/coretest($|\/)/')
        go test -count=1 $race $packages
    )
    for m in engine/asr engine/windows; do
        (cd "$root/$m" && go test -count=1 $race ./...)
    done
fi

if [ "$ui" = 1 ]; then
    (cd "$root/app" && go test -count=1 $race ./internal/ui/...)
fi

if [ "$swift" = 1 ]; then
    out=$(mktemp -d "${TMPDIR:-/tmp}/tomoru-test.XXXXXX")
    trap 'rm -rf "$out"' EXIT HUP INT TERM
    export CLANG_MODULE_CACHE_PATH="$out/cache"
    src="$root/engine/mac/Sources/tomoru-engine"
    swiftc -module-cache-path "$out/cache" "$src/Protocol.swift" \
        "$root/engine/mac/Tests/ProtocolContract.swift" -o "$out/protocol-contract"
    "$out/protocol-contract" "$root/protocol/fixtures.json"
    # Synthetic PCM through the production hearer: the silence limit and continuous timestamps,
    # with no capture or model.
    swiftc -target "$(uname -m)-apple-macos26.0" -module-cache-path "$out/cache" \
        "$src/Protocol.swift" "$src/Hear.swift" "$src/Voice.swift" "$src/Capture.swift" "$src/File.swift" \
        "$root/engine/mac/Tests/HearerContract.swift" -o "$out/hearer-contract"
    "$out/hearer-contract"
fi

echo "Tests passed."
