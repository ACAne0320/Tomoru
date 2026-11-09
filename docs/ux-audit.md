# UX audit, November 2026

[中文](ux-audit_zh.md)

A walk through every surface on 2026-11-09, at 0.1.4: Home, the menu bar panel, each Settings
page and each step of the first-run guide, in English and Chinese, light and dark. Each finding
was judged on six things: information architecture, flow, efficiency, feedback, visual hierarchy
and consistency between components. This file lists what is left to do; strike an item through,
or remove it, once it ships.

## Fixed in this pass

- When translating stops with an error, Home and the panel show the action that resolves it beside
  the message: Open Speech & translation, Open System Settings, or Try again
  (`shared.ErrorFix`). Before, the message named the page to go to and left the user to find it.
- The guide's translation step no longer says, in English and Korean, that an on-device model
  downloads by itself; one downloads only when chosen.
- Then findings 1, 2, 3, 6, 14 and 15: Home swaps the languages as the panel does and names the
  sound picker "Sound from" as the panel does; the English count of characters is "Translated
  text"; a step of the guide taller than its room fades at its foot (`kit.FadeScroll`); an engine
  ended by the system says the engine quit; the panel's numbers keep their unit ("1 line",
  "1,234 lines") and go on to a second line between the time and the count, never inside it.

## Next

| # | Area | Problem | Change | Where |
|---|---|---|---|---|
| 4 | Load | A scene's background note says "not to the on-device 2B model": a model name the user may not have. | Say it only when the translation engine in use ignores the background, in plain words. | `settings/scenes.go`, locales |
| 5 | Information architecture | The guide's crumbs are Set up › Try it › Get started, but Shortcuts and Select to translate sit under Try it; they are settings. | First decide whether Try it should come right after the engines; if so, move the two steps under Set up, or name that phase for what it holds. | the guide's phases, `guide/guide.go` |

## Polish

| # | Area | Problem | Change |
|---|---|---|---|
| 7 | Hierarchy | Home's heatmap fills the card, up to a year: a user of two weeks sees some fifty weeks of empty squares. | Start at the earlier of the first week used and eight weeks ago, capped by the card's width. |
| 8 | Hierarchy | Statistics lists engines with no lines in the period ("Index-Translate 2B · 0 lines"), an empty bar. | Leave out engines with no lines and no latency in the period. |
| 9 | Load | Each engine's row on Statistics repeats the latency card's numbers (first words, line, speech to translation). | Engine rows keep usage (lines, characters, tokens, failures); latency stays in its card, which splits by day already. |
| 10 | Discoverability | A new scene's title shows "New scene" in placeholder grey: it looks disabled, not like a name to type. | Focus the title as the scene is made, or show it is editable on hover. |
| 11 | Hierarchy | A recognizer's "Supported languages" is a paragraph of thirty names. | The spoken language first and bold, the rest folded as "+28". |
| 12 | Copy | "Engine not recorded" names an implementation detail of old data. | "Earlier versions", or fold it into a faint "Other" row. |
| 13 | Product | "Keep subtitles" is off by default, for privacy, so Recent, Learn names and learning a scene's names never show for most users. | Keep the default; offer the switch where it pays, in a scene's empty names list, saying it lets names be learned. |

## What was not verified

The pages were judged from the galleries the tests render (`TOMORU_GALLERY=<dir> go test
./internal/ui/settings ./internal/ui/guide ./internal/ui/panel -run 'Gallery|Panel'`, with
`TOMORU_GALLERY_LOCALE=zh-Hans` for Chinese), then checked in the real app with `verify-tomoru`:

- The guide's translation step shows the new sentence; the Cloud card was cut at the list's bottom
  (finding 6, fixed since).
- With cloud recognition chosen and no API key (set up in `settings.json`, as when a key is lost;
  the UI does not let one choose it without a key), Start in the panel and Start translating on
  Home both show the message and Open Speech & translation, and either click opens Settings on
  that page, showing Qwen ASR. Home also showed findings 2 (fixed since) and 7 as a new user sees them.
- Translating an English video in QuickTime Player into Chinese, with QuickTime in front: the
  subtitles come in and translate live. Killing the engine stops translating; the panel says the
  recognition engine quit unexpectedly, with Try again, which starts a new engine and translating
  goes on. The panel's numbers read "Listening for 0:25 · 7 lines" on one line, "1 line" for one,
  and go on to a second line whole ("Listened today 0:35 ·" / "10 lines").
- Home names the picker "Sound from" and swaps the languages from between them (saved in
  `settings.json`); the guide's translation step fades at its foot.

Not driven in the real app: Open System Settings, which needs the audio permission refused.
Windows was not checked.
