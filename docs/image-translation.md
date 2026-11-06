# Image translation

[中文](image-translation_zh.md)

## Goal

Subtitles can only translate what is heard. A game's dialogue box, signs and captions in a video,
a stream's chat: none of these make a sound. With a shortcut, the user sees the translation of a
part of the screen. The whole Index-Translate family (2B / 9B / 35B) takes images natively, so no
separate OCR is needed: the image goes with the prompt, and the translation comes back.

## Decisions

| Item | Decision |
|---|---|
| What to capture | A selected area, not the whole screen. In the experiment, selecting just the dialogue box was fastest and most accurate; the whole screen mixes in status bars and menus, and repeats lines |
| Permission | None by default: translate the screenshot on the clipboard. Select-to-translate is optional, and needs the user to turn on Screen Recording in System Settings |
| Translation | An engine of its own, chosen in Speech & Translation › Picture Translation, among those that read pictures, with the current scene's whole glossary ([scenes](scenes.md)), which got the names right in the experiment. See [Which engine](#which-engine) |
| Result | A floating card: the thumbnail above, the translation below, streamed with MyGo's `markdown.Stream` (an even pace, a character at a time, fading in), as GitHub Flavored Markdown, which a model may write of a page it reads, its single line breaks kept, as a game's dialogue has one a speaker; can be copied; closed with Esc or the close button |
| Keeping | Neither the image nor its translation is kept. The statistics count how many were translated, a translation got for each, by day, under the engine that translated them; never what they said |
| Shortcut | ⌥⇧S by default (`prefs.ImageShortcut`), customizable; it can't be the same as the start/stop shortcut. The menu bar panel's Screenshot button does the same |
| Automatic detection | No: capturing every so often and translating changes costs more and translates the wrong things more often |

## Getting the image

On macOS, an app needs the Screen Recording permission to read **other apps' windows**. The
system's own screenshot (⌃⇧⌘4) isn't limited by this. Tested: system screenshots that Tomoru starts
(`screencapture -i`, `-R`) all count as Tomoru's; without the permission, the request pops up as
the mouse is released, and if refused nothing is captured. The system's window picker
`SCContentSharingPicker` needs no permission, but it's made for screen sharing, picks only whole
windows and doesn't feel like taking a screenshot, so it wasn't used.

So there are two ways:

| Way | Permission | How |
|---|---|---|
| **Clipboard** (default) | None | The user copies a screenshot with ⌃⇧⌘4 first, then presses ⌥⇧S. The clipboard is read with `mygo.Clipboard.ReadImage()`, which turns TIFF into PNG |
| **Select-to-translate** (optional) | Screen Recording | ⌥⇧S starts selecting, and the selection is translated once done; Esc cancels. Selection always goes through the engine's `select` (`internal/capture`): on macOS it runs the system's `screencapture -i -x` and watches where the left mouse button goes down and up while it runs, to report the area; on Windows it's the engine's own selection window. The screenshot goes in a temporary directory and is deleted once read |

In the first-run guide, select-to-translate is a step that can be skipped: it asks first; if the
user wants it, they turn on Tomoru's Screen Recording in System Settings, choose a shortcut, and
try it on the sample dialogue on the right (`assets/samples/`, made by `guide/samples_test.go`).

- macOS only offers "Open System Settings"; the app can't grant it with one click. When the system
  then asks to "Quit & Reopen", the user can choose "Later": the permission check and the capture
  both happen in a freshly started engine process each time, so it works without relaunching. If
  relaunched, the guide returns to the step it stopped on.
- Checking and requesting the permission happen in the engine (`screen-access`,
  `request-screen-access`, see the [engine protocol](engine-protocol.md)): the engine is a child
  process Tomoru starts, and the system counts it as Tomoru. While the relevant screens are open,
  it's checked every 3 seconds, and again at once when a selection fails (turned off in System
  Settings, say).
- The first selection, macOS confirms once more, and again from time to time after. These are the
  system's rules for screen recording.
- Without select-to-translate, ⌥⇧S still translates the screenshot on the clipboard; the feature
  never goes away.

Tomoru's overlay can be hidden from screen recordings (optional, `sharingType = none`), so it
doesn't appear in the capture.

## Which engine

Pictures have an engine of their own (`Preferences.ImageMT`), apart from the subtitles'
(`Preferences.MT`): some engines read no pictures, as most text-only APIs a user adds. It is
always an explicit choice, in Speech & Translation › Picture Translation, from a menu of the
engines that read pictures now (`Core.ImageEngines`): Index-Translate 35B in the cloud (the
default), the local 2B and 9B once downloaded, and the user's APIs but those whose test found
they read none. Under it, with an engine in the cloud, a line says the service receives the
pictures. The picture's shortcut is in that group too, as in Preferences.

