# Tomoru benchmark

[中文](README_zh.md)

Tomoru's whole pipeline (recognition → segmenting → translate ahead → translation), measured on
real videos, to answer one question: **how good are the subtitles and translations, in as little
time as possible**. English, Japanese, Korean and Chinese, across the kinds of video people
watch. Each case is a stretch of a YouTube video (mostly 5 minutes), and the reference answers are
the video's own human subtitles and human translations.

## Running

```sh
cd app
go run ./cmd/tomoru-bench -asr sensevoice -mt cloud -max 120 -case zh-talk-yixi,en-talk-ted,ja-anime-pv,ko-variety-runningman   # the quick set, 2 minutes
go run ./cmd/tomoru-bench -asr apple -mt cloud -j 10              # all 29 cases with references, about 20 minutes
go run ./cmd/tomoru-bench -mt local-2b -mt-only                     # translation only: translate the human subtitles directly
cd .. && uv run bench/score.py apple_cloud_1017-1030 sensevoice_cloud_1017-1100   # score, compared side by side
```

- **The quick set** is for everyday checks (a refactor, a small change of behavior): one case a
  language, with references, their first 2 minutes. The whole benchmark is for when its numbers
  matter: a new model, a large change of segmenting or of performance. Compare a change with the
  same cases run before it, under another `-name`.
- The engine "plays" each case at normal speed with `listen --file`, without loudspeakers or the
  audio recording permission; the session is exactly the app's (segmenting and translate ahead
  on). So each case takes its full length, and `-j` sets how many run at once.
- `-asr`: `apple`, `sensevoice`, `qwen3-asr-1.7b`, `qwen-cloud[:model]` (reads
  `DASHSCOPE_API_KEY`, billed by duration), `openai[:model]` (live transcription; with another model than
  `gpt-live-transcribe`, the transcription API; reads `OPENAI_API_KEY`, and `OPENAI_BASE_URL` for a
  compatible service; billed by duration). `-mt`: `cloud`, `local-2b`, `local-9b`, or `chat` with `-mt-url` and `-mt-model` (the
  key from `MT_API_KEY`).
- Results go in `runs/<name>/` (not in the repository): one JSON per case, recording each change
  of the subtitles with its time; `score.py` writes `score.md`. Running again under the same name
  merges in the new cases, so runs can be split into batches.
- Too many cases at once with local engines compete for the CPU and worsen latency: keep
  SenseVoice within `-j 6`, and local translation models at `-j 1`.

## Metrics

Times are all measured from **when the word was spoken**, taken from the human subtitles: each
cue's time is spread evenly over its characters; cues usually stay up until the next begins, so
their end is tightened to the stretch's normal speaking rate. The subtitles' own timing is off by
about ±0.3 s, so latencies are mainly for comparing configurations.

