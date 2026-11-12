// Vibe Club — постройки Create Aeronautics (Sable), упавшие в бездну, возвращаются и падают сверху,
// как игроки с Forgiving Void. Раз в секунду сервер проверяет все загруженные постройки:
// если постройка ниже порога, она переносится на ту же X/Z над миром, скорость обнуляется.
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

function vcSableRescue(ctx, dim) {
  var cfg = VC_SV_DIMS[dim]
  var subs = VC_SV.Arg.getSubLevels(ctx, 'targets')
  var sys = VC_SV.Sys.get(ctx.source.level)
  if (!sys) return 0
  var pipe = sys.getPipeline()
  var moved = 0
  var it = subs.iterator()
  while (it.hasNext()) {
    var sub = it.next()
    if (sub.isRemoved()) continue
    var pose = sub.logicalPose()
    var pos = pose.position()
    var y = pos.get(1)
    if (y > cfg.below) continue
    var x = pos.get(0)
    var z = pos.get(2)
    var dropY = cfg.drop
    if (VC_SV.HM) {
      try {
        var ground = ctx.source.level.getHeight(VC_SV.HM, Math.floor(x), Math.floor(z))
        if (ground > -60) dropY = ground + cfg.lift
      } catch (e3) { }
    }
    pipe.resetVelocity(sub)
    pipe.teleport(sub, new VC_SV.Vec(x, dropY, z), pose.orientation())
    pipe.resetVelocity(sub)
    moved++
    var name = sub.getName()
    console.info('[Vibe Club] Постройка ' + (name ? '«' + name + '» ' : '') + 'вернулась из бездны: X ' + Math.round(x) + ', Y ' + Math.round(dropY) + ', Z ' + Math.round(z))
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
