# ADR 009: Engines and platforms

[中文](009-engines-and-platforms_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
platform engines (`engine/mac`, `engine/windows`, `engine/linux`) and the code the Go engines
share (`engine/shared`); those of `engine/asr` are all in the docs already. What
[the engine protocol](../engine-protocol.md), [local recognition](../local-asr.md),
[architecture](../architecture.md) and [image translation](../image-translation.md) already say
is not repeated here.

## Decision

### Shared by the Go engines

1. `engine_linux.h` and `engine_windows.h` keep the calls into PulseAudio, GTK and COM in C,
   because those APIs call back into C functions and COM's interfaces are C vtables to call.
2. `selectArea` (`linux/select_linux.go`, `windows/select_windows.go`), `run`
   (`windows/capture_windows.go`) and `speak` (`windows/speak_windows.go`) hold
   `runtime.LockOSThread` for the whole call: GTK, COM objects, a Win32 window and a SAPI voice
   belong to the OS thread that made them, and a goroutine may otherwise move.
3. `FillSilence` (`shared/hear.go`), as the macOS engine's `Hearer` (`Hear.swift`), sends silence
   only once the source has given nothing for 0.3 s. While it delivers, its samples are the
   timeline, though its clock drifts from the wall's, because tomoru-asr's pauses and clock come
   from the audio alone. After a sleep or suspension it sends at most 2 s of silence, enough to
   finish a sentence, and rebases the clock instead of replaying the whole gap.
4. `decodeWAV` (`shared/file.go`) skips the pad byte after an odd-sized chunk, as RIFF requires.
5. `resample` (`shared/file.go`) averages the samples each output spans to go down to 16 kHz and
   interpolates linearly to go up, rather than filtering properly: enough for speech.
6. `newSilero` (`shared/voice.go`) decides as the macOS engine's Core ML detector does. Its
   threshold is 0.5 because sherpa-onnx puts the end threshold 0.15 under the start, making it the
   macOS engine's 0.35. `MinSpeechDuration` and `MinSilenceDuration` are one window short,
   because sherpa-onnx counts a run from the end of its first window.

### Linux

7. `appID` (`linux/select_linux.go`) and `session` (`linux/select_linux.c`) register the engine
   with `org.freedesktop.host.portal.Registry` under the ID MyGo gives the app (the desktop entry
   that launched it, else the parent's program name), so the portals count its screenshots and
   their permission as the app's. A failed Register is ignored: older portals have no registry and
   know the app by its cgroup.
8. `portal_file` (`linux/select_linux.c`) reads the permission store when the portal answers 2,
   because GNOME and KDE answer a no as 2, a generic failure, rather than 1.
9. `tc_select` (`linux/select_linux.c`) takes the picture from the portal under Wayland, because
   XWayland sees only X11 windows. Under X11 it presents the first window with the X server's time,
   so the window manager takes it for the user's doing and gives it the focus.
10. `stream_read` (`linux/capture_linux.c`) feeds a hole (no data from `pa_stream_peek`, with a
    size) as silence and drops it, rather than skipping it, so the audio's time stays whole.
11. `open_locked` (`linux/capture_linux.c`) opens the monitor with `PA_STREAM_DONT_MOVE`: a monitor
    that goes away ends the stream, and the engine opens the new default, rather than the server
    moving it to the microphone.

### Windows

12. `formats` (`windows/capture_windows.go`) asks WASAPI for 16 kHz mono first, else for 48 kHz
    stereo, which the audio engine always gives, and converts that in Go.
13. `onTaskbar` (`windows/apps_windows.c`) leaves out cloaked windows: a suspended Store app keeps
    its window visible but cloaked.
14. `activationParams` (`windows/capture_windows.c`) declares the process loopback parameters
    itself, because mingw-w64 has no `audioclientactivationparams.h`.
15. `handler` (`windows/capture_windows.c`) answers `IAgileObject`: `ActivateAudioInterfaceAsync`
    requires an agile completion handler and calls it from a thread of its own.
16. `speak_windows.c` looks up voices in OneCore's category before SAPI's own, because the voices
    Windows installs with a language are OneCore's, and SAPI lists them apart.
17. `tc_select` (`windows/select_windows.c`) makes the process per-monitor DPI aware (v2), so each
    screen is taken at its own scale in pixels, not stretched to the system's. It registers Escape
    as a hotkey, because Windows may keep the keyboard from a process that did not take the user's
    last input.

### macOS

18. `Capture.swift` hears through a Core Audio process tap (macOS 14.2+), because a tap needs only
    the System Audio Recording permission, not Screen Recording.
19. `request-access` (`main.swift`) starts a tap and waits for the answer, a minute at most:
    there is no call to ask, the system asks the first time a tap records. A denied permission
    gives silence, not an error, so `TapCapture` logs what arrives every 30 s.
20. `start` (`Capture.swift`), for `--app`, taps every process whose bundle ID is the app's or
    starts with it and a dot, or whose responsible process is the app, because an app plays
    through helpers with bundle IDs of their own (a browser's audio process).
21. `responsiblePID` and `tccPreflight` (`Capture.swift`) load the private
    `responsibility_get_pid_responsible_for_pid` and `TCCAccessPreflight` with `dlsym`, since no
    public API tells a helper's app or checks System Audio Recording. Where they are gone, they
    fall back to the pid itself and to `unknown`.
22. `audioAccess` (`Capture.swift`) counts Screen Recording as granting the tap too.
23. `requestScreenAccess` (`Capture.swift`) calls `CGRequestScreenCaptureAccess` once, while the
    state is `unknown`, and waits a minute at most for the answer; the system only offers to open
    System Settings. The app checks again in a fresh engine process each time, so the permission
    needs no relaunch (`core/system.go` `ask`).
24. `installAssets` (`Listen.swift`) always makes the asset installation request, even for a
    locale already installed: it reserves the locale for the app (at most five), and then takes no
    time. A new locale took 13–21 s, with no progress reported.
25. `quietTail` (`Listen.swift`): after 2 s of silence the engine stops handing audio to
    SpeechAnalyzer, which hears things in it ("あ", 0.91 confident, 6 s into the zeros a tap gives
    while nothing plays). The next sound comes with its time, so the timeline still counts the
    silence.
26. `copyBuffer` (`Listen.swift`) copies the tap's buffers, whose memory the tap reuses once its
    callback returns.
27. `Voice.swift` runs the Core ML detector `.cpuOnly`: for a tiny model run 31 times a second,
    dispatching it costs more than running it.
28. `selecting` (`Select.swift`) passes the engine's SIGTERM on to `screencapture`, which runs in a
    process group of its own and would leave its crosshair on the screen.

## Consequences

The macOS engine depends on two private symbols and on undocumented behavior (a silent denial,
the locale reservation); its fallbacks keep it working, with less, if Apple changes them. The Go
engines' C code and thread locking are tested only on their own platforms, as AGENTS.md asks.

## Evidence

- `engine/shared/engine_test.go`: `TestHearer`, `TestHearerResumesWithoutSilenceBacklog` (the
  2 s cap after a suspension), `TestDecodeWAV`, `TestDetector`.
- `engine/shared/engine_test.go` and `engine/asr/protocol_test.go`: `TestSharedProtocolFixtures`,
  against `protocol/fixtures.json`.
