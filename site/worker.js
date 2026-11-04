// Tomoru 官网的 Worker。/download/mac 和 /download/windows 给访客最新的安装包：在中国大陆从 R2 下载，
// 别处跳到 GitHub；?from=github 或 ?from=cloudflare 指定来源。每半小时把 GitHub 上最新发布的安装包复制进 R2。
// 其余请求都是静态页面。
const repo = 'https://github.com/ACAne0320/Tomoru';

// 每个系统的安装包，按 SHA256SUMS 里的文件名认
const installers = { mac: /^tomoru-.+-arm64\.dmg$/, windows: /^tomoru-setup-.+-amd64\.exe$/ };

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const os = url.pathname.match(/^\/download\/(mac|windows)\/?$/)?.[1];
    if (!os) return env.ASSETS.fetch(request);
    const latest = await readLatest(env);
    if (!latest) return Response.redirect(`${repo}/releases/latest`, 302);
    const name = latest.files[os];
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

// serve 从 R2 发出 name，支持单段续传；R2 里没有就是 null
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

// sync 把 GitHub 上最新发布（不含草稿和预发布）的安装包复制进 R2，R2 按 SHA256SUMS 校验。
// 安装包都到了才换 latest.json，上一版的留着，让正在下载的人下完。
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
