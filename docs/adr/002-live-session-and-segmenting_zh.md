# ADR 002：实时会话与分句

[English](002-live-session-and-segmenting.md)

日期：2026-11-21。状态：采用。

## 背景

这些决策原先写在代码注释里，直到代码不再保留注释。它们涉及 Tomoru 认识的语言（`lang`）、识别文本如何成句（`segment`），以及实时会话如何生成草稿译文（`live`），只记录[识别](../recognition_zh.md)和[场景](../scenes_zh.md)尚未写明的部分。

## 决策

1. `lang.All`（`app/internal/lang/lang.go`）是 Qwen ASR 能识别的 30 种语言（涵盖系统识别和 SenseVoice 的语言），加上只用于书写的繁体中文（`zh-Hant`）。同一份列表既是翻译目标语言，也是识别所听的语言；`zh` 指普通话，以简体中文书写。
2. `clean`（`app/internal/segment/segment.go`）统一识别结果的空格：Apple 的 SpeechAnalyzer 在中文和日文里写出 " ，"，并在英文单词前加空格。
3. `alignEnd`（`segment.go`）用字母和数字上的编辑距离（忽略大小写）找出已提交文本在修订后假设中的位置，从不按字符串偏移：识别器会原地改写先前的文本，包括标点和字。
4. `firstSentenceEnd` 和 `periodEnds`（`app/internal/segment/sentence.go`）："。！？" 等总是结束句子；句点在以下情况不结束：小数点或词内（"3.5"、"e.g."）、缩写或首字母（"Mr."、"J."）、后接小写词（"etc. and"），与 ICU 带英文例外的断句一致。规则偏向不断句：漏掉句末只让一段推迟到下一个停顿，误断则拆开一句，翻译更差。
5. `maybeDraft`（`app/internal/live/live.go`）在以空格分词的语言（`lang.Spaced`）中去掉待定文本的最后一个词，除非其后已有空格：它可能只听到一半（"tips" 听成 "ti"），模型会去猜剩下的部分。
6. 已提交的行在译文 `Done` 之前显示其 `Line.Draft`（`app/internal/ui/subtitle/subtitle.go`），而不是正在流入的译文：流式译文开头比被替换的草稿短，行会缩短。

## 影响与取舍

英文等以空格分词的语言，草稿比说话者落后一个词。句点规则会漏掉一些真正的句末，这些句子要等停顿或两行切分。一行的译文在完成后一次出现，取代草稿。

## 证据

- `app/internal/segment/segment_test.go`：`TestClean`、`TestAlignEnd`、`TestFirstSentenceEnd`。
- `app/internal/ui/subtitle/subtitle_test.go`：`TestVisible`（流式译文期间显示草稿）。
