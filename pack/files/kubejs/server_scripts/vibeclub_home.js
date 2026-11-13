// Vibe Club — /vibeclub: вернуться в мир летающих островов (minecraft:overworld) из мира шахтёра
// (vibeclub:mines) или из «Теста» (vibeclub:test) — туда, откуда игрок ушёл; если такого места нет —
// к кровати, а без кровати — к точке спавна мира. Место ухода запоминается, поэтому /mine и /test
// потом вернут туда же. Из Незера и Энда команда не работает (это была бы бесплатная телепортация домой).
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_HOME = { ok: false, Tag: null, cooldown: {}, worlds: { 'vibeclub:mines': 'vc_mine', 'vibeclub:test': 'vc_test' } }

try {
  VC_HOME.Tag = Java.loadClass('net.minecraft.nbt.CompoundTag')
  VC_HOME.ok = true
} catch (e) {
  console.error('[Vibe Club] /vibeclub выключена: ' + e)
}

function vcHomeDimId(level) {
  var s = String(level.dimension)
  var m = s.match(/([a-z0-9_.-]+:[a-z0-9_.\/-]+)\]?$/)
  return m ? m[1] : s
}

function vcHomeLevel(server, id) {
  var it = server.getAllLevels().iterator()
  while (it.hasNext()) {
    var l = it.next()
    if (vcHomeDimId(l) == id) return l
  }
  return null
}

function vcHomeData(pl, key) {
  var root = pl.persistentData
  if (!root.contains('PlayerPersisted')) root.put('PlayerPersisted', new VC_HOME.Tag())
  var pp = root.getCompound('PlayerPersisted')
  if (!pp.contains(key)) pp.put(key, new VC_HOME.Tag())
  return pp.getCompound(key)
}

function vcHome(ctx) {
  var src = ctx.source
  var pl = src.player
  if (!pl) return 0
  var name = String(pl.username)
  if (!/^[A-Za-z0-9_]{1,16}$/.test(name)) return 0
  var server = src.server
  var here = vcHomeDimId(src.level)
  if (here == 'minecraft:overworld') {
    src.sendFailure(Text.yellow('Ты уже в мире летающих островов.'))
    return 0
  }
  var key = VC_HOME.worlds[here]
  if (!key) {
    src.sendFailure(Text.red('/vibeclub работает из мира шахтёра и из «Теста». Отсюда добирайся своим ходом.'))
    return 0
  }
  var now = Date.now()
  if (now - (VC_HOME.cooldown[name] || 0) < 5000) {
    src.sendFailure(Text.red('Подожди пару секунд.'))
    return 0
  }
  VC_HOME.cooldown[name] = now

  var d = vcHomeData(pl, key)
  // запоминаем, где игрок был, чтобы /mine и /test вернули его сюда же
  var p = key == 'vc_mine' ? 'm_' : 't_'
  d.putString(p + 'dim', here); d.putDouble(p + 'x', pl.x); d.putDouble(p + 'y', pl.y); d.putDouble(p + 'z', pl.z)

  // из «Теста» — сначала вернуть обычные вещи (сторож в vibeclub_test.js сделал бы это через тик)
  if (key == 'vc_test' && d.getBoolean('active') && typeof vcTestLeave === 'function') vcTestLeave(server, pl)

  var t = null
  if (d.contains('r_dim') && String(d.getString('r_dim')) == 'minecraft:overworld') {
    t = { x: d.getDouble('r_x'), y: d.getDouble('r_y'), z: d.getDouble('r_z') }
  } else {
    var bed = pl.getRespawnPosition()
    if (bed && String(pl.getRespawnDimension().location()) == 'minecraft:overworld') {
      t = { x: bed.getX() + 0.5, y: bed.getY() + 0.6, z: bed.getZ() + 0.5 }
    } else {
      var sp = vcHomeLevel(server, 'minecraft:overworld').getSharedSpawnPos()
      t = { x: sp.getX() + 0.5, y: sp.getY() + 1, z: sp.getZ() + 0.5 }
    }
  }
  server.runCommandSilent('execute in minecraft:overworld run tp ' + name + ' ' + Number(t.x).toFixed(2) + ' ' + Number(t.y).toFixed(2) + ' ' + Number(t.z).toFixed(2))
  src.sendSystemMessage(Text.gold('☁ С возвращением в мир летающих островов Vibe Club!'))
  return 1
}

ServerEvents.commandRegistry(function (event) {
  if (!VC_HOME.ok) return
  event.register(event.commands.literal('vibeclub').executes(function (ctx) { return vcHome(ctx) }))
})
