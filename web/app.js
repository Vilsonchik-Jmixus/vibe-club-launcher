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
  if (v === 'skin' && typeof loadSkin === 'function') loadSkin();
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
  else if (S.profile && S.profile.kind && S.profile.kind !== 'official') hint = 'Игра запускается через ' + S.profile.name + (S.profile.profile ? ' (профиль «' + S.profile.profile + '»)' : '') + (S.profile.loaderOk === false ? ' — проверь версию NeoForge в профиле!' : '');
  $('mainHint').textContent = hint;
  renderNews(R ? R.news : null);

  // install form
  setPathField('installDir', st.installDir);
  const need = R ? R.size : 0;
  $('spaceLine').textContent = R ? `Нужно ${fmtBytes(need)} • свободно ${fmtBytes(S.freeBytes)}` : (S.remoteError ? 'Нет связи с сервером обновлений' : '');
  $('btnDownload').disabled = !R || !!busy;
  $('btnDownload').querySelector('span:not(.icon)').textContent = S.installed ? 'ОБНОВИТЬ СБОРКУ' : 'СКАЧАТЬ СБОРКУ';
  $('installHint').textContent = S.installed ? 'В этой папке уже есть сборка — докачаю только недостающее' : 'Скопируй путь к папке и вставь в поле выше (Ctrl+V), или нажми «Выбрать папку». Можно указать уже созданный профиль Minecraft — сборка встанет прямо в него';

  // settings
  $('memRange').value = st.memoryGB; setMemLabel(st.memoryGB);
  setPathField('settingsDir', st.installDir);
  $('tglUpdates').classList.toggle('on', st.checkUpdates);
  $('tglClose').classList.toggle('on', st.closeAfterLaunch);
  $('btnRepair').disabled = !S.installed || !!busy;
  $('btnApplyOptions').disabled = !S.installed || !!busy;
  $('btnReinstall').disabled = !S.installed || !!busy;
  $('launcherVer').textContent = 'Версия лаунчера ' + S.launcherVersion;
  $('btnSelfUpdate').classList.toggle('hidden', !S.launcherUpdate);
  if (S.launcherUpdate) $('btnSelfUpdate').querySelector('span').textContent = 'Обновить лаунчер до ' + S.launcherUpdate.version;

  // skin
  $('tglSlim').classList.toggle('on', !!S.skinSlim);
}

function jobName(k) { return { install: 'установка', update: 'обновление', repair: 'проверка файлов', reinstall: 'переустановка' }[k] || 'операция'; }

