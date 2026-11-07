# Releasing

[中文](release_zh.md)

How to build a release of Tomoru for macOS and Windows 11 x64 and publish it, so that installed apps
update to it.

For local contributor builds, use the [development signing instructions](development.md#tools-and-running).
The certificate and update key below are for official releases.

## Once

- **Signing certificate**: "Tomoru Self-Signed" in the login keychain, with a backup (the `.p12`
  and its password) kept outside the repository. Every release must be
  signed with the same certificate: macOS keys the System Audio Recording permission to it, so
  another certificate would ask users for the permission again.
- **Update key**: `mygo-update.key`, made with `go tool mygo keygen` and kept outside the
  repository (the commands below assume `~/.config/tomoru/update-keys/`). Installed apps take only
  updates signed with it, so keep a copy in a password manager: losing it
  strands every installed app. Its public half is `updates.publicKey` in `app/mygo.json`.
- **GitHub secrets**, which the release workflow (`.github/workflows/release.yml`) signs with:

  ```sh
  base64 -i tomoru-signing.p12 | gh secret set MACOS_CERTIFICATE
  gh secret set MACOS_CERTIFICATE_PASSWORD   # the password of the .p12
  gh secret set MYGO_UPDATER_PRIVATE_KEY < ~/.config/tomoru/update-keys/mygo-update.key
  ```

## Each release

1. Set the version in `app/mygo.json`, and write its section in `app/CHANGELOG.md`
   (`## 0.1.1`). That section is the release's notes, on GitHub and in the app's update window.
   End each item with a link to its pull request, or to its commits when it had none:
   `([6a25ce5](https://github.com/ACAne0320/Tomoru/commit/6a25ce5))`.
2. Commit to `main`, and wait for Checks and Tests to pass.
3. Push the version's tag:

   ```sh
   git tag v0.1.1
   git push origin v0.1.1
   ```

   The Release workflow checks that the tag is the version and that the changelog has its
   section, creates a draft release with that section as its notes, followed by the installers'
   links (on GitHub, and the website's for mainland China), and builds on macOS and on
   Windows (MSYS2) at the same time: `scripts/setup.sh`, the license release check, then
   `go tool mygo build -upload`, which signs the apps and their updates and uploads them to the
   draft. A tag with a hyphen (`v0.2.0-beta.1`) makes a prerelease. "Re-run failed jobs" builds a
   failed platform again into the same draft.
4. Try the draft's files before publishing. On a Mac, `tomoru-<version>-arm64.dmg`: first-run
   permissions, real audio with Apple and local ASR, language and scene changes, shortcuts,
   subtitle placement, history after a slow translation, settings after relaunch, and quitting
   during active work. On Windows 11 x64, `tomoru-setup-<version>-amd64.exe`: the guide from the
   start, all sound and one app's, the tray's panel, the shortcuts, selecting a part of the screen,
   a local translation model, and quitting while translating. Check that both find their bundled
   engines and libraries and carry their [license texts and notices](third-party-licenses.md) in
   `licenses/`: the license check looks at the resources before packaging, not at the package.
5. Publish the draft on GitHub. Installed apps check soon after they launch, then once a day, and
   find it then. Within half an hour the website copies its installers to R2 and its download
   buttons give them ([site/README.md](../site/README.md)).

## Building by hand

Without the workflow, each platform builds where `gh` is signed in and the update key is in
`~/.config/tomoru/update-keys/`; `go tool mygo build -upload` uploads to the draft release of the
version, creating it if there is none. On a Mac, with the certificate in the login keychain:

```sh
scripts/setup.sh
scripts/package-licenses.sh --release-check darwin-arm64
cd app
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

On Windows 11 x64, in MSYS2's MINGW64 shell with the tools the [README](../README.md#windows)
lists and `diffutils` (`pacman -S diffutils`); MyGo downloads NSIS for the installer the first
time:

```sh
export PATH="$PATH:/c/Program Files/Go/bin"
scripts/setup.sh
scripts/package-licenses.sh --release-check windows-amd64
cd app
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

`build/darwin-arm64/` then holds the app, the disk image, the update archive, deltas from earlier
versions, `update-darwin-arm64.json` and `install.sh`; `build/windows-amd64/` holds `Tomoru.exe`
with `bin/`, `vad/` and `licenses/` beside it, and the installer. The Windows app is not signed:
SmartScreen warns on its first launch ("More info", then "Run anyway").

## What users do the first time

The app is signed, but not notarized: macOS stops it on its first launch. Say on the download page:

> Open Tomoru. When macOS says it can't verify it, open System Settings › Privacy & Security, and
> click "Open Anyway" next to Tomoru.

Updates need nothing of the user: the updater checks their signature, and the app keeps its
permissions, as it keeps its certificate.
