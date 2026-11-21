const repo = 'https://github.com/ACAne0320/Tomoru';

const installers = { mac: /^tomoru-.+-arm64\.dmg$/, windows: /^tomoru-setup-.+-amd64\.exe$/, linux: /^tomoru_.+_amd64\.deb$/ };

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const os = url.pathname.match(/^\/download\/(mac|windows|linux)\/?$/)?.[1];
    if (!os) return env.ASSETS.fetch(request);
    const latest = await readLatest(env);
    const name = latest?.files[os];
    if (!name) return Response.redirect(`${repo}/releases/latest`, 302);
    const github = `${repo}/releases/download/${latest.tag}/${name}`;
    const from = url.searchParams.get('from') ?? (request.cf?.country === 'CN' ? 'cloudflare' : 'github');
    if (from !== 'cloudflare') return Response.redirect(github, 302);
    return (await serve(request, env, name)) ?? Response.redirect(github, 302);
  },

  async scheduled(controller, env) {
    await sync(env);
  },
};

const readLatest = async (env) => (await env.RELEASES.get('latest.json'))?.json();

async function serve(request, env, name) {
  const head = await env.RELEASES.head(name);
  if (!head) return null;
  const size = head.size;
  const m = request.headers.get('range')?.match(/^bytes=(\d*)-(\d*)$/);
  let range;
  if (m && (m[1] || m[2])) {
    const offset = m[1] ? Number(m[1]) : Math.max(0, size - Number(m[2]));
    const end = m[1] && m[2] ? Math.min(Number(m[2]), size - 1) : size - 1;
    if (offset > end) return new Response(null, { status: 416, headers: { 'content-range': `bytes */${size}` } });
    range = { offset, length: end - offset + 1 };
  }
  const obj = request.method === 'HEAD' ? head : await env.RELEASES.get(name, { range });
  if (!obj) return null;
  const headers = new Headers();
  obj.writeHttpMetadata(headers);
  headers.set('etag', obj.httpEtag);
  headers.set('accept-ranges', 'bytes');
  headers.set('content-length', String(range ? range.length : size));
  if (range) headers.set('content-range', `bytes ${range.offset}-${range.offset + range.length - 1}/${size}`);
  return new Response(obj.body ?? null, { status: range ? 206 : 200, headers });
}

async function sync(env) {
  const res = await fetch(`${repo}/releases/latest`, { redirect: 'manual' });
  const tag = res.headers.get('location')?.split('/releases/tag/')[1];
  if (!tag) throw new Error(`no latest release: ${res.status}`);
  const old = await readLatest(env);
  if (old?.tag === tag) return;

  const sums = await fetch(`${repo}/releases/download/${tag}/SHA256SUMS`);
  if (!sums.ok) throw new Error(`${tag}/SHA256SUMS: ${sums.status}`);
  const found = {};
  for (const line of (await sums.text()).split('\n')) {
    const [sha256, name] = line.trim().split(/\s+/);
    for (const [os, re] of Object.entries(installers)) if (re.test(name ?? '')) found[os] = { name, sha256 };
  }
  for (const os of Object.keys(installers)) if (!found[os]) throw new Error(`${tag}: no installer for ${os}`);

  for (const { name, sha256 } of Object.values(found)) {
    if (await env.RELEASES.head(name)) continue;
    const file = await fetch(`${repo}/releases/download/${tag}/${name}`);
    if (!file.ok) throw new Error(`${tag}/${name}: ${file.status}`);
    await env.RELEASES.put(name, file.body, {
      sha256,
      httpMetadata: { contentType: 'application/octet-stream', contentDisposition: `attachment; filename="${name}"` },
    });
  }
  const files = Object.fromEntries(Object.entries(found).map(([os, f]) => [os, f.name]));
  await env.RELEASES.put('latest.json', JSON.stringify({ tag, files }), { httpMetadata: { contentType: 'application/json' } });

  const keep = new Set(['latest.json', ...Object.values(files), ...Object.values(old?.files ?? {})]);
  const listed = await env.RELEASES.list();
  const stale = listed.objects.map((o) => o.key).filter((k) => !keep.has(k));
  if (stale.length) await env.RELEASES.delete(stale);
  console.log(`synced ${tag}`, files);
}