| Metric | Meaning |
|---|---|
| Error rate % | Recognition error rate: by word for English, by character for Chinese, Japanese and Korean (English inside Chinese by word); (substitutions + deletions + insertions) ÷ reference length. Case, traditional/simplified characters and numbers (二十一 = 21) are normalized first, and punctuation and annotations such as [Music] or (laughs) removed |
| Missed % / Extra % | The missing and extra characters in the error rate |
| Noise chars/min | Extra characters where voice detection says no one spoke in the 4 s before: music or noise recognized as words. Speech the subtitles left out (lines without subtitles in a trailer, say) doesn't count |
| Shown p50 / p90 | From a word being spoken until it shows on screen **as it should be** (still-volatile gray text counts): the seconds within which half / 90% of words show |
| Final p50 | From a word being spoken until its sentence is final (no longer changes) |
| Translation seen p50 / p90 | From the end of a sentence until its translation can be seen: the gist from translate ahead at finalizing, or the full translation starting, whichever is earlier |
| Full translation p50 | From the end of a sentence until its full translation is out |
| chrF | Similarity of translations to the human ones (character n-gram F score, 0–100, no word segmentation for CJK). The 14 cases with both a reference transcript and human translations. Scores of different target languages can't be compared; compare only configurations on the same cases |
| Rewrite % | Characters that showed on screen and were later erased or changed, as a percentage of the final count: at each change of the screen, the two screens are aligned character by character, counting characters deleted or replaced (only adding text after doesn't count). Over 100 means each character was changed more than once on average |
| Long % / Fragment % | Sentence lengths: over two lines (about 45 Chinese characters or 90 letters), and under 3 characters (2 words in English) |
| CPU s/min | CPU seconds Tomoru and the processes it starts (engine, tomoru-asr, llama-server) use per minute of audio. Apple's recognition runs in a system service and isn't counted |
| s/sentence | Translation only: median time to translate a sentence |

Translation only (`-mt-only`) gives the human subtitles' text to the translation model sentence by
sentence; compared with the chrF of a normal run, it shows how much of the loss comes from
recognition and segmenting.

## Files

- `cases.tsv`: the cases. One line per stretch: language, scene, video ID, start and end, which
  subtitle track is the reference (`ref`), which human translations there are (`tgt`), notes.
- `fetch.py`: downloads the cases into `media/`. Only subtitles the uploader provided are used,
  never YouTube's automatic ones.
- `media/`: the downloads, not in the repository (the videos and subtitles belong to their
  owners).

```sh
python3 bench/fetch.py --list                                   # cases per scene and language
YTDLP="uvx --from yt-dlp yt-dlp" python3 bench/fetch.py --cookies-from-browser chrome
python3 bench/fetch.py --cookies-from-browser chrome zh-talk-yixi   # just this one
```

YouTube takes requests without a signed-in user for bots, so `--cookies-from-browser` borrows the
browser's session. When the system's yt-dlp is too old, `YTDLP` names the latest.

Each case gives:

| File | Contents |
|---|---|
| `<id>.wav` | The stretch's audio, 16 kHz mono, which `TOMORU_E2E` can play directly |
| `<id>.ref.srt` / `.ref.txt` | The human subtitles in its language, cut to the stretch, starting at 0 |
| `<id>.<lang>.srt` | Human translations (TED, Kurzgesagt, Apple keynotes and others have them), to score translation |

## Cases (2026-10-17)

46 stretches, about 3.5 hours: 29 have human subtitles as references, 15 have human translations,
and 14 have both. `python3 bench/fetch.py --list` shows the latest reference counts by language.

| Scene | en | ja | ko | zh |
|---|---|---|---|---|
| talk | ✓ | ✓ | ✓ | ✓✓ |
| science | ✓ | ✓ | ✓ | – |
| lecture | ✓ | todo | ✓ | ✓ |
| news | ✓ | todo | todo | todo |
| podcast | ✓ | ✓ | ✓ | ✓ |
| keynote | ✓ | ✓ | – | ✓ |
| street interviews | ✓ | todo | – | ✓ |
| variety | ✓ | todo | ✓ | ✓ |
| drama / anime | ✓ | ✓ | todo | todo |
| music | ✓ | ✓ | ✓ | todo |
| live streams | todo | todo | todo | todo |
| game commentary | todo | todo | todo | todo |

## Scenes

| Scene | What | What's hard |
|---|---|---|
| `talk` | TED, TEDx, 一席 and other talks | The baseline: one speaker, prepared |
| `science` | Science animation (Kurzgesagt) | Constant background music |
| `lecture` | Courses, lectures | Many terms, lots of English terms in Chinese lectures; classroom reverb |
| `news` | News | Fast speech |
| `podcast` | Podcasts, conversations | Two or three people talking over each other |
| `keynote` | Product launches | Product names, background music, hall reverb |
| `street` | Street interviews | Outdoor noise, unprepared speech |
| `variety` | Variety and comedy shows | Many people at once, laughter, sound effects |
| `drama` / `anime` | Trailers, anime PVs | Score and effects over the lines, emotional delivery |
| `music` | Music videos, live singing | Singing itself is hard to recognize; mainly whether the backing gets recognized as gibberish |
| `live` | Live streams, VTubers | Fast speech, reading chat aloud, several people on a call |
| `game` | Game commentary | Game sounds, talking to oneself |

## Cases without references (`ref` is `todo`)

Live streams, game commentary, Japanese, Korean and Chinese news, and Korean and Chinese dramas
have no usable human subtitles: none at all, or burned into the picture. These cases download
audio only for now. To make a reference: transcribe with the best recognizer (Qwen ASR in the
cloud, or Whisper large), check it by hand sentence by sentence, save it as `media/<id>.ref.srt`,
and change `ref` to `manual`. For songs, use the official lyrics; their timing can be loose.

## Caveats

- Human subtitles aren't always verbatim: some drop fillers, repeats and interjections, some
  rephrase for reading. Punctuation and numbers are normalized before scoring, and scores are for
  comparing engines; don't read too much into their absolute values.
- TED translations begin with a line crediting the translator, which isn't speech.
- In bilingual Chinese-English subtitles (Lei Jun's keynote, say), the English lines or cues are
  removed, leaving only the Chinese.
- The same Kurzgesagt video exists in English and Japanese (`en-science-kurz` and
  `ja-science-kurz`), to compare languages.
- When the subtitles end before the video while people still speak, `score.py` warns: everything
  after would count as extra, so move the case's end earlier.

## Baseline (2026-10-17, M5, 29 cases, 137 minutes)

| Configuration | Error % | Shown p50 / p90 | Final p50 | Translation seen p50 / p90 | Full translation p50 | chrF | Rewrite % | CPU s/min |
|---|---|---|---|---|---|---|---|---|
| Apple + cloud translation | 14.1 | 0.77 / 1.68 | 2.59 | 1.15 / 2.53 | 1.34 | 34.6 | 5 | 1.3* |
| SenseVoice + cloud translation | 15.4 | 1.13 / 2.90 | 2.55 | 1.41 / 3.15 | 1.58 | 33.7 | 2 | 26.5 |
| Qwen 3.1 cloud recognition + cloud translation | 13.6 | 1.27 / 2.74 | 3.27 | 2.10 / 3.84 | 2.29 | 35.7 | 10 | 1.2 |

\* Not counting the system's recognition service. Error rates by language (Apple / SenseVoice /
Qwen 3.1): en 12.9 / 16.9 / 11.1, ja 22.4 / 20.0 / 22.8, ko 11.4 / 14.6 / 13.0, zh 10.5 / 10.2 /
7.8. Between two runs of the same configuration, per-language error rates differ by about a point.

Qwen 3.1 first scored 16.9%, held back by two problems of Tomoru's own, which the benchmark found
and which were fixed (["Cloud recognition" in `docs/local-asr.md`](../docs/local-asr.md)):

- Hanging up after 10 s without a voice hurt music: on a Billie Eilish song, voice detection never
  found a voice from start to end; error rate 98.5% → 57% after the fix. Now it hangs up only with
  "no voice and no sound".
- The preview at the end of the service's volatile results added stray periods, and the segmenter
  committed on them, committing the same speech twice ("청중들**다우리실적**한테…우리실적이"): Korean
  20.9% → 13.0%.

SenseVoice used to rewrite 24%: decoding again every 0.4 s, words already shown flipped back and
forth. Now only what two consecutive decodes agree on shows: 2% rewritten, words shown about 0.5 s
later (0.64 s before), the same error rate ([`docs/local-asr.md`](../docs/local-asr.md)).

Apple's latency (tried 2026-10-17, none adopted): SpeechTranscriber gives out words in bursts about
every 1 s; changing the analysis priority, or feeding audio in 10 ms pieces as the system records
instead of 100 ms, changed nothing. "End of speech to translation seen" is almost all waiting for
it to put a period and finalize (1.12 s; translation itself takes 0.03 s). A/B tests on 9–11 cases
of 2.5 minutes each:

| Approach | Gain | Cost |
|---|---|---|
| Run DictationTranscriber too (words every 0.2–0.3 s), appending what it has beyond the gray text | Shown p50 −0.19 s (0.80 → 0.61) | Rewrites 4% → 38% (Japanese 55%: it writes kana where SpeechTranscriber writes kanji). Showing only what it agrees on twice in a row: still 35% rewritten, the gain down to −0.07 s |
| Make SpeechTranscriber finalize at every pause (as Chinese does now) | Translation seen −0.3 s, final −1.2 s | English error rate 12.9% → 31.3%: finalized early, the model garbles what follows (whole sentences turn into "..........", half sentences swallowed) |
| The same, Korean only (2026-10-20, 6 cases, old and new run side by side: `vad_ko_off` / `vad_ko_on`) | Translation seen p50 1.07 → 0.69 s, p90 3.22 → 2.38 s; final −1.2 s | Error rate 11.1% → 12.5%, a fast podcast 11.3% → 14.8%: words around the cuts garbled (「집으로 가신 겁니다」 → 「으로 가 겁니다」, 「60번째 주주총회」 → 「60 번째 14. / 총회.」); fragments 5% → 8% |
| Commit after 0.7 / 1.0 s of silence by voice detection, instead of waiting 1.3 s for text to stop | None | At 0.7 s, fragments 8% → 26%: the last few words arrive after the commit |

Cutting long sentences (changed 2026-10-19, since replaced; see below): long lines used to be
cut only at commas, with an English limit of 90 letters. Japanese and Korean have almost no commas
midway, so a line grew past a hundred characters; in English, a third of the lines in subtitles
users saved ran over two lines. Now the English limit is 60; past 1.2 times the limit (1.5 for
Japanese and Korean) with no comma, a line is cut at a joint: in English before a conjunction, in
Japanese after a conjunctive particle, in Korean after a connective ending. Chinese is unchanged,
cut by pauses.

| Language | Long % | Fragment % | chrF |
|---|---|---|---|
| English (old and new side by side, `seg_en`) | 9 → 1 | 11 → 9 | 27.3 → 26.6 |
| Japanese (against `apple_cloud`, `seg_ja`) | 7 → 2 | 5 → 6 | 38.2 → 37.5 |
| Korean (against `apple_cloud`, `seg_ko`) | 12 → 1 | 7 → 6 | 19.1 → 19.4 |

- "Translation seen" p90 gets longer (English 2.71 → 3.03 s), an artifact of the timing: it's per
  line, and when a first half is cut off, its start is its own last word, while more has been said
  since. In fact the first half's translation shows earlier than the whole sentence's used to.
- The thresholds and joints were chosen offline first, with the human subtitles' line breaks as
  the reference, simulating several thresholds. 1.2 times cut Japanese and Korean too finely; 2
  times hardly did anything.
- Letting an LLM (`typesafe/jev-1.14` on OpenRouter) choose the cuts matched the human breaks no
  better than the rule (precision 61.3 vs 62.4), and took 1.3–2.1 s each time, so it wasn't used.
- The joint rule (`internal/segment/joint.go`) was removed with the recognition redesign
  ([recognition](../docs/recognition.md), Sentences, step 5): a line wider than two lines is now
  cut at its last clause mark within them, else between the last two words that fit. Chinese or
  Japanese with no comma is not cut.

Translation only (14 cases, translating the human subtitles by sentence):

| Translation | chrF | s/sentence |
|---|---|---|
| Index-Translate in the cloud | 38.6 | 0.28 |
| Local 2B | 38.9 | 0.52 |
| Local 9B | 40.1 | 2.28** |

\** Two batches ran at once, two copies of the model competing, so slower than it should be.
