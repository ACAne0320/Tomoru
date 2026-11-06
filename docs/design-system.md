# Tomoru's design language: Night Lantern

[中文](design-system_zh.md)

This is the design specification for the settings window, the menu bar panel, the first-run guide
and the image translation card. The subtitles' own look is the user's to set, and this
specification doesn't govern it.

## Direction

Tomoru is the app that "lights up" subtitles: its name is 灯る, and its mascot, the girl Tomoru
(ともる), holds a paper lantern; Rubi (ルビ), her penguin, sits on her head. The interface is a
lantern's paper at night:

- **The frame is the night**: indigo, from the mascot's hair. The sidebar sits on the night.
- **The content is paper**: cream, from her hoodie. Every page's content is on the paper.
- **The accent is the lantern's light**: amber, from the lantern. Only for the one most important
  action, the selected state and a few hints.
- **Subtitles always sit in the night sky**: the subtitle preview's "stage" is deep blue, with a
  few stars and a warm glow at the bottom, matching the subtitles the user actually sees.

Principles:

1. **State first, then actions.** The first thing on the home page is "Translating" and the stop
   button; settings come after.
2. **Plain.** Buttons say the action (Start translating, Stop translating, Use Model Studio
   instead); no metaphors (not "put out the lantern"), no icons the user has to guess.
3. **No repeats.** Say each thing in one place. If the button already shows the state, don't add a
   label; if the sentence already names the languages, the stage doesn't name them again.
4. **Change it where it is.** The home page's sentence "Hear [Japanese] in [Safari], in
   [Chinese]" can be clicked directly; don't send the user to another page to change one value.
5. **Group what grows.** Providers, scenes and records can all grow: group them first (on device /
   cloud, by provider, by date), then list them; no flat lists that stretch on forever.

## Rules

