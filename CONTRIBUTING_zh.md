# 为 Tomoru 做贡献

[English](CONTRIBUTING.md)

谢谢你的帮助。欢迎报告问题、提出想法和提交 pull request。

## Issue

两种都从 设置 › 关于 › 反馈与建议 开始：选「问题」或「建议」，写下发生了什么或你希望怎样。点「复制给 AI Agent」，粘贴给 Claude Code、Codex 这样的 AI Agent，它会和你一起整理成 issue、查有没有相同的，你同意后再提交；点「在 GitHub 上填写」，会打开对应的表单，已经填好你写的内容。

- **问题**：写明平台和 Tomoru 版本、识别和翻译选的引擎、你做了什么、预期是什么、实际发生了什么（前三项 app 会填好）。附上日志会很有帮助：设置 › 关于 › 显示日志。附上之前先看一遍：里面可能有你收听的应用的名字。
- **想法**：多说说你在看什么、哪里不顺手，而不只是你想到的做法。不需要日志和复现步骤。
- **安全问题**：不要提交 issue，见 [安全策略](SECURITY_zh.md)。

## Pull request

1. 比小修复更大的改动，请先开 issue，商量好做法。
2. 先读 [架构](docs/architecture_zh.md) 和 [开发指南](docs/development_zh.md)：代码放在哪里由测试检查（`app/layers_test.go`、`app/design_test.go`）。
3. 改动保持聚焦，为你改变的行为补上测试。提交 pull request 前运行 `scripts/check.sh` 和 `scripts/test.sh`；CI 运行的是同样的检查和测试。
4. 界面改动：遵循 [设计系统](docs/design-system_zh.md)，在 `app/locales/` 的五个语言文件里都补上新文字，并附上截图（`TOMORU_GALLERY=<目录> go test ./internal/ui/<包> -run Gallery` 可以生成）。
5. 协议改动：同时更新 `protocol/fixtures.json`、所有实现和 [协议文档](docs/engine-protocol_zh.md)。
6. 影响识别、断句或翻译质量的改动：附上 [评测](bench/README_zh.md) 的数字。

代码注释和提交信息用英文，提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)（`fix(asr): …`）。文档成对存放：`name.md` 是英文，`name_zh.md` 是中文；两份都更新，或者在 pull request 里说明哪一份还需要更新。

提交贡献即表示你同意你的贡献和其余代码一样，以 GPL-3.0-or-later 授权。名称、图标和吉祥物不在其中：见 [商标说明](TRADEMARKS_zh.md)。
