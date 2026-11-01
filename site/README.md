# Website

[中文](README_zh.md)

The site at <https://tomoru.acane.dev>, served as Cloudflare Workers static assets.

- `src/`: the pages, as templates.
- `locales/`: their text, one file per language. `zh-Hans.json` is the source.
- `public/`: styles, script, images and other static files. `build.mjs` writes the pages and
  `sitemap.xml` here; they are not committed.
- `build.mjs`: fills the pages for each language, with no dependencies. Wrangler runs it before
  `wrangler dev` and `wrangler deploy`.

Run `npx wrangler dev` here to see the site; it rebuilds as `src/` and `locales/` change.
Pushing to `main` deploys it (`.github/workflows/site.yml`).

## Pages

`{{key}}` in a page is a value from the language's file and may hold HTML. `{{key|plain}}` is
the value as text, for attributes and titles; `{{key|json}}` is it as JSON, for scripts.
`{{$name}}` is made by `build.mjs` for each language: `$lang`, `$url`, `$home`, the hreflang links,
the language menu. `main.js` reads its text from the page's `#strings` (the `js` group).

## Languages

zh-Hans is at `/`; zh-Hant, en, ja and ko are at `/zh-hant/`, `/en/`, `/ja/`, `/ko/`. Every
language file has the same keys as `zh-Hans.json`. `null` is a value not translated yet: a
language with one left is not built, so it stays off the site and out of the language menu. The
build fails on a missing or unknown key, or a key no page uses.

A text change goes into every language that is on the site. A new language goes into the list in
`build.mjs` and into `.gitignore`.
