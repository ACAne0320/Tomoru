# 官网

[English](README.md)

<https://tomoru.acane.dev>，用 Cloudflare Workers 的静态资源托管。

- `src/`：页面模板。
- `locales/`：页面文字，每种语言一个文件，`zh-Hans.json` 是源。
- `public/`：样式、脚本、图片等静态文件。`build.mjs` 把页面和 `sitemap.xml` 写到这里，不进仓库。
- `build.mjs`：按语言生成页面，不依赖任何包。`wrangler dev` 和 `wrangler deploy` 之前，Wrangler 会先运行它。

在这里运行 `npx wrangler dev` 查看网站，`src/` 和 `locales/` 改了会自动重新生成。推到 `main` 就会部署（`.github/workflows/site.yml`）。

## 页面

页面里的 `{{key}}` 取自对应语言的文件，可以含 HTML；`{{key|plain}}` 是去掉标签的纯文本，用在属性和标题里；`{{key|json}}` 是 JSON，用在脚本里。`{{$name}}` 由 `build.mjs` 按语言生成：`$lang`、`$url`、`$home`、hreflang 链接和语言菜单。`main.js` 的文字从页面的 `#strings` 读（`js` 这一组）。

## 语言

简体中文在 `/`；繁体中文、英文、日文、韩文在 `/zh-hant/`、`/en/`、`/ja/`、`/ko/`。每种语言的文件和 `zh-Hans.json` 键完全相同。`null` 表示还没翻译：还有 `null` 的语言不会生成，也就不上线、不出现在语言菜单里。缺键、多出不认识的键、有键没有页面用到，都会让生成失败。

改文字时，已上线的每种语言都要一起改。加新语言要改 `build.mjs` 里的列表和 `.gitignore`。
