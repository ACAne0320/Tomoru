// Builds the website into public/: each page in src/ filled with the text of each language in
// locales/, and the sitemap. zh-Hans is the source and its pages sit at the root; the others sit
// under their own directory. A language with a null left is not translated yet: it is skipped.
// Wrangler runs this before `wrangler dev` and `wrangler deploy`.
//
// In the pages, {{key}} is a value from the language's file, which may hold HTML; {{key|plain}}
// is it as text, for attributes and titles; {{key|json}} is it as JSON, for scripts. {{$name}} is
// made here for each language.
import { readFileSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const site = 'https://tomoru.acane.dev';
const pages = ['index.html', '404.html'];

// The languages, the source first. dir is where its pages go (and is in .gitignore).
const languages = [
  { id: 'zh-Hans', dir: '', og: 'zh_CN', name: '简体中文' },
  { id: 'zh-Hant', dir: 'zh-hant', og: 'zh_TW', name: '繁體中文' },
  { id: 'en', dir: 'en', og: 'en_US', name: 'English' },
  { id: 'ja', dir: 'ja', og: 'ja_JP', name: '日本語' },
  { id: 'ko', dir: 'ko', og: 'ko_KR', name: '한국어' },
];

const tag = /\{\{\s*([$\w.]+)(?:\|(\w+))?\s*\}\}/g;
const templates = Object.fromEntries(pages.map((p) => [p, readFileSync(join(root, 'src', p), 'utf8')]));
const text = (lang) => JSON.parse(readFileSync(join(root, 'locales', `${lang.id}.json`), 'utf8'));
const errors = [];

// The keys the pages use, and their checks against the source's text.
const used = new Set();
for (const t of Object.values(templates)) for (const [, k] of t.matchAll(tag)) if (!k.startsWith('$')) used.add(k);
const source = text(languages[0]);
const get = (o, k) => k.split('.').reduce((v, p) => (v == null ? undefined : v[p]), o);
for (const k of used) if (get(source, k) === undefined) errors.push(`zh-Hans: no ${k}`);
// Every value is used: by a page, or as part of an object a page uses whole.
(function unused(o, path) {
  if (used.has(path)) return;
  if (o === null || typeof o !== 'object' || Array.isArray(o)) return errors.push(`zh-Hans: ${path} is used by no page`);
  for (const [k, v] of Object.entries(o)) unused(v, path ? `${path}.${k}` : k);
})(source, '');

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

const texts = new Map();
for (const lang of languages) {
  const t = text(lang);
  const missing = shape(source, t, '', lang.id);
  if (missing === 0) texts.set(lang, t);
  else if (lang === languages[0]) errors.push(`zh-Hans: ${missing} values are null`);
  else console.log(`${lang.id}: skipped, ${missing} values not translated yet`);
}
if (errors.length) {
  console.error(errors.join('\n'));
  process.exit(1);
}

const built = [...texts.keys()];
const home = (lang) => (lang.dir ? `/${lang.dir}/` : '/');
const fallback = built.find((l) => l.id === 'en') ?? languages[0];
const escape = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const plain = (s) => s.replace(/<[^>]*>/g, '');

function vars(lang) {
  const alternates = built.length < 2 ? '' : [
    ...built.map((l) => `<link rel="alternate" hreflang="${l.id}" href="${site}${home(l)}">`),
    `<link rel="alternate" hreflang="x-default" href="${site}${home(fallback)}">`,
  ].join('\n  ');
  const menu = built.map((l) => `<li><a href="${home(l)}" hreflang="${l.id}" lang="${l.id}"${l === lang ? ' aria-current="page"' : ''}>${l.name}</a></li>`).join('');
  return { lang: lang.id, og: lang.og, url: site + home(lang), home: home(lang), alternates, name: lang.name, menu, single: built.length < 2 ? ' hidden' : '' };
}

function render(template, t, v) {
  return template.replace(tag, (_, k, filter) => {
    const value = k.startsWith('$') ? v[k.slice(1)] : get(t, k);
    if (filter === 'json') return JSON.stringify(typeof value === 'string' ? plain(value) : value).replace(/</g, '\\u003c');
    if (typeof value !== 'string') throw new Error(`${k} is not text: use {{${k}|json}}`);
    return filter === 'plain' ? escape(plain(value)) : value;
  });
}

for (const lang of languages) if (lang.dir) rmSync(join(root, 'public', lang.dir), { recursive: true, force: true });
for (const lang of built) {
  const t = texts.get(lang);
  const v = vars(lang);
  mkdirSync(join(root, 'public', lang.dir), { recursive: true });
  for (const p of pages) writeFileSync(join(root, 'public', lang.dir, p), render(templates[p], t, v));
}

const links = built.length < 2 ? '' : built.map((l) => `\n    <xhtml:link rel="alternate" hreflang="${l.id}" href="${site}${home(l)}"/>`).join('')
  + `\n    <xhtml:link rel="alternate" hreflang="x-default" href="${site}${home(fallback)}"/>`;
writeFileSync(join(root, 'public', 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${built.map((l) => `  <url>\n    <loc>${site}${home(l)}</loc>${links}\n  </url>`).join('\n')}
</urlset>
`);
console.log(`built ${built.map((l) => l.id).join(', ')}`);