let NEWS = []; let newsKey = ''; let newsSel = 0;
function renderNews(list) {
  const key = JSON.stringify(list || []);
  if (key === newsKey) return;
  newsKey = key; NEWS = list || [];
  const box = $('newsList');
  $('btnAllNews').classList.toggle('hidden', !NEWS.length);
  if (!NEWS.length) { box.innerHTML = '<div class="news-empty">Новостей пока нет.<br>Здесь появятся обновления сборки и события сервера.</div>'; $('feedList').innerHTML = ''; $('feedRead').innerHTML = ''; return; }
  const marks = ['✧', '◇', '✦'];
  box.innerHTML = NEWS.map((n, i) => `<article data-n="${i}" title="Читать подробнее"><time>${esc(n.date)}</time><h2>${esc(n.title)}</h2><p>${esc(n.text)}</p><span class="news-mark">${marks[i % 3]}</span></article>`).join('');
  box.querySelectorAll('article').forEach((a) => a.addEventListener('click', () => openNews(+a.dataset.n)));
  $('feedList').innerHTML = NEWS.map((n, i) => `<button data-n="${i}"><time>${esc(n.date)}</time><strong>${esc(n.title)}</strong></button>`).join('');
  $('feedList').querySelectorAll('button').forEach((b) => b.addEventListener('click', () => showNews(+b.dataset.n)));
  showNews(Math.min(newsSel, NEWS.length - 1));
}
function newsBody(t) {
  let html = ''; let inList = false;
  const close = () => { if (inList) { html += '</ul>'; inList = false; } };
  String(t || '').split('\n').forEach((raw) => {
    const l = raw.trim();
    if (!l) { close(); return; }
    if (/^[-•*] /.test(l)) { if (!inList) { html += '<ul>'; inList = true; } html += '<li>' + esc(l.slice(2)) + '</li>'; return; }
    close();
    if (l.startsWith('## ')) html += '<h3>' + esc(l.slice(3)) + '</h3>'; else html += '<p>' + esc(l) + '</p>';
  });
  close();
  return html;
}
function showNews(i) {
  const n = NEWS[i]; if (!n) return;
  newsSel = i;
  $('feedList').querySelectorAll('button').forEach((b) => b.classList.toggle('sel', +b.dataset.n === i));
  $('feedRead').innerHTML = `<time>${esc(n.date)}</time><h2>${esc(n.title)}</h2><p class="lead">${esc(n.text)}</p>${newsBody(n.more)}`;
  $('feedRead').scrollTop = 0;
}
function openNews(i) { go('news'); showNews(i); const b = $('feedList').querySelector(`[data-n="${i}"]`); if (b) b.scrollIntoView({ block: 'nearest' }); }
$('btnAllNews').addEventListener('click', () => openNews(0));

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
  try {
    const r = await api('play', { method: 'POST' });
    toast(r.loaderOk === false ? r.message + '. Внимание: в профиле стоит ' + r.loader + ', а нужен NeoForge ' + (S.remote && S.remote.loader ? S.remote.loader.version : '') : r.message);
  }
  catch (e) {
    if (e.message.startsWith('NO_LAUNCHER:')) {
      toast('Не нашёл ' + e.message.slice(12) + ' на этом компьютере. Открой его сам и нажми Play на профиле со сборкой.', true);
    } else if (e.message === 'NO_OFFICIAL_LAUNCHER') {
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
$('btnApplyOptions').onclick = async () => {
  if (!confirm('Перенести настройки графики, звука и управление из сборки Vibe Club? Твой текущий options.txt сохранится рядом как резервная копия.')) return;
  try {
    const r = await api('apply-options', { method: 'POST' });
    toast(r.changed ? `Готово: обновлено параметров — ${r.changed}. Если игра запущена, перезапусти её.` : 'Настройки уже совпадают со сборкой');
  } catch (e) { toast(e.message, true); }
};
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
$('btnSelfUpdate').onclick = async () => {
  try {
    toast('Скачиваю новую версию лаунчера, он перезапустится сам…');
    await api('update-launcher', { method: 'POST' });
    setTimeout(() => window.close(), 1200); // the new launcher opens its own window
  } catch (e) { toast(e.message, true); }
};

// ---- folder
function setPathField(id, value) {
  const el = $(id);
  if (document.activeElement === el || el.dataset.dirty === '1') return; // the player is typing
  el.value = value || '';
  el.title = (value ? value + '\n' : '') + 'Можно вставить скопированный путь: Ctrl+V и Enter';
}
async function applyFolder(r) {
  if (!r || !r.path) return;
  S = await api('settings', { method: 'POST', json: { installDir: r.path } });
  ['installDir', 'settingsDir'].forEach((id) => { $(id).dataset.dirty = ''; });
  render();
  toast(r.mode === 'profile' ? 'Найден профиль' + (r.kind && r.kind !== 'official' ? ' ' + r.launcher : ' Minecraft') + ' — сборка встанет прямо в него: ' + r.path + '. Миры и настройки останутся, старые моды уберу в папку mods_backup_vibeclub.'
    : r.mode === 'installed' ? 'Здесь уже стоит Vibe Club: ' + r.path : 'Папка сборки: ' + r.path);
}
async function pickFolder() {
  try {
    let r;
    try { r = await api('pick-folder', { method: 'POST' }); }
    catch (e) {
      if (e.message !== 'NO_PICKER') throw e;
      const typed = prompt('Введи путь к папке сборки (например ~/Games/VibeClub или папка профиля Minecraft):', (S && S.settings.installDir) || '');
      if (!typed) return;
      r = await api('check-folder', { method: 'POST', json: { path: typed } });
    }
    await applyFolder(r);
  } catch (e) { toast(e.message, true); }
}
async function usePastedPath(el) {
  const v = el.value.trim();
  if (!v || (S && v === S.settings.installDir)) { el.dataset.dirty = ''; render(); return; }
  try {
    const r = await api('check-folder', { method: 'POST', json: { path: v } });
    el.dataset.dirty = '';
    el.blur();
    await applyFolder(r);
  } catch (e) { toast(e.message, true); }
}
['installDir', 'settingsDir'].forEach((id) => {
  const el = $(id);
  el.addEventListener('input', () => { el.dataset.dirty = '1'; });
  el.addEventListener('paste', () => { el.dataset.dirty = '1'; setTimeout(() => usePastedPath(el), 0); });
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); usePastedPath(el); }
    if (e.key === 'Escape') { el.dataset.dirty = ''; el.blur(); render(); }
  });
  el.addEventListener('focus', () => el.select());
  el.addEventListener('blur', () => { if (el.dataset.dirty === '1') usePastedPath(el); });
});
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
let skinImg = null;
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
$('btnSkinReset').onclick = async () => { await api('skin', { method: 'DELETE' }); skinImg = null; buildModel(); await refresh(false); toast('Скин сброшен'); };
$('tglSlim').onclick = async () => {
  const on = !$('tglSlim').classList.contains('on');
  $('tglSlim').classList.toggle('on', on);
  await api('skin-model?slim=' + (on ? '1' : '0'), { method: 'POST' });
  if (S) S.skinSlim = on;
  buildModel();
};

