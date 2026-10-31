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
- **GitHub**: `gh` signed in, with the right to publish releases of the repository in
  `updates.github`.

## Each release

1. Set the version in `app/mygo.json`, and write its section in `app/CHANGELOG.md`
   (`## 0.1.1`). The update window shows that section as the release notes.
2. Fetch and build what the app ships:

   ```sh
   scripts/setup.sh
   scripts/package-licenses.sh --check
   scripts/package-licenses.sh --release-check darwin-arm64
   ```

   The license checks verify the source license package and `app/resources/<platform>/licenses`,
   which are the inputs to packaging. They also block unresolved redistribution items; review the
   [third-party license inventory](third-party-licenses.md) and resolve every failed check before
   uploading a public binary.

3. Run `scripts/check.sh --full --race` from the repository root. The Go modules, UI and
   dependency gates, all protocol fixtures, the full Swift engine build and the Windows app and
   engine builds must pass. mingw-w64 is required. See
   [validation tiers](development.md#validation-tiers). Then try the built app: first-run permissions,
   real audio with Apple and local ASR, language/scene changes, shortcuts, subtitle placement,
   history after a slow translation, settings after relaunch, and quitting during active work.
   Confirm the packaged app finds all bundled engines and libraries and includes their
   [license texts and notices](third-party-licenses.md) in `licenses/`. The automated gate does
   not exercise signing, permissions or paid cloud services.
4. Build, sign, and upload a draft release:

   ```sh
   cd app
   MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
   ```

   It writes `build/darwin-arm64/`: the app, `tomoru-<version>-arm64.dmg`, the update archive,
   deltas from the earlier versions, `update-darwin-arm64.json` and `install.sh`. It uploads them to a draft
   release tagged `v<version>`.
5. Review the draft on GitHub and publish it. Installed apps check soon after they launch, then
   once a day, and find it then.

## Windows

Built on a Windows 11 x64 machine, in MSYS2's MINGW64 shell, with the tools the
[README](../README.md#windows) lists and `diffutils` (`pacman -S diffutils`) for the license check.
The update key goes in `~/.config/tomoru/update-keys/` there too, copied from the password
manager, and `gh` is signed in there. MyGo downloads NSIS for the installer the first time:

```sh
export PATH="$PATH:/c/Program Files/Go/bin:$PWD/app/resources/windows-amd64/bin"
scripts/setup.sh
scripts/package-licenses.sh --check
scripts/package-licenses.sh --release-check windows-amd64
scripts/check.sh --portable
cd app
go test ./...
MYGO_UPDATER_PRIVATE_KEY="$(cat ~/.config/tomoru/update-keys/mygo-update.key)" go tool mygo build -upload
```

It can also be cross-compiled on the Mac, with mingw-w64 (`brew install mingw-w64`) and NSIS
(`brew install makensis`): `scripts/setup.sh windows-amd64`, then
`go tool mygo build -platform windows/amd64 -upload`.

It writes `build/windows-amd64/`: `Tomoru.exe` with `bin/`, `vad/` and `licenses/` beside it, and
`tomoru-setup-<version>-amd64.exe`. Try it on a Windows 11 x64 machine first: the guide from the start
(SenseVoice downloads to `%LocalAppData%\Tomoru\models`), all sound and one app's, the tray's
panel, the shortcuts, selecting a part of the screen, a local translation model, and quitting while
translating. The app is not signed: SmartScreen warns on its first launch ("More info", then "Run
anyway").

The license release check examines the platform resources before packaging; it does not inspect
the final installer. Confirm the finished package includes the license texts and notices too.

## What users do the first time

The app is signed, but not notarized: macOS stops it on its first launch. Say on the download page:

> Open Tomoru. When macOS says it can't verify it, open System Settings › Privacy & Security, and
> click "Open Anyway" next to Tomoru.

Updates need nothing of the user: the updater checks their signature, and the app keeps its
permissions, as it keeps its certificate.
