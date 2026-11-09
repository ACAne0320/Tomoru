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

## Next

| # | Area | Problem | Change | Where |
|---|---|---|---|---|
| 1 | Consistency, efficiency | Between Home's two languages is a still arrow; the panel and the guide have a swap button there. Home can't swap. | Put the panel's swap `IconButton` (Quiet, Small, with `kit.Off` and `k.CanSwap()`) in place of the arrow, keeping the sentence "Listen to […], scene […]". | `settings/home.go` `homeLine` |
| 2 | Information architecture | The same sound picker is "Listen to" on Home and "Sound from" in the panel (听 / 音源). | One name for both: "Sound from", a noun, suits a picker. Remove `homeListen`. | locales, `home.go` |
| 3 | Load | In English, Home's numbers "Translated 70 lines" and "Translations 980 chars" sit side by side with almost the same name. Chinese (已翻译 / 译文) reads fine. | Rename `statChars` to "Characters"; check ja and ko. | locales |
| 4 | Load | A scene's background note says "not to the on-device 2B model": a model name the user may not have. | Say it only when the translation engine in use ignores the background, in plain words. | `settings/scenes.go`, locales |
| 5 | Information architecture | The guide's crumbs are Set up › Try it › Get started, but Shortcuts and Select to translate sit under Try it; they are settings. | First decide whether Try it should come right after the engines; if so, move the two steps under Set up, or name that phase for what it holds. | the guide's phases, `guide/guide.go` |
| 6 | Feedback | In the guide's translation step, the cloud's privacy note is cut mid-line at the bottom of the scrolling list, with no fade or hint that it scrolls. | A fade at the list's bottom (a kit component), or the note under the Cloud heading instead of at the end. | `guide/engines.go` |

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

## Found while translating live

Seen in the real app, translating an English video with burned-in bilingual subtitles in
QuickTime Player into Chinese.

| # | Area | Problem | Change | Where |
|---|---|---|---|---|
| 14 | Hierarchy | At their default place, the subtitles land on the video's own burned-in subtitles, and the two stacks can't be read. Moving them is an icon in the panel's footer, found only by hovering. | Say once, as the subtitles first show, that they can be dragged, and where; or start them a little higher than where videos put theirs. Measure on a few videos with burned-in subtitles before choosing. | `overlay/`, the guide's last step |
| 15 | Feedback | When the engine dies by a signal (a crash, or killed), the panel and the subtitles say "Translation stopped: signal: killed", not that the engine quit. `errExited` covers only an engine that exits without an error; the `*exec.ExitError` falls through to `StatusStopped`. Try again shows and works. | Treat an `*exec.ExitError` from the engine as the engine quitting (`ErrEngineExited`), in `ErrorText` and `ErrorFix`. | `core/session_controller.go`, `shared/names.go` |
| 16 | Hierarchy | The panel's state line is cut once the count has two digits: "Listened today 0:39 · 11 lin…". | Give the line the room the Start button leaves, or drop the unit's word for a shorter form ("11 lines" to "11"), and test it at 340 wide with three-digit counts. | `panel/panel.go` `state` |

## What was not verified

The pages were judged from the galleries the tests render (`TOMORU_GALLERY=<dir> go test
./internal/ui/settings ./internal/ui/guide ./internal/ui/panel -run 'Gallery|Panel'`, with
`TOMORU_GALLERY_LOCALE=zh-Hans` for Chinese), then checked in the real app with `verify-tomoru`:

- The guide's translation step shows the new sentence; the Cloud card is cut at the list's bottom
  (finding 6).
- With cloud recognition chosen and no API key (set up in `settings.json`, as when a key is lost;
  the UI does not let one choose it without a key), Start in the panel and Start translating on
  Home both show the message and Open Speech & translation, and either click opens Settings on
  that page, showing Qwen ASR. Home also showed findings 2 and 7 as a new user sees them.
- Translating an English video in QuickTime Player into Chinese, with QuickTime in front: the
  subtitles come in and translate live (finding 14). Killing the engine stopped translating, and
  the panel showed Try again (with finding 15's message); Try again started a new engine and
  translating went on.

Not driven in the real app: Open System Settings, which needs the audio permission refused.
Windows was not checked.
