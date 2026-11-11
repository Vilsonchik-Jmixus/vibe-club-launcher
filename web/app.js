'use strict';
const API = 'api/';
const $ = (id) => document.getElementById(id);

const ICONS = {
  skin: '<path d="M8 3h12v10H8ZM4 15h20v10H4ZM9 15v10m10-10v10"/>',
  settings: '<path d="m11 3-1 4-4 1-3 5 3 3-1 4 5 3 3-2 4 2 5-3-1-4 3-3-3-5-4-1-1-4Z"/><circle cx="13.5" cy="13.5" r="4"/>',
  folder: '<path d="M3 8h9l3 3h10v13H3Z"/><path d="M3 8V5h9l3 3h10v3"/>',
  close: '<path d="m7 7 14 14M21 7 7 21"/>',
  play: '<path d="m9 5 13 9-13 9Z"/>',
  update: '<path d="M23 11a10 10 0 0 0-17-5L3 9m0-6v6h6M5 17a10 10 0 0 0 17 5l3-3m0 6v-6h-6"/>',
  check: '<path d="m5 14 6 6L24 7"/>',
  left: '<path d="m17 5-9 9 9 9"/>',
  right: '<path d="m11 5 9 9-9 9"/>',
};
document.querySelectorAll('[data-icon]').forEach((el) => {
  el.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28" fill="none" stroke="#F0A040" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">' + ICONS[el.dataset.icon] + '</svg>';
});

// ---- scale the 1280x720 design to the window
function fit() {
  const s = Math.min(innerWidth / 1280, innerHeight / 720);
  const st = $('stage');
  st.style.transform = `scale(${s})`;
  st.style.left = Math.max(0, (innerWidth - 1280 * s) / 2) + 'px';
  st.style.top = Math.max(0, (innerHeight - 720 * s) / 2) + 'px';
}
addEventListener('resize', fit); fit();

// ---- helpers
async function api(name, opts = {}) {
  const init = { method: opts.method || 'GET', headers: {} };
  if (init.method !== 'GET') init.headers['X-VC'] = '1';
  if (opts.json !== undefined) { init.body = JSON.stringify(opts.json); init.headers['Content-Type'] = 'application/json'; }
  if (opts.body !== undefined) init.body = opts.body;
  const r = await fetch(API + name, init);
  const ct = r.headers.get('content-type') || '';
  const data = ct.includes('json') ? await r.json() : null;
  if (!r.ok) throw new Error((data && data.error) || ('Ошибка ' + r.status));
  return data;
}
function fmtBytes(n) {
  if (!n || n < 0) return '0 МБ';
  if (n >= 1 << 30) return (n / (1 << 30)).toFixed(1).replace('.', ',') + ' ГБ';
  if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1).replace('.', ',') + ' МБ';
  return Math.max(1, Math.round(n / 1024)) + ' КБ';
}
function plural(n, a, b, c) { n = Math.abs(n) % 100; const d = n % 10; if (n > 10 && n < 20) return c; if (d > 1 && d < 5) return b; if (d === 1) return a; return c; }
function esc(s) { return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }
let toastT;
function toast(text, bad) {
  const t = $('toast'); t.textContent = text; t.classList.toggle('bad', !!bad); t.classList.remove('hidden');
  clearTimeout(toastT); toastT = setTimeout(() => t.classList.add('hidden'), bad ? 7000 : 4500);
}
function confirmBox(title, text, okText = 'Да') {
  return new Promise((res) => {
    $('modalTitle').textContent = title; $('modalText').textContent = text; $('modalOk').querySelector('span').textContent = okText;
    $('modal').classList.remove('hidden');
    const done = (v) => { $('modal').classList.add('hidden'); $('modalOk').onclick = $('modalCancel').onclick = null; res(v); };
    $('modalOk').onclick = () => done(true); $('modalCancel').onclick = () => done(false);
  });
}

// ---- navigation
let view = 'main';
function go(v) {
  view = v;
  document.querySelectorAll('.view').forEach((el) => el.classList.toggle('active', el.id === 'view-' + v));
  document.querySelectorAll('nav [data-go]').forEach((el) => el.classList.toggle('active-nav', el.dataset.go === v));
  if (v === 'install') showInstallPart(job && (job.running || job.finished) && jobShown ? null : 'form');
  render();
}
document.querySelectorAll('[data-go]').forEach((el) => el.addEventListener('click', () => go(el.dataset.go)));

// ---- state
let S = null; let job = null; let jobShown = false; let polling = false;

