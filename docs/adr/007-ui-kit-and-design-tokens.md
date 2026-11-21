# ADR 007: UI kit and design tokens

[中文](007-ui-kit-and-design-tokens_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover
`app/internal/ui/kit` and `app/internal/ui/assets`: the controls, the frame, the shortcut fields
and the tokens. What [the design system](../design-system.md) already says is not repeated here.

## Decision

1. `Destructive` (`kit/button.go`) is for what stops, cancels a running task, deletes, closes or
   quits: its face turns red on hover, as a title bar's close button does. A dialog's Cancel only
   backs out, so `confirmActions` (`kit/dialog.go`) gives it a plain button.
2. `press` (`kit/button.go`) starts a destructive button with no face at rest from
   `CloseHover.Alpha(0)`, not `ui.Transparent`. MyGo's `AnimateColor` interpolates straight RGBA,
   so a fade from transparent black would pass through grey.
3. `TurnedIconButton` (`kit/button.go`) turns the icon, not the button. MyGo draws a turned icon
   sharp, but resamples a turned element, which looks soft.
4. `choiceRow` (`kit/choice.go`) and `listItem` (`kit/list.go`) set `Background(ui.Transparent)`
   on their tabs and draw their own `Shown` or `Band` face, because MyGo's
   `ButtonBase.Selected` paints the theme's accent.
5. `offOpacity` (`kit/controls.go`) fades a slider in a `SettingRowOff`, because a MyGo slider
   draws in its own colors, which the row can't recolor. For the same reason, MyGo fading
   everything inside a disabled element, `optionRow` (`kit/option.go`) disables only an `Off`
   option's radio, so its text stays legible.
6. `Parts` (`kit/list.go`) remembers the width it laid out in and asks for one more frame
   (`c.After(0)`) when it changes, because MyGo's `LayoutBounds` is the previous frame's layout.
7. `AppFrame` (`kit/frame.go`) puts no wordmark or title in the sidebar: the pages start right
   under the band of the window controls, since the window's title and the Dock name the app.
8. `AppFrame` keys the page column by the selected page (`c.Key(*selected)`), so switching shows
   the new page at once instead of animating the old page's elements into it.
9. `Heatmap` (`kit/heatmap.go`) fixes the weekday labels' height to `heatSize`. Segoe UI's lines
   are taller than SF's, and on Windows they would space the rows wider than the columns.
10. `Accelerator` (`kit/shortcut.go`) takes a key with Control, Option or Command, or a function
    key alone. Shift alone is refused: as a global shortcut it would take a capital letter from
    every app.
11. `ShortcutField` (`kit/shortcut_field.go`) keeps the current shortcut registered while it
    records. The system takes those keys before any app sees them, so pressing it does what it
    does and never reaches the field.
12. `ShortcutField` records `ev.PhysicalKey`, falling back to `ev.Key` only when the position is
    unknown, because global shortcuts are registered by key position, not by the layout's
    character.
13. `screenSettings` (`kit/screen.go`) is the one System Settings URL for both recording
    permissions (`Privacy_ScreenCapture`): macOS lists System Audio Recording Only under Screen &
    System Audio Recording and gives it no URL of its own.
14. `AppIcon` (`assets/assets.go`) is drawn on the macOS icon grid, as `scripts/app-icon.swift`
    makes it: an 824/1024 rounded square, centred, with a soft drop shadow, in a margin of about a
    tenth. The UI shows it as it is and adds no corners or shadow of its own.
15. `Arrive` (`kit/tokens.go`) plays the entrance as keyframes, because a MyGo enter transition
    needs a frame before the element to come in after, and does not play for what is built with
    its container.

## Consequences

Decisions 2 to 6 work around how MyGo draws and lays out today; when MyGo is upgraded, check them
before keeping or dropping the workaround. The shortcut rules apply on every platform, so a
shortcut that works on one works on the others.

## Evidence

- `app/internal/ui/kit/shortcut_test.go`: `TestAccelerator` refuses Shift alone.
- `app/internal/ui/kit/choice_test.go`: `TestOptionOff`: an `Off` option can't be chosen.
- `app/internal/ui/kit/heatmap_test.go`: the heatmap's layout and hover.
