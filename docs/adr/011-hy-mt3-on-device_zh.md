# ADR 011：本机一个 Hy-MT 家族，Hy-MT3 排在已下载的 Hy-MT2 前面

[English](011-hy-mt3-on-device.md)

日期：2026-11-25。状态：已采纳。

## 背景

腾讯在 2026-11-24 发布了 Hy-MT3，有 1.8B、7B 和 30B-A3B，Hugging Face 和 ModelScope 上都有官方 GGUF（Apache-2.0）。Tomoru 本机提供 Hy-MT2 1.8B 和 7B，云端提供腾讯云 TokenHub 上的 Hy-MT2。1.8B 和 7B 的文件与 Hy-MT2 架构（`hunyuan-dense`）、分词器和对话模板相同，Tomoru 带的 llama-server（`b11960`）能运行。

## 决定

1. `models.All`（`app/internal/models/models.go`）：本机提供 Hy-MT3 1.8B 和 7B，排在 Hy-MT2 前面。两个都是 Q4_K_M，和 Hy-MT2 一样；Hy-MT3 没有测 Q8_0，由全量集对比 Hy-MT2 Q4_K_M 决定。全量集上（只测翻译，M5）1.8B 得 40.8，Hy-MT2 是 40.5，每句 0.32 s 对 0.33 s；7B 得 41.8 对 40.4，1.19 s 对 1.29 s。所以 Hy-MT3 的 7B 是更准的大小（`shared.SizeNote`）。
2. `Model.Legacy`：Hy-MT2 1.8B 和 7B 原样留在 `models.All` 里，文件、大小和校验值都不变，但不再下载（`Manager.Download`），只在文件已在本机时列出（`EngineService.catalog`，`app/internal/core/catalog.go`）。下载过 Hy-MT2 的用户保留它，照常可选可用，直到自己删除（确认框写明删除后不能再下载）；删除后它就从列表里消失。没下载过的用户只看到 Hy-MT3。不替用户删除或改动任何东西：设置里选的 Hy-MT2 仍是本地模型，文件在就在本机运行，文件不在就提示模型未下载。如果改成删掉 Hy-MT2 的 ID，这个 ID 会对不上任何引擎，字幕会在用户不知道的情况下发到 Index-Translate 云端。
3. 一个家族 Hy-MT，和 Index-Translate 一样：四项的 `Family` 都是 `Hy-MT`，ID 不变；每个尺寸的 `Label` 带上代号（`Hy-MT3 1.8B` … `Hy-MT2 7B`），这样设置页的尺寸、列表的状态行、首次引导和删除确认不用各自加代码就能分清两代。`Model.Name` 在规格名以家族名开头时只用规格名，所以不会出现「Hy-MT Hy-MT3 7B」。分成两个家族会把 Hy-MT2 显示成另一个模型，而不是同一个模型的旧版。为家族挑尺寸（`Core.Pick`，列表的单选钮）和推荐尺寸（`Core.Suggested`，首次引导）都先跳过 `Legacy` 的：Hy-MT3 在 Hy-MT2 之前，但正在用的仍保留，只下载了 Hy-MT2 且这台电脑跑得动时，单选钮就用它。列表状态行按磁盘上有什么来说已下载，不按这台机器能不能跑。Legacy 的尺寸不标「更快」「更准」（`shared.SizeNote`）：这两个词是给能下载的尺寸做选择用的。
4. 不提供 30B-A3B：它的 Q4_K_M 有 19.5 GB，超过了最大的本地模型需要的 16 GB。
5. 腾讯云仍是 Hy-MT2（Plus、Pro、Lite）：我们没找到 TokenHub 是否提供 Hy-MT3、叫什么的公开资料。
6. 保留 `hymtPrompt`（`app/internal/translate/translate.go`）：它就是 Hy-MT3 官方的中文个性化格式，任务的最后一条是官方的句子。解码仍是贪心（temperature 0），不用作者建议的采样（0.7、top-p 0.6、top-k 20）。格式改名为 Hy-MT，因为两代都用它。

## 后果

新用户在原来 Hy-MT2 的位置看到 Hy-MT，里面是 Hy-MT3 的两个尺寸。本机用 Hy-MT2 的用户在同一个 Hy-MT 里保留它，排在可以下载的 Hy-MT3 后面；Hy-MT2 删除后不能再下载，没下载完留下的 `.part` 仍在磁盘上，不再列出。Hy-MT3 稍大一些（1.3 GB 和 4.8 GB，原来 1.1 GB 和 4.3 GB）。统计页里 Hy-MT2 的颜色会变，现在按同一个家族取色。在这里的 llama-server 参数下占用的内存稍多（M5 上 Metal 内存：1.8B 1586 MiB，7B 5540，原来 1380 和 5043），仍在标明需要的 8 GB 和 16 GB 内存、4 GB 和 8 GB 显存之内。

## 证据

- `app/internal/core/legacy_test.go`：没有 Hy-MT2 时只列 Hy-MT3；已下载的 Hy-MT2 在同一个家族里列在 Hy-MT3 后面、可以选；`TestHyMT2NotSuggested` 和 `TestPickHyMT`：Hy-MT3 优先于 Hy-MT2，正在用的保留；设置选着它时在本机运行，启动时设置不变，文件不在时提示未下载、不发到云端；删除后该尺寸从列表消失。
- `app/internal/models/models_test.go`：`TestLegacyModels`，已下载的 Hy-MT2 能识别，不再下载。
- `app/internal/ui/settings/settings_test.go`：`TestHyMTFamily`、`TestDownloadedHyMT2`、`TestBothHyMTGenerations`、`TestHyMT2TooLarge`，家族的尺寸、状态行和单选钮，名字不重复；`TestDeleteHyMT2InUse`，在用和没在用时删除各尺寸及其确认框，详情停留或跟着切到云端；`TestHyMTNotForPictures`；`app/internal/ui/shared/names_test.go`：各模型的名字、`TestSizeNote`；`app/internal/ui/guide/guide_test.go`：`TestFamilyRows`、`TestHyMTRow`。
- `bench/README_zh.md`：运行 `mtfull1125nostyle_hy-mt3-1.8b` 和 `mtfull1125nostyle_hy-mt3-7b`，对比 `mtfull1107nostyle_hy-mt2-*`。
