// Vibe Club — прогрузка мира (замена Chunky: Chunky 1.4.23 роняет сервер вместе с Distant Horizons 3.3.3,
// «Chunky is not loaded» при запуске). Генерирует чанки квадратом от центра кольцами наружу,
// в фоне, по несколько штук одновременно; если сервер начинает тормозить — сбавляет темп.
// Прогресс сохраняется, после перезапуска сервера прогрузка продолжается сама.
//
//   /vc_pregen start <мир> <центр X> <центр Z> <радиус в блоках>   — запустить (квадрат 2·радиус)
//   /vc_pregen status | pause | resume | stop
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_PG = { ok: false, ChunkPos: null, TicketType: null, ticket: null, job: null, inflight: [], tick: 0,
  slots: 6, maxSlots: 24, lastSave: 0, lastReport: 0, doneSinceReport: 0 }

try {
  VC_PG.ChunkPos = Java.loadClass('net.minecraft.world.level.ChunkPos')
  VC_PG.TicketType = Java.loadClass('net.minecraft.server.level.TicketType')
  var Comparator = Java.loadClass('java.util.Comparator')
  VC_PG.ticket = VC_PG.TicketType.create('vibeclub_pregen', Comparator.comparingLong(function (p) { return p.toLong() }))
  VC_PG.IntArg = Java.loadClass('com.mojang.brigadier.arguments.IntegerArgumentType')
  VC_PG.IdArg = Java.loadClass('net.minecraft.commands.arguments.ResourceLocationArgument')
  VC_PG.ok = true
} catch (e) {
  console.error('[Vibe Club] Прогрузка мира выключена: ' + e)
}

function vcPgDimId(level) {
  var s = String(level.dimension)
  var m = s.match(/([a-z0-9_.-]+:[a-z0-9_.\/-]+)\]?$/)
  return m ? m[1] : s
}

function vcPgLevel(server, id) {
  var it = server.getAllLevels().iterator()
  while (it.hasNext()) {
    var l = it.next()
    if (vcPgDimId(l) == id) return l
  }
  return null
}

// k-я клетка кольца r (r > 0): обход квадрата по периметру, 8r клеток.
function vcPgCell(r, k) {
  if (r == 0) return [0, 0]
  var side = Math.floor(k / (2 * r))
  var t = k % (2 * r)
  if (side == 0) return [-r + t, -r]
  if (side == 1) return [r, -r + t]
  if (side == 2) return [r - t, r]
  return [-r, r - t]
}

function vcPgTotal(R) { return (2 * R + 1) * (2 * R + 1) }

function vcPgDoneCount(job) {
  if (job.ring == 0) return job.k
  return (2 * job.ring - 1) * (2 * job.ring - 1) + job.k
}

function vcPgSave(server) {
  var d = server.persistentData
  if (!VC_PG.job) { d.remove('vc_pregen'); return }
  d.putString('vc_pregen', JSON.stringify(VC_PG.job))
}

function vcPgLoad(server) {
  var d = server.persistentData
  if (!d.contains('vc_pregen')) return
  try {
    VC_PG.job = JSON.parse(String(d.getString('vc_pregen')))
    VC_PG.job.paused = VC_PG.job.paused || false
    // чанки, которые генерировались в момент остановки сервера, могли не сохраниться — повторяем их
    for (var n = 0; n < VC_PG.maxSlots; n++) {
      if (VC_PG.job.k > 0) VC_PG.job.k--
      else if (VC_PG.job.ring > 0) { VC_PG.job.ring--; VC_PG.job.k = VC_PG.job.ring == 0 ? 0 : 8 * VC_PG.job.ring - 1 }
    }
    console.info('[Vibe Club] Прогрузка мира продолжается: ' + VC_PG.job.dim + ', ' + vcPgPercent(VC_PG.job) + '%')
  } catch (e) {
    console.error('[Vibe Club] Не прочитал сохранённую прогрузку: ' + e)
  }
}

function vcPgPercent(job) {
  return (100 * vcPgDoneCount(job) / vcPgTotal(job.R)).toFixed(1)
}

function vcPgRelease(level, item) {
  try { level.getChunkSource().removeRegionTicket(VC_PG.ticket, item.pos, 0, item.pos) } catch (e) { }
}

