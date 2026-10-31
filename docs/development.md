# Development and validation

[中文](development_zh.md)

The repository holds three Go modules, `app`, `engine/asr` and `engine/windows`, and the
`engine/mac` Swift package. Read the [architecture](architecture.md) first for who owns what; the
product's and UI's rules are in the [design system](design-system.md).

## Tools and running

Desktop development needs Apple silicon, macOS 26, Go 1.27.3 (as the `go.mod` files say) and
Xcode 26 or its Command Line Tools (Swift 6.2 or later). Downloading Go dependencies and preparing the bundled resources for
the first time needs a network connection.

From the repository root:

```sh
scripts/setup.sh              # once, and again after changing engine/
cd app
go tool mygo dev              # rebuilds and restarts the app as you edit
go test ./...
go tool mygo build -sign -    # a local package in app/build/darwin-arm64/
```

`scripts/setup.sh` builds the engine and tomoru-asr and fetches the voice detector and
llama-server into `app/resources/<os>-<arch>/`. `go tool mygo dev` watches that directory too, so
running it, or only `scripts/build-engine-mac.sh`, while the app runs restarts the app with the new
engine. The development app is `Tomoru Dev` (`dev.acane.tomoru.dev`): it asks for permissions and
keeps its settings apart from an installed Tomoru. `go run . --guide` runs the first-run guide
again.

`-sign -` signs the app ad hoc for local testing and writes the package to
`app/build/darwin-arm64/`. It overrides `macos.signingIdentity` for this command, so contributors
do not need the release certificate or a change to `app/mygo.json`. `go tool mygo dev` also uses
ad-hoc signing by default. To keep a stable identity for local permission testing, pass your own
certificate with `-sign "Your certificate name"`; see MyGo's [build flags](https://github.com/egoist/mygo/blob/main/docs/cli.md#mygo-build).
Official releases use the existing certificate and update key, as described in the
[release guide](release.md).