async function refresh(force) {
  try { S = await api('state' + (force ? '?refresh=1' : '')); } catch (e) { toast(e.message, true); return; }
  if (S.job) job = S.job;
  render();
}

function render() {
  if (!S) return;
  const st = S.settings; const R = S.remote;
  // footer
  const ver = S.installed ? (S.localVersion || '?') : (R ? R.version : '—');
  const nf = R ? `NeoForge ${R.minecraft}` : 'NeoForge';
  const mods = R ? `${R.mods} ${plural(R.mods, 'мод', 'мода', 'модов')}` : '';
  $('footVersion').innerHTML = `Версия сборки: ${esc(ver)} <em>•</em> ${esc(nf)}${mods ? ' <em>•</em> ' + esc(mods) : ''}`;
  if (R && R.server && R.server.address) $('footAddress').textContent = R.server.address;

  // main
  const busy = job && job.running;
  $('btnPlayText').textContent = S.installed ? 'ИГРАТЬ' : 'УСТАНОВИТЬ';
  $('btnPlay').disabled = !!busy;
  const upd = S.installed && S.updateAvailable;
  $('btnUpdate').disabled = !S.installed || !!busy || (!upd && !R);
  $('updateBadge').classList.toggle('hidden', !upd);
  if (upd) $('updateBadge').textContent = 'доступна версия ' + R.version;
  let hint = 'Уютный мир. Большие приключения.';
  if (S.remoteError) hint = 'Нет связи с сервером обновлений: ' + S.remoteError;
  else if (!S.installed) hint = 'Сначала установи сборку — это займёт несколько минут';
  else if (busy) hint = 'Идёт ' + jobName(job.kind) + '… можно открыть вкладку установки';
  $('mainHint').textContent = hint;
  renderNews(R ? R.news : null);

  // install form
  $('installDir').textContent = st.installDir;
  $('installDir').title = st.installDir;
  const need = R ? R.size : 0;
  $('spaceLine').textContent = R ? `Нужно ${fmtBytes(need)} • свободно ${fmtBytes(S.freeBytes)}` : (S.remoteError ? 'Нет связи с сервером обновлений' : '');
  $('btnDownload').disabled = !R || !!busy;
  $('btnDownload').querySelector('span:not(.icon)').textContent = S.installed ? 'ОБНОВИТЬ СБОРКУ' : 'СКАЧАТЬ СБОРКУ';
  $('installHint').textContent = S.installed ? 'В этой папке уже есть сборка — докачаю только недостающее' : 'Можно выбрать уже созданный профиль Minecraft (папку с mods и config) — сборка встанет прямо в него';

  // settings
  $('memRange').value = st.memoryGB; setMemLabel(st.memoryGB);
  $('settingsDir').textContent = st.installDir; $('settingsDir').title = st.installDir;
  $('tglUpdates').classList.toggle('on', st.checkUpdates);
  $('tglClose').classList.toggle('on', st.closeAfterLaunch);
  $('btnRepair').disabled = !S.installed || !!busy;
  $('btnReinstall').disabled = !S.installed || !!busy;
  $('launcherVer').textContent = 'Версия лаунчера ' + S.launcherVersion;
  $('btnSelfUpdate').classList.toggle('hidden', !S.launcherUpdate);
  if (S.launcherUpdate) $('btnSelfUpdate').querySelector('span').textContent = 'Обновить лаунчер до ' + S.launcherUpdate.version;

  // skin
  $('tglSlim').classList.toggle('on', !!S.skinSlim);
  $('skinStatus').textContent = S.hasSkin ? 'Выбран свой скин — он появится в игре при следующем входе.' : 'Скин не выбран — в игре будет стандартный.';
}

function jobName(k) { return { install: 'установка', update: 'обновление', repair: 'проверка файлов', reinstall: 'переустановка' }[k] || 'операция'; }

function renderNews(list) {
  const box = $('newsList');
  if (!list || !list.length) { box.innerHTML = '<div class="news-empty">Новостей пока нет.<br>Здесь появятся обновления сборки и события сервера.</div>'; return; }
  const marks = ['✧', '◇', '✦'];
  box.innerHTML = list.slice(0, 3).map((n, i) => `<article><time>${esc(n.date)}</time><h2>${esc(n.title)}</h2><p>${esc(n.text)}</p><span class="news-mark">${marks[i % 3]}</span></article>`).join('');
}

// ---- server status
async function pollServer() {
  try {
    const s = await api('server');
    const chip = $('serverChip');
    chip.classList.toggle('off', !s.online); chip.classList.remove('wait');
    chip.querySelector('span').textContent = s.online ? `Сервер онлайн • ${s.players}/${s.max}` : 'Сервер спит';
  } catch (e) { /* ignore */ }
}

