# ADR 010: Tooling, benchmark and site

[中文](010-tooling-benchmark-and-site_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
workflows in `.github/`, `.golangci.yml`, `scripts/`, the repository's own tests
(`app/design_test.go`, `app/locales/catalog_test.go`), the benchmark (`bench/`,
`app/cmd/tomoru-bench`) and the site (`site/`). What [development](../development.md), [release](../release.md),
[local recognition](../local-asr.md), [the design system](../design-system.md), `bench/README.md`
and `site/README.md` already say is not repeated here.

## Decision

### CI and lint

1. `checks.yml` and `tests.yml` ask for `pull-requests: read`: `dorny/paths-filter` reads a pull
   request's changed files from GitHub's API, and fails on pull requests without it.
2. `.golangci.yml` excludes unparam's "`ui.Element`) is never used" under `internal/ui/`: every
   builder returns its element, as the rest of kit does, used or not. unparam is off for
   `_test.go` too: a test helper takes `t` and returns what some tests leave.

### Release and site deploy

3. `release.yml` runs the platform jobs one after another, each after the last whether it passed
   or not (`!cancelled()`): `go tool mygo build -upload` merges the release's `SHA256SUMS` with its
   own, and two uploads at once replace each other's.
4. `release.yml` gives a prerelease's notes `sh install.sh <tarball>` instead of the
   `curl … | sh` line, because `install.sh` downloads `releases/latest`, which a prerelease is not.
5. `release.yml` builds the Debian asset's name with `tr -- '-+' '..'`: MyGo writes a
   prerelease's Debian version with `~`, so that it sorts before its release, and GitHub turns
   `~` into `.` in an asset's name (0.2.0-beta.1's is `tomoru_0.2.0.beta.1_amd64.deb`).
6. `release.yml` sets MSYS2's `path-type: inherit`, so its shell finds setup-go's Go and the
   runner's `gh`, which the default hides.
7. `site.yml` deploys with the repository secrets `CLOUDFLARE_API_TOKEN`, a token made from
   Cloudflare's "Edit Cloudflare Workers" template, and `CLOUDFLARE_ACCOUNT_ID`.

### Build scripts

8. `build-asr.sh`, `build-engine-linux.sh` and `build-engine-windows.sh` remove a library, copy it,
   then `chmod` it: Go's module cache is read-only and `cp` keeps the mode, so the next copy could
   not write over the last.
9. `build-engine-linux.sh` sets the rpath to `$ORIGIN` with `patchelf`, as `build-asr.sh` does
   with `@executable_path` on macOS: cgo points the programs at the module cache.
10. `build-asr.sh` re-signs ad hoc after `install_name_tool`, which breaks the linker's signature;
    Apple silicon runs no binary without a valid one.
11. `build-engine-mac.sh` removes the old engine before copying the new one: macOS kills a program
    started from a file written over while it ran, its signature cached as the old one's.
12. `fetch-llama.sh` pins `TAG` to the llama.cpp release local translation was measured with. On
    Windows and Linux it takes the Vulkan build, which runs on any maker's GPU, and on the CPU
    without one.
13. `fetch-llama.sh` copies llama-server and the libraries it needs, through those they need
    (`readelf` on Linux, mingw-w64's `objdump` on Windows), each under the name it is needed by
    (`cp -L` through a link). ggml's CPU and Vulkan backends are added by name, because ggml loads
    them as it starts and no import lists them.
14. `fetch-llama.sh` writes `readelf`'s and `objdump`'s output to a file before reading it in a
    loop, because `set -e` does not stop at a failing command substitution in a `for` list.
15. `package-licenses.sh`'s copy mode only updates the notices it manages and deletes nothing:
    unknown files are left for `--release-check` to report, for a person to review.

### Checks and tests

16. `test.sh`'s `go` part leaves out `internal/ui/...` and `core/coretest` because they need a
    desktop session; the benchmark is headless and stays in.
17. `unstyled` (`app/design_test.go`) only shrinks: the test fails when a file on it is gone or
    draws with kit's tokens now, and no file joins it.
18. `TestDependencies` (`app/design_test.go`) holds `golang.org/x/sys` for the models' free disk
    space, and for the memory and the taskbar's theme on Windows.
19. `TestComplete` (`app/locales/catalog_test.go`) fails when a catalog lacks a message, because
    `mygo i18n check` only warns and the app would show the English.

### Benchmark

20. `cpuTimes` (`app/cmd/tomoru-bench/cpu_windows.go`) gives 0 for the children's CPU on Windows,
    which keeps no total for children that ended. CPU s/min of a Windows run counts the benchmark
    alone.
21. `fetch` (`bench/fetch.py`) downloads the whole audio and cuts the stretch with ffmpeg: yt-dlp's
    cutting took over 15 minutes for a stretch of an hour's lecture, as YouTube slows the requests
    of seeking.

### Site

22. `tween` (`site/public/studio.js`) moves the cursor and what it drags frame by frame with
    `requestAnimationFrame`, a new move ending the old. Writing the end into the style and
    animating from the start sometimes showed the end for a frame first, which looked like a twitch.
23. `.cam` (`site/public/styles.css`) declares its scale twice. A browser that can divide a length
    by a length takes the second, `calc(100cqw / (var(--w) * 1px))`; one that can't drops it and
    takes the first, `tan(atan2(…))`, also a/b. The order matters: Safari 26 gets `cqw` wrong inside
    `atan2` but can divide. Without scripts this is the scale; a playing demo writes its own
    transform over it.
24. `icons.html` (`site/src/partials/`) is a sprite of Lucide icons (ISC), the same set the app
    uses.

## Consequences

The release jobs take longer one after another, and a later one still runs when an earlier fails;
"Re-run failed jobs" builds only what failed. Several script steps exist only for how the module
cache, macOS signing and ggml's loading behave; dropping one breaks a build on one platform.

## Evidence

- `app/design_test.go`: `TestDesignTokens` (the `unstyled` list), `TestDesignScale`,
  `TestDependencies`.
- `app/locales/catalog_test.go`: `TestComplete`.
- `scripts/check.sh` and `scripts/test.sh`, as CI runs them.