| Rule | How | How it's kept |
|---|---|---|
| Reuse mature libraries | All interaction uses MyGo `ui`'s Base components (`ButtonBase`, `SwitchBase`, `SliderBase`, `SegmentedBase`, `TextInputBase`, `TextAreaBase`, `PopoverBase`, `ToastBase`, `TooltipBase`), which handle keyboard, focus and screen readers; we draw the looks. Dropdowns are the system's native menus (`MenuButton`). Icons are Lucide only, fonts are system fonts only | No other UI library, icon set or font file |
| Own the components' source | Every visual component is in `internal/ui/kit`, one file each, each with Tester tests; `kit` has a gallery test that renders every component in each state as pictures, to review one by one. MyGo's problems are fixed upstream with PRs | The layering check (`layers_test.go`); pages only compose `kit` components |
| One design language | Colors, type sizes, spacing, corners, shadows and motion are all in `kit/tokens.go` | `design_test.go`: UI code outside `kit` may not use `ui.Hex` / `ui.RGB` / `ui.RGBA` / `ui.Oklch`, nor pass numeric literals (other than 0) to `FontSize`, `Padding`, `Gap` or `Radius`. The only exceptions are the overlay (`overlay/`) and subtitle drawing (`subtitle/subtitle.go`): the user sets the subtitles' colors and size. No UI code, `kit`'s included, makes a step of its own out of a token and a number (`TextBody+1`, `RadiusCard+2`, `RadiusInner/2`, a padding of `Space(3)+2`), and `Space` takes whole steps (`TestDesignScale`; `Space(2.5)` doesn't compile) |
| Few dependencies | `go.mod`'s direct dependencies are only `mygo`, `x/sys`, `coder/websocket` for cloud recognition, and `purego`, `x/image` and `x/text`, which MyGo brings anyway. Each pattern has one implementation: one button system, one toast, one dialog, one card, one empty state | `design_test.go` pins the list of `go.mod`'s direct dependencies: adding one means changing the test, which a review can't miss |

## Tokens

### Colors

Named by use, not by hue; defined in `kit.Light` and `kit.Dark` in `kit/tokens.go`, and read with
`kit.Colors(c)` for the current appearance. `kit.Paper(c)` maps them onto `ui.Theme`, so MyGo's own
controls follow; the settings window and the panel call it through `AppFrame` and `PanelFrame`,
and the guide calls it directly.

Light values:

| Token | Value | Use | `ui.Theme` |
|---|---|---|---|
| `Night` | `#1b1949` | Window frame, sidebar, panel header | |
| `NightLine` | `#2c2970` | Frame outline | |
| `NightText` | `#d3cff6` | Text and icons on the night | |
| `NightTitle` | `#fbf3e2` | Titles on the night (the Tomoru wordmark) | |
| `NavOn` | as `Paper` | The selected sidebar item's ground | |
| `Stage` | `#15133d` | The subtitle stage | |
| `SkyTop` / `SkyBottom` | `#13123a` / `#33215c` | The night sky's gradient in the guide | |
| `Glass` / `GlassLine` | white 8% / 14% | Glass panels on the stage | |
| `Starlight` | white | Text on the night sky; check marks on colored dots | |
| `Shade` | black | Shadows of the menu bar and keys | |
| `KeyLit` | `#4d8dff` | A key the guide saw pressed right | |
| `Paper` | `#fbf6ec` | Page paper | `Background` |
| `Card` | `#ffffff` | White cards, inputs | `Surface` |
| `Sand` | `#f4ead6` | Sunk into the paper: list items, a note where a list is empty | |
| `Fill` | `#f1e5cb` | The track of pickers, segmented controls and steppers; a disabled button's face | `SurfaceHover` |
| `SegOn` | `#ffffff` | The chosen segment, raised on its track | |
| `Line` | `#efe4cc` | Card outlines | `Border` |
| `LineSoft` | `#f3ead8` | Dividers inside cards | |
| `LineStrong` | `#e6d8bb` | Dashed boxes, pressed faces | |
| `Raised` / `RaisedLine` | `#ffffff` / `#e6d8bb` | A plain button's face and outline, a step above its card | |
| `Ink` | `#232057` | Body text, titles | `Text` |
| `InkMuted` | `#6b6590` | Secondary text (5.0:1 on `Paper`, 4.5:1 on `Sand`) | |
| `Faint` | `#9d9ab9` | What isn't there yet or can't be used: placeholders, a disabled button's label | `TextMuted` (MyGo draws placeholders in it) |
| `InkWarm` | `#73644a` | Times and numbers on sand (4.8:1 on `Sand`) | |
| `Hatch` / `Pencil` / `Lift` | `#efe4cc` / `#aaa5c4` / `#3a2a10` 5% | Empty states' drawings: the hatching of their ground, their icons and marks, the shadow under what stands on it | |
| `Lantern` | `#f5a524` | Primary action, the chosen radio's dot, switch on | `Accent` |
| `LanternInk` | `#232057` | Text on amber | `AccentText` |
| `LanternTint` | `#fff0d1` | Icon grounds, hint grounds, the "Learn names" button | |
| `LanternText` | `#8a5200` | Amber text, links | |
| `Amber` | `#f3ead8` `#fbefd6` `#fadda9` `#fcc771` `#f5a524` `#d27908` | The lantern's ramp, opaque, from none to most: a heatmap's days, a histogram's bars; `Dim` takes amber two steps down it, other colors toward the card, beside what is chosen. Never `Lantern.Alpha` | |
| `Band` | `#fff3d9` | The chosen row's ground across the row: options, list items, tiles, shares | |
| `DotRing` / `SwitchOff` | `#c9b997` / `#d1c3a6` | A radio's ring, a switch's track while off | |
| `Strong` / `StrongInk` | `#232057` / `#fbf6ec` | A Strong button's face and label, a chosen chip | |
| `Ok` | `#3fa463` / text `#2f6b45` / ground `#e7f3ea` | In use, ready, the "On device" tag | `Success` |
| `Cloud` | text `#3b3592` / ground `#e8e6fb` | The "Cloud" tag | |
| `Danger` | `#a4472f` | Delete | `Danger` |
| `Scrim` | night 42% | Dims the window under a dialog, in the night's indigo rather than grey | |
| `CardShadow` / `SegShadow` / `RaisedShadow` | `0 10 24 −14` warm brown 22% / `0 1 3` 18% / `0 1 2` 8% | A card raised a little, a chosen segment, a plain button | |
| `SubSrc` / `SubTgt` | `#f4f2ff` / `#f5a524` | Original and translation on the stage, as the "Amber" scheme colors them: the translation in the lantern's amber (the subtitles' default is "Ice", immersive and outlined) | |
| `Series` | `#f5a524` `#8a5200` `#e8cf9f` `#4a3a24` | A chart's series of no color of their own: on the statistics page, one's own APIs and OpenAI's recognition, each by its id, so always the same; what no engine was noted of is `LineStrong` | |
| `Apple` / `Qwen` / `Alibaba` | `#007aff` / `#615ced` / `#ff6a00` | The engines' makers' colors, so an engine keeps its color in any chart: Apple's speech recognition, Qwen's (its cloud and Qwen3-ASR), SenseVoice (Alibaba's Tongyi Lab) | |
| `Bilibili` | `#fb7299` `#d63f73` `#fdb2c9` | Index-Translate's, bilibili's pink: in the cloud, deeper for 9B, paler for 2B | |

**Dark appearance**: by default it follows the system; UI code checks `c.Theme().Dark`, and
windows redraw when the system switches. The Appearance preference chooses system / light / dark
(`Prefs.Appearance`), applied with `mygo.Theme.SetSource` before `App.Run`. Dark has values of its own,
not the light ones turned over: four layers a step of lightness apart, `Night` `#0e0a2f` < `Sand`
`#171537` < `Paper` `#222049` < `Card` `#312f61` (the sidebar's chosen page is `Paper`); tracks
sink (`Fill` `#181639`) and the chosen segment rises (`SegOn` `#4c4a88`); `Strong` is a brighter
indigo, `#5455b6`, never the cream of `Ink`; the tint and the ramp warm from the indigo through
mauve (`LanternTint` `#603d6b`, `Band` `#4c3155`, `Amber` `#181639` … `#f5a524`), never through
brown; shadows are black. Text is the cream of `NightTitle`, `Faint` `#8784ac`; the amber stays.

### Type

System fonts only (PingFang for Chinese). Numbers use tabular figures (`tabular-nums`), so live
numbers don't wobble.

| Token | Size / weight | Use |
|---|---|---|
| `Display` | 32 / 800 | Page titles (Translating, Subtitle style) |
| `Stat` | 26 / 800 | Numbers in live statistics |
| `Title` | 22 / 800 | Detail titles (a scene's name, a record's name), a dialog's, a step of the guide's |
| `Section` | 15 / 700 | Group titles; a large button's label |
| `Body` | 14 / 400, 600, 700 | Body text, a setting's and an option's name |
| `Control` | 13 / 600, 700 | Buttons, segments, pickers, chips, tables |
| `Caption` | 12 | Descriptions, states, paths |
| `Micro` | 11 | Tags (On device / Cloud), count badges, a chart's ticks |

Eight steps, named. A size between them is a ninth to name in `kit/tokens.go`, never arithmetic on
these. A number keeps its unit: sizes and memory join them with a no-break space (`shared.Size`,
`{gb} GB` in the catalogs), so `16 GB` never breaks.

### Spacing, corners, shadows

- **Spacing**: whole steps of 4, `Space(n)`: 4 between a name and the note under it, 8 between
  buttons, 12 between the parts of a row, 16 between cards (`CardGap`, across and down alike) and a
  card's top and bottom, 20 a card's sides, 24 between a page's parts (`SectionGap`), 32 its
  margins; a group's title is 8 above its card (`GroupGap`). Inside a component, its own geometry
  (a track's 4, a segment's 2).
- **Rows**: a `SettingRow` is 52 tall, 64 with its note, whatever its control, so that the rules of
  cards side by side line up. In any row with something at its start and its end (a setting, an
  option, a list item), both are centred on the row's name and note together; what goes below
  them (a progress bar, a note) never moves them.
- **Corners**:
  - pills and round buttons: half their height;
  - cards, the stage, dialogs: 20 (`RadiusCard`);
  - tiles, list items, the rows in a list's card: 16 (`RadiusTile`);
  - inputs, tracks, pickers: 14 (`RadiusField`);
  - segments, a stepper's buttons, small icon grounds, table cells: 10 (`RadiusInner`);
  - a chart's bars and tips, a mark in a line of text: 4 (`RadiusMark`);
  - paper: 14;
  - window frame: 18.
  - One inside another is the outer less the padding between: a segment (10) in its track (14),
    4 in; an option row (16) in its card (20), 4 in.
- **Outlines and shadows**:
  - cards have a 1 px `Line` outline and a faint warm shadow (`CardShadow`, black in dark); a
    group's title sits above its card, outside it;
  - what is chosen in a list (an option, a list item, a tile, a share) is the `Band` across its
    row, with no outline; the chosen segment is `SegOn`, raised by `SegShadow`; only swatches keep
    an amber ring;
  - a plain button is raised a step (`Raised`, `RaisedLine`, `RaisedShadow`);
  - only the amber primary button has a warm shadow (`0 8 24 Lantern 45%`).
- **Warm glow**: MyGo has no radial gradients (only linear and conic). The glow at the stage's
  bottom is an ellipse with a wide amber shadow.

### Motion

All of it is in `kit/tokens.go`; nothing else picks its own durations.

- `Spring` 460 ms (bounce 0.08): changes of position, such as the selection sliding between
  options.
- `Rise`: what appears fades in while rising 8 DIPs on the spring, a step of the guide included;
  the items of a group one after another, 40 ms apart (`Rise(i)`). Nothing comes in sideways.
  `Arrive` is the same entrance played as an element is first built, for what comes with what
  holds it: a step's parts, the first window frame, text in a pane that has just appeared.
- `Grow` (`Resize`, a 340 ms spring without bounce): a pane taking another size as its content
  changes, clipping it on the way; what comes into it rises after `Settle` (200 ms), when the pane
  is about there, so text never meets its edge.
- `Fade` 200 ms: colors and states changing in place (lit, dimmed). `FadeQuick` 120 ms: what goes
  before what replaces it comes, so that the two never show over each other.
- With the system's Reduce Motion on, everything switches directly.

The guide's engine steps show the way of a sentence as one timeline: a light goes down from what
is heard to recognition, the sentence comes in there, a light goes on to translation, its
translation comes in. The pane of the step's choice is lit: brighter glass and an amber hairline;
the others dim only their content; choosing another engine flashes its pane once. Trying it, the
same panes run on what is really heard, each check ticking as the light reaches its pane.

The stage's stars and its glow don't move.

### Icons

- Lucide only, stroke width 2.
- Sizes: in small buttons 14, in a header's buttons 15, menu rows 16, sidebar 17, tiles and a
  large button 18 (`IconSize…`).
- Copy SVGs from Lucide into `internal/ui/assets/icons/`, only the ones used (license in that
  directory's `LICENSE`).
- Icon-only buttons must have screen reader text (`Label`) and a hover tooltip (`Tooltip`).
- The app icon is Tomoru peeking out from behind a glowing paper lantern at night, the
  subtitles' two lines lit on its paper, Rubi sitting on top. `swift scripts/app-icon.swift <art>` makes
  `assets/icon.png` and `app/icon/Tomoru.icns` (`macos.icon`) of the mascot art, the same picture
  at every size. The menu bar panel's header shows the mascots as a sticker (`assets/sticker.png`):
  Tomoru looks up at Rubi, who holds a lantern on her head, in a cream outline, cut out of its background. While not translating both
  sleep, eyes closed (`sticker-asleep.png`), as the menu bar icon does; the two cross-fade.
- The menu bar icon is Tomoru peeking up, her head tilted and her ahoge up: eyes closed when not
  translating, open while translating. Sources are `assets/tray-idle.svg` and `tray-on.svg`;
  `swift scripts/tray-icons.swift` makes the PNGs.
- A click on it opens the menu bar panel, Tomoru's own; a right-click (Control-click on macOS) the
  system's menu, on macOS and Windows alike: start or stop translating and Screenshot, each with its
  shortcut, Open Tomoru and About Tomoru, then Quit Tomoru. It does one thing at once, or quits,
  without the panel; what one adjusts stays in the panel, which quits too, for those who never
  right-click.

## Components

`kit`'s components by category. "Built on" is the MyGo component each uses.

**Frames**

| Component | Use | Built on |
|---|---|---|
| `AppFrame` | The settings window: night frame and paper, navigation on the left; the system title bar hidden | `Row`, `Column` |
| `NavItem` | A sidebar item: icon and text; a paper pill when selected. The sidebar's width follows the longest page name in the current UI language, 184–252 (`kit.NavWidth`) | `ButtonBase` |
| `Group` / `Groups` | A part of a page: its title (and a control at its end) above its card, outside it; `Groups` sets them side by side, `CardGap` apart, as tall as the tallest | |
| `Card` / `RowsCard` / `OptionsCard` / `SandCard` | White card raised a little; the same for `SettingRow`s (padded only at the sides) and for `Options` (rows 4 in); sand, sunk, for a note | |
| `SettingRow` / `SettingRowUnder` / `SettingRowOff` | A setting: name and its one-line note left, control right, 52 or 64 tall; `SettingRowUnder` adds what goes under it across the row (a move's progress); `SettingRowOff` keeps a setting that does nothing for now in its place, greyed: its name `Faint`, its control disabled (a segmented control's labels faint with none raised, a slider faded). A `Control` puts a control in the 220-wide column the controls of a card line up in, a `Picker`, a `Segmented` or a size slider filling it, a field from its start; switches, steppers and buttons keep to the row's end. Cards side by side take `ControlCardWidth` as their basis, so they wrap before a name is cut | |
| `PanelFrame` | The menu bar panel: night header, paper, footer; a flyout with the system's shadow, as tall as its content | the flyout `PopoverBase` lives in |
| `Dialog` / `Confirm` / `Popconfirm` | Dialogs, over the window dimmed by `Scrim`; the confirmations before a deletion that can't be undone: `Popconfirm` by its button, `Confirm` when more changes with it; in a dialog, a test's `Result`, with `ResultMark`s under it (an icon and a word, green when it is good: reads pictures) | `DialogBase`, `Popover` |

**Actions**

| Component | Use | Built on |
|---|---|---|
| `Button` | `Primary` (amber, at most one per screen: Start translating, Use instead, Use this scene), `Strong` (indigo, a state switched: Stop translating, New scene), `Plain` (raised: Download, Adjust position, Show in Finder), `Soft` (light amber: Learn from history), `Quiet` (ground only on hover: actions beside list rows), `Danger` (red: only on the button confirming a delete or a discard). `Off` disables one with kit's own face: the track's fill, a faint label | `ButtonBase` |
| `IconButton` | Round, three sizes 30 / 36 / 46: manage, all, reset | `ButtonBase` + `Tooltip` |
| `DeleteButton` / `CancelButton` | The one delete, a trash can, and the one cancel, a cross: Quiet until hovered, then red. Models, records, names, everything | `IconButton` |

**Choosing**

| Component | Use | Built on |
|---|---|---|
| `Picker` | A value on a track as a segmented control's, 36 tall, that opens a system menu, after a short label ("Listen to [Safari]", "Scene [General] [Japanese] → [Chinese]") | `MenuButton` |
| `PickerField` | The two big language buttons in the panel and the guide, with a swap button between | `MenuButton` |
| `Chip` | Scenes, filters; `Strong` when selected; may carry a `CountBadge` | `ButtonBase` |
| `Segmented` | Background, edges, alignment, On device / Cloud; every segment as wide as the widest, and, grown into a `Control`, sharing its width alike | `SegmentedBase` |
| `Switch` / `SwitchPill` | On and off; `SwitchPill` names it on a raised pill, a whole page's setting beside its title (Keep subtitles) | `SwitchBase` |
| `Stepper` | How many sentences show at once | `ButtonBase` ×2 |
| `Slider` | A subtitle size: MyGo's `ui.Slider`, amber with `Paper`'s theme, its value at the column's end | `ui.Slider` |
| `SchemeSwatch` / `CustomSwatch` / `Swatches` | A color scheme's disc, 28 across, seven filling the `Control` column, named in its tooltip: "Aa" on the edge color, "A" in the original's color and "a" in the translation's; the last, "Custom", a disc of all hues, which shows the original, translation and edge colors under the cards; `Swatches`: a color of a list, or any from the system's picker with the well after them | `ButtonBase`, `ColorWell` |
| `Options` | Radio rows in an `OptionsCard`, the settings' engines and the guide's alike: dot, name and tag, its description, the row's action at the end (Download, delete, cancel, Set up, Edit, Star), dot and action centred on the name and description; under them, across the row, what `Below` builds (`OptionProgress`, a note); the chosen row on the `Band`, a rule between two not chosen; `AddRow` ends a list with a plus where the dots are, "Add API" | `ButtonBase` (radio role) |
| `ListCard` / `MenuRow` | The panel's source and scene rows, which open system menus; a row's icon is `MenuIcon`, or `MenuLevels` for the source while listening | `MenuButton` |
| `Levels` | A few bars rising and falling as speech does while a voice is heard (the engine's voice detection), low and still-ish while not; still with Reduce Motion. On the guide's stage, and in place of the source's icon in the panel | `Draw` |

**Input**

| Component | Use | Built on |
|---|---|---|
| `TextField` / `TextBox` | Paper-style input and multi-line input (scene description, API key) | `TextInputBase`, `TextAreaBase` |
| `CellInput` | A table cell: looks like text, shows a ground on hover, an amber outline with focus | `TextInputBase` |
| `ShortcutField` | Recording a shortcut; shared by the start/stop and image shortcuts, which can't be the same | |

**Display**

| Component | Use | Built on |
|---|---|---|
| `StatStrip` | Live statistics: a white strip of a few numbers, with an optional note | |
| `Facts` | Small figures, each under what it counts, wrapping: what each engine did on the statistics page | |
| `Stage` | The subtitle stage: deep blue, stars, warm glow; subtitles drawn by the real renderer in `internal/ui/subtitle` | `Draw` |
| `Heatmap` | A value a day as squares, a week a column from Monday, deeper amber for more; month names over the weeks, Mon / Wed / Fri beside them, a Less–More legend; a square's date and value on hovering, outlined in ink; a click (or the arrows and Enter, the squares one stop of Tab) picks the day | |
| `BarChart` | The statistics page's chart, its bars of amber or stacked in `Series` colors: select a bar with the pointer or arrow keys, its value shows beside it at the top, a line for each part of a stacked bar. Charts name their unit once, on the highest tick | `ui/chart` |
| `ShareRow` | A row of a list of shares under a stacked chart, as a button: its color, name with a line under it (a model), a bar of its share and its value; clicked, it picks its part out of the chart, on the `Band` | `ButtonBase` |
| `PickTiles` | Choose one of a few figures, in named groups: a tile each with its value, the chosen one on the `Band` (the latencies, by stage) | `SegmentedBase` |
| `Histogram` | How one value spreads: bars as high as each bin's share is dense, lines at the median and 90th percentile named over the plot; select a bin for its share | `ui/chart` |
| `WhiskerChart` | A value by day: a dot at the median on a line up to the 90th percentile; select a day for both | `ui/chart` |
| `Tag` | On device / Cloud / Free | |
| `CountBadge` | A small rounded amber number (12 names) | |
| `InUse` / `InUseLabel` | The one mark of what is in use: a green disc with a check; `InUseLabel` says "In use" beside it, in a detail's title | |
| `ListTile` | Scene and record list items: icon or time, title, a line of detail; on the `Band` when selected | `ButtonBase` |
| `Sticker` | The mascot's sticker in the panel's header | |

**Feedback**: toasts (`ToastBase`), dialogs (`Modal`), download progress (`Progress`), work in
progress (`Waiting`: a small amber spinner and a line such as "Testing…" or "Translating…", using
MyGo's `plugins/loading`; still when the system asks for reduced motion), and empty states
(`Empty`: a drawing, a sentence, and a button only when the page has none for the next step).

**Empty states** draw a small likeness of the page itself (`Art`), on a hatched ground, 160×120:
placeholder bars for text, dashed outlines for what will come, Lucide icons in `Pencil`. The
lantern marks the next step once, at most: the page's own button for it (New Scene, the glossary's
+, Learn from History), or the play key of the user's own player when the step is outside Tomoru.
The drawings are SVG written from the palette, light and dark, in what MyGo draws: no filters or
patterns, so shadows are rectangles a little lower and the hatching is lines in a clip path.

| Where | When | `Art` | Action |
|---|---|---|---|
| Scenes, under General | only the general scene | `ArtScenes` | the header's New Scene |
| History | the switch off, no sessions | `ArtHistoryOff` | Keep Subtitles, in the state |
| History | the switch on, no sessions | `ArtHistory` | playing a video or audio |
| History | a scene's filter shows none, others have some | a caption only | |
| Statistics | nothing ever translated (whatever the switch) | `ArtStats` | playing a video or audio |
| A glossary | no names, the scene has sessions | `ArtTermsLearn` | the header's Learn from History |
| A glossary | no names, no sessions | `ArtTerms` | the header's + |

One drawing to a screen: while the scenes' list shows its own, the general scene's glossary says
its sentence only.

Gallery: `TOMORU_GALLERY=<dir> go test ./internal/ui/kit -run Gallery` writes an overview in light
and dark, buttons at rest, hovered and pressed, and the six empty states. The `settings` and `guide` packages have tests
of the same name that picture each page.

## Pages

The sidebar, top to bottom: Home, Subtitle style, Speech & Translation, Scenes, History,
Statistics; at the bottom, Preferences and About. The settings window opens at 1040×720, at least
860×600.

| Page | Contents |
|---|---|
| Home | Title (Translating / Not translating yet) and Start / Stop; one row of menus: what is heard (Listen to: all system sound or an app), then the scene (each with its languages, then New scene… and Manage scenes…) and its languages, spoken → translation, the two groups wrapping whole in a narrow window; live statistics (sentences, speech time, characters translated, and today's totals); lines by day as a `Heatmap`, as many weeks as fit up to a year, its total and a button to Statistics at the top right; clicking a day opens Statistics on the shortest period with it, its bar chosen (no subtitles here: they show over the screen); recent records |
| Subtitle style | Preview; two cards side by side, four rows each, their rules and control column lined up: look (background, edges, alignment, font) and text (the original's size, the translation's, which is on top, color scheme); with the translation only, the original's size and the order stay, greyed; with Custom, a card across the page under them: the original's, the translation's and, with an edge or a box, the edge's color; showing (sentences at once, hide when no new subtitles come, and after how many seconds, or keep them, hide from screen recordings); adjust position on screen (a plain button), reset |
| Speech & Translation | A group each for recognition and translation; On device / Cloud at the end of its title only switches which side you look at, not the engine. both sides are the same list (`Options`): choosing a row uses it, once a model is downloaded (which choosing starts; cancel and delete at the row's end, its progress under its description) or a service set up (which choosing opens); the cloud's rows end with Star, Edit or Set up, and translation's list with Add API; under the cloud's list, once, what the service chosen receives (the sound, or the recognized text); across the page under them, Picture Translation: a card of its engine, a `Picker` of the engines that read pictures (the cloud's and the local models downloaded, a separator, the user's APIs), and its shortcut, both in the `Control` column, and under the card, with an engine in the cloud, that it receives the pictures; then a card of two settings: translate ahead and, with local models, the model folder: its path, Show and Change…, which moves the models into a "Tomoru Models" folder in the folder chosen, a progress bar under the path in place of the buttons and the models' own actions held while they move |
| Scenes | The scene list on the left, details on the right: name, languages, description, glossary (an editable table of `CellInput`s; learned names starred), learn from history. See [scenes](scenes.md) |
| History | "Kept on this computer only", the keep switch, delete all; filter by scene; a list grouped by day; bilingual subtitles on the right, corrected names highlighted with "Recognized as '…', corrected by the glossary" beneath; learn names, show in Finder |
| Statistics | Laid out as Screen Time: range (last 7 days / 30 days / all); one card of recognition or translation (switched at its top right, translation first): the total (listening hours or lines) with a day's average and speech time or characters beside it (and the pictures translated, on the translation side), and how much more or less than the 7 or 30 days before (not for all, nor with nothing before), a bar per day (or month) stacked by engine, and the engines as `ShareRow`s, each with `Facts` of what it did and how soon (clicking one shows it alone in the chart, the others dimmed; clicking it again shows all); listening time in hours, minutes or seconds throughout; then the latencies: `PickTiles` of the five (what you wait; recognition's first words and settling; translation's first words and whole line), each with its median, to look into one; what it measures, and its median against the days before (faster in green, slower in red, about the same within 5% or 0.05 s); and its `Histogram`, or by day (or month) a `WhiskerChart`. Numbers only, never what was said |
| Preferences | Appearance (system / light / dark), UI language; start on open, open at login, shortcuts |
| About | Version, check for updates, licenses and credits, run the guide again, show logs |
| Menu bar panel | Header: the mascot's sticker, state, live statistics, Start / Stop; one card: the scene, its two languages and swap under it with no rule between (the scene keeps them), then, after a rule, the source; footer: a row of Quiet icon buttons, each named in its tooltip: Screenshot (translating a part of the screen, as its shortcut does; the panel steps out of the way first; its tooltip adds the shortcut), a rule, Move subtitles (their frame shows, to find and move them while no subtitles show; unlocks them first), the lock (open while unlocked; shut on the lantern's tint while locked, named for what a click does: Lock subtitles / Unlock subtitles), then at the far end Open Settings and Quit Tomoru (a power icon, red when hovered), apart from Stop so that quitting is never a slip. Show original and sentences at once are in Subtitle style, About in the menu bar icon's menu and the settings window. While listening, the source's icon is `Levels`, rising while a voice is heard: with no lines yet, it tells whether sound comes. The shortcuts are not spelled out, for want of room: hovering Start / Stop and Screenshot shows theirs |

**First-run guide**: 800×580, paper on the left, the night sky stage on the right. Three phases,
"Set up › Try it › Start", with the phase's name at the top and an amber progress bar beneath.
Eight steps: welcome and languages, permission, speech recognition, translation (each one choice, in the settings' rows: recognition only on this device, the system's, SenseVoice Small and Qwen3-ASR 1.7B, the cloud's being set up in Settings; translation "On device" then "Cloud" as the settings' sides, the 2B, the 9B, Recommended, and Index-Translate 35B, Recommended too and chosen to begin with; a model's row says its size, then for SenseVoice that it runs on the CPU, with no GPU needed, and for Qwen3-ASR and the translation models the memory they need, the GPU's on Windows (Qwen3-ASR's also that it runs on the GPU); under the cloud's, what it receives; the stage lights the engine the step chooses); try subtitles, the
start/stop shortcut, select-to-translate (can be skipped; once on, two checks one after the other: press the shortcut, then press it again and select the sample dialogue on the stage, which is translated for real, its translation under the check; until then the stage says to select it); ready.

- The permission page has one row per permission, checked once granted; on the right, "Your
  privacy": nothing is recorded, your screen isn't seen (only images you capture yourself are
  translated), history stays on this computer.
- Shortcuts are verified before they're used: the keys the user presses light up (`KeyLit`), and
  "That's it" shows on the left once they're right. While verifying, the guide holds the
  shortcuts (`core.Held`), so they don't start translating or selecting.
- Quit midway, or relaunched by the system, the guide opens again at the step it stopped on.

**Image translation card** (`ui/imagecard/`): a panel window floating above every app without
taking focus, under the selected area (above it when there's no room); from the clipboard, beside
the menu bar icon. Paper with round corners, as tall as its picture and four lines of translation
need until the user resizes it, and then of that size, which is kept; never resized as the
translation comes, which scrolls in it: a title row, the thumbnail (clicked, the whole picture
in a window of its own in the middle of the screen, as large as it was there, moved by the picture
and closed by its button or Escape), the translation (until it comes, three breathing placeholder lines),
"Powered by Index-Translate" at the bottom, its name a link, with what is kept in its tooltip, and
Copy. The title row has the language it translates into, a `Picker`; with no image, the card asks
for Screen Recording instead. See [image translation](image-translation.md).

**Overlay**: while hovered or being moved, the subtitle box and the toolbar above it sit on the
system's Liquid Glass (`overlay/glass_darwin.go`):

- AppKit's `NSGlassEffectView` (macOS 26), inserted in the overlay's window under what MyGo draws,
  so other apps behind show through. MyGo's glass plugin only shows what the same window draws.
- The clear style (regular gets more frosted the larger it is). The ground is light, only enough
  to read the text: 40% with a box, 27% immersive; the toolbar 12% darker.
- Clear glass is only clear in the active window. AppKit's private `_setForceMainAppearance:`
  keeps the overlay looking active without taking focus; without that method the system falls
  back to frosted. Check again after each major macOS update.
- Platforms without system glass (Windows, Linux) use a translucent black ground and a shadow.

The toolbar: pause or start; the scene with its languages, a menu of the scenes (each with its
languages), the spoken and translation languages as submenus, swap, New scene… (made, switched to, and shown on the Scenes page to be named) and Manage scenes… (the Scenes page, at the scene in use); lock,
and the panel. The scene comes before the languages: while watching, one switches what one
watches more than its languages. Text size is set in Subtitle style, not here.

## Wording

- Buttons say the action, titles the state: "Stop translating", "Translating", "Use Model Studio
  instead".
- At most one sentence of explanation, and none if it can go; say it with numbers when you can
  (12 names, 1.2 GB).
- No metaphors or brand words in place of a feature's name (not "put out the lantern", "light
  up", "hear and translate").
- The text of all five UI languages lives in `locales/`; add new text in all of them. Buttons must
  fit in all five (the panel's "Start" pairs with "Stop", not "Start translating", or the live
  numbers beside it are squeezed into an ellipsis).

## Deleting

Deleting what can be undone doesn't ask: it happens at once, and a toast offers Undo (scenes,
names, sessions, translation APIs). Deleting a local model can't be undone, so it asks first and
says how much will have to be downloaded again: `kit.Popconfirm` by its button, or `kit.Confirm`
when the model is in use, which also says what takes over. Both have the question, Cancel (focused
by default) and a red Delete.

A dialog with changes not saved asks before Escape or a click outside closes it: `kit.Guard` keeps
it open and `kit.Discard` asks in its buttons' place, Keep Editing (focused) or a red Discard.
Cancel closes it without asking. While Learn names works, Escape stops it, as its Cancel does.