// ---- main actions
$('btnPlay').onclick = async () => {
  if (!S) return;
  if (!S.installed) { go('install'); return; }
  if (S.updateAvailable && S.settings.checkUpdates) {
    const ok = await confirmBox('Вышло обновление', `Доступна версия ${S.remote.version}. Без неё на сервер может не пустить. Обновить сейчас?`, 'Обновить');
    if (ok) { startJob('update'); return; }
  }
  try { await api('play', { method: 'POST' }); toast('Открываю лаунчер Minecraft: выбери профиль «Vibe Club» и нажми «Играть»'); }
  catch (e) {
    if (e.message === 'NO_OFFICIAL_LAUNCHER') {
      const ok = await confirmBox('Не найден лаунчер Minecraft', 'Сборка установлена, профиль «Vibe Club» готов. Кнопка «Играть» открывает официальный Minecraft Launcher, но на этом компьютере его нет. Скачать его с minecraft.net? После установки войди в аккаунт и снова нажми «Играть».', 'Скачать');
      if (ok) api('open-url?u=' + encodeURIComponent('https://www.minecraft.net/download'), { method: 'POST' });
    } else toast(e.message, true);
  }
};
$('btnUpdate').onclick = () => startJob('update');
$('btnDownload').onclick = () => startJob(S && S.installed ? 'update' : 'install');
$('btnManual').onclick = () => api('open-manual', { method: 'POST' }).then(() => toast('Открываю страницы загрузки в браузере…'));
$('btnRetry').onclick = () => startJob(lastKind || 'install');
$('btnRepair').onclick = () => startJob('repair');
$('btnReinstall').onclick = async () => {
  const ok = await confirmBox('Переустановить сборку?', 'Все файлы сборки будут удалены и скачаны заново. Миры, скриншоты, настройки управления и графики сохранятся.', 'Переустановить');
  if (ok) startJob('reinstall');
};
$('btnOpenFolder').onclick = $('btnOpenFolderTop').onclick = () => api('open-folder', { method: 'POST' });
$('btnLog').onclick = $('lnkLog').onclick = (e) => { if (e) e.preventDefault(); api('open-log', { method: 'POST' }); };
$('btnClose').onclick = async () => {
  if (job && job.running) { const ok = await confirmBox('Идёт загрузка', 'Если закрыть лаунчер, загрузка остановится. Закрыть?', 'Закрыть'); if (!ok) return; }
  try { await api('quit', { method: 'POST' }); } catch (e) {}
  window.close();
};
$('footAddress').onclick = () => { navigator.clipboard && navigator.clipboard.writeText($('footAddress').textContent); toast('Адрес сервера скопирован'); };
$('btnSelfUpdate').onclick = async () => { try { await api('update-launcher', { method: 'POST' }); toast('Скачиваю новую версию лаунчера, он перезапустится сам…'); } catch (e) { toast(e.message, true); } };

// ---- folder
async function pickFolder() {
  try {
    let r;
    try { r = await api('pick-folder', { method: 'POST' }); }
    catch (e) {
      if (e.message !== 'NO_PICKER') throw e;
      const typed = prompt('Введи путь к папке сборки (например ~/Games/VibeClub или папка профиля Minecraft):', (S && S.settings.installDir) || '');
      if (!typed) return;
      r = { path: typed };
    }
    if (!r.path) return;
    S = await api('settings', { method: 'POST', json: { installDir: r.path } });
    render();
    toast(r.mode === 'profile' ? 'Найден профиль Minecraft — сборка встанет прямо в него: ' + r.path + '. Миры и настройки останутся, старые моды уберу в папку mods_backup_vibeclub.'
      : r.mode === 'installed' ? 'Здесь уже стоит Vibe Club: ' + r.path : 'Папка сборки: ' + r.path);
  } catch (e) { toast(e.message, true); }
}
$('btnPick').onclick = pickFolder;
$('btnChangeDir').onclick = pickFolder;

// ---- settings
function setMemLabel(v) { $('memTitle').textContent = `Память для игры: ${v} ГБ`; $('memRange').style.setProperty('--p', ((v - 4) / 12 * 100) + '%'); }
$('memRange').oninput = (e) => setMemLabel(e.target.value);
$('memRange').onchange = async (e) => { try { S = await api('settings', { method: 'POST', json: { memoryGB: +e.target.value } }); render(); } catch (er) { toast(er.message, true); } };
$('tglUpdates').onclick = async () => { S = await api('settings', { method: 'POST', json: { checkUpdates: !S.settings.checkUpdates } }); render(); };
$('tglClose').onclick = async () => { S = await api('settings', { method: 'POST', json: { closeAfterLaunch: !S.settings.closeAfterLaunch } }); render(); };

