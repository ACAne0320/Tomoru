(() => {
  const t = JSON.parse(document.getElementById('strings').textContent);

  const ua = navigator.userAgent;
  const plat = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
  let label = '', os = '';
  if (/iPhone|iPad|Android/i.test(ua)) label = '';
  else if (/Mac/i.test(plat) || /Macintosh/.test(ua)) label = t.forMac, os = 'mac';
  else if (/Win/i.test(plat) || /Windows/.test(ua)) label = t.forWindows, os = 'windows';
  else if (/Linux/i.test(plat) || /Linux/.test(ua)) label = t.forLinux, os = 'linux';
  if (label) document.querySelectorAll('[data-platform]').forEach((el) => { el.textContent = label; });

  if (os) {
    document.querySelectorAll('[data-download]').forEach((a) => { a.href = `/download/${os}`; });
    document.querySelectorAll('[data-alt]').forEach((p) => {
      p.querySelectorAll('[data-from]').forEach((a) => { a.href = `/download/${os}?from=${a.dataset.from}`; });
      p.hidden = false;
    });
  }

  document.querySelectorAll('[data-copy]').forEach((btn) => {
    const code = btn.parentElement.querySelector('code');
    const text = btn.querySelector('span');
    const idle = text.textContent;
    let timer;
    btn.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(code.textContent);
      } catch {
        getSelection().selectAllChildren(code);
        return;
      }
      btn.classList.add('done');
      text.textContent = t.copied;
      clearTimeout(timer);
      timer = setTimeout(() => { btn.classList.remove('done'); text.textContent = idle; }, 2000);
    });
  });

  const menu = document.querySelector('.lang-menu');
  if (menu) {
    document.addEventListener('click', (e) => { if (!menu.contains(e.target)) menu.open = false; });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') menu.open = false; });
  }

  const reveal = document.querySelectorAll('.section-head, .tile, .feature-copy, .feature-stage, .use, .card, .steps li, .start-stage, .mascot-grid');
  const seen = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (!e.isIntersecting) continue;
      e.target.classList.add('in-view');
      seen.unobserve(e.target);
    }
  }, { rootMargin: '0px 0px -8% 0px', threshold: 0.12 });
  reveal.forEach((el) => { el.classList.add('rv'); seen.observe(el); });

  const play = new IntersectionObserver((entries) => {
    for (const e of entries) e.target.classList.toggle('playing', e.isIntersecting);
  }, { threshold: 0.2 });
  document.querySelectorAll('[data-anim]').forEach((el) => play.observe(el));
})();
