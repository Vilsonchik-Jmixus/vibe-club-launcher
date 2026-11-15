// Vibe Club — сезоны Serene Seasons по настоящему календарю (московское время).
// Метеорологические сезоны: март — ранняя весна, апрель — середина весны, май — поздняя весна,
// июнь–август — лето, сентябрь–ноябрь — осень, декабрь–февраль — зима. Внутри месяца положение
// тоже соответствует дате (15 ноября — середина «поздней осени»).
// Длина подсезона в конфиге (sub_season_duration = 2190 игровых дней ≈ 30,4 суток реального
// времени), поэтому и между проверками сезон идёт почти с реальной скоростью; скрипт сверяет его
// с календарём при запуске сервера и раз в минуту и поправляет, если разошлось больше чем на 2 часа.
//
//   /vc_season        — показать, какой сезон должен быть сейчас и какой стоит в мире (операторы)
//   /vc_season sync   — принудительно выставить сезон по дате
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_SEA = { ok: false, SH: null, ST: null, tick: 0, checked: false,
  names: ['ранняя весна', 'середина весны', 'поздняя весна', 'раннее лето', 'середина лета', 'позднее лето',
    'ранняя осень', 'середина осени', 'поздняя осень', 'ранняя зима', 'середина зимы', 'поздняя зима'] }

try {
  VC_SEA.SH = Java.loadClass('sereneseasons.season.SeasonHandler')
  VC_SEA.ST = Java.loadClass('sereneseasons.season.SeasonTime')
  VC_SEA.ok = true
} catch (e) {
  console.warn('[Vibe Club] Синхронизация сезонов выключена (нет Serene Seasons?): ' + e)
}

// Положение в году в «подсезонах» (0 — начало ранней весны, 12 — конец поздней зимы).
function vcSeaCalendar() {
  var d = new Date(Date.now() + 3 * 3600 * 1000) // МСК, дальше только UTC-геттеры
  var m = d.getUTCMonth()
  var days = new Date(Date.UTC(d.getUTCFullYear(), m + 1, 0)).getUTCDate()
  var frac = (d.getUTCDate() - 1 + d.getUTCHours() / 24 + d.getUTCMinutes() / 1440) / days
  return ((m - 2 + 12) % 12) + frac
}

function vcSeaState(server) {
  var level = server.overworld()
  var data = VC_SEA.SH.getSeasonSavedData(level)
  var sub = Number(VC_SEA.ST.ZERO.getSubSeasonDuration())
  var cycle = Number(VC_SEA.ST.ZERO.getCycleDuration())
  var want = Math.floor(vcSeaCalendar() * sub) % cycle
  return { level: level, data: data, sub: sub, cycle: cycle, want: want, have: Number(data.seasonCycleTicks) }
}

function vcSeaSync(server, force) {
  var s = vcSeaState(server)
  var diff = Math.abs(s.want - s.have)
  diff = Math.min(diff, s.cycle - diff)
  // 2 часа реального времени = 1/365 подсезона… берём с запасом: sub / 360
  if (!force && diff <= s.sub / 360) return false
  s.data.seasonCycleTicks = s.want
  s.data.setDirty()
  VC_SEA.SH.sendSeasonUpdate(s.level)
  console.info('[Vibe Club] Сезон выставлен по календарю: ' + VC_SEA.names[Math.floor(s.want / s.sub) % 12] +
    ' (' + Math.round(100 * (s.want % s.sub) / s.sub) + '%), было ' + s.have + ', стало ' + s.want)
  return true
}

ServerEvents.loaded(function (event) {
  if (!VC_SEA.ok) return
  VC_SEA.checked = true
  try { vcSeaSync(event.server, true) } catch (e) { console.error('[Vibe Club] Сезоны: ' + e) }
})

ServerEvents.tick(function (event) {
  if (!VC_SEA.ok) return
  VC_SEA.tick++
  // после /reload событие loaded не приходит — сверяемся на первом тике, дальше раз в минуту
  if (VC_SEA.checked && VC_SEA.tick % 1200 != 0) return
  VC_SEA.checked = true
  try { vcSeaSync(event.server, false) } catch (e) {
    if (VC_SEA.tick % 72000 == 0 || VC_SEA.tick == 1) console.error('[Vibe Club] Сезоны: ' + e)
  }
})

ServerEvents.commandRegistry(function (event) {
  if (!VC_SEA.ok) return
  var C = event.commands
  var info = function (ctx) {
    var s = vcSeaState(ctx.source.server)
    var name = function (t) { return VC_SEA.names[Math.floor(t / s.sub) % 12] + ' ' + Math.round(100 * (t % s.sub) / s.sub) + '%' }
    ctx.source.sendSystemMessage(Text.yellow('[Сезоны] По календарю: ' + name(s.want) + ', в мире: ' + name(s.have)))
    return 1
  }
  event.register(C.literal('vc_season').requires(function (src) { return src.hasPermission(2) })
    .executes(info)
    .then(C.literal('sync').executes(function (ctx) { vcSeaSync(ctx.source.server, true); return info(ctx) })))
})