- **Settings from before**: an empty `ImageMT` is set once, at launch, from `MT` where that reads
  pictures (a local model, an API not found to read none), else the cloud. In the first-run guide,
  choosing the translation engine chooses it for pictures too, where it reads them.
- **Testing an API**: the add/edit API dialog's Test sends, alongside the sample sentence, a small
  picture of it (`testPicture`). When the sentence comes back, the picture's answer says whether
  the API reads pictures: translated, it does; an error status (400 at DeepSeek, 500 at a
  llama-server without a vision projector; not 429) and it does not; anything else, not known. The
  result shows under the translation (reads pictures / can't read pictures) and is kept on the
  endpoint (`Endpoint.Vision`), at once if it is saved and the same API, else when it is saved.
  Changing its address, model or format makes it not known again. APIs saved before stay listed
  until a test finds they read none.
- **Gone**: a model deleted, an API deleted or found to read none, while chosen for pictures, gives
  way to the cloud, as live translation does with `Successor`; undoing the API's deletion chooses it
  again. A test that finds so says it under its result (`TestResult.ToCloud`).
- **Only the one chosen** (`EngineService.imageEndpoint`): a picture never goes to another engine.
  When the chosen one can't translate it (its model missing, the models moving, the API failing),
  the card says why.

| Engine | Images |
|---|---|
| Index-Translate in the cloud | as before. Its first words come in about 2 s; when they haven't in 10 s, the picture is asked again, once, on a new connection (`Endpoint.ImageWait`): through a proxy, a connection to it at times takes a picture and answers nothing until the server closes it minutes later, and HTTP/2 would send the picture again on that one (2026-11-01). If that one doesn't answer either, the card says so as the subtitles do |
| A local model (2B / 9B) | read by llama-server with the model's vision projector, `--mmproj` (Q8_0: 345 MB for 2B, 592 MB for 9B, from the same repositories). It is one of the model's files (`models.Model.Vision`), downloaded and deleted with it, so a local model always reads images. llama-server loads it only for the first image (`Server.EnsureVision`): it costs 430 MB more memory for 2B, 680 MB for 9B, which subtitles alone needn't take. A server translating subtitles starts again with it on the same port, so the session's translator keeps its URL (the first image takes about 2 s, the next 0.6 s), and keeps it. If no session translates with a local model, llama-server, started for an image, stops when the card closes |
| A local model while another translates the subtitles | fails with `ErrModelBusy`, which the card says: llama-server runs one model at a time, and starting the other would stop the subtitles' |
| An API of the user's | sent the image; if it fails (it may read no images), the card says so, with the API's error: no other engine is asked, as the picture goes only where the user chose |

On this Mac (M-series, 2026-10-28), the guide's Japanese dialogue (1040 × 360, 390 tokens) took
0.9 s with 2B and 2.7 s with 9B once loaded, with the names right. A local model reads at most 1280
pixels on the long side (`provider.LocalImageSide`): llama-server's 4096 tokens of context are
shared by its slots, and a 2048-pixel screenshot took 2971 of them; the next image then crashed
the server. 2B asked for English at times says a name and nothing else, line after line, until it
runs out of tokens (15–27 s); a line said three times in a row ends the translation there.

## The request

`translate/image.go`:

- **Format**: OpenAI's multimodal format, a text part and an `image_url` (a base64 data URL) in the
  same message.
- **Size**: Index recommends 512–2048 pixels on the long side; beyond 2048 (1280 for a local
  model) it's scaled down with CatmullRom. Retina screenshots are 2×, so a 600-point selection is 1200 pixels, usually left as
  is. Below 1000 pixels it's scaled up two or three times: on a screen at 100% scale (usual on
  Windows), text is about 12 pixels high, and the model lost words, garbled Korean and repeated
  Chinese lines; twice as large, it read them right.
