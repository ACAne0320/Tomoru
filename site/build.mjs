// Builds the website into public/: each page in src/ filled with the text of each language in
// locales/, and the sitemap. zh-Hans is the source and its pages sit at the root; the others sit
// under their own directory. A language with a null left is not translated yet: it is skipped.
// Wrangler runs this before `wrangler dev` and `wrangler deploy`.
//
// In the pages, {{key}} is a value from the language's file, which may hold HTML; {{key|plain}}
// is it as text, for attributes and titles; {{key|json}} is it as JSON, for scripts. {{$name}} is
// made here for each page and language. {{>name}} is the file src/partials/name, put in as it is.
import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const site = 'https://tomoru.acane.dev';

// The pages: the template, where it goes under a language's directory ("" for its home), and the
// texts it takes besides the site's. A text other than the site's is in locales/<name>/, under the
// key <name>; a page is built in the languages that have all its texts.
const pages = [
  { src: 'index.html', path: '' },
  { src: '404.html', path: '404.html', sitemap: false },
  { src: 'help/install.html', path: 'help/install/', texts: ['help'] },
  { src: 'help/offline.html', path: 'help/offline/', texts: ['help'] },
];

// The languages, the source first. dir is where its pages go (and is in .gitignore).
const languages = [
  { id: 'zh-Hans', dir: '', og: 'zh_CN', name: '简体中文' },
  { id: 'zh-Hant', dir: 'zh-hant', og: 'zh_TW', name: '繁體中文' },
  { id: 'en', dir: 'en', og: 'en_US', name: 'English' },
  { id: 'ja', dir: 'ja', og: 'ja_JP', name: '日本語' },
  { id: 'ko', dir: 'ko', og: 'ko_KR', name: '한국어' },
];

const tag = /\{\{\s*([$\w.]+)(?:\|(\w+))?\s*\}\}/g;
const partial = /\{\{>\s*([\w.-]+)\s*\}\}/g;
const templates = Object.fromEntries(pages.map((p) => [p.src, readFileSync(join(root, 'src', p.src), 'utf8')
  .replace(partial, (_, name) => readFileSync(join(root, 'src', 'partials', name), 'utf8').trim())]));
// read is a language's text of a kind, "" for the site's; null when the language has none.
const read = (lang, kind) => {
  const file = join(root, 'locales', kind, `${lang.id}.json`);
  return existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : null;
};
const kinds = ['', ...new Set(pages.flatMap((p) => p.texts ?? []))];
const errors = [];

// The keys the pages use, and their checks against the source's text: the site's and the others',
// each under its key.
const get = (o, k) => k.split('.').reduce((v, p) => (v == null ? undefined : v[p]), o);
const source = Object.fromEntries(kinds.map((kind) => [kind, read(languages[0], kind)]));
for (const kind of kinds) if (kind && kind in source['']) errors.push(`zh-Hans: ${kind} is the key of locales/${kind}/`);
const sourceAll = { ...source[''], ...Object.fromEntries(kinds.filter(Boolean).map((kind) => [kind, source[kind]])) };
const used = new Set();
for (const t of Object.values(templates)) for (const [, k] of t.matchAll(tag)) if (!k.startsWith('$')) used.add(k);
for (const k of used) if (get(sourceAll, k) === undefined) errors.push(`zh-Hans: no ${k}`);
// Every value is used: by a page, or as part of an object a page uses whole.
(function unused(o, path) {
  if (used.has(path)) return;
  if (o === null || typeof o !== 'object' || Array.isArray(o)) return errors.push(`zh-Hans: ${path} is used by no page`);
  for (const [k, v] of Object.entries(o)) unused(v, path ? `${path}.${k}` : k);
})(sourceAll, '');

// shape checks a language's text has the source's keys and kinds, and counts the values still
// null. A list's items each take the shape of the source's first.
function shape(ref, v, path, id) {
  if (v === null) return 1;
  const kind = (x) => (Array.isArray(x) ? 'list' : typeof x);
  if (kind(v) !== kind(ref)) return errors.push(`${id}: ${path} should be a ${kind(ref)}`), 0;
  if (Array.isArray(ref)) return v.reduce((n, item, i) => n + shape(ref[0], item, `${path}.${i}`, id), 0);
  if (typeof ref !== 'object') return 0;
  const at = (k) => (path ? `${path}.${k}` : k);
  for (const k of Object.keys(v)) if (!(k in ref)) errors.push(`${id}: ${at(k)} is not in zh-Hans`);
  let n = 0;
  for (const k of Object.keys(ref)) {
    if (k in v) n += shape(ref[k], v[k], at(k), id);
    else errors.push(`${id}: no ${at(k)}`);
  }
  return n;
}

