# Changelog

## Unreleased

### Fixed

- On Windows, all system audio captures the default playback device, including Teams call
  audio, and reconnects when the default output changes. Apps routed to other outputs are
  not captured in this mode. Selected-app capture still uses process loopback.

## 0.1.8

Locked subtitles can be unlocked right where they are, and subtitles can go onto another screen.
Settings no longer keeps the processor busy while it is open, and stopping frees the memory of the
models on your device.

### New

- Locked subtitles show a lock when the pointer rests on them: click it to unlock them. Clicks on
  the rest of the subtitles still go through to what is under them, and the lock goes again after
  a few seconds of a still pointer. ([#35](https://github.com/ACAne0320/Tomoru/pull/35))
- Subtitles can be dragged onto another screen, and stay there after relaunching. When that screen
  is unplugged, they go to the main one, and come back when it is plugged in again.
  ([#32](https://github.com/ACAne0320/Tomoru/pull/32))

### Improved

- Stopping or pausing frees the models on your device, which held gigabytes of memory until
  Tomoru quit. Starting again loads them in a few seconds.
  ([#30](https://github.com/ACAne0320/Tomoru/pull/30))

### Fixed

- Settings › Home kept the processor busy (25–35% while idle) as long as it was open.
  ([#30](https://github.com/ACAne0320/Tomoru/pull/30))
- After plugging a screen in or out, or changing its resolution, the subtitles stayed where they
  were until Tomoru was relaunched. ([#32](https://github.com/ACAne0320/Tomoru/pull/32))

## 0.1.7

Subtitles can show the original alone, for streams in a language you already understand: nothing
is translated then. Scenes and History keep their details in sight in a narrow window.

### New

- Subtitle style › Show › Original only: the subtitles show what is said, and nothing is
  translated. No request goes out and no translation model loads, so it starts even with no
  translation engine set up. Home, the menu bar panel and the subtitles say "Start subtitles"
  and "Showing subtitles" then, not translating. ([#25](https://github.com/ACAne0320/Tomoru/pull/25))

### Improved

- History lists sessions of the original only by the language spoken, and writes the names right
  in them without translating. Statistics count their lines as heard, not translated.
  ([#25](https://github.com/ACAne0320/Tomoru/pull/25))
- Starting subtitles no longer stops a picture being translated on your device.
  ([#25](https://github.com/ACAne0320/Tomoru/pull/25))
- A session stopped by an error says "Stopped", not "Translation stopped".
  ([#25](https://github.com/ACAne0320/Tomoru/pull/25))

### Fixed

- In a narrow window, Scenes and History put a scene's or a session's details under the list, out
  of sight. They stay side by side now. ([#26](https://github.com/ACAne0320/Tomoru/pull/26))

## 0.1.6

Index-Translate's cloud works again. Its service changed on November 10, and since then every
cloud translation in Tomoru 0.1.5 and earlier failed, subtitles and pictures alike. Pictures can
go to the cloud's 9B and 2B now too.

### Improved

- The cloud's 9B and 2B translate pictures too: choose one in Speech & translation › Picture
  translation. That menu lists its engines under Cloud and On device, so the cloud's 9B and the
  one on your device are told apart. ([#21](https://github.com/ACAne0320/Tomoru/pull/21))

### Fixed

- Translating with Index-Translate in the cloud failed with "400 Bad Request" since the service
  changed on November 10: Tomoru sent it a setting it now refuses.
  ([#21](https://github.com/ACAne0320/Tomoru/pull/21))
- A picture sent to the cloud no longer fails when the service is slow to start: it waits up to a
  minute for the first words, where it gave up after 20 s.
  ([#21](https://github.com/ACAne0320/Tomoru/pull/21))

## 0.1.5

Feedback can start in the app now: a problem or an idea, handed to an AI agent that turns it into
an issue with you, or to GitHub's form. An error on Home or the panel comes with the button that
fixes it.

### New

- Settings › About › Feedback & ideas: choose Problem or Idea and say what happened or what you'd
  like. Copy for an AI agent, paste it into one such as Claude Code or Codex, and it goes through
  the logs with you (for a problem) or shapes the idea, looks for the same issue, and opens it
  once you say yes; or open GitHub's form, started with what you wrote. What is copied shows
  first, and has no API keys, subtitles or recordings. ([#14](https://github.com/ACAne0320/Tomoru/pull/14), [#15](https://github.com/ACAne0320/Tomoru/pull/15), [#16](https://github.com/ACAne0320/Tomoru/pull/16))

### Improved

- An error on Home and on the menu bar panel has the button that fixes it: Speech & translation,
  System Settings, or Try again. ([#8](https://github.com/ACAne0320/Tomoru/pull/8))
- Home swaps the languages from between them and names the sound picker "Sound from", as the panel
  does. ([#13](https://github.com/ACAne0320/Tomoru/pull/13))
- A guide step taller than its room fades at its foot, so it reads as going on. ([#13](https://github.com/ACAne0320/Tomoru/pull/13))

### Fixed

- The panel's numbers keep their unit and plural ("1 line"), and are no longer cut in English,
  Japanese and Korean once they reach two digits. ([#13](https://github.com/ACAne0320/Tomoru/pull/13))
- A recognition engine ended by the system says it quit, not "signal: killed". ([#13](https://github.com/ACAne0320/Tomoru/pull/13))
- The guide no longer says in English and Korean that an on-device model downloads by itself: it
  downloads once chosen. ([#8](https://github.com/ACAne0320/Tomoru/pull/8))

## 0.1.4

Hy-MT2 joins Index-Translate, on your device and on Tencent Cloud, and Speech & translation is
redesigned around model families, with more sizes to choose from. Pictures get an engine of their
own, the models a folder of your choosing. A line that isn't translated now says why, and
recognition that falls behind stops instead of freezing.

### New

- Hy-MT2, Tencent's translation model: 1.8B and 7B download to translate on your device; on
  Tencent Cloud it translates with your TokenHub API key (Plus, Pro or Lite); and an API of yours
  can be prompted as Hy-MT2 (Add API › Prompt). It reads no pictures, so it isn't offered for
  picture translation. ([#7](https://github.com/ACAne0320/Tomoru/pull/7))
- Index-Translate in the cloud, free from the bilibili team, comes in 9B and 2B too. In our tests
  both translated closer to the human subtitles than 35B, and sooner; 35B stays the default, and
  the one that translates pictures. ([968ccda](https://github.com/ACAne0320/Tomoru/commit/968ccda))
- Qwen3-ASR 0.6B, for GPUs without the memory of the 1.7B: about 2 GB, a little quicker, as
  accurate but in Korean. ([968ccda](https://github.com/ACAne0320/Tomoru/commit/968ccda))
- Picture translation has its own engine, in Speech & translation › Picture translation, chosen
  from the engines that read pictures: Index-Translate 35B in the cloud by default, a downloaded
  on-device model, or an API of yours. Testing an API says whether it reads pictures, and one
  that can't isn't offered. A picture still goes only to the engine you chose.
  ([dd36f5e](https://github.com/ACAne0320/Tomoru/commit/dd36f5e),
  [60f4cd2](https://github.com/ACAne0320/Tomoru/commit/60f4cd2))
- The on-device models can live in a folder you choose, on another drive if the system one is
  full: Speech & translation › Model folder › Change… moves them into a "Tomoru Models" folder
  there. ([#5](https://github.com/ACAne0320/Tomoru/issues/5),
  [dd36f5e](https://github.com/ACAne0320/Tomoru/commit/dd36f5e))

### Improved

- Speech & translation is redesigned: each model family is listed once, on device and in the
  cloud together, with its sizes or its setup beside the list. Download only downloads; a model
  is used by its radio or Use once it is in. Each size says whether it is the quicker or the more
  accurate one, and whether this computer runs it. Each model links to where it is from. A cloud
  service asks for its API key first, with its model and address folded away.
  ([968ccda](https://github.com/ACAne0320/Tomoru/commit/968ccda))
- The settings, the menu bar panel and the first-run guide share one tightened design, in light
  and dark. Home, Scenes, History, Statistics and About are laid out alike; a session on Home
  opens in History; lists never wrap a line; Subtitle style lines its two cards up row for row;
  the lines-by-day chart fills a maximized window. English labels use sentence case.
  ([dd36f5e](https://github.com/ACAne0320/Tomoru/commit/dd36f5e),
  [e51858f](https://github.com/ACAne0320/Tomoru/commit/e51858f),
  [9c0d150](https://github.com/ACAne0320/Tomoru/commit/9c0d150),
  [9075a60](https://github.com/ACAne0320/Tomoru/commit/9075a60),
  [d4113ab](https://github.com/ACAne0320/Tomoru/commit/d4113ab),
  [7f9a251](https://github.com/ACAne0320/Tomoru/commit/7f9a251))
- When a translation service says it gets too many requests, translate ahead pauses for a
  minute, so the lines' own translations still get through.
  ([968ccda](https://github.com/ACAne0320/Tomoru/commit/968ccda))

### Fixed

- A subtitle line without a translation says why: the key refused, the quota used up, the service
  not answering, or translating fell behind. ([553894d](https://github.com/ACAne0320/Tomoru/commit/553894d))
- When on-device recognition falls behind, translating stops and says so, instead of the
  subtitles freezing while it still says it listens. ([b1e794a](https://github.com/ACAne0320/Tomoru/commit/b1e794a))
- Long lines are no longer cut short by the translation's length limit; a translation that is cut
  ends with an ellipsis. ([8e1a9d5](https://github.com/ACAne0320/Tomoru/commit/8e1a9d5))
- Deleting the translation API in use stops translating and says so, instead of sending the next
  lines to the cloud; Undo puts it back. ([962ded7](https://github.com/ACAne0320/Tomoru/commit/962ded7))
- English messages name the Speech & Translation tab rightly, and on Windows the Locked message
  says to click the taskbar icon. ([86a8e30](https://github.com/ACAne0320/Tomoru/commit/86a8e30))

## 0.1.3

- Deleting a scene, a session or a translation API no longer asks first: Undo in the message that
  follows brings it back. Deleting a downloaded model asks by its button, and, when the model is in
  use, says what takes over. ([#2](https://github.com/ACAne0320/Tomoru/pull/2))
- A dialog closed with Esc or a click outside asks before throwing away what you typed.
  ([#2](https://github.com/ACAne0320/Tomoru/pull/2))
- The on-device speech recognition models say where they run: SenseVoice on the CPU, no GPU
  needed; Qwen3-ASR on the GPU, with the memory it needs. ([#2](https://github.com/ACAne0320/Tomoru/pull/2))

## 0.1.2

- The app's icon is redrawn as Tomoru behind a glowing paper lantern. ([3485da8](https://github.com/ACAne0320/Tomoru/commit/3485da8))
- A picture Index-Translate's cloud doesn't start answering within 10 seconds is asked again on
  a new connection, instead of failing after a minute and failing again on retry. ([ce1b8b3](https://github.com/ACAne0320/Tomoru/commit/ce1b8b3))
- The website is translated into Traditional Chinese, English, Japanese and Korean, with
  localized example subtitles and an on-page language switcher. ([7e56878](https://github.com/ACAne0320/Tomoru/commit/7e56878))

## 0.1.1

- Subtitles start immersive, the text alone with an outline, in the Ice colors. Settings you
  saved keep their look; Reset style takes this one. ([6a25ce5](https://github.com/ACAne0320/Tomoru/commit/6a25ce5))
- Hiding the subtitles when no new ones come is now a switch: turn it off to keep the last
  subtitles on screen until you stop translating. While it is on, choose how many seconds they
  stay under it. ([f5c6ec2](https://github.com/ACAne0320/Tomoru/commit/f5c6ec2))
- When Index-Translate's cloud doesn't answer a picture, its card says so plainly instead of
  showing a raw error. ([92e35c0](https://github.com/ACAne0320/Tomoru/commit/92e35c0))

## 0.1.0

The first beta.

- Live subtitles over whatever your computer plays: the original and its translation, in any direction
  between 31 languages, Chinese, English, Japanese and Korean among them, and into Traditional
  Chinese. The languages to speak are those the recognition chosen hears: 10 with Apple's
  recognizer on macOS 26, 5 with SenseVoice, 30 with Qwen ASR. Only Chinese, English, Japanese and
  Korean have been benchmarked; the others segment by rules for languages written alike.
- On Windows 11 x64 too: from the taskbar's corner, recognizing with SenseVoice or Qwen3-ASR,
  downloaded in the first-run guide, or in the cloud, set up in Settings. Selecting a part of the screen to translate
  works there as on a Mac, and the local translation runs on any GPU (Vulkan), or on the CPU.
- Hear all of the system's sound, or one app's.
- Start and stop from anywhere with ⌥⇧T, or a shortcut you record in Settings.
- Speech recognition on your computer: with Apple's recognizer on a Mac, or with SenseVoice or
  Qwen3-ASR 1.7B, models downloaded in the first-run guide or Settings (offline; Qwen3-ASR is recommended where a GPU can
  run it); or in the cloud, with your API key: Qwen ASR on Alibaba Cloud Model Studio, which stops
  sending audio while nothing plays, or OpenAI's live transcription, or its transcription API or a
  service compatible with it, which are sent only speech. Chinese clauses show as soon as the speaker pauses.
- Translation in the cloud (Index-Translate, free), on your computer with a model downloaded from
  Settings (Index-Translate 2B or 9B, offline), or with a translation API of your own.
- Translate ahead: the gist of a sentence shows before it ends.
- Reaches the cloud through the system's proxy, as browsers do, or one set in Preferences, or
  directly; models on your computer never go through it.
- Subtitles you can move, resize and lock, in styles of your own, hidden from screen recordings if
  you like: six color schemes, or any color you pick.
- Subtitles in the order you like: the translation above the original, or under it.
- Scenes: what you watch, each with its languages, a description and a glossary of names, so a
  name comes out the same every time, and a name the recognizer mishears is corrected. The
  description goes with each line to the translator (but the on-device 2B model, which made names
  up from it), to help with titles and misheard names.
- History, if you turn it on: the subtitles of each session kept on your computer, by scene and by day,
  with the names corrected marked, each a file you can show in Finder or Explorer. A chat model of your own
  can learn a scene's names from them.
- Statistics: how long you translated, how much, and with which engines, day by day in a chart;
  the minutes a cloud recognizer was sent and the tokens a translation API took; and how long you
  wait, from the end of a sentence to its translation, step by step.
- A menu bar panel to start, switch languages, sources and scenes, and adjust the subtitles; its
  icon is Tomoru's girl, eyes open while translating.
- Light and dark, as your Mac is, or as you choose.
- Deleting asks first, and a scene, a session or an API deleted can be put back.
- Translate text on screen, such as a game's dialogue: press ⌥⇧S and select it, once you turn on
  selecting (it needs Screen Recording; only what you select is captured), or copy part of the
  screen with ⌃⇧⌘4 first, then press ⌥⇧S. The translation streams into a card by what you selected (by
  the menu bar icon for a copied picture), with the scene's names; click the picture on it to see
  it whole. Neither the picture nor its translation is kept. Pictures go
  to the translation engine you chose: an on-device model reads them itself (its download includes
  the file for it, 360 MB for 2B, 620 MB for 9B); an API of yours that can't read them hands them
  to the cloud, and the card says so.
- A first-run guide that has you try it for real, on a video or on sentences your Mac says aloud,
  and checks that hearing, recognition and translation all work.
- Smoother subtitles: outlines and shadows drawn on the GPU, and a translation that replaces its
  draft fades in.
- Hovered or moved, the subtitles and their toolbar sit on clear Liquid Glass, what plays showing
  through.
- In English, Simplified and Traditional Chinese, Japanese and Korean.