function vcPgStep(server) {
  var job = VC_PG.job
  if (!job || job.paused) return
  var level = vcPgLevel(server, job.dim)
  if (!level) return
  var cs = level.getChunkSource()

  // завершённые чанки отпускаем — они выгрузятся и сохранятся на диск
  var still = []
  for (var i = 0; i < VC_PG.inflight.length; i++) {
    var it = VC_PG.inflight[i]
    if (cs.getChunkNow(it.pos.x, it.pos.z) != null || VC_PG.tick - it.t > 1200) {
      vcPgRelease(level, it)
      VC_PG.doneSinceReport++
    } else {
      still.push(it)
    }
  }
  VC_PG.inflight = still

  // темп: сервер успевает (< 35 мс на тик) — больше чанков одновременно, тормозит (> 45 мс) — меньше
  var mspt = server.getAverageTickTimeNanos() / 1000000
  if (VC_PG.tick % 40 == 0) {
    if (mspt < 35 && VC_PG.slots < VC_PG.maxSlots) VC_PG.slots++
    if (mspt > 45 && VC_PG.slots > 1) VC_PG.slots = Math.max(1, Math.floor(VC_PG.slots / 2))
  }

  while (VC_PG.inflight.length < VC_PG.slots && job.ring <= job.R) {
    var c = vcPgCell(job.ring, job.k)
    var pos = new VC_PG.ChunkPos(job.cx + c[0], job.cz + c[1])
    cs.addRegionTicket(VC_PG.ticket, pos, 0, pos)
    VC_PG.inflight.push({ pos: pos, t: VC_PG.tick })
    job.k++
    if (job.ring == 0 || job.k >= 8 * job.ring) { job.ring++; job.k = 0 }
  }

  var now = Date.now()
  if (now - VC_PG.lastSave > 10000) { VC_PG.lastSave = now; vcPgSave(server) }
  if (now - VC_PG.lastReport > 60000) {
    var secs = Math.max(1, (now - (VC_PG.lastReport || now)) / 1000)
    console.info('[Vibe Club] Прогрузка ' + job.dim + ': ' + vcPgPercent(job) + '% (' + vcPgDoneCount(job) + '/' + vcPgTotal(job.R) +
      ' чанков), ' + (VC_PG.doneSinceReport / secs).toFixed(1) + ' чанк/с, одновременно ' + VC_PG.slots + ', ' + mspt.toFixed(1) + ' мс/тик')
    VC_PG.lastReport = now
    VC_PG.doneSinceReport = 0
  }

  if (job.ring > job.R && VC_PG.inflight.length == 0) {
    console.info('[Vibe Club] Прогрузка ' + job.dim + ' завершена: ' + vcPgTotal(job.R) + ' чанков.')
    server.runCommandSilent('tellraw @a[permissions=4] {"text":"[Vibe Club] Прогрузка ' + job.dim + ' завершена.","color":"green"}')
    VC_PG.job = null
    vcPgSave(server)
  }
}

ServerEvents.loaded(function (event) {
  if (VC_PG.ok && !VC_PG.job) { VC_PG.checked = true; vcPgLoad(event.server) }
})

ServerEvents.tick(function (event) {
  if (!VC_PG.ok) return
  // после /reload событие loaded не приходит — подхватываем сохранённое задание на первом тике
  if (!VC_PG.checked) { VC_PG.checked = true; if (!VC_PG.job) vcPgLoad(event.server) }
  if (!VC_PG.job) return
  VC_PG.tick++
  try {
    vcPgStep(event.server)
  } catch (e) {
    console.error('[Vibe Club] Прогрузка мира остановлена из-за ошибки: ' + e)
    VC_PG.job.paused = true
    vcPgSave(event.server)
  }
})

function vcPgMsg(src, text) { src.sendSystemMessage(Text.yellow('[Прогрузка] ' + text)) }

ServerEvents.commandRegistry(function (event) {
  if (!VC_PG.ok) return
  var C = event.commands
  var I = VC_PG.IntArg
  var op = function (src) { return src.hasPermission(4) }
  event.register(C.literal('vc_pregen').requires(op)
    .then(C.literal('start').then(C.argument('dim', VC_PG.IdArg.id()).then(C.argument('x', I.integer())
      .then(C.argument('z', I.integer()).then(C.argument('radius', I.integer(16, 30000)).executes(function (ctx) {
        var src = ctx.source
        var dim = String(VC_PG.IdArg.getId(ctx, 'dim'))
        if (!vcPgLevel(src.server, dim)) { vcPgMsg(src, 'Нет мира ' + dim); return 0 }
        var r = I.getInteger(ctx, 'radius')
        VC_PG.inflight = []
        VC_PG.job = { dim: dim, cx: I.getInteger(ctx, 'x') >> 4, cz: I.getInteger(ctx, 'z') >> 4,
          R: Math.ceil(r / 16), ring: 0, k: 0, paused: false }
        vcPgSave(src.server)
        vcPgMsg(src, 'Старт: ' + dim + ', квадрат ' + (2 * r) + '×' + (2 * r) + ' блоков, ' + vcPgTotal(VC_PG.job.R) + ' чанков.')
        return 1
      }))))))
    .then(C.literal('status').executes(function (ctx) {
      var j = VC_PG.job
      vcPgMsg(ctx.source, j ? (j.dim + ': ' + vcPgPercent(j) + '%' + (j.paused ? ' (пауза)' : '') + ', одновременно ' + VC_PG.slots) : 'Ничего не прогружается.')
      return 1
    }))
    .then(C.literal('pause').executes(function (ctx) {
      if (VC_PG.job) { VC_PG.job.paused = true; vcPgSave(ctx.source.server) }
      vcPgMsg(ctx.source, 'Пауза.')
      return 1
    }))
    .then(C.literal('resume').executes(function (ctx) {
      if (VC_PG.job) { VC_PG.job.paused = false; vcPgSave(ctx.source.server) }
      vcPgMsg(ctx.source, 'Продолжаю.')
      return 1
    }))
    .then(C.literal('stop').executes(function (ctx) {
      VC_PG.job = null
      vcPgSave(ctx.source.server)
      vcPgMsg(ctx.source, 'Остановлено.')
      return 1
    })))
})