// texts holds the text of each language the site is in: the site's, and the others' it has in
// full, under their key. A language whose site text has a null left is not on the site.
const texts = new Map();
for (const lang of languages) {
  let t = null;
  for (const kind of kinds) {
    const v = read(lang, kind);
    const at = kind ? `${kind}/${lang.id}` : lang.id;
    if (v === null) {
      if (!kind || lang === languages[0]) errors.push(`${at}: no file`);
      continue;
    }
    const missing = shape(source[kind], v, kind, lang.id);
    if (missing === 0) t = kind ? { ...t, [kind]: v } : v;
    else if (lang === languages[0]) errors.push(`${at}: ${missing} values are null`);
    else console.log(`${at}: skipped, ${missing} values not translated yet`);
    if (!t) break; // no site text, no pages
  }
  if (t) texts.set(lang, t);
}
if (errors.length) {
  console.error(errors.join('\n'));
  process.exit(1);
}

const built = [...texts.keys()];
// langsOf are the languages a page is built in, the source first.
const langsOf = (page) => built.filter((l) => (page.texts ?? []).every((kind) => kind in texts.get(l)));
const home = (lang) => (lang.dir ? `/${lang.dir}/` : '/');
const href = (lang, page) => home(lang) + page.path;
const escape = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const plain = (s) => s.replace(/<[^>]*>/g, '');

// alternates are a page's versions in each language, and English for the others when it has one.
function alternates(page) {
  const langs = langsOf(page);
  if (langs.length < 2) return [];
  const fallback = langs.find((l) => l.id === 'en') ?? langs[0];
  return [...langs.map((l) => [l.id, site + href(l, page)]), ['x-default', site + href(fallback, page)]];
}

function vars(lang, page) {
  const langs = langsOf(page);
  const links = alternates(page).map(([id, url]) => `<link rel="alternate" hreflang="${id}" href="${url}">`).join('\n  ');
  const menu = langs.map((l) => `<li><a href="${href(l, page)}" hreflang="${l.id}" lang="${l.id}"${l === lang ? ' aria-current="page"' : ''}>${l.name}</a></li>`).join('');
  return {
    lang: lang.id, og: lang.og, url: site + href(lang, page), home: home(lang), homeUrl: site + home(lang),
    alternates: links, name: lang.name, menu, single: langs.length < 2 ? ' hidden' : '',
  };
}

function render(template, t, v) {
  return template.replace(tag, (_, k, filter) => {
    const value = k.startsWith('$') ? v[k.slice(1)] : get(t, k);
    if (filter === 'json') return JSON.stringify(typeof value === 'string' ? plain(value) : value).replace(/</g, '\\u003c');
    if (typeof value !== 'string') throw new Error(`${k} is not text: use {{${k}|json}}`);
    return filter === 'plain' ? escape(plain(value)) : value;
  });
}

// Clear what an earlier build wrote: the other languages' directories and the source's pages in
// directories of their own.
for (const lang of languages) if (lang.dir) rmSync(join(root, 'public', lang.dir), { recursive: true, force: true });
for (const page of pages) if (page.path.includes('/')) rmSync(join(root, 'public', page.path.split('/')[0]), { recursive: true, force: true });
for (const page of pages) {
  for (const lang of langsOf(page)) {
    const file = join(root, 'public', lang.dir, page.path.endsWith('/') || !page.path ? `${page.path}index.html` : page.path);
    mkdirSync(dirname(file), { recursive: true });
    writeFileSync(file, render(templates[page.src], texts.get(lang), vars(lang, page)));
  }
}

const urls = pages.filter((p) => p.sitemap !== false).flatMap((page) => langsOf(page).map((lang) => {
  const links = alternates(page).map(([id, url]) => `\n    <xhtml:link rel="alternate" hreflang="${id}" href="${url}"/>`).join('');
  return `  <url>\n    <loc>${site + href(lang, page)}</loc>${links}\n  </url>`;
}));
writeFileSync(join(root, 'public', 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${urls.join('\n')}
</urlset>
`);
console.log(pages.map((p) => `${p.src}: ${langsOf(p).map((l) => l.id).join(', ')}`).join('\n'));