async function loadSkin() {
  try {
    const r = await fetch(API + 'skin?t=' + Date.now());
    if (!r.ok) { skinImg = null; buildModel(); skinSource(''); return; }
    skinSource(r.headers.get('X-Skin-Source') || '');
    if (r.headers.get('X-Skin-Slim') === '1') $('tglSlim').classList.add('on');
    const blob = await r.blob();
    const img = new Image();
    img.onload = () => { skinImg = img; buildModel(); };
    img.src = URL.createObjectURL(blob);
  } catch (e) { skinImg = null; buildModel(); }
}

function skinSource(src) {
  const t = { game: 'Сейчас в игре этот скин (из файлов Quick Skin в папке сборки).', account: 'Сейчас в игре скин твоего аккаунта Minecraft.', pending: 'Новый скин выбран — он включится при следующем входе в игру.', launcher: 'Скин выбран в лаунчере.' }[src];
  $('skinStatus').textContent = t || 'Скин не выбран — на модели пример. Загрузи свой PNG, и он появится здесь и в игре.';
}

// ---- 3D skin viewer (CSS 3D, no libraries)
const PX = 8.5;                       // screen pixels per skin pixel
let yaw = -28, pitch = -8, autoSpin = true, dragging = null;

// Neutral original skin used when the player has not uploaded one.
function defaultSkin() {
  const c = document.createElement('canvas'); c.width = 64; c.height = 64; const x = c.getContext('2d');
  const box = (u, v, w, h, d, col) => { x.fillStyle = col; x.fillRect(u, v + d, 2 * (w + d), h); x.fillRect(u + d, v, 2 * w, d); };
  box(0, 0, 8, 8, 8, '#c99a7a');                         // head
  x.fillStyle = '#4a2b23'; x.fillRect(8, 0, 8, 8); x.fillRect(0, 8, 32, 2); x.fillRect(24, 8, 8, 6); // hair
  x.fillStyle = '#fff'; x.fillRect(9, 12, 2, 1); x.fillRect(13, 12, 2, 1);
  x.fillStyle = '#5b2a7e'; x.fillRect(10, 12, 1, 1); x.fillRect(13, 12, 1, 1);
  x.fillStyle = '#a0675a'; x.fillRect(11, 14, 2, 1);
  box(16, 16, 8, 12, 4, '#7b2f86');                      // shirt
  x.fillStyle = '#e8a64f'; x.fillRect(20, 20, 8, 1);
  box(40, 16, 4, 12, 4, '#c99a7a'); x.fillStyle = '#6a2774'; x.fillRect(40, 20, 16, 5); x.fillRect(44, 16, 4, 4);
  box(32, 48, 4, 12, 4, '#c99a7a'); x.fillRect(32, 52, 16, 5); x.fillRect(36, 48, 4, 4);
  box(0, 16, 4, 12, 4, '#2c2140'); x.fillStyle = '#1a1424'; x.fillRect(0, 29, 16, 3);
  box(16, 48, 4, 12, 4, '#2c2140'); x.fillStyle = '#1a1424'; x.fillRect(16, 61, 16, 3);
  return c;
}