// ---- jobs
let lastKind = null;
async function startJob(kind) {
  lastKind = kind;
  try { await api('start?kind=' + kind, { method: 'POST' }); }
  catch (e) { toast(e.message, true); return; }
  jobShown = true; go('install'); showInstallPart('progress'); pollJob();
}
function showInstallPart(p) {
  if (!p) p = job && job.running ? 'progress' : (job && job.finished ? (job.error ? 'error' : 'done') : 'form');
  $('installForm').classList.toggle('hidden', p !== 'form');
  $('installProgress').classList.toggle('hidden', p !== 'progress');
  $('installDone').classList.toggle('hidden', p !== 'done');
  $('installError').classList.toggle('hidden', p !== 'error');
}
async function pollJob() {
  if (polling) return; polling = true;
  try {
    for (;;) {
      job = await api('job');
      renderJob();
      if (!job.running) break;
      await new Promise((r) => setTimeout(r, 400));
    }
  } catch (e) { toast(e.message, true); }
  polling = false;
  await refresh(false);
  if (job && job.finished) {
    showInstallPart(job.error ? 'error' : 'done');
    if (!job.error) {
      const k = job.kind;
      $('doneTitle').textContent = k === 'install' || k === 'reinstall' ? 'Сборка установлена!' : (k === 'repair' ? 'Файлы проверены и исправлены' : 'Сборка обновлена!');
      $('doneText').innerHTML = 'Запусти её из официального лаунчера —<br>профиль «Vibe Club» уже создан';
    } else {
      const man = job.manual || [];
      $('errText').textContent = job.error;
      $('manualBox').classList.toggle('hidden', !man.length);
      $('btnLog').classList.toggle('hidden', !!man.length);
      if (man.length) {
        $('errText').textContent = 'Почти готово! Авторы этих модов разрешают скачивать их только со своей страницы. Нажми «Скачать», сохрани файлы в папку «Загрузки» и нажми «Повторить» — лаунчер сам их найдёт.';
        $('manualList').innerHTML = man.map((m) => `<div class="manual-row"><span>${esc(m.name)}</span><small>${esc(m.file)}</small></div>`).join('');
      }
    }
  }
}
function renderJob() {
  if (!job) return;
  const ph = job.phase;
  let title = job.phaseText || '';
  let pct = 0;
  if (ph === 'download') {
    title = `Загрузка файлов: ${job.done} из ${job.total}`;
    pct = job.bytesTotal ? job.bytesDone / job.bytesTotal * 100 : 100;
  } else if (ph === 'check') {
    title = `Проверка файлов: ${job.done} из ${job.total}`;
    pct = job.total ? job.done / job.total * 100 : 0;
  } else if (ph === 'loader') { pct = 100; }
  $('progTitle').textContent = job.paused ? 'Пауза' : title;
  $('progPct').textContent = Math.floor(pct) + '%';
  $('progBar').style.width = Math.min(100, pct) + '%';
  if (ph === 'download') {
    const left = job.speed > 0 ? Math.max(0, (job.bytesTotal - job.bytesDone) / job.speed) : 0;
    const eta = left > 0 ? (left < 60 ? 'меньше минуты' : Math.ceil(left / 60) + ' мин') : '…';
    $('progStats').textContent = `${fmtBytes(job.bytesDone)} из ${fmtBytes(job.bytesTotal)} • ${fmtBytes(job.speed)}/с • осталось ${eta}`;
  } else {
    $('progStats').textContent = job.phaseText || ' ';
  }
  $('progFile').textContent = job.current || ' ';
  $('btnPause').querySelector('span').textContent = job.paused ? 'Продолжить' : 'Пауза';
  $('btnPause').disabled = ph !== 'download';
}
$('btnPause').onclick = () => api('pause', { method: 'POST' });
$('btnCancel').onclick = async () => { const ok = await confirmBox('Остановить?', 'Уже скачанные файлы сохранятся, продолжить можно позже.', 'Остановить'); if (ok) api('cancel', { method: 'POST' }); };

