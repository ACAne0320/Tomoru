// Tomoru 官网的一点交互：下载按钮按系统改说明、复制安装命令、滚动进场、动画只在看得见时跑、首屏字幕滚动。
(() => {
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

  // 下载按钮下的小字：按访客的系统说明适用平台
  const ua = navigator.userAgent;
  const plat = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
  let label = '';
  if (/iPhone|iPad|Android/i.test(ua)) label = '';
  else if (/Mac/i.test(plat) || /Macintosh/.test(ua)) label = '适用于 macOS · Apple 芯片';
  else if (/Win/i.test(plat) || /Windows/.test(ua)) label = '适用于 Windows 11 · x64';
  if (label) document.querySelectorAll('[data-platform]').forEach((el) => { el.textContent = label; });

  // 安装命令的复制按钮：复制后两秒内显示「已复制」；不能写剪贴板时选中命令，好让人自己复制
  document.querySelectorAll('[data-copy]').forEach((btn) => {
    const code = btn.parentElement.querySelector('code');
    const text = btn.querySelector('span');
    let timer;
    btn.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(code.textContent);
      } catch {
        getSelection().selectAllChildren(code);
        return;
      }
      btn.classList.add('done');
      text.textContent = '已复制';
      clearTimeout(timer);
      timer = setTimeout(() => { btn.classList.remove('done'); text.textContent = '复制'; }, 2000);
    });
  });

  // 滚动进场：一次就好
  const reveal = document.querySelectorAll('.section-head, .pillar, .feature-copy, .feature .art, .step, .use, .card, .steps li, .shot, .mascot-grid, .faq');
  const seen = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (!e.isIntersecting) continue;
      e.target.classList.add('in-view');
      seen.unobserve(e.target);
    }
  }, { rootMargin: '0px 0px -8% 0px', threshold: 0.12 });
  reveal.forEach((el) => { el.classList.add('rv'); seen.observe(el); });

  // SVG 动画：离开视口就暂停
  const play = new IntersectionObserver((entries) => {
    for (const e of entries) e.target.classList.toggle('playing', e.isIntersecting);
  }, { threshold: 0.2 });
  document.querySelectorAll('[data-anim]').forEach((el) => play.observe(el));

  // 首屏：字幕像 app 里一样滚动——新的一句在最下面，旧的被推上去、变淡
  const roll = document.querySelector('[data-roll]');
  if (!roll || reduce) return;

  // 和 app/internal/samples 里的示例直播一样的句子。src 分块出现，draft 是说到一半时的大意。
  const lines = [
    { src: ['まずは', 'コメントを見てみましょう。'], tgt: '先看一下评论吧。' },
    { src: ['みんな、', 'ギフトありがとう！'], tgt: '谢谢大家的礼物！' },
    { src: ['よし、', 'じゃあ始めましょう。'], tgt: '好，那我们开始吧。' },
    { src: ['前回は第三章の', '最後で', '詰まっちゃいました。'], draft: '上次在第三章……', tgt: '上次卡在了第三章的最后。' },
    { src: ['今回は', 'もっといい装備を', '用意してきました。'], draft: '这次准备了……', tgt: '这次准备了更好的装备。' },
    { src: ['あ、', 'ボスが出てきた！'], tgt: '啊，Boss 出来了！' },
    { src: ['このボスの', '第二形態が', 'めっちゃ強いんだよね。'], draft: '这个 Boss 的第二阶段……', tgt: '这个 Boss 的第二阶段超强的。' },
    { src: ['皆さん、', 'こんばんは。'], tgt: '大家晚上好。' },
    { src: ['今日の配信は', 'いつもより', '少し長くなります。'], draft: '今天的直播……', tgt: '今天的直播会比平时长一点。' },
  ];
  const keep = 2;
  const spring = 'cubic-bezier(.3, 1.25, .5, 1)';
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  let heroVisible = true;
  new IntersectionObserver(([e]) => { heroVisible = e.isIntersecting; }).observe(roll);
  const ready = async () => { while (!heroVisible || document.hidden) await sleep(400); };

  const fadeIn = (el, ms = 200) => el.animate([{ opacity: 0 }, { opacity: 1 }], { duration: ms, easing: 'ease-out' });

  function push(entry) {
    const old = [...roll.children];
    const before = old.map((el) => el.getBoundingClientRect().top);
    roll.append(entry);
    old.forEach((el, i) => {
      const dy = before[i] - el.getBoundingClientRect().top;
      el.classList.add('old');
      if (dy) el.animate([{ transform: `translateY(${dy}px)` }, { transform: 'none' }], { duration: 460, easing: spring });
    });
    entry.animate([{ opacity: 0, transform: 'translateY(6px)' }, { opacity: 1, transform: 'none' }], { duration: 200, easing: 'ease-out' });
    // 推出视野的旧句子淡出后移走
    while (roll.children.length > keep + 1) roll.firstElementChild.remove();
    const extra = roll.children.length > keep ? roll.firstElementChild : null;
    if (extra) extra.animate([{ opacity: 0.55 }, { opacity: 0 }], { duration: 460, fill: 'forwards' }).finished.then(() => extra.remove());
  }

  async function say(line) {
    const entry = document.createElement('div');
    entry.className = 'line';
    const src = document.createElement('p');
    src.className = 'src';
    src.lang = 'ja';
    const tgt = document.createElement('p');
    tgt.className = 'tgt';
    tgt.innerHTML = '&nbsp;';
    entry.append(src, tgt);

    // 第一块一出来就上屏
    for (let i = 0; i < line.src.length; i++) {
      const chunk = document.createElement('span');
      chunk.textContent = line.src[i];
      src.append(chunk);
      if (i === 0) push(entry); else fadeIn(chunk);
      if (i === 1 && line.draft) {
        tgt.textContent = line.draft;
        tgt.classList.add('draft');
        fadeIn(tgt);
      }
      await sleep(520);
    }
    await sleep(260);
    // 说完了：换成完整译文（交叉淡入）
    await tgt.animate([{ opacity: tgt.classList.contains('draft') ? 0.7 : 1 }, { opacity: 0 }], { duration: 120, fill: 'forwards' }).finished;
    tgt.textContent = line.tgt;
    tgt.classList.remove('draft');
    tgt.getAnimations().forEach((a) => a.cancel());
    fadeIn(tgt, 240);
  }

  (async () => {
    await sleep(1200);
    for (let i = 0; ; i = (i + 1) % lines.length) {
      await ready();
      await say(lines[i]);
      await sleep(1700);
    }
  })();
})();
