# Changelog

## Unreleased

- Picture translation has its own engine, in Speech & translation › Picture translation, chosen
  from the engines that read pictures: Index-Translate 35B in the cloud by default, a downloaded
  on-device model, or an API of yours. Testing an API now says whether it reads pictures, and one
  that can't isn't offered. A picture still goes only to the engine you chose. Settings from
  before keep using the subtitles' engine for pictures where it reads them.
  ([#5](https://github.com/ACAne0320/Tomoru/issues/5))

- The on-device models can live in a folder you choose, on another drive if the system one is
  full: Speech & translation › Model folder › Change… moves the models into a "Tomoru Models"
  folder there, with its progress, and keeps them there. ([#5](https://github.com/ACAne0320/Tomoru/issues/5))
- The settings, the menu bar panel and the first-run guide share one tightened design: rows of
  one height whose dividers line up across cards, group titles above their cards, the chosen
  engine on a tinted band with its button and dot on one line, and a dark appearance with clearly
  separated layers and no cream buttons.
- Subtitle style lines its two cards up row for row, their controls in one column. With
  Translation only, the original's size and the order stay in place, greyed, instead of
  disappearing; Custom colors open in their own card under them, each with the system color
  picker as before.
- Home, Scenes and History use the same design. A session looks the same on Home as in History,
  with its scene as the title and its languages, lines and app under it. Clicking one on Home
  opens it in History. Lists never wrap a line: what doesn't fit is dropped from the end and shows
  in a tooltip. Keep subtitles is a setting row at the top of History, and the scene filter shows
  once there is something to filter.
- Statistics titles its two cards, Usage and Latency, each with its switch beside the title;
  latency names too long for a tile take two lines instead of being cut. About lists the version,
  the first-run guide and the logs as rows, each with its button. English labels use sentence
  case throughout ("New scene", "Learn names"), and Preferences no longer cuts "Screenshot
  translation" short.

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