// ---- skin
let skinImg = null; let skinBack = false;
$('btnSkinUpload').onclick = () => $('skinFile').click();
$('skinFile').onchange = async (e) => {
  const f = e.target.files[0]; e.target.value = '';
  if (!f) return;
  const buf = await f.arrayBuffer();
  try {
    await api('skin?slim=' + ($('tglSlim').classList.contains('on') ? '1' : '0'), { method: 'POST', body: buf });
    toast('Скин сохранён! Он появится в игре при следующем входе на сервер.');
    await loadSkin(); await refresh(false);
  } catch (er) { toast(er.message, true); }
};
$('btnSkinReset').onclick = async () => { await api('skin', { method: 'DELETE' }); skinImg = null; drawSkin(); await refresh(false); toast('Скин сброшен'); };
$('tglSlim').onclick = async () => {
  const on = !$('tglSlim').classList.contains('on');
  $('tglSlim').classList.toggle('on', on);
  await api('skin-model?slim=' + (on ? '1' : '0'), { method: 'POST' });
  if (S) S.skinSlim = on;
  drawSkin();
};
$('rotL').onclick = $('rotR').onclick = () => { skinBack = !skinBack; drawSkin(); $('rotLabel').textContent = skinBack ? 'Сзади' : 'Спереди'; };

async function loadSkin() {
  try {
    const r = await fetch(API + 'skin?t=' + Date.now());
    if (!r.ok) { skinImg = null; drawSkin(); return; }
    const blob = await r.blob();
    const img = new Image();
    img.onload = () => { skinImg = img; drawSkin(); };
    img.src = URL.createObjectURL(blob);
  } catch (e) { skinImg = null; drawSkin(); }
}

// Draws a flat front/back view (16x32 px) of a Minecraft skin, including the outer layer.
function drawSkin() {
  const c = $('skinCanvas'); const ph = $('skinPlaceholder');
  if (!skinImg) { c.classList.add('hidden'); ph.classList.remove('hidden'); return; }
  c.classList.remove('hidden'); ph.classList.add('hidden');
  const x = c.getContext('2d'); x.imageSmoothingEnabled = false; x.clearRect(0, 0, 16, 32);
  const legacy = skinImg.height === 32;
  const slim = $('tglSlim').classList.contains('on');
  const aw = slim ? 3 : 4;
  const part = (sx, sy, w, h, dx, dy, mirror) => {
    if (mirror) { x.save(); x.translate(dx + w, dy); x.scale(-1, 1); x.drawImage(skinImg, sx, sy, w, h, 0, 0, w, h); x.restore(); }
    else x.drawImage(skinImg, sx, sy, w, h, dx, dy, w, h);
  };
  const ax = 4 - aw; // arm offset on the left side for slim
  if (!skinBack) {
    part(8, 8, 8, 8, 4, 0); part(20, 20, 8, 12, 4, 8);
    part(44, 20, aw, 12, ax, 8);                                   // right arm (viewer left)
    if (legacy) part(44, 20, aw, 12, 12, 8, true); else part(36, 52, aw, 12, 12, 8); // left arm
    part(4, 20, 4, 12, 4, 20);
    if (legacy) part(4, 20, 4, 12, 8, 20, true); else part(20, 52, 4, 12, 8, 20);
    part(40, 8, 8, 8, 4, 0);
    if (!legacy) { part(20, 36, 8, 12, 4, 8); part(44, 36, aw, 12, ax, 8); part(52, 52, aw, 12, 12, 8); part(4, 36, 4, 12, 4, 20); part(4, 52, 4, 12, 8, 20); }
  } else {
    part(24, 8, 8, 8, 4, 0); part(32, 20, 8, 12, 4, 8);
    const rb = 44 + aw + 4; // back of right arm
    part(rb, 20, aw, 12, 12, 8);
    if (legacy) part(rb, 20, aw, 12, ax, 8, true); else part(36 + aw + 4, 52, aw, 12, ax, 8);
    part(12, 20, 4, 12, 8, 20);
    if (legacy) part(12, 20, 4, 12, 4, 20, true); else part(28, 52, 4, 12, 4, 20);
    part(56, 8, 8, 8, 4, 0);
    if (!legacy) { part(32, 36, 8, 12, 4, 8); part(rb, 36, aw, 12, 12, 8); part(52 + aw + 4, 52, aw, 12, ax, 8); part(12, 36, 4, 12, 8, 20); part(12, 52, 4, 12, 4, 20); }
  }
}

// ---- boot
(async function boot() {
  await refresh(true);
  go(S && !S.installed ? 'install' : 'main');
  if (S && S.job && S.job.running) { jobShown = true; go('install'); showInstallPart('progress'); pollJob(); }
  loadSkin(); pollServer();
  setInterval(pollServer, 30000);
  setInterval(() => api('ping').catch(() => {}), 3000);
  setInterval(() => { if (!(job && job.running)) refresh(false); }, 60000);
})();