- **Prompt**: `请将图片中的文本翻译为<target language>，直接输出翻译结果。` ("Translate the text in the
  image into <target language>; output only the translation."). When the current scene has a
  glossary, it switches to the instTrans form, adding `【硬性要求】专名/术语对照: A→B、C→D` in
  【约束要求】, as subtitle translation does. Subtitles carry only the entries relevant to the
  sentence; which names are in an image isn't known in advance, so the scene's whole glossary goes
  along. Image translation sends one request at a time, so a longer prompt doesn't matter.
- **Source language**: not given, as an image may mix languages. The target is the subtitles'
  target language, unless another is chosen on the card (`prefs.ImageTgt`): choosing one translates
  the image shown again and keeps it for the next ones, without changing the subtitles or restarting
  them; choosing the subtitles' language again follows it as it changes. The glossary goes along only
  in the subtitles' language, the one its names are in.
- **Cleaning the output**: consecutive repeated lines are removed (whole lines repeated twice in
  the experiment), while streaming too; a line said three times in a row ends the translation.

The state is in `core/image.go` (`ImageService`); closing the card cancels a request still
running.

## The card

`ui/imagecard/`: a MyGo Panel window that floats above all apps and, on macOS, takes the keyboard
without activating Tomoru.

- **Where**: under the selected area, aligned to its left, or above it when there's no room
  below; from the clipboard, or a window picked with Space on macOS, beside the menu bar (taskbar)
  icon. Drag the title bar to move it; clicking outside doesn't close it.
- **Size**: 440 wide by default, and as tall as the thumbnail and four lines of translation need
  (`imagecard.FitHeight`), 440 at most, so a strip of dialogue gets a short card; the user resizes
  it by its edges (340 × 300 at least), and from then on every card opens at that size
  (`prefs.ImageW`, `ImageH`), within the screen, fitted no more. Once shown, the card never resizes itself: a translation longer than it scrolls, following its end as it streams in
  unless the user scrolled up. Growing with the text showed, at each line, a frame of the card cut
  short before the window caught up.
- **Contents**: the thumbnail, the translation (streamed; until it comes, three placeholder lines
  breathe in its place) and Copy. The image shows too: numbers were mistranslated in the
  experiment (49980 became 49800), so users can check against it. Clicked, the picture shows whole in
  a window of its own, in the middle of the screen, as large as it was on the screen (smaller,
  to nine tenths of the screen, where there isn't room; never cut). Drag the picture to move it,
  its edges to resize it, the picture fitting; its close button, a see-through disc that shows only
  while the pointer is over the picture and turns red under it, or Escape closes it, and so does
  closing the card.
- **Foot**: what translated it, the engine chosen for pictures, "Powered by Index-Translate, in the
  cloud" (or on this computer, or the user's API), Index-Translate a link to its page; its tooltip says neither the picture nor
  its translation is kept.
- **No image**: without Screen Recording and with no image on the clipboard, it asks for Screen
  Recording, as the guide does (turn it on; denied, open System Settings); once on, a Screenshot
  button selects a part of the screen.

## Later, maybe

- Detecting game dialogue automatically; drawing the translation over the original (the model
  returns only text, no positions).

## Tests

- Building the request: the prompt with and without a glossary, scaling, the data URL; streaming
  and removing repeats against an httptest stand-in for Index.
- The engine (`core/image_engine_test.go`): settings from before, the guide's choice, the menu
  (no text-only API, no model not downloaded), a picture going only to the engine chosen, a busy
  local model, an API's test finding whether it reads pictures, and deletions.
- The card: Tester tests of the permission states, streaming, Copy, choosing the language, Esc and errors; its height for a strip and a page, waiting, and the whole picture; where its window goes and how large (`pictureBounds`).
- By hand: selection and the permission request in a packaged build without Screen Recording (a
  development machine's terminal usually has it already, which hides the problem). Afterwards,
  `tccutil reset ScreenCapture dev.acane.tomoru` returns to never asked.

## Experiment (2026-10-20)

Index-Translate 35B in the cloud, `enable_thinking: false`, streamed. Images rendered with
headless Chrome.

| Image | First character | All | Result |
|---|---|---|---|
| A Japanese game, full screen (dialogue + choice menu + status bar), 1280×720 | 0.34 s | 1.19 s | Fluent, but the speaker's name was left out, and クロード came out as 克洛德 |
| The same, with a glossary (クロード→克劳德, エルナ→艾露娜, 星の欠片→星之碎片) | 0.32 s | 1.21 s | Names all as in the glossary, speaker included; but the status bar line came out twice |
| Only the dialogue box, 1120×170 | 0.24 s | **0.50 s** | Cleanest: "埃尔娜：克劳德。那座塔顶上发光的东西，该不会就是“星之碎片”吧？……" |
| A Japanese news page, 1280×800 | 0.31 s | 3.40 s | Good overall; "4万9980円" became 49,800 yen, "来春" (next spring) became "spring" |
| A Korean stream page (chat), 1000×640 | 0.29 s | 1.40 s | "도배하면 차단됩니다" (spamming gets you banned) mistranslated as "if you help, I'll block you"; the platform name "치지직" half translated; the last line repeated |

In the app (a Japanese dialogue box, 1120×340): first character about 0.5 s, all of it 1.3–1.9 s;
without a glossary the names came out as 克洛德 and 埃尔娜, with it as 克劳德 and 艾露娜, and 49,980
was right.

Conclusions:

- a small area reads in under a second, a whole web page in over 3;
- glossaries work for images too;
- the more is captured, the more repeated and missed lines, so select;
- numbers and slang are sometimes wrong, so the card shows the image too.