// Converts an old 64x32 skin to 64x64 (left limbs = mirrored right limbs).
function toModern(img) {
  const c = document.createElement('canvas'); c.width = 64; c.height = 64; const x = c.getContext('2d');
  x.imageSmoothingEnabled = false; x.drawImage(img, 0, 0);
  if (img.height !== 32) return c;
  const mirrorLimb = (su, sv, du, dv) => {
    const faces = [[4, 0, 4, 4, 4, 0], [8, 0, 4, 4, 8, 0], [0, 4, 4, 12, 8, 4], [4, 4, 4, 12, 4, 4], [8, 4, 4, 12, 0, 4], [12, 4, 4, 12, 12, 4]];
    for (const [sx, sy, w, h, dx, dy] of faces) {
      x.save(); x.translate(du + dx + w, dv + dy); x.scale(-1, 1);
      x.drawImage(img, su + sx, sv + sy, w, h, 0, 0, w, h); x.restore();
    }
  };
  mirrorLimb(0, 16, 16, 48);  // leg
  mirrorLimb(40, 16, 32, 48); // arm
  return c;
}

function cuboid(tex, u, v, w, h, d, grow) {
  const el = document.createElement('div'); el.className = 'cube';
  const W = (w + grow) * PX, H = (h + grow) * PX, D = (d + grow) * PX;
  const faces = [
    ['front', u + d, v + d, w, h, W, H, `translateZ(${D / 2}px)`],
    ['back', u + 2 * d + w, v + d, w, h, W, H, `rotateY(180deg) translateZ(${D / 2}px)`],
    ['right', u, v + d, d, h, D, H, `rotateY(-90deg) translateZ(${W / 2}px)`],
    ['left', u + d + w, v + d, d, h, D, H, `rotateY(90deg) translateZ(${W / 2}px)`],
    ['top', u + d, v, w, d, W, D, `rotateX(90deg) translateZ(${H / 2}px)`],
    ['bottom', u + d + w, v, w, d, W, D, `rotateX(-90deg) translateZ(${H / 2}px)`],
  ];
  for (const [, sx, sy, sw, sh, cw, ch, tr] of faces) {
    const c = document.createElement('canvas'); c.width = sw; c.height = sh;
    const x = c.getContext('2d'); x.imageSmoothingEnabled = false; x.drawImage(tex, sx, sy, sw, sh, 0, 0, sw, sh);
    c.style.width = cw + 'px'; c.style.height = ch + 'px';
    c.style.transform = `translate(-50%,-50%) ${tr}`;
    el.appendChild(c);
  }
  return el;
}

