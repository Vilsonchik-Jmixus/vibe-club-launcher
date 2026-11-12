// Vibe Club — постройки Create Aeronautics (Sable), упавшие в бездну, возвращаются и падают сверху,
// как игроки с Forgiving Void. Раз в секунду сервер проверяет все загруженные постройки:
// если постройка ниже порога, она появляется в 30 блоках над землёй — на той же X/Z, а если там
// пустота (небесные биомы SkyLands), то над последним местом, где под ней была земля. Скорость обнуляется.
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_SV = { ok: false, tick: 0, Arg: null, Sys: null, Vec: null, HM: null }
// below — порог по Y; drop — запасная высота, если землю под постройкой не видно;
// lift — на сколько блоков над землёй постройка появляется (невысоко, чтобы не разогналась
// и не провалилась сквозь блоки на большой скорости).
var VC_SV_DIMS = {
  overworld: { id: 'minecraft:overworld', below: -150, drop: 330, lift: 30 },
  end: { id: 'minecraft:the_end', below: -100, drop: 120, lift: 30 }
}

try {
  VC_SV.Arg = Java.loadClass('dev.ryanhcode.sable.api.command.SubLevelArgumentType')
  VC_SV.Sys = Java.loadClass('dev.ryanhcode.sable.sublevel.system.SubLevelPhysicsSystem')
  VC_SV.Vec = Java.loadClass('org.joml.Vector3d')
  VC_SV.ok = true
  try {
    VC_SV.HM = Java.loadClass('net.minecraft.world.level.levelgen.Heightmap$Types').MOTION_BLOCKING_NO_LEAVES
  } catch (e2) {
    console.warn('[Vibe Club] Карта высот недоступна, постройки будут падать с высоты по умолчанию: ' + e2)
  }
} catch (e) {
  console.error('[Vibe Club] Возврат построек из бездны выключен: ' + e)
}

// Последняя «безопасная» точка каждой постройки (как returnToLastGrounded у Forgiving Void):
// запоминаем позицию, когда под постройкой есть земля. Ключ — UUID постройки.
var VC_SV_SAFE = {}

function vcGroundAt(level, x, z) {
  if (!VC_SV.HM) return null
  try {
    var g = level.getHeight(VC_SV.HM, Math.floor(x), Math.floor(z))
    return g > -60 ? g : null
  } catch (e) {
    return null
  }
}

function vcSableRescue(ctx, dim) {
  var cfg = VC_SV_DIMS[dim]
  var level = ctx.source.level
  var subs = VC_SV.Arg.getSubLevels(ctx, 'targets')
  var sys = VC_SV.Sys.get(level)
  if (!sys) return 0
  var pipe = sys.getPipeline()
  var moved = 0
  var it = subs.iterator()
  while (it.hasNext()) {
    var sub = it.next()
    if (sub.isRemoved()) continue
    var id = dim + ':' + String(sub.getUniqueId())
    var pose = sub.logicalPose()
    var pos = pose.position()
    var x = pos.get(0)
    var y = pos.get(1)
    var z = pos.get(2)
    if (y > cfg.below) {
      if (y > 0) {
        var g = vcGroundAt(level, x, z)
        if (g !== null) VC_SV_SAFE[id] = { x: x, z: z, ground: g }
      }
      continue
    }
    var tx = x
    var tz = z
    var ty = cfg.drop
    var safe = VC_SV_SAFE[id]
    var groundHere = vcGroundAt(level, x, z)
    if (groundHere !== null) {
      ty = groundHere + cfg.lift
    } else if (safe) {
      tx = safe.x
      tz = safe.z
      var g2 = vcGroundAt(level, tx, tz)
      ty = (g2 !== null ? g2 : safe.ground) + cfg.lift
    }
    pipe.resetVelocity(sub)
    pipe.teleport(sub, new VC_SV.Vec(tx, ty, tz), pose.orientation())
    pipe.resetVelocity(sub)
    moved++
    var name = sub.getName()
    console.info('[Vibe Club] Постройка ' + (name ? '«' + name + '» ' : '') + 'вернулась из бездны: X ' + Math.round(tx) + ', Y ' + Math.round(ty) + ', Z ' + Math.round(tz))
  }
  return moved
}

ServerEvents.commandRegistry(function (event) {
  if (!VC_SV.ok) return
  var Commands = event.commands
  var root = Commands.literal('vc_sable_void')
  Object.keys(VC_SV_DIMS).forEach(function (dim) {
    root = root.then(Commands.literal(dim).then(Commands.argument('targets', VC_SV.Arg.subLevels())
      .executes(function (ctx) { return vcSableRescue(ctx, dim) })))
  })
  event.register(root)
})

ServerEvents.tick(function (event) {
  if (!VC_SV.ok) return
  VC_SV.tick++
  if (VC_SV.tick % 20 != 0) return
  Object.keys(VC_SV_DIMS).forEach(function (dim) {
    event.server.runCommandSilent('execute in ' + VC_SV_DIMS[dim].id + ' run vc_sable_void ' + dim + ' @e')
  })
})