The Windows version is built on Windows itself, in MSYS2's MINGW64 shell, as the
[README](../README.md#windows) says; `go tool mygo dev` runs it there as on a Mac. The modules of
the engine and tomoru-asr load the sherpa-onnx libraries from beside them, so their tests need
those on the `PATH`:

```sh
export PATH="$PATH:/c/Program Files/Go/bin:$PWD/app/resources/windows-amd64/bin"
(cd engine/asr && go test ./...)
(cd engine/windows && go test ./...)
scripts/check.sh --portable                    # MSYS2's MinGW-w64 is the compiler it asks for
pacman -S diffutils                            # once, for package-licenses.sh --release-check
```

It can be cross-compiled on a Mac as well, with mingw-w64 (`brew install mingw-w64`; the engine
and tomoru-asr call sherpa-onnx through cgo):

```sh
scripts/setup.sh windows-amd64   # tomoru-engine.exe, tomoru-asr.exe, the voice detector, llama-server
cd app && go tool mygo build -platform windows/amd64
```

The output is in `app/build/windows-amd64/` (`Tomoru.exe` with `bin/`, `vad/` and `licenses/`
beside it), and an installer, `tomoru-setup-<version>-amd64.exe`: on Windows MyGo downloads NSIS
for it the first time; on a Mac it needs NSIS installed (`brew install makensis`). The supported
Windows platform is Windows 11 x64. The process-loopback API requires
[Windows build 20348 or later](https://learn.microsoft.com/en-us/windows/win32/api/audioclientactivationparams/ns-audioclientactivationparams-audioclient_activation_params),
so Windows 10 is not supported.

The build scripts put programs in `app/resources/<os>-<arch>/bin/`. Models users download go in
the user data directory; unit tests don't need them. The directories and the protocol are in the
[architecture](architecture.md) and the [engine protocol](engine-protocol.md).

## One entry point for checks

From the repository root:

```sh
scripts/check.sh                   # macOS: the full local check
scripts/check.sh --full --race     # before a release, and in macOS CI
scripts/check.sh --portable        # no desktop runtime: the portable checks of the Go modules
scripts/check.sh --portable --race
```

The full check runs all the app's tests (UI, layering, design rules and the Go protocol contract),
the tests of the ASR and Windows engine modules, `gofmt`, Windows cross-builds of the app and both
engines, the Swift protocol checker, the hearer's suspension/resume contract, and a build of the
whole Swift engine. `HearerContract.swift` feeds synthetic PCM into the production hearer to check
the silence limit and continuous audio timestamps without starting capture or a model. The full check requires
mingw-w64 and fails if it is missing; Windows builds are part of the gate. The Swift module cache,
checkers and build outputs use a temporary directory that is removed afterwards. `--race` turns
on the race detector for the Go modules.

The portable check runs all of the app's domain and runtime packages, the benchmark, the static
layering and design checks, the ASR module and the platform-neutral tests of the Windows engine
module, and cross-builds the Windows app and both engines. It leaves out `ui` tests and the
`core/coretest` desktop adapter, so it doesn't exercise desktop windows or check the Swift engine.
It needs a native C compiler, because local ASR uses cgo, and mingw-w64 for the required Windows
builds (`brew install mingw-w64` on macOS, `apt install gcc-mingw-w64-x86-64` on Debian/Ubuntu).
Linux CI and macOS CI run one tier each; the macOS runner is pinned to `macos-26` ([image
list](https://github.com/actions/runner-images/blob/main/README.md)).

To test only what you changed:

```sh
cd app
go test ./internal/live ./internal/core
go test ./internal/ui/settings
go test layers_test.go design_test.go
cd ../engine/asr
go test ./...
```

If your environment forbids listening on loopback, tests using `httptest` need one that allows
local ports. Tests use only local fake servers and need no external API by default.

## Validation tiers

| Tier | What it covers | When |
| --- | --- | --- |
| Tests of affected packages | Current behavior, failure and cancellation paths | Every change |
| `--portable` | Domain rules, persistence, child processes, the Go protocol implementations, package boundaries and Windows app/engine builds | Linux CI; environments without a desktop runtime |
| `--full --race` | All of the above, plus UI tests, concurrency, Swift protocol and hearer contracts, the Swift engine build and Windows app/engine builds | macOS CI; the release gate |
| Real audio and desktop acceptance | Permissions, real audio, shortcuts, subtitles across screens, quitting, packaged resources | Before a release, and when platform behavior changes |
| A real Windows 11 x64 machine | `go test ./...` of the three modules and `scripts/check.sh --portable` in MSYS2 (or test binaries cross-compiled with `GOOS=windows GOARCH=amd64 go test -c`, run in the package directory), the whole guide, capture, screen selection, the tray | Windows changes, before a release |
| Optional real benchmark | Quality and latency with given audio, models or cloud services | Changes to recognition, segmenting or translation quality |

The benchmark's inputs, options and scoring are in the [benchmark guide](../bench/README.md).
Variables such as `TOMORU_E2E` turn on real end-to-end tests, and `TOMORU_DOWNLOAD` the download
tests; ordinary checks leave them unset, so nothing downloads or calls the cloud. UI galleries
write screenshots with `TOMORU_GALLERY`, to review layouts by eye.

## Where changes go

- UI controls edit copies; persisting, updating the glossary and switching sessions belong in app
  commands. A new menu needs a test that its callback still commits after its frame has ended.
- A new background task must say who cancels it, when it is waited for, and how stale results
  are rejected. A new child process needs a test that it exits when it stops responding.
- A new consumer of sentence records subscribes to `OnLine`; it must not guess completion from the
  ten-line display snapshot.
- A new package family updates the layering rules and their tests; don't open up a whole upper
  layer to make a test pass.
- A protocol change updates the shared fixtures, the contract checks of all four implementations
  (app, tomoru-asr, the Windows engine, Swift) and the protocol document together.
- Platform differences stay at the edges (engines, `capture`, `resource.Hide`, the tray icon in
  `shell`); when only the wording differs on screen, use `shared.OnWindows`. Domain packages
  never check the platform.

Releasing and signing are in the [release guide](release.md). How to contribute is in
[CONTRIBUTING](../CONTRIBUTING.md).
