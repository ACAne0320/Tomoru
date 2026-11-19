# Website

[中文](README_zh.md)

The site at <https://tomoru.acane.dev>, served as Cloudflare Workers static assets, with
`worker.js` for the downloads.

- `src/`: the pages, as templates, and `src/partials/`: the pieces they share (the header, the
  footer, the icons, the author's JSON-LD).
- `locales/`: their text, one file per language. `zh-Hans.json` is the source. `locales/help/`
  holds the help pages' text the same way.
- `public/`: styles, script, images and other static files. `build.mjs` writes the pages,
  `sitemap.xml` and `llms.txt` here; they are not committed.
- `build.mjs`: fills the pages for each language, with no dependencies. Wrangler runs it before
  `wrangler dev` and `wrangler deploy`.

Run `npx wrangler dev` here to see the site; it rebuilds as `src/` and `locales/` change.
Pushing to `main` deploys it (`.github/workflows/site.yml`).

## Pages

`{{key}}` in a page is a value from the language's file and may hold HTML. `{{key|plain}}` is
the value as text, for attributes and titles; `{{key|json}}` is it as JSON, for scripts.
`{{$name}}` is made by `build.mjs` for each page and language: `$lang`, `$url`, `$home`,
`$homeUrl`, the hreflang links, the language menu. `{{>name}}` puts in `src/partials/name`.
`main.js` reads its text from the page's `#strings` (the `js` group).

The home page's demos are recordings drawn in HTML: each `.studio` lays out its screen at the size
in `--w` and `--h`, and `studio.js` scales it to fit and plays its script (pointer, clicks, zoom,
keys) while it is on screen. The HTML holds the frame shown without the script or with reduced
motion; the mock app's words follow the app's catalogs in `app/locales/`.

The streamer in the live-stream demos is Tomoru rigged with [Rev2D](https://github.com/RevStudio/Rev2D)
and rendered to `assets/vtuber.mp4`: each frame holds three tiles, idle, talking and alpha, and
`studio.js` composites them with WebGL, showing the talking tile while a line is being said.

`/llms.txt` is generated from `src/llms.txt`: a short factual description of Tomoru for language
models, with the version filled in from `app/mygo.json`.

The pages are listed in `build.mjs`. The help pages, `/help/install/` and `/help/offline/`, also
take `locales/help/<language>.json`, under the key `help`. They are built only in the languages
that have that file in full, now zh-Hans and en, and their hreflang links, language menu and
sitemap entries name only those. What they say follows the app: check it against the code when
the app changes (models, sizes, settings names, permissions).

## Downloads

`/download/mac`, `/download/windows` and `/download/linux` give the latest installer for the system
(on Linux, the Debian package). Visitors in
mainland China, where GitHub is often slow, get it from the R2 bucket `tomoru-releases`; others
are sent to the GitHub release. `?from=github` or `?from=cloudflare` picks the source, for the
line under the download buttons. `main.js` points the buttons at the visitor's system; without
it they open the latest release page.

Every half hour the cron trigger copies the installers of the latest published release (not a
draft or a pre-release) to the bucket, checked against its `SHA256SUMS`, then switches
`latest.json` to it and keeps the version before, for downloads under way. Run the copy locally
with `npx wrangler dev` and `curl "localhost:8787/cdn-cgi/handler/scheduled"`.

## Languages

zh-Hans is at `/`; zh-Hant, en, ja and ko are at `/zh-hant/`, `/en/`, `/ja/`, `/ko/`. Every
language file has the same keys as `zh-Hans.json`. `null` is a value not translated yet: a
language with one left is not built, so it stays off the site and out of the language menu. The
build fails on a missing or unknown key, or a key no page uses.

A text change goes into every language that is on the site; a help text change, into every
language that has the help pages. A new language goes into the list in `build.mjs` and into
`.gitignore`, as does a page in a directory of its own.
