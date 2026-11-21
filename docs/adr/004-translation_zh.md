# ADR 004：翻译

[English](004-translation.md)

日期：2026-11-21。状态：采用。

## 背景

这些决策原先写在代码注释里，直到代码不再保留注释。它们涉及提供哪些翻译引擎（`core/cloud.go`）以及 `translate` 如何请求它们，只记录[场景](../scenes_zh.md)和[图片翻译](../image-translation_zh.md)没有写到的部分。

## 决策

1. `CloudSizes`（`app/internal/core/cloud.go`）：Index-Translate 云端在默认的 35B-A3B 之外还提供 2B 和 9B，因为实测它们的译文更接近人工字幕，也更快（`bench/README.md`）。
2. `MTServices`（`cloud.go`）：TokenHub 的 Hy-MT2 默认用 Plus 7B，质量与 Pro 30B-A3B 相当，更快且很少被拒；Pro 每分钟只接受 60 个请求。也提供 Lite 1.8B。
3. `Index` 和 `HyMT`（`app/internal/translate/translate.go`）按作者建议用贪心解码（temperature 0）；通用聊天模型（`Chat`）保持 0.2。
4. `hymtPrompt`（`translate.go`）不像 Index-Translate 那样要求 Hy-MT2 译成口语字幕：要求之后，7B 在讲座里加了"呗"和"啦"，全量集上的 chrF 从 40.5 降到 38.1（1.8B），从 40.4 降到 36.1（7B）。
5. `Endpoint.request`（`translate.go`）只向应用自己的 llama-server（`Endpoint.Llama`）发送 llama.cpp 专有参数 `cache_prompt` 和 `chat_template_kwargs`：其他服务可能拒绝它们，运行在 vLLM 上的 Index 云端对 `cache_prompt` 返回 400（2026-11-10）。
6. `Endpoint.request` 同时用各厂商的写法要求其他聊天模型不思考：`thinking: {type: disabled}`（DeepSeek、GLM、豆包）、`reasoning_effort: none`（DeepSeek、OpenAI、Gemini）、`chat_template_kwargs.enable_thinking`（vLLM、SGLang）和 `enable_thinking`（Qwen 云端）。一行译文若要等思考，只会更慢，不会更好。
7. `Endpoint.body`（`translate.go`）把 `max_tokens` 设为 128 加上该行的字符（rune）数：足够容纳长句的译文，又随行长受限，让自我重复的模型尽早停下。
8. `unfinished`（`translate.go`）在被 token 上限截断的译文末尾加 "…"，让它不像已经译完；它的开头仍值得显示。
9. `Cloud`（`translate.go`）：Index-Translate 云端在 2 秒没有首字时再问一次（`FirstWait`），间隔 2 秒无新字或总计 8 秒即放弃，同时最多 3 个请求，并预热空闲 30 秒的连接（`WarmIdle`）。2026-10-26 实测：首字通常 0.5 秒到，4 个并发时最多 1.2 秒；偶尔（一半请求，持续数小时）6 秒后才到或根本不到，有一次 30 秒后才到；整行在 1.5 秒内完成；它会关闭空闲 40 到 60 秒的连接，新建连接要多花 0.4 到 0.5 秒。
10. `Endpoint.do`（`translate.go`）每个请求都发送 `User-Agent: Index-Translate-Client/1.0`：公共端点的防火墙要求如此。
11. `ErrEmpty`（`translate.go`）：没有文本的回复是错误，而不是空译文。Index-Translate 云端故障时曾这样回复（2026-10-20），空行看起来像是译完了。
12. `LimitedPause`（`translate.go`）：服务返回请求过多（429）后，草稿暂停一分钟（`ErrDraftPaused`），剩下的请求留给正式行。草稿可以没有，而腾讯云上的 Hy-MT2 Pro 每分钟只接受 60 个请求，语速快时光草稿就会超过。
13. `imageTimeout`（`app/internal/translate/image.go`）：一张图片的请求总共 2 分钟，除非 `Endpoint.Client` 自带超时：晚一分钟才开始，长译文还要再流半分钟。
14. `TestImageSlow`（`image_test.go`）和 `TestFirstWait`（`translate_test.go`）的处理函数先读完请求体：Go 的 HTTP 服务器只有在请求体读完后才察觉客户端断开，不读则测试会挂住。

## 影响与取舍

被限流的服务继续翻译正式行，但一分钟内不显示草稿。每个新的聊天 API 都会收到各厂商的思考开关；拒绝它们的服务只多一次重试，见[场景](../scenes_zh.md)。被截断的译文显示为未完成，而不是缺失。

## 证据

- `app/internal/translate/translate_test.go`：`TestNoThinking`、`TestTokenLimit`、`TestLimited`、`TestFirstWait`、`TestGap`、`TestWarm`、`TestCloud`、`TestNoTranslation`、`TestEndpoint`。
- `app/internal/translate/image_test.go`：`TestImageSlow`。
- `app/internal/core/engines_test.go`：`TestCloudSizes`。
