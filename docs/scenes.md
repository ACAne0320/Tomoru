# Translation scenes

[中文](scenes_zh.md)

## Goal

Recognition mishears names ("インフィア" should be ニンフィア, Sylveon; "メデオ" should be メテオ,
Meteor), and translation models translate names recognized right word by word ("クラウド", Cloud,
translated as "云", cloud). Users can make translation scenes, such as "Final Fantasy VII",
"Pokémon" or "The Apothecary Diaries". Each scene has its own description, glossary, languages and history. A sentence is
translated with only the glossary entries that concern it. After watching, the user can choose
"Learn names" on the history, have an LLM of their own go through it, confirm its suggestions into
the scene, and the next viewing uses them.

## Decisions

| Item | Decision |
|---|---|
| Unit | The scene. Descriptions, glossaries, languages and history are kept per scene |
| Default scene | "General", always first. Learning from its records asks where to add the names: General, an existing scene, or a new one |
| Scene description | Sent with each sentence to the translation model, and background for the LLM that scans; not to the on-device 2B model (it put the description's names in sentences that never mentioned them). See [the 2026-10-28 experiment](#experiment-2026-10-28-the-scene-description-in-the-translation-prompt) |
| Languages | A scene keeps the language heard and the target language: switching to a scene switches languages, and changing languages in a scene saves them to it |
| Learning | **Not while watching**: nothing pops up during viewing. Scanning happens only when the user chooses "Learn names" on the history; the results are listed as suggestions, all checked, added to the scene once confirmed, and can be undone. Unchecked ones are remembered and not suggested again |
| Scan chunks | Records are split every 120 seconds or 60 sentences, one request per chunk |
| The scanning LLM | An OpenAI-compatible API the user connected (the same list as custom translation APIs, chat-format ones only), chosen in the learning dialog; the last choice is remembered |
| Retranslating | After adopting, the records can be retranslated with the new glossary and written back to the history, with the current translation engine |
| History | Local only, off by default, turned on by the user. A settings page shows and manages it |

## Data

### Scenes

Stored in `scenes.json` in the user data directory, apart from the settings (`internal/scene`).

```go
type Scene struct {
	ID       string
	Name     string
	About    string // the description: sent with each sentence to the translator (but 2B), and to the model that learns names
	Src, Tgt string // the languages, "" until the scene is first used
	Terms    []Term
	Rejected []string // suggestions the user didn't take, as heard: not suggested again
}

type Term struct {
	Src   string   // the right spelling in the original: クラウド
	Tgt   string   // the translated name: 克劳德
	Heard []string // how recognition misheard it: メデオ (for メテオ)
	Auto  bool     // learned; false for the user's own
	Added time.Time
}
```

The related `Prefs` fields: `Scene` (the scene in use, by ID; `""` for General), `History` (keep
the subtitles, default false) and `Learn` (the API that learns names, as last chosen).

### History

Once turned on, each viewing is one file: `history/<scene ID or general>/<start time>.jsonl`
(`internal/history`). The first line holds the scene ID, the scene's name then, the languages, the
start time and the app heard (empty for all the system's sound); then one line per sentence: time,
the original as shown, the original as recognized (only when the glossary changed it), the
translation, and why it failed if it did. The glossary entries each sentence used aren't stored:
they can be worked out again from the glossary at any time.

When lines are saved:

- A sentence is saved once translated, strictly in order: later ones wait for the one before.
- On stopping, sentences not yet translated are saved as they are, with the draft if there is no
  translation.
- Switching scene or restarting (after changing languages, say) starts a new file.
- Turning saving on midway also saves the sentences still on screen.
- No sentences, no file.

Turning saving off doesn't delete existing files. Deleting first renames the file to `.deleted`,
so it can be undone; it's really deleted the first time the list is opened after the next launch.
The list and contents are read in the background.

## Using the glossary when translating

- **Only relevant entries**: an entry goes with a sentence only if the sentence contains its
  original spelling or one of its misheard spellings; at most `scene.MaxTerms` entries.
  - **Matching**: Latin letters match whole words, ignoring case, so `Eva` doesn't match
    `evaluate`; other scripts match after normalizing (NFKC, full and half width, case, spaces,
    "・"; Ⅶ and VII are the same).
- **Index format**: in `【约束要求】` (constraints), add `【硬性要求】专名/术语对照: メデオ→陨石、メテオ→陨石`
  (hard requirement: name/term mapping), Index-Translate's official instTrans form. An entry
  carries only the spellings that matched.
- **General chat models**: append to the system prompt `Render these names as given: A → B; C → D.`
- **The original shown**: confirmed misheard spellings are replaced with the right spelling on
  commit, so the original line shows "メテオ". Only literal replacement, no fuzzy matching. The same
  goes for uncommitted text and translate-ahead drafts.
- **The scene description**: the Index format adds `【注意】背景：<description>` after the glossary
  (a note, the soft constraint official instTrans uses for disambiguation); general chat models get
  `What is being watched: <description>.` in the system prompt. `translate.Request.About` and
  `live.Config.About`, provided by core and safe for concurrent use like the glossary, take an
  edited description or a new scene from the next sentence. The on-device 2B model gets none
  (`models.Model.NoAbout`).
- **Implementation**: `translate.Request.Terms`; `live.Config.Terms`, provided by core and safe for
  concurrent use, rewrites the original on commit and picks entries from the current glossary at
  translation time. New entries take effect from the next sentence without restarting the
  session. Switching scenes switches languages and glossary, restarting the session only if the
  languages changed.

## Learning names

- **Where**: the History page in Settings. Learn from one record, or from all the records shown
  by the filter (all of one scene's, say). Never triggered while watching.
- **Input**:
  - the scene's name and description;
  - the current glossary, and suggestions the user didn't take (marked "don't suggest again");
  - each chunk's sentences as "original + the translation at the time".
- **Prompt**: see "Appendix: the scanning prompt" below. The LLM looks only for names that really
  went wrong: misheard, or recognized right but translated wrong. It returns
  `heard / src / tgt / sure`.
- **Filtering** (`internal/learn`, from the experiment):
  - suggested only if `sure` is true and `heard` is found literally in that chunk's original;
  - dropped if it differs only in case, spaces or punctuation;
  - if `heard` starts with `src` (a name with a particle or ending attached, such as
    개미들→개미, 엔터주로→엔터주), the original isn't replaced: it only becomes a translation entry;
  - not listed if it has the same name as one of the user's own entries, which win;
  - the same name found in several chunks merges into one suggestion, its misheard spellings
    combined, with how often it was seen.
- **Dialog**: choose an API (if there is none, a prompt to add one, with the Add API dialog right
  there; it says the subtitle text will be sent to it) → progress ("chunk 3 of 8", cancellable) →
  suggestions (all checked; misheard → right spelling → translation, sentence count and an example)
  → which scene to add them to (or a new one) → adopt (undoable) → optionally "Retranslate with the
  new glossary".
- **Failures**: a chunk that fails (API error, timeout, quota used up, unparseable reply) is
  skipped; at the end it says how many chunks failed and the last reason, and suggestions already
  found are listed as usual.
- **Requests**: `translate.Endpoint.Chat`, a plain question and answer, with the same logic that
  turns thinking off and retries without parameters the server rejects; 2-minute timeout. No
  reliance on `response_format`: the JSON is cut out of the reply.
- **Retranslating**: only sentences the glossary matches are retranslated, the rest stay; the
  original is rewritten with the glossary and translated with the entries, with the current
  translation engine, and written back to the file.

## Later, maybe

- Learning while watching: the same scanning, with live subtitles every 120 seconds as input;
  waiting for evidence that long viewings make it worthwhile.
- Marking Apple recognition's confidence: in the experiment it was only a hint, and it would have
  to travel from the engine through the segmenter to each sentence, a large change with unproven
  gain.
- Importing and exporting glossaries.

## Tests

- `scene`: matching edge cases (kana, Latin word boundaries, normalizing), replacement, storage.
- `learn`: the filtering rules (format-only, particle attached, not found literally, same name as
  the user's own, previously rejected) and merging, with real replies from the experiment as test
  data.
- `translate`: the prompt carries the entries.
- `live`: a `Terms` that returns fixed entries; the original is replaced and the request carries
  the entries.
- Benchmark: `tomoru-bench -glossary <dir>`, with a `<case>.json` (`[]scene.Term`) per case, to
  compare with and without a glossary. With the glossaries learned in the experiment, on Lei Jun's
  keynote and a Nintendo Direct: "刘北 / 刘备 / 刘伟" in the original became 纽北 (the Nürburgring),
  "メデオ" became メテオ, "インフィア" became ニンフィア; the translation went from "Liu Bei" to
  "Nürburgring", and the title came out as 《最终幻想7 重生》 (Final Fantasy VII Rebirth). Time to see
  the translation, p50, 0.93 → 0.96 s, within noise.

## Experiment (2026-10-19)

- **Data**: the 6 benchmark cases richest in names, about 27 minutes, with Apple's on-device
  recognition.
- **Translation**: Index-Translate in the cloud.
- **Scanning**: tried qwen3.8-flash, deepseek-v4.1-flash and qwen3.8-max.
- **A starter glossary generated from a one-line description did more harm than good.** Only about
  10% of its words appeared in the videos. Entries like `iPhone→iPhone` only reword sentences, and
  forced names lose their title marks. So at first the scene description went only to scanning;
  [the 2026-10-28 experiment](#experiment-2026-10-28-the-scene-description-in-the-translation-prompt)
  sent it to the translator too.
- **Scanning "original + current translation" every 90 seconds works.**
  - Lei Jun's keynote: "刘备 / 刘北" → 纽北 (the Nürburgring), "舒七" → SU7, "Autra" → Ultra
  - Nintendo Direct: 《最终幻想7 重生》 (Final Fantasy VII Rebirth), 《塞尔达无双 灾厄启示录》 (Hyrule
    Warriors: Age of Calamity)
  - The COTEN podcast: 《我的奋斗》 (Mein Kampf)
- **Models differ a lot**:
  - qwen3.8-flash "corrected" 7 things wrongly, such as "刘北" → "徕卡" (Leica)
  - deepseek-v4.1-flash made no wrong corrections, but missed many
  - qwen3.8-max was best
- **Biasing recognition isn't possible**: Apple's SpeechTranscriber ignores
  `contextualStrings`, Qwen's realtime recognition has no hotword field, SenseVoice has no
  hotwords. (Since 2026-10-31 the recognizers that take hints are told the scene's names:
  OpenAI's, Qwen cloud's `vocabulary` (its 3.1 model has one now) and Qwen3-ASR; see
  [recognition](recognition.md), "Hints".)
- **Apple's confidence**: most misheard words score clearly low ("基金" 0.26, "占女" 0.31), but
  the wrong "インフィア" scored 0.999. So it can only be a hint, not a gate.
- **Usage**: about 60,000 tokens per hour watched.

## Experiment (2026-10-28): the scene description in the translation prompt

The 2026-10-19 experiment only tried a starter glossary generated from the description; in the
translation prompt, the description was only seen to make names up. This one compares sentences
with and without it.

- **Data**: the 7 benchmark cases richest in names (the Apothecary Diaries PV, Nintendo Direct,
  Apple's event, Lei Jun's keynote, Running Man, Lex Fridman, COTEN), about 31 minutes. Each
  description is one sentence as a user would write it, such as "药屋少女的呢喃 第3季 PV，角色有猫猫、壬氏"
  (bench `-about <dir>`).
- **Translating the human subtitles** (`-mt-only`; chrF only on the 3 cases with human
  translations):

| Translation | Without | With | Seen |
|---|---|---|---|
| Index cloud 35B | 24.0 | 24.9 | the misheard "刘北" became the Nürburgring, the Apothecary Diaries title came out right, COTEN's "お父さん" no longer "my dad"; a few Pokémon names in the Nintendo Direct came out more made-up |
| On-device 9B | 25.4 | 26.2 | 舒淇 → SU7, 刘北 → Nürburgring, the title right, NVIDIA → 英伟达; no name made up seen |
| On-device 2B | 23.3 | 24.0 | makes names up: "いなご" (locusts) became 壬氏 (Jinshi), and 刘在石 (Yoo Jae-suk) was put at the head of a Korean line that never named him |

- **Apple's recognition + the cloud**: chrF 21.8 → 23.1, translation seen p50 2.23 → 2.02 s
  (noise); the Nürburgring 0 → 3 times in Lei Jun's keynote; in Running Man a line recognized as
  gibberish was translated as "that's Jae-suk and Suk-jin's voices", perhaps rightly.
- **So**: the cloud, 9B and the user's own APIs get the description; 2B doesn't.

## Appendix: the scanning prompt

The system prompt `internal/learn` uses (the experiment's second round). `{src}` and `{tgt}` are
language names. The user message holds, in order: the scene's name and description, the current
glossary, and each sentence as "原文：… / 译文：…" (original / translation). The prompt is in
Chinese, as sent:

```
你帮实时字幕纠正专名。下面每行是语音识别出的{src}原文，和它当时的{tgt}译文。
只找出确实出了问题的专名（人名、角色名、作品名、地名、组织、产品名、专有术语）：
- 原文被识别成了同音、近音的错字：heard 写原文里的原样（只写专名本身，不带助词、词尾），src 写正确写法；
- 原文对了但译文把它译错了（例如把角色名按字面意思翻译）：heard 和 src 都写原样。
tgt 写通行的{tgt}译名（官方译名优先）。已经译对的不要写，普通词不要写，只差大小写、空格、标点的不要写。没把握就不要写，sure 表示你是否确信。
只输出 JSON：{"terms":[{"heard":"...","src":"...","tgt":"...","sure":true}]}
```

In English: "You help live subtitles correct names. Each line below is the {src} original from
speech recognition, and its {tgt} translation at the time. Find only names (people, characters,
works, places, organizations, products, proper terms) that really went wrong: if the original was
recognized as a homophone or near-homophone, write it as it appears in `heard` (the name alone,
without particles or endings) and the right spelling in `src`; if the original is right but the
translation got it wrong (a character's name translated literally, say), write it as it is in both
`heard` and `src`. Write the usual {tgt} name in `tgt` (official ones first). Leave out names
already translated right, ordinary words, and differences of only case, spaces or punctuation. If
unsure, leave it out; `sure` says whether you are sure. Output only JSON: …"
