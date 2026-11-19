# Development and validation

[中文](development_zh.md)

The repository holds five Go modules, `app`, `engine/asr`, `engine/shared`, `engine/windows` and
`engine/linux`, and the `engine/mac` Swift package. Read the [architecture](architecture.md) first for who owns what; the
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
again. `go tool mygo dev -background` starts it, and again after each rebuild, without bringing it
to the front, so the editor keeps the focus as you save.

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
(cd engine/shared && go test ./...)
scripts/check.sh go && scripts/test.sh go     # MSYS2's MinGW-w64 is the compiler they ask for
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
Windows platform is Windows 11 x64. All system audio uses endpoint loopback on the default
playback device and reconnects when that device changes. Apps routed to other outputs are not
heard in this mode. A selected app still uses process loopback; communication audio may be
missing there, so use all system audio with the app playing to the default output.
The process-loopback API requires
[Windows build 20348 or later](https://learn.microsoft.com/en-us/windows/win32/api/audioclientactivationparams/ns-audioclientactivationparams-audioclient_activation_params),
so Windows 10 is not supported.

The supported Linux platform is x64, Ubuntu 22.04 or a distribution as new, with GNOME or KDE and
PipeWire or PulseAudio. It hears all system sound, which is what the default output plays, not
one app alone. Its engine and tomoru-asr are built on Linux x64, which needs a C compiler,
pkg-config, libpulse's and GTK 3's headers and patchelf; running the app needs `libayatana-appindicator3-1`
for its tray icon, and outside GNOME `libgtk-layer-shell0` for the subtitles to stay over other
windows under Wayland (on GNOME its windows go through XWayland):

```sh
sudo apt install build-essential pkg-config libpulse-dev libgtk-3-dev patchelf libayatana-appindicator3-1 libgtk-layer-shell0
scripts/setup.sh                 # linux-amd64: tomoru-engine, tomoru-asr, the voice detector, llama-server
cd app && go tool mygo build -platform linux/amd64
```

The output is in `app/build/linux-amd64/`: `tomoru` with `bin/`, `vad/` and `licenses/` beside it,
a Debian package and a `tar.gz` with `install.sh`. On Linux the tray icon's click opens its menu,
whose first item shows the panel.

The build scripts put programs in `app/resources/<os>-<arch>/bin/`. Models users download go in
the user data directory, or a "Tomoru Models" folder in a folder they choose in Settings, which moves them there; unit tests
don't need them. The directories and the protocol are in the
[architecture](architecture.md) and the [engine protocol](engine-protocol.md).

## Checks and tests

From the repository root:

```sh
scripts/check.sh            # static checks: no tests, no builds
scripts/test.sh             # tests
scripts/test.sh --race      # with Go's race detector
```

Each takes the parts to run; with none it runs all this platform can (`ui` and `swift` need macOS
26 and Xcode 26):

| | Part | What it runs |
|---|---|---|
| `check.sh` | `go` | The license inventory, `gofmt`; `go vet`, the linters of `.golangci.yml` and functions nothing reaches (`deadcode`) of the app for macOS, Windows and Linux and of the engines for this platform and Windows (mingw-w64 needed: `brew install mingw-w64`, `apt install gcc-mingw-w64-x86-64`), the Linux engine on Linux only (`apt install libpulse-dev libgtk-3-dev`); and the app's layering, platform and design rules. The linters are pinned in the script and built the first time into `~/.cache/tomoru-tools` |
| | `swift` | A type check of the Swift engine, warnings as errors |
| `test.sh` | `go` | The tests of `app`, `engine/asr` and `engine/shared` but the UI's and the `core/coretest` desktop adapter, the benchmark's included |
| | `ui` | The app's UI tests |
| | `swift` | The Swift protocol checker against `protocol/fixtures.json`, and `HearerContract.swift`, which feeds synthetic PCM to the production hearer to check the silence limit and continuous timestamps without starting capture or a model |

Neither builds what is shipped: that is the release's (`go tool mygo build`, see the
[release guide](release.md)). Tests run uncached. Local ASR uses cgo, so `test.sh go` needs a C
compiler.

CI runs the same scripts, in the Checks and Tests workflows, on pushes to `main` and on pull
requests. A job runs only when a path it looks at changed (`.github/paths.yml`), so a change to
docs alone runs none; a skipped job counts as passing. Go runs on Linux; the UI and Swift on
`macos-26` ([image list](https://github.com/actions/runner-images/blob/main/README.md)).

To test only what you changed:

```sh
cd app
go test ./internal/live ./internal/core
go test ./internal/ui/settings
cd ../engine/asr
go test ./...
```

If your environment forbids listening on loopback, tests using `httptest` need one that allows
local ports. Tests use only local fake servers and need no external API by default.

## Validation tiers

| Tier | What it covers | When |
| --- | --- | --- |
| Tests of affected packages | Current behavior, failure and cancellation paths | Every change |
| `check.sh` | Formatting, vet and lint for macOS, Windows and Linux, unused code, package and platform boundaries, design rules, licenses, the Swift engine's types | Every code change; CI |
| `test.sh --race` | Domain rules, persistence, child processes, concurrency, the protocol implementations, the UI, the Swift contracts | Every code change; CI; the release gate |
| Real audio and desktop acceptance | Permissions, real audio, shortcuts, subtitles across screens, quitting, packaged resources | Before a release, and when platform behavior changes |
| A real Windows 11 x64 machine | `scripts/check.sh go` and `scripts/test.sh go` in MSYS2 (or test binaries cross-compiled with `GOOS=windows GOARCH=amd64 go test -c`, run in the package directory), the whole guide, capture, screen selection, the tray | Changes to Windows's own code (`engine/windows`, `capture`, `shell`); before a release, what changed since the last one |
| A real Linux x64 desktop | `scripts/check.sh go` and `scripts/test.sh go`, the whole guide, capture, screen selection, the tray's menu and panel, the shortcuts, on GNOME and on KDE: Wayland and X11 differ in permissions, shortcuts and where windows go | Changes to Linux's own code (`engine/linux`, `shell`); before a release, what changed since the last one |
| Optional real benchmark | Quality and latency with given audio, models or cloud services | Changes to recognition, segmenting or translation quality |

Other changes are tested on the platform they were made on, and the pull request says which
platforms are left; the release tests those on the others.

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
  `shell`); when only the wording differs on screen, use `shared.OnWindows`. Domain packages may
  read a platform fact (what the system has, where files go) but never behave differently by
  platform. A file that names the platform joins `platformFiles` in `app/layers_test.go`.

Releasing and signing are in the [release guide](release.md). How to contribute is in
[CONTRIBUTING](../CONTRIBUTING.md).
