# ADR 011: Hy-MT3 on the device, beside a downloaded Hy-MT2

[中文](011-hy-mt3-on-device_zh.md)

Date: 2026-11-25. Status: accepted.

## Context

Tencent released Hy-MT3 on 2026-11-24 in 1.8B, 7B and 30B-A3B, each with official GGUF files on
Hugging Face and ModelScope (Apache-2.0). Tomoru offered Hy-MT2 1.8B and 7B on the device, and
Hy-MT2 on Tencent Cloud's TokenHub. The 1.8B and 7B files have Hy-MT2's architecture
(`hunyuan-dense`), tokenizer and chat template; the llama-server Tomoru ships (`b11960`) runs them.

## Decision

1. `models.All` (`app/internal/models/models.go`): Hy-MT3 1.8B and 7B are offered on the device,
   listed before Hy-MT2. Both are Q4_K_M, as Hy-MT2 is; Q8_0 was not measured for Hy-MT3, the
   full set against Hy-MT2 Q4_K_M decided. On the full set (translation only, an M5) the 1.8B
   scored 40.8 against Hy-MT2's 40.5, 0.32 s a sentence against 0.33; the 7B 41.8 against 40.4,
   1.19 s against 1.29. So Hy-MT3's 7B is its more accurate size (`shared.SizeNote`).
2. `Model.Legacy`: Hy-MT2 1.8B and 7B stay in `models.All` as they were, their files, sizes and
   checksums unchanged, but are no longer downloaded (`Manager.Download`) and are listed only
   once on disk (`EngineService.catalog`, `app/internal/core/catalog.go`). One who downloaded
   Hy-MT2 keeps it, to choose and use as before, until deleting it, which asks saying it can't be
   downloaded again; then it is gone from the list.
   One who never did sees only Hy-MT3. Nothing is removed or changed for the user: a setting that
   names Hy-MT2 still names a local model, so with its file there it runs on the device, and with
   it gone the subtitles say the model is not downloaded. Removing Hy-MT2's IDs instead would have
   matched no engine, and the subtitles would have gone to the Index-Translate cloud unasked.
3. 30B-A3B is not offered: its Q4_K_M is 19.5 GB, past the 16 GB the largest local model needs.
4. Tencent Cloud stays Hy-MT2 (Plus, Pro, Lite): no source we found says whether TokenHub serves
   Hy-MT3, or under what name.
5. `hymtPrompt` (`app/internal/translate/translate.go`) is kept: it is Hy-MT3's own Chinese
   personalization form, the task's last item its sentence. Decoding stays greedy (temperature 0)
   rather than the authors' sampling (0.7, top-p 0.6, top-k 20). The format is named Hy-MT, as it
   prompts both generations.

## Consequences

New users find Hy-MT3 where Hy-MT2 was. Users of Hy-MT2 on the device keep it, with Hy-MT3 above
it to download if they like; deleted, Hy-MT2 cannot be downloaded again, and a `.part` of it left
from an unfinished download stays on disk, unlisted. Hy-MT3 is a little larger (1.3 GB and 4.8 GB,
from 1.1 GB and 4.3 GB). It takes a little more memory with llama-server's flags here (on an M5,
Metal's: 1586 MiB for the 1.8B, 5540 for the 7B, from 1380 and 5043), within the 8 GB and 16 GB,
4 GB and 8 GB of video memory, it is said to need.

## Evidence

- `app/internal/core/legacy_test.go`: with no Hy-MT2 only Hy-MT3 is listed; a downloaded Hy-MT2
  is listed after Hy-MT3, and can be chosen; a setting that names it runs it on the device, the
  settings unchanged at start, and with its file gone says it is missing, not sending to the cloud;
  deleted, a size leaves the list.
- `app/internal/models/models_test.go`: `TestLegacyModels`, a downloaded Hy-MT2 found, none
  downloaded.
- `app/internal/ui/settings/settings_test.go`: `TestDownloadedHyMT2`, its place in the list, and
  deleting each size, in use and not, asked so;
  `app/internal/ui/guide/guide_test.go`: `TestFamilyRows`.
- `bench/README.md`: runs `mtfull1125nostyle_hy-mt3-1.8b` and `mtfull1125nostyle_hy-mt3-7b` against
  `mtfull1107nostyle_hy-mt2-*`.
