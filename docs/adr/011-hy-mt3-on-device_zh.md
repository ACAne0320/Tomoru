# ADR 011：本机的 Hy-MT3

[English](011-hy-mt3-on-device.md)

日期：2026-11-25。状态：已采纳。

## 背景

腾讯在 2026-11-24 发布了 Hy-MT3，有 1.8B、7B 和 30B-A3B，Hugging Face 和 ModelScope 上都有官方 GGUF（Apache-2.0）。Tomoru 本机提供 Hy-MT2 1.8B 和 7B，云端提供腾讯云 TokenHub 上的 Hy-MT2。1.8B 和 7B 的文件与 Hy-MT2 架构（`hunyuan-dense`）、分词器和对话模板相同，Tomoru 带的 llama-server（`b11960`）能运行。

## 决定

1. `models.All`（`app/internal/models/models.go`）：本机用 Hy-MT3 1.8B 和 7B 替换 Hy-MT2，同一系列只留一代。两个都是 Q4_K_M，和 Hy-MT2 一样；Hy-MT3 没有测 Q8_0，由全量集对比 Hy-MT2 Q4_K_M 决定。全量集上（只测翻译，M5）1.8B 得 40.8，Hy-MT2 是 40.5，每句 0.32 s 对 0.33 s；7B 得 41.8 对 40.4，1.19 s 对 1.29 s。所以 7B 现在是更准的大小（`shared.SizeNote`）。
2. 不提供 30B-A3B：它的 Q4_K_M 有 19.5 GB，超过了最大的本地模型需要的 16 GB。
3. 腾讯云仍是 Hy-MT2（Plus、Pro、Lite）：我们没找到 TokenHub 是否提供 Hy-MT3、叫什么的公开资料。
4. 保留 `hymtPrompt`（`app/internal/translate/translate.go`）：它就是 Hy-MT3 官方的中文个性化格式，任务的最后一条是官方的句子。解码仍是贪心（temperature 0），不用作者建议的采样（0.7、top-p 0.6、top-k 20）。格式改名为 Hy-MT，因为两代都用它。
5. `models.Retired` 和 `Core.retireModels`（`app/internal/core/engines.go`）：启动时，设置里选的本机 Hy-MT2 换成同尺寸的 Hy-MT3，不替用户下载。如果不换，这个 ID 对不上任何引擎，字幕会在用户不知道的情况下发到 Index-Translate 云端。从磁盘读回的设置文件里已经没有 Hy-MT2、模型也没在移动时，删除模型文件夹里的 Hy-MT2 文件和它们的 `.part`：已经没有地方读它们。设置文件读不完整时（比如被截断、类型不对），启动时不改写它，也不删文件，因为不知道用户选了什么；这时只在内存里换成 Hy-MT3，保证不会发到云端，并同步到「已保存」的快照，退出时也不会因此写盘。只删这些名字的普通文件，同名的文件夹不删。它们的名字留在 `Retired` 里给统计页用，统计里仍按名字和颜色显示 Hy-MT2。

## 后果

本机用 Hy-MT2 的用户会看到选的是 Hy-MT3、还没下载：下载之前字幕会这样提示，不会发到云端。第一次启动后他们的 Hy-MT2 文件就删掉了。Hy-MT3 稍大一些（1.3 GB 和 4.8 GB，原来 1.1 GB 和 4.3 GB）。在这里的 llama-server 参数下占用的内存稍多（M5 上 Metal 内存：1.8B 1586 MiB，7B 5540，原来 1380 和 5043），仍在标明需要的 8 GB 和 16 GB 内存、4 GB 和 8 GB 显存之内。

## 证据

- `app/internal/core/retire_test.go`：两个退役 ID、设置没保存、被截断、类型不对或不存在、模型在移动、类型不对时退出。
- `app/internal/models/models_test.go`：`TestRemoveRetired`。
- `app/internal/ui/shared/names_test.go`：`TestEngineName`。
- `bench/README_zh.md`：运行 `mtfull1125nostyle_hy-mt3-1.8b` 和 `mtfull1125nostyle_hy-mt3-7b`，对比 `mtfull1107nostyle_hy-mt2-*`。
