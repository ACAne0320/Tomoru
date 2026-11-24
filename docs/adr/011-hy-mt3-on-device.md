# ADR 011: Hy-MT3 on the device

[中文](011-hy-mt3-on-device_zh.md)

Date: 2026-11-25. Status: accepted.

## Context

Tencent released Hy-MT3 on 2026-11-24 in 1.8B, 7B and 30B-A3B, each with official GGUF files on
Hugging Face and ModelScope (Apache-2.0). Tomoru offered Hy-MT2 1.8B and 7B on the device, and
Hy-MT2 on Tencent Cloud's TokenHub. The 1.8B and 7B files have Hy-MT2's architecture
(`hunyuan-dense`), tokenizer and chat template; the llama-server Tomoru ships (`b11960`) runs them.

## Decision

1. `models.All` (`app/internal/models/models.go`): Hy-MT3 1.8B and 7B replace Hy-MT2 on the
   device, so the family keeps one generation. Both are Q4_K_M, as Hy-MT2 was; Q8_0 was not
   measured for Hy-MT3, the full set against Hy-MT2 Q4_K_M decided. On the full set
   (translation only, an M5) the 1.8B scored 40.8 against Hy-MT2's 40.5, 0.32 s a sentence
   against 0.33; the 7B 41.8 against 40.4, 1.19 s against 1.29. So the 7B is now the more
   accurate size (`shared.SizeNote`).
2. 30B-A3B is not offered: its Q4_K_M is 19.5 GB, past the 16 GB the largest local model needs.
3. Tencent Cloud stays Hy-MT2 (Plus, Pro, Lite): no source we found says whether TokenHub serves
   Hy-MT3, or under what name.
4. `hymtPrompt` (`app/internal/translate/translate.go`) is kept: it is Hy-MT3's own Chinese
   personalization form, the task's last item its sentence. Decoding stays greedy (temperature 0)
   rather than the authors' sampling (0.7, top-p 0.6, top-k 20). The format is named Hy-MT, as it
   prompts both generations.
5. `models.Retired` and `Core.retireModels` (`app/internal/core/engines.go`): at start a setting
   that names Hy-MT2 on the device names the Hy-MT3 of its size, which is not downloaded for the
   user. Left as it was, the ID matched no engine and the subtitles went to the Index-Translate
   cloud unasked. Once the settings file reads back, with no Hy-MT2 in it, and while the models
   aren't moving, the Hy-MT2 files and their `.part` in the models folder are removed: nothing
   reads them any more. Settings that don't read back whole, as a file cut short or of a wrong
   type, are not rewritten at start and keep the files, for the user's choice is unknown; the
   setting is changed in memory only, so nothing goes to the cloud, and what start filled in
   (this change, the picture engine, the defaults) counts as saved, so quitting doesn't write it
   over the file; what the user changes is saved as before. Only regular files of those
   names go, not a folder. Their
   names stay in `Retired` for the statistics, which still show Hy-MT2 by name and color.

## Consequences

Users of Hy-MT2 on the device find Hy-MT3 chosen and not downloaded: their subtitles say so until
they download it, and nothing goes to the cloud. Their Hy-MT2 files are gone after the first start.
Hy-MT3 is a little larger (1.3 GB and 4.8 GB, from 1.1 GB and 4.3 GB). They take a little more memory with
llama-server's flags here (on an M5, Metal's: 1586 MiB for the 1.8B, 5540 for the 7B, from 1380
and 5043), within the 8 GB and 16 GB, 4 GB and 8 GB of video memory, they are said to need.

## Evidence

- `app/internal/core/retire_test.go`: both retired IDs, the settings unsaved, cut short,
  mistyped or missing, the models moving, quitting after a mistyped one, starting and quitting twice
  after one cut short or missing.
- `app/internal/models/models_test.go`: `TestRemoveRetired`.
- `app/internal/ui/shared/names_test.go`: `TestEngineName`.
- `bench/README.md`: runs `mtfull1125nostyle_hy-mt3-1.8b` and `mtfull1125nostyle_hy-mt3-7b` against
  `mtfull1107nostyle_hy-mt2-*`.
