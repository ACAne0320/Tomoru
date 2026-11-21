# ADR 002: Live session and segmenting

[中文](002-live-session-and-segmenting_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
languages Tomoru knows (`lang`), how recognized text becomes sentences (`segment`) and how the live
session drafts them (`live`), where [recognition](../recognition.md) and [scenes](../scenes.md)
don't already say so.

## Decision

1. `lang.All` (`app/internal/lang/lang.go`) is the 30 languages Qwen ASR recognizes, which take in
   the system's and SenseVoice's, plus Traditional Chinese (`zh-Hant`), which is only written. One
   list serves as the languages to translate into and those recognition hears; `zh` is Mandarin,
   written in Simplified Chinese.
2. `clean` (`app/internal/segment/segment.go`) evens out the recognizer's spacing: Apple's
   SpeechAnalyzer writes " ，" in Chinese and Japanese and starts English words with a space.
3. `alignEnd` (`segment.go`) finds where committed text sits inside a revised hypothesis by edit
   distance over letters and digits, case ignored, never by string offsets: the recognizer rewrites
   earlier text, its punctuation and characters, in place.
4. `firstSentenceEnd` and `periodEnds` (`app/internal/segment/sentence.go`): "。！？" and the like
   always end a sentence; a period does not when it is a decimal point or inside a word ("3.5",
   "e.g."), ends an abbreviation or an initial ("Mr.", "J."), or is followed by a lowercase word
   ("etc. and"), as ICU's sentence breaks with English exceptions do. The rules lean toward no end:
   a missed end only delays a segment to the next pause, a false one splits a sentence, which
   translates worse.
5. `maybeDraft` (`app/internal/live/live.go`) leaves out the last word of the pending text in
   languages that space words (`lang.Spaced`), unless a space follows it: it may be half heard
   ("tips" as "ti"), and the model would guess the rest.
6. A committed line shows its `Line.Draft` until its translation is `Done`
   (`app/internal/ui/subtitle/subtitle.go`), not the translation streaming in: a stream starts
   shorter than the draft it replaces, and the line would shrink.

## Consequences

Drafts in English and other spaced languages trail the speaker by a word. Period rules miss some
real sentence ends, which then wait for a pause or the two-line cut. A line's translation appears
all at once when it is done, after the draft.

## Evidence

- `app/internal/segment/segment_test.go`: `TestClean`, `TestAlignEnd`, `TestFirstSentenceEnd`.
- `app/internal/ui/subtitle/subtitle_test.go`: `TestVisible` (a translation streaming in shows the
  draft).
