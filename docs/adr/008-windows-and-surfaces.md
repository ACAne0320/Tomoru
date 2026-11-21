# ADR 008: Windows and surfaces

[中文](008-windows-and-surfaces_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
desktop entry (`app/main.go`, `app/osversion_*.go`) and the UI surfaces: `ui/shell`,
`ui/settings`, `ui/guide`, `ui/overlay`, `ui/subtitle`, `ui/imagecard`, `ui/panel` and
`ui/shared`. What [architecture](../architecture.md), [the design system](../design-system.md),
[scenes](../scenes.md) and [image translation](../image-translation.md) already say is not
repeated here.

## Decision

### App and shell

1. `main` (`app/main.go`) starts the app with the regular activation policy until the first-run
   guide is done, rather than switching to it later. Switched to once in front, macOS leaves the
   app out of the apps it goes back to, and an answered permission prompt gives the front to the
   app before, often System Settings. The guide still takes the front back with `nav.Focus()`
   once `Asking` or `ScreenAsking` turns off (`guide/permission.go`, `guide/picture.go`).
2. `main` opens Settings when the app is started again (`OnSecondInstance`, and `OnActivate`,
   since macOS asks the running app to reopen instead of starting a second one), because the menu
   bar icon may be hidden behind the notch.
3. `main` writes llama-server's log to a file of its own, apart from `tomoru.log`, as it says much.
4. `osVersion` (`app/osversion_windows.go`) names the system Windows 11 from build 22000, because
   Windows 11 still reports version 10.0.
5. `Launch` (`shell/shell.go`) does not start translating when the system opened the app at login
   (`WasOpenedAtLogin`), whatever the settings say: the user has not asked for subtitles yet.
6. `openTrayMenu` (`shell/tray.go`) sets the tray menu only for the right-click popup and removes
   it after, because a tray icon with a menu opens it on a left click too, and the left click opens
   the panel.
7. `OpenPanel` (`shell/nav.go`) also opens the panel from a button in the overlay, and the overlay
   shows translating's state as a notice (`notice`, `overlay/overlay.go`), because the eye is on
   the subtitles and the menu bar icon may be out of sight: macOS hides status items behind the
   notch, and capturing adds a recording indicator that takes more room.
8. `ShowSettingsPage` (`shell/nav.go`) makes the settings window's title buttons light on Linux
   with `SetGTKCSS`. GTK draws them in the theme's color, dark in a light theme, whatever the
   window's, and here they sit on the night.
9. `ShowGuide` (`shell/nav.go`) hides the system title bar on Windows, whose title bar is white,
   and the guide draws its own on the paper under the window controls; the window is `TitleBar`
   (`guide/guide.go`) taller for it.

### Settings

10. `modelsFolder` (`settings/engines.go`) and `export` (`settings/history.go`) call
    `mygo.Dialog.Open` and `mygo.Dialog.Save` on a goroutine and send the result back with
    `mygo.RunOnMain`. Both block until the user answers, and the main thread draws.

### First-run guide

11. `stepButton` (`guide/guide.go`): Allow calls `RequestAccess`, which starts recording, and stays
    disabled while the system asks, because macOS asks for System Audio Recording only when the
    engine first records. The app shows no prompt of its own.
12. `permission` (`guide/permission.go`) checks the audio permission every 1.5 s while it is not
    granted, because the user may answer the system's prompt after the request stopped waiting.
    Turned on in System Settings it takes effect once the app is reopened, so a denied permission
    offers Reopen.
13. `part` (`guide/guide.go`) has a step's parts rise in, never slide: the step's room scrolls and
    would cut a part coming in from the side. The step before goes at once, so the two never show
    over each other.
14. `modelOption` (`guide/engines.go`) makes a model's download button an icon, named in its
    tooltip: the guide's column is narrow, and a label would wrap the model's name and needs in a
    long language. The same button resumes a stopped download; the progress under the name says so.
15. `keycaps` (`guide/shortcuts.go`) lights a modifier while it is held, from the window's
    modifier state, but the key only once the shortcut fires: the registered global shortcut takes
    the key before the window sees it.
16. `tryStep` (`guide/try.go`) starts translating as the step shows. Going back stops it; going on
    leaves it running, so the guide ends with subtitles on.

### Overlay

17. `windowOptions` (`overlay/overlay.go`) makes a window that covers the work area plus `below`
    and never moves or resizes; the subtitle block moves inside it. A window sized to its text grew
    as tall as the screen with many sentences, and a window can't go above the screen's top to keep
    its bottom up.
18. `windowOptions` sets `DisableFullScreen` with `SetVisibleOnAllWorkspaces`, which MyGo turns
    into an auxiliary of other apps' full-screen Spaces (egoist/mygo#309), the way macOS documents
    for showing over them.
19. `toolWindow` (`overlay/window_windows.go`) makes the overlay a tool window
    (`WS_EX_TOOLWINDOW`) before it first shows. Chromium's browsers and Electron apps stop drawing
    under a window they deem opaque (`IsWindowVisibleAndFullyOpaque`, ui/gfx/win/hwnd_util.cc) and
    can't see DirectComposition's transparency; a drag in the overlay brought it to the front and
    turned a video under it black (2026-10-28). A tool window is passed over, and stays out of
    Alt+Tab.
20. `toolWindow` (`overlay/window_linux.go`) calls `gtk_window_set_accept_focus(false)`: an X11
    window manager gives the keyboard to a window clicked in, or to the one left when another
    closes, so the subtitles would take it from the video and Tomoru's other windows would open
    behind them. Clicks and drags still arrive.
21. `hasGlass` (`overlay/glass_darwin.go`) looks up `NSGlassEffectView` on first use behind a
    `sync.Once`, not at the program's start, when AppKit is not loaded yet.
22. `watchPointer` (`overlay/handles.go`) polls the cursor every 50 ms, because a window that lets
    clicks through hears no pointer. The block takes the mouse only once the pointer rests on it
    for 400 ms and it shows something, so passing by does nothing. Where the pointer is seen only
    over the app's own windows (`pointerUnseen`, Wayland), the unlocked block takes the mouse while
    it shows something and trusts its own hover until the pointer is still for 3 s, as XWayland
    does not tell an X11 window that the pointer left for a Wayland one; locked, it can't show the
    lock there.
23. `watchPointer` parks while nothing can be grabbed: its ticker stops until `View` or
    `SetLocked` wakes it, and the window ignores the mouse with no input region, since an empty
    region still has MyGo follow every mouse move on macOS and poll at each system tick on Windows.
    Idle at 50 ms the app used 1.4% of a core, parked 0.0% (2026-11-20). Parking ignores the mouse
    before clearing the region, and waking sets the region before taking the mouse: ignoring wins
    over the region and no region is the whole window, so the other order would take every click
    for a moment.
24. `capsule` (`overlay/handles.go`) shades the toolbar 12% more than the block: the system's glass
    lightens a pane as thin as a capsule more, all of it curved rim. At the block's shade the
    toolbar measured (160, 154, 167) to the block's (134, 125, 145) (2026-10-21).
25. `sceneMenu` (`overlay/handles.go`) marks the menu with `assets.Chevron`, not a "▾". Where the
    system font lacks the glyph, a fallback font's taller line set the languages higher than the
    scene's name (Noto Sans on Linux, #100).
26. `follow` (`overlay/handles.go`) moves and resizes while the handle is `Dragged()`, not
    `Pressed()`: a press ends as the pointer leaves the element, which a narrow side handle,
    following a frame behind, does at once.
27. `fades` (`overlay/overlay.go`) animates colors only (`Colors: true`). A transition animates
    position too unless told otherwise, and the controls trailed the dragged block.
28. `View` (`overlay/overlay.go`) keeps the block at the height it had when hovered, or while the
    lock shows, and clips new text at its top, so the toolbar or the lock stays under the pointer.
29. `View` builds the toolbar every frame, unseen when not hovered: MyGo delivers a menu's choice
    only in a frame that builds its button, and the scene menu reaches below the block, where
    leaving the block ends the hover before the choice.
30. `notePill` (`overlay/overlay.go`) comes and goes only through a blur, because the system's
    glass under it does not scale. Its place moves with its width, so it stays centred as it grows.

### Subtitles

31. `presets` (`subtitle/subtitle.go`) are one family list per font for every platform: macOS's,
    then Windows's, then Linux's Noto CJK, the first with the glyph drawing it. Windows and Linux
    have no rounded CJK font, so the system font draws Rounded there.
32. `Scheme` (`subtitle/subtitle.go`) puts the color on the translation, the original white or
    grey, because the translation is what people read.
33. `Visible` (`subtitle/subtitle.go`), with one sentence at a time, keeps the previous
    translation, dimmed, while the next sentence is recognized: it is not that sentence's
    translation. A committed line shows its draft until its streamed translation is done, since the
    stream starts shorter than the draft and would shrink the line to a few words.
34. `Failure` (`subtitle/subtitle.go`) says the service is not answering only for a timeout, a
    stall or a failed connection (`net.OpError`). Go's HTTP client wraps every failure as a
    `net.Error`, a bad URL's too.
35. `Align` (`subtitle/subtitle.go`) offers lines starting on the left for live text: the words
    come in at a fixed place and lines don't shift as they grow, as the BBC asks of live subtitles.
    Centred looks like a film's.
36. `Draw` (`subtitle/subtitle.go`) puts the newest sentence at the bottom, where the eye rests,
    the older ones fainter above it. Nothing moves but by as much as the text grows, as roll-up
    live captions go: a new line pushes those above up smoothly, those farther up a moment later,
    as Apple Music's lyrics do, and only up and down. A finished sentence wraps in lines of about
    the same length, as films' subtitles do; one still coming wraps as it comes.
37. `Draw` shows a sentence still being recognized as one line that rolls ("…" and its newest
    words), not as a second line filling word by word.
38. `line` (`subtitle/subtitle.go`) tags each line with its language, so that Han characters take
    that language's glyphs and Japanese reads as Japanese in a Chinese UI.
39. `edgeShader` (`subtitle/subtitle.go`) draws the outline and the shadow from the glyphs'
    coverage in one shader pass. Drawing the text again for each offset costs a run of glyphs a
    copy and notches the outline's corners.
40. `gpuOnly` (`subtitle/subtitle.go`) skips the edge in MyGo's in-memory renderer, as in tests,
    which runs a shader pixel by pixel; there the text shows as it is.

### Image card, panel and names

41. `translation` (`imagecard/imagecard.go`) parses GitHub Flavored Markdown without math: a
    dialogue's dollars are money.
42. `lines` (`imagecard/imagecard.go`) keeps a translation's line breaks as two trailing spaces,
    not a backslash, which would show as the text streams in; code blocks are left alone.
43. `foot` (`panel/panel.go`) puts Quit in the panel's footer, apart from Pause in its header, so
    that nobody quits by mistake.
44. `CloudName` (`shared/names.go`) is "Index-Translate 35B", named by its size as the local models
    are, without "cloud": it is free, the default and only in the cloud. Its other sizes say cloud
    (`locales.EngineCloudSize`), as local models share them.
45. `SourceMenu` (`shared/status.go`) is a native menu, as macOS's pop-up buttons are: it is not
    cut off by the window, and scrolls when the apps are many.

## Consequences

Much of the overlay depends on how MyGo and each window system treat a transparent window that
lets clicks through; a MyGo upgrade or a new desktop is checked on screen with another app in
front, as AGENTS.md asks. The guide relies on macOS's own permission prompts and relaunch, which
are only tested by hand.

## Evidence

- `app/internal/ui/overlay/overlay_test.go`: `TestWindowOverFullScreen`, `TestToolbarLine`,
  `TestInputRegion`, `TestParks`, `TestWakesWatcher`, `TestSceneMenu`, `TestUnlock`.
- `app/internal/ui/subtitle/subtitle_test.go`: `TestVisible`, `TestFailure`, `TestBalance`.
- `app/internal/ui/shared/names_test.go`: the engines' names, `CloudName` among them.
