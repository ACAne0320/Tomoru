(() => {
  const t = JSON.parse(document.getElementById('strings').textContent);
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const mac = /Mac/i.test(navigator.platform) || /Macintosh/.test(navigator.userAgent);
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
  const glide = 'cubic-bezier(.65, 0, .35, 1)';
  const spring = 'cubic-bezier(.3, 1.25, .5, 1)';

  const stage = document.querySelector('[data-expand]');
  if (stage && !reduce) {
    let queued = false;
    const update = () => {
      queued = false;
      const from = innerHeight * 0.85, to = 72;
      stage.style.setProperty('--p', clamp((from - stage.getBoundingClientRect().top) / (from - to), 0, 1).toFixed(3));
    };
    addEventListener('scroll', () => { if (!queued) queued = requestAnimationFrame(update); }, { passive: true });
    addEventListener('resize', update);
    update();
  }

  function push(roll, entry, keep = 2) {
    const old = [...roll.children];
    const before = old.map((el) => el.getBoundingClientRect().top);
    roll.append(entry);
    const k = roll.getBoundingClientRect().height / roll.offsetHeight || 1;
    old.forEach((el, i) => {
      const dy = (before[i] - el.getBoundingClientRect().top) / k;
      el.classList.add('old');
      if (dy) el.animate([{ transform: `translateY(${dy}px)` }, { transform: 'none' }], { duration: 460, easing: spring });
    });
    entry.animate([{ opacity: 0, transform: 'translateY(8px)' }, { opacity: 1, transform: 'none' }], { duration: 220, delay: old.length ? 120 : 0, easing: 'ease-out', fill: 'backwards' });
    while (roll.children.length > keep + 1) roll.firstElementChild.remove();
    const extra = roll.children.length > keep ? roll.firstElementChild : null;
    if (extra) extra.animate([{ opacity: 0.54 }, { opacity: 0 }], { duration: 460, fill: 'forwards' }).finished.then(() => extra.remove());
  }
  const fadeIn = (el, ms = 200) => el.animate([{ opacity: 0 }, { opacity: 1 }], { duration: ms, easing: 'ease-out' });

  const vtuber = (() => {
    const tileWidth = 300, tileHeight = 360, tileStride = 308;
    const views = new Set();
    let gl, glc, video, tex, size, last = -1;
    function init() {
      glc = document.createElement('canvas');
      glc.width = tileWidth * 2;
      glc.height = tileHeight;
      gl = glc.getContext('webgl', { premultipliedAlpha: true, preserveDrawingBuffer: true });
      if (!gl) return false;
      const shader = (type, code) => {
        const sh = gl.createShader(type);
        gl.shaderSource(sh, code);
        gl.compileShader(sh);
        return sh;
      };
      const prog = gl.createProgram();
      gl.attachShader(prog, shader(gl.VERTEX_SHADER, 'attribute vec2 p; varying vec2 v; void main() { v = p; gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0); }'));
      gl.attachShader(prog, shader(gl.FRAGMENT_SHADER, `precision mediump float; varying vec2 v; uniform sampler2D t; uniform vec2 k;
        void main() {
          float tile = step(0.5, v.x), x = fract(v.x * 2.0) * ${tileWidth}.0, y = (1.0 - v.y) * ${tileHeight}.0;
          vec3 c = texture2D(t, vec2(tile * ${tileStride}.0 + x, y) * k).rgb;
          float a = clamp((texture2D(t, vec2(${tileStride * 2}.0 + x, y) * k).r - 0.02) / 0.96, 0.0, 1.0); // 压缩后遮罩的黑白不纯
          gl_FragColor = vec4(c * a, a);
        }`));
      gl.linkProgram(prog);
      if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return false;
      gl.useProgram(prog);
      gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
      gl.enableVertexAttribArray(0);
      gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
      tex = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, tex);
      for (const [p, val] of [[gl.TEXTURE_MIN_FILTER, gl.LINEAR], [gl.TEXTURE_MAG_FILTER, gl.LINEAR], [gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE], [gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE]]) gl.texParameteri(gl.TEXTURE_2D, p, val);
      size = gl.getUniformLocation(prog, 'k');
      video = document.createElement('video');
      video.muted = true;
      video.loop = true;
      video.playsInline = true;
      video.src = '/assets/vtuber.mp4';
      requestAnimationFrame(frame);
      return true;
    }
    const onScreen = (el) => {
      const r = el.getBoundingClientRect();
      return r.bottom > 0 && r.top < innerHeight && r.width > 0;
    };
    function frame() {
      requestAnimationFrame(frame);
      for (const v of views) if (!v.el.isConnected) views.delete(v);
      const shown = !document.hidden && [...views].some((v) => onScreen(v.el));
      if (!shown) { if (!video.paused) video.pause(); return; }
      if (video.paused) video.play().catch(() => {});
      if (video.readyState < 2) return;
      if (video.currentTime !== last) {
        last = video.currentTime;
        gl.viewport(0, 0, tileWidth * 2, tileHeight);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGB, gl.RGB, gl.UNSIGNED_BYTE, video);
        gl.uniform2f(size, 1 / video.videoWidth, 1 / video.videoHeight);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      }
      for (const v of views) {
        v.ctx.clearRect(0, 0, tileWidth, tileHeight);
        v.ctx.drawImage(glc, v.talking ? tileWidth : 0, 0, tileWidth, tileHeight, 0, 0, tileWidth, tileHeight);
        v.el.parentElement.classList.add('on');
      }
    }
    let ok;
    return (el) => {
      const view = { el, ctx: el?.getContext('2d'), talking: false };
      if (el && (ok ??= init())) views.add(view);
      return (on) => { view.talking = on; };
    };
  })();

  async function say(s, roll, line, talk = () => {}) {
    const entry = document.createElement('div');
    entry.className = 'line';
    const src = document.createElement('p');
    src.className = 'src';
    src.lang = t.rollLang;
    const tgt = document.createElement('p');
    tgt.className = 'tgt';
    tgt.innerHTML = '&nbsp;';
    entry.append(src, tgt);
    talk(true);
    for (let i = 0; i < line.src.length; i++) {
      const chunk = document.createElement('span');
      chunk.textContent = line.src[i];
      src.append(chunk);
      if (i === 0) push(roll, entry); else fadeIn(chunk);
      if (i === 1 && line.draft) {
        tgt.textContent = line.draft;
        tgt.classList.add('draft');
        fadeIn(tgt);
      }
      await s.wait(520);
    }
    talk(false);
    await s.wait(260);
    await swap(tgt, line.tgt);
  }
  const clock = (sec) => (sec >= 3600 ? `${Math.floor(sec / 3600)}:${String(Math.floor(sec / 60) % 60).padStart(2, '0')}` : Math.floor(sec / 60)) + `:${String(sec % 60).padStart(2, '0')}`;
  const wrapBetween = (el, head, tail) => {
    const a = document.createElement('span'), b = document.createElement('span');
    a.textContent = head;
    b.textContent = tail;
    el.replaceChildren(a, ' ', b);
  };
  const stats = (el, label, sec, n) => wrapBetween(el, `${label} ${clock(sec)} ·`, (n === 1 ? el.dataset.one : el.dataset.lines).replace('{n}', n));
  const fillStats = (root) => root.querySelectorAll('[data-stats][data-sec]').forEach((el) => stats(el, el.dataset.session, +el.dataset.sec, +el.dataset.n));
  fillStats(document);

  async function swap(el, text) {
    await el.animate([{ opacity: el.classList.contains('draft') ? 0.7 : 1 }, { opacity: 0 }], { duration: 120, fill: 'forwards' }).finished;
    el.textContent = text;
    el.classList.remove('draft');
    el.getAnimations().forEach((a) => a.cancel());
    fadeIn(el, 240);
  }

  function studio(el, script) {
    const W = +el.style.getPropertyValue('--w'), H = +el.style.getPropertyValue('--h');
    const cam = el.querySelector('.cam');
    const tpl = cam.innerHTML;
    let fit = 1, view = { x: 0, y: 0, z: 1 };
    const apply = (ms = 0) => {
      cam.style.transition = ms ? `transform ${ms}ms ${glide}, opacity .4s` : '';
      cam.style.transform = `scale(${fit}) translate(${view.x}px, ${view.y}px) scale(${view.z})`;
    };
    new ResizeObserver(() => { fit = el.clientWidth / W; apply(); }).observe(el);
    if (reduce) return;

    let seen = false;
    new IntersectionObserver(([e]) => { seen = e.isIntersecting; }, { threshold: 0.25 }).observe(el);
    const shown = () => seen && !document.hidden;
    const wait = async (ms) => {
      for (let left = ms; left > 0;) {
        if (shown()) { const d = Math.min(left, 100); await sleep(d); left -= d; } else await sleep(250);
      }
    };

    let cursor, cur;
    const q = (sel) => cam.querySelector(sel);
    const pos = (target, fx = 0.5, fy = 0.5) => {
      if (Array.isArray(target)) return target;
      const r = (typeof target === 'string' ? q(target) : target).getBoundingClientRect();
      const c = cam.getBoundingClientRect(), k = c.width / W;
      return [(r.left - c.left + r.width * fx) / k, (r.top - c.top + r.height * fy) / k];
    };
    const tip = { x: 6, y: 3 };
    const at = (x, y) => `translate(${x - tip.x}px, ${y - tip.y}px)`;
    let run = 0;
    const tween = (ms, frame) => new Promise((done) => {
      const id = ++run, t0 = performance.now();
      const step = (now) => {
        if (id !== run) return done();
        const k = Math.min(1, (now - t0) / ms);
        frame(k < 0.5 ? 4 * k * k * k : 1 - (-2 * k + 2) ** 3 / 2);
        if (k < 1) requestAnimationFrame(step); else done();
      };
      requestAnimationFrame(step);
    });
    const put = (x, y) => { cur = { x, y }; cursor.style.transform = at(x, y); };
    const s = {
      q, pos, wait,
      get small() { return el.clientWidth < 640; },
      jump(x, y) { run++; put(x, y); },
      move(target, { ms, fx, fy } = {}) {
        const [x, y] = pos(target, fx, fy), from = cur;
        ms ??= clamp(Math.hypot(x - from.x, y - from.y) * 1.3, 450, 1100);
        return tween(ms, (e) => put(from.x + (x - from.x) * e, from.y + (y - from.y) * e));
      },
      async click() {
        const r = document.createElement('div');
        r.className = 'ripple';
        r.style.left = `${cur.x}px`;
        r.style.top = `${cur.y}px`;
        cam.append(r);
        r.animate([{ transform: 'scale(.3)', opacity: 1 }, { transform: 'scale(1.5)', opacity: 0 }], { duration: 520, easing: 'ease-out' }).finished.then(() => r.remove());
        cursor.firstElementChild.animate([{ transform: 'scale(1)' }, { transform: 'scale(.82)' }, { transform: 'scale(1)' }], { duration: 240 });
        await wait(160);
      },
      drag(target, dx, dy, ms = 1000) {
        const ox = +(target.dataset.dx || 0), oy = +(target.dataset.dy || 0), from = cur;
        target.dataset.dx = ox + dx;
        target.dataset.dy = oy + dy;
        return tween(ms, (e) => {
          target.style.translate = `${ox + dx * e}px ${oy + dy * e}px`;
          put(from.x + dx * e, from.y + dy * e);
        });
      },
      hide(on = true) { cursor.classList.toggle('hide', on); },
      zoom(x, y, z, ms = 1000) {
        view = { z, x: clamp(W / 2 - x * z, W - W * z, 0), y: clamp(H / 2 - y * z, H - H * z, 0) };
        apply(ms);
        return wait(ms);
      },
      unzoom(ms = 900) {
        view = { x: 0, y: 0, z: 1 };
        apply(ms);
        return wait(ms);
      },
      async keys(list, ms = 1300) {
        const box = el.querySelector('.keycast');
        box.replaceChildren(...list.map((k) => Object.assign(document.createElement('span'), { textContent: k })));
        box.classList.add('show');
        box.animate([{ transform: 'translateX(-50%) scale(.92)' }, { transform: 'translateX(-50%)' }], { duration: 260, easing: spring });
        await wait(ms);
        box.classList.remove('show');
      },
    };

    (async () => {
      for (;;) {
        while (!shown()) await sleep(300);
        cam.innerHTML = tpl;
        fillStats(cam);
        cursor = q('.cursor');
        view = { x: 0, y: 0, z: 1 };
        apply();
        el.classList.add('live');
        el.classList.remove('fade');
        await script(s);
        el.classList.add('fade');
        await sleep(450);
      }
    })();
  }

  let heroLine = 0;
  const scripts = {
    async hero(s) {
      const roll = s.q('[data-roll]'), tray = s.q('[data-tray]'), panel = s.q('[data-panel]'), notice = s.q('[data-notice]');
      const title = s.q('[data-title]'), btn = s.q('[data-start]'), av = s.q('[data-av]'), chat = s.q('[data-chat]'), stat = s.q('[data-stats]');
      const talk = vtuber(s.q('[data-vt] canvas'));
      const today = { sec: 760, n: 186 }, run = { sec: 0, n: 0 };
      const count = () => (panel.classList.contains('on')
        ? stats(stat, stat.dataset.session, run.sec, run.n)
        : stats(stat, stat.dataset.today, today.sec, today.n));
      const idle = { title: title.textContent, btn: btn.querySelector('span').textContent };
      const running = (on) => {
        panel.classList.toggle('on', on);
        tray.classList.toggle('on', on);
        title.textContent = on ? title.dataset.on : idle.title;
        btn.querySelector('span').textContent = on ? btn.dataset.on : idle.btn;
        btn.querySelector('use').setAttribute('href', on ? '#i-pause' : '#i-play');
        av.src = on ? '/assets/sticker.webp' : '/assets/sticker-asleep.webp';
        if (!on) { today.sec += run.sec; today.n += run.n; }
        count();
      };
      roll.replaceChildren();
      running(false);
      let live = true;
      (async () => {
        while (live) {
          await s.wait(1000);
          if (live && panel.classList.contains('on')) { run.sec++; count(); }
        }
      })();
      (async () => {
        while (live) {
          await s.wait(1700);
          const first = chat.firstElementChild;
          chat.append(first);
          first.animate([{ opacity: 0, transform: 'translateY(12px)' }, { opacity: 1, transform: 'none' }], { duration: 300, easing: 'ease-out' });
        }
      })();

      const near = (x, y, z, ms) => (s.small ? s.zoom(x, y, z, ms) : null);
      near(1000, 190, 2, 0);
      s.jump(s.small ? 1000 : 820, s.small ? 300 : 300);
      await s.wait(400);
      await s.move(tray, { ms: 600 });
      await s.click();
      tray.classList.add('active');
      panel.classList.add('open');
      await s.wait(450);
      await s.move(btn, { ms: 450 });
      await s.click();
      running(true);
      await s.wait(350);
      panel.classList.remove('open');
      tray.classList.remove('active');
      s.move([1010, 600]).then(() => s.hide());
      notice.classList.add('show');
      s.zoom(s.small ? 420 : 490, 450, s.small ? 2.1 : 1.55, 1100);
      await s.wait(1100);
      notice.classList.remove('show');
      for (let i = 0; i < 6; i++) {
        if (i === 4 && !s.small) await s.unzoom();
        await say(s, roll, t.roll[heroLine++ % t.roll.length], talk);
        run.n++;
        count();
        await s.wait(1500);
      }
      s.hide(false);
      await near(1000, 190, 2, 900);
      await s.move(tray);
      await s.click();
      tray.classList.add('active');
      panel.classList.add('open');
      await s.wait(500);
      await s.move(btn);
      await s.click();
      running(false);
      roll.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 400, fill: 'forwards' });
      await s.wait(600);
      panel.classList.remove('open');
      tray.classList.remove('active');
      await s.wait(1000);
      live = false;
    },

    async overlay(s) {
      const block = s.q('[data-block]'), fight = s.q('[data-fight]'), lock = s.q('[data-lock]');
      block.style.translate = '0 -210px';
      block.dataset.dy = -210;
      s.jump(980, 470);
      await s.wait(900);
      await s.move(block, { fx: 0.42, fy: 0.55 });
      await s.wait(450);
      block.classList.add('hover');
      await s.wait(600);
      await s.drag(block, 0, 210, 1100);
      await s.wait(400);
      await s.move(lock);
      lock.classList.add('lit');
      await s.wait(200);
      await s.click();
      block.classList.remove('hover');
      lock.classList.remove('lit');
      await s.wait(500);
      await s.move(fight, { fx: 0.9 });
      await s.wait(250);
      await s.click();
      fight.classList.add('hit');
      await s.wait(1800);
    },

    async capture(s) {
      const row = s.q('[data-source]'), val = row.querySelector('[data-value]'), menu = s.q('[data-menu]');
      const all = s.q('[data-all]'), app = s.q('[data-app]'), check = all.querySelector('svg');
      const chosen = val.textContent;
      val.textContent = all.textContent.trim();
      s.jump(540, 420);
      await s.wait(1000);
      await s.move(row, { fx: 0.62 });
      row.classList.add('hl');
      await s.click();
      menu.classList.add('open');
      await s.wait(600);
      await s.move(app, { fx: 0.4 });
      app.classList.add('hl');
      await s.wait(300);
      await s.click();
      menu.classList.remove('open');
      row.classList.remove('hl');
      val.textContent = chosen;
      app.prepend(check);
      app.classList.remove('hl');
      await s.move([540, 420]);
      await s.wait(2800);
    },

    async engines(s) {
      const body = s.q('.eng-body'), local = s.q('[data-local]'), cloud = s.q('[data-cloud]');
      const pick = (onCloud) => {
        local.classList.toggle('on', !onCloud);
        cloud.classList.toggle('on', onCloud);
        body.classList.toggle('cloud-on', onCloud);
      };
      s.jump(470, 430);
      await s.wait(1400);
      await s.move(cloud);
      await s.click();
      pick(true);
      await s.move([470, 420]);
      await s.wait(2600);
      await s.move(local);
      await s.click();
      pick(false);
      await s.move([470, 430]);
      await s.wait(2200);
    },

    async draft(s) {
      const chunks = [...s.q('[data-src]').children], tgt = s.q('[data-tgt]'), note = s.q('[data-note]');
      const full = tgt.textContent, talk = vtuber(s.q('[data-vt] canvas'));
      chunks.forEach((c) => c.classList.add('wait'));
      tgt.innerHTML = '&nbsp;';
      await s.wait(600);
      await s.zoom(520, 470, 1.3, 1100);
      talk(true);
      chunks[0].classList.remove('wait');
      await s.wait(650);
      chunks[1].classList.remove('wait');
      tgt.textContent = tgt.dataset.draft;
      tgt.classList.add('draft');
      fadeIn(tgt);
      note.classList.add('show');
      await s.wait(1300);
      chunks[2].classList.remove('wait');
      await s.wait(600);
      talk(false);
      await s.wait(100);
      note.classList.add('final');
      await swap(tgt, full);
      await s.wait(2600);
      note.classList.remove('show');
      await s.unzoom();
      await s.wait(1000);
    },

    async scenes(s) {
      const roll = s.q('[data-roll]'), after = s.q('[data-after]'), before = s.q('template[data-before]').content.firstElementChild;
      const row = s.q('[data-scene]'), val = row.querySelector('[data-value]'), menu = s.q('[data-menu]');
      const general = s.q('[data-general]'), game = s.q('[data-game]'), gloss = s.q('[data-gloss]'), check = general.querySelector('svg');
      const name = val.textContent;
      val.textContent = general.textContent.trim();
      after.remove();
      gloss.classList.add('away');
      s.jump(560, 560);
      await s.wait(700);
      push(roll, before.cloneNode(true));
      await s.wait(2200);
      await s.move(row, { fx: 0.62 });
      row.classList.add('hl');
      await s.click();
      menu.classList.add('open');
      await s.wait(500);
      await s.move(game, { fx: 0.4 });
      game.classList.add('hl');
      await s.wait(250);
      await s.click();
      menu.classList.remove('open');
      row.classList.remove('hl');
      game.classList.remove('hl');
      game.prepend(check);
      val.textContent = name;
      gloss.classList.remove('away');
      await s.move([520, 600]);
      await s.wait(900);
      push(roll, after);
      await s.wait(3400);
    },

    async guide(s) {
      const play = s.q('[data-play]'), checks = [...s.q('.checks').children], next = s.q('[data-next]'), roll = s.q('[data-roll]');
      const label = next.textContent;
      checks.forEach((c) => c.classList.remove('done'));
      next.classList.remove('go');
      next.textContent = next.dataset.skip;
      roll.replaceChildren();
      s.jump(600, 540);
      await s.wait(900);
      await s.move(play);
      await s.click();
      play.classList.add('on');
      s.move([300, 560]).then(() => s.hide());
      await s.wait(700);
      checks[0].classList.add('done');
      const first = say(s, roll, t.roll[0]);
      await s.wait(500);
      checks[1].classList.add('done');
      await first;
      checks[2].classList.add('done');
      next.textContent = label;
      next.classList.add('go');
      await s.wait(1400);
      await say(s, roll, t.roll[1]);
      await s.wait(1600);
      s.hide(false);
      await s.move(next);
      await s.wait(1600);
    },

    async picture(s) {
      const box = s.q('[data-box]'), dim = s.q('[data-dim]'), sel = s.q('[data-sel]'), card = s.q('[data-card]'), text = s.q('[data-text]');
      const cursor = s.q('.cursor');
      const parts = [...text.childNodes];
      card.classList.add('away');
      text.replaceChildren();
      s.jump(640, 260);
      await s.wait(900);
      await s.keys(mac ? ['⌥', '⇧', 'S'] : ['Alt', 'Shift', 'S']);
      dim.classList.add('show');
      cursor.classList.add('crosshair');
      const [x0, y0] = s.pos(box, 0, 0), [x1, y1] = s.pos(box, 1, 1);
      await s.move([x0 - 10, y0 - 10]);
      Object.assign(sel.style, { left: `${x0 - 10}px`, top: `${y0 - 10}px` });
      sel.classList.add('show');
      sel.animate([{ width: '0px', height: '0px' }, { width: `${x1 - x0 + 20}px`, height: `${y1 - y0 + 20}px` }], { duration: 1000, easing: glide, fill: 'forwards' });
      await s.move([x1 + 10, y1 + 10], { ms: 1000 });
      await s.wait(300);
      dim.classList.remove('show');
      sel.classList.remove('show');
      cursor.classList.remove('crosshair');
      card.classList.remove('away');
      await s.wait(500);
      for (const node of parts) {
        if (node.nodeType !== Node.TEXT_NODE) { text.append(node); continue; }
        const run = document.createTextNode('');
        text.append(run);
        for (const ch of node.textContent) {
          run.textContent += ch;
          await s.wait(45);
        }
      }
      await s.move([700, 560]);
      await s.wait(2600);
    },
  };

  document.querySelectorAll('.studio[data-demo]').forEach((el) => studio(el, scripts[el.dataset.demo]));
})();