function buildModel() {
  const root = $('skinModel'); if (!root) return;
  root.innerHTML = '';
  const tex = skinImg ? toModern(skinImg) : defaultSkin();
  const legacy = skinImg && skinImg.height === 32;
  const slim = $('tglSlim').classList.contains('on');
  const aw = slim ? 3 : 4;
  // [u, v, w, h, d, overlayU, overlayV, x, y] — x/y = centre in skin pixels, y up from the feet
  const parts = [
    [0, 0, 8, 8, 8, 32, 0, 0, 28, 1],
    [16, 16, 8, 12, 4, 16, 32, 0, 18, 0.5],
    [40, 16, aw, 12, 4, 40, 32, -(4 + aw / 2), 18, 0.5],
    [32, 48, aw, 12, 4, 48, 48, 4 + aw / 2, 18, 0.5],
    [0, 16, 4, 12, 4, 0, 32, -2, 6, 0.5],
    [16, 48, 4, 12, 4, 0, 48, 2, 6, 0.5],
  ];
  for (const [u, v, w, h, d, ou, ov, cx, cy, g] of parts) {
    const pos = `translate3d(${cx * PX}px, ${-cy * PX}px, 0)`;
    const base = cuboid(tex, u, v, w, h, d, 0); base.style.transform = pos; root.appendChild(base);
    if (!legacy || ou === 32) { const o = cuboid(tex, ou, ov, w, h, d, g); o.style.transform = pos; o.classList.add('overlay'); root.appendChild(o); }
  }
  $('skinStage').classList.toggle('placeholder', !skinImg);
  applyRot();
}

function applyRot() { const r = $('skinModel'); if (r) r.style.transform = `translateY(150px) rotateX(${pitch}deg) rotateY(${yaw}deg)`; }

(function setupViewer() {
  const st = $('skinStage');
  st.addEventListener('pointerdown', (e) => { dragging = { x: e.clientX, y: e.clientY, yaw, pitch }; autoSpin = false; st.setPointerCapture(e.pointerId); st.classList.add('grabbing'); });
  st.addEventListener('pointermove', (e) => {
    if (!dragging) return;
    yaw = dragging.yaw + (e.clientX - dragging.x) * 0.6;
    pitch = Math.max(-35, Math.min(25, dragging.pitch - (e.clientY - dragging.y) * 0.3));
    applyRot();
  });
  const end = () => { dragging = null; st.classList.remove('grabbing'); };
  st.addEventListener('pointerup', end); st.addEventListener('pointercancel', end);
  $('rotL').onclick = () => { autoSpin = false; yaw -= 45; applyRot(); };
  $('rotR').onclick = () => { autoSpin = false; yaw += 45; applyRot(); };
  $('rotLabel').onclick = () => { autoSpin = !autoSpin; };
  let last = performance.now();
  const tick = (t) => { const dt = (t - last) / 1000; last = t; if (autoSpin && !dragging && $('view-skin').classList.contains('active')) { yaw += dt * 25; applyRot(); } requestAnimationFrame(tick); };
  requestAnimationFrame(tick);
})();

// ---- boot
// Ping right away (the launcher closes itself when the window stops pinging). If the launcher
// process is gone (closed or restarted after an update), close this stale window.
let pingFails = 0;
setInterval(() => api('ping').then(() => { pingFails = 0; }).catch(() => { if (++pingFails >= 3) window.close(); }), 3000);
(async function boot() {
  await refresh(true);
  go(S && !S.installed ? 'install' : 'main');
  if (S && S.job && S.job.running) { jobShown = true; go('install'); showInstallPart('progress'); pollJob(); }
  loadSkin(); pollServer();
  setInterval(pollServer, 30000);
  setInterval(() => { if (!(job && job.running)) refresh(false); }, 60000);
})();

// ---- browser choice
async function loadBrowsers() {
  try {
    const r = await api('browsers');
    const sel = $('selBrowser');
    sel.innerHTML = r.options.map((o) => `<option value="${esc(o.id)}">${esc(o.name)}</option>`).join('');
    sel.value = r.current;
    if (sel.value !== r.current) sel.value = 'auto';
  } catch (e) { /* old backend */ }
}
$('selBrowser').addEventListener('change', async (e) => {
  try {
    S = await api('settings', { method: 'POST', json: { browser: e.target.value } });
    toast('Готово. Со следующего запуска лаунчер откроется в: ' + e.target.selectedOptions[0].textContent + '. Чтобы открыть сейчас — нажми «Открыть».');
  } catch (err) { toast(err.message, true); }
});
$('btnOpenInBrowser').onclick = async () => {
  try { await api('open-in-browser', { method: 'POST' }); toast('Открываю в выбранном браузере — это окно можно закрыть'); } catch (e) { toast(e.message, true); }
};
loadBrowsers();
