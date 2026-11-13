// Vibe Club — DLC 3 «Мир шахтёра»: команды /mine и /mines (русские буквы в командах Minecraft не работают).
// Из любого мира /mine переносит в мир шахт (vibeclub:mines, датапак «Vibe Club - DLC3 Mines»),
// а из мира шахт — обратно туда, откуда игрок пришёл. В мире шахт игрок появляется на поверхности:
// там, где был в прошлый раз, а в первый раз — рядом со своими координатами, на суше (не в воде).
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_MINE = { dim: 'vibeclub:mines', ok: false, HM: null, BlockPos: null, Tag: null, cooldown: {} }

try {
  VC_MINE.HM = Java.loadClass('net.minecraft.world.level.levelgen.Heightmap$Types').MOTION_BLOCKING_NO_LEAVES
  VC_MINE.BlockPos = Java.loadClass('net.minecraft.core.BlockPos')
  VC_MINE.Tag = Java.loadClass('net.minecraft.nbt.CompoundTag')
  VC_MINE.ok = true
} catch (e) {
  console.error('[Vibe Club] /mine выключена: ' + e)
}

// В KubeJS level.dimension — это свойство (а не метод), и в разных версиях оно отдаёт либо
// "minecraft:overworld", либо "ResourceKey[minecraft:dimension / minecraft:overworld]".
function vcDimId(level) {
  var s = String(level.dimension)
  var m = s.match(/([a-z0-9_.-]+:[a-z0-9_.\/-]+)\]?$/)
  return m ? m[1] : s
}

function vcFindLevel(server, id) {
  var it = server.getAllLevels().iterator()
  while (it.hasNext()) {
    var l = it.next()
    if (vcDimId(l) == id) return l
  }
  return null
}

// Данные, которые переживают смерть игрока (NeoForge копирует PlayerPersisted при возрождении).
function vcMineData(pl) {
  var root = pl.persistentData
  if (!root.contains('PlayerPersisted')) root.put('PlayerPersisted', new VC_MINE.Tag())
  var pp = root.getCompound('PlayerPersisted')
  if (!pp.contains('vc_mine')) pp.put('vc_mine', new VC_MINE.Tag())
  return pp.getCompound('vc_mine')
}

function vcSavePos(tag, prefix, dim, x, y, z) {
  tag.putString(prefix + 'dim', dim)
  tag.putDouble(prefix + 'x', x)
  tag.putDouble(prefix + 'y', y)
  tag.putDouble(prefix + 'z', z)
}

// Поверхность без воды и лавы рядом с (x, z): ищем по спирали шагом 32 блока (до 33 точек —
// каждая новая точка может сгенерировать чанк, поэтому их немного).
function vcSafeSurface(level, x, z) {
  var offs = [[0, 0]]
  for (var r = 32; r <= 128; r += 32) {
    offs.push([r, 0], [-r, 0], [0, r], [0, -r], [r, r], [-r, -r], [r, -r], [-r, r])
  }
  for (var i = 0; i < offs.length; i++) {
    var px = Math.floor(x) + offs[i][0]
    var pz = Math.floor(z) + offs[i][1]
    // level.getHeight() для непрогруженного чанка отдаёт дно мира, поэтому сначала
    // прогружаем (при необходимости — генерируем) чанк и берём высоту из него самого.
    var chunk = level.getChunk(px >> 4, pz >> 4)
    var y = chunk.getHeight(VC_MINE.HM, px & 15, pz & 15) + 1
    if (y < -50 || y > 310) continue
    var below = level.getBlockState(new VC_MINE.BlockPos(px, y - 1, pz))
    if (!below.getFluidState().isEmpty()) continue
    if (below.isAir()) continue
    return { x: px + 0.5, y: y, z: pz + 0.5 }
  }
  return null
}

function vcTp(server, name, dim, x, y, z) {
  server.runCommandSilent('execute in ' + dim + ' run tp ' + name + ' ' + x.toFixed(2) + ' ' + y.toFixed(2) + ' ' + z.toFixed(2))
}

function vcMine(ctx) {
  var src = ctx.source
  var pl = src.player
  if (!pl) {
    src.sendFailure(Text.red('Эту команду может использовать только игрок.'))
    return 0
  }
  var name = String(pl.username)
  if (!/^[A-Za-z0-9_]{1,16}$/.test(name)) {
    src.sendFailure(Text.red('Не получилось. Напиши администратору сервера.'))
    return 0
  }
  var server = src.server
  var now = Date.now()
  var last = VC_MINE.cooldown[name] || 0
  if (now - last < 10000) {
    src.sendFailure(Text.red('Подожди ' + Math.ceil((10000 - (now - last)) / 1000) + ' с перед следующим переходом.'))
    return 0
  }
  var mines = vcFindLevel(server, VC_MINE.dim)
  if (!mines) {
    src.sendFailure(Text.red('Мир шахтёра не загружен на сервере (нет датапака DLC 3).'))
    return 0
  }
  VC_MINE.cooldown[name] = now
  var data = vcMineData(pl)
  var here = vcDimId(src.level)

  if (here == VC_MINE.dim) {
    // обратно
    vcSavePos(data, 'm_', here, pl.x, pl.y, pl.z)
    var dim = data.contains('r_dim') ? data.getString('r_dim') : 'minecraft:overworld'
    if (data.contains('r_dim') && vcFindLevel(server, dim)) {
      vcTp(server, name, dim, data.getDouble('r_x'), data.getDouble('r_y'), data.getDouble('r_z'))
    } else {
      var ow = vcFindLevel(server, 'minecraft:overworld')
      var sp = ow.getSharedSpawnPos()
      vcTp(server, name, 'minecraft:overworld', sp.getX() + 0.5, sp.getY() + 1, sp.getZ() + 0.5)
    }
    src.sendSystemMessage(Text.yellow('Ты вернулся из мира шахтёра.'))
    return 1
  }

  // туда
  vcSavePos(data, 'r_', here, pl.x, pl.y, pl.z)
  var target = null
  if (data.contains('m_dim')) {
    target = vcSafeSurface(mines, data.getDouble('m_x'), data.getDouble('m_z'))
    if (target && data.getDouble('m_y') < target.y) {
      // в прошлый раз игрок ушёл из-под земли — возвращаем туда же, в свою шахту
      target = { x: data.getDouble('m_x'), y: data.getDouble('m_y'), z: data.getDouble('m_z') }
    }
  }
  if (!target) target = vcSafeSurface(mines, pl.x, pl.z)
  if (!target) target = vcSafeSurface(mines, 0, 0)
  if (!target) {
    src.sendFailure(Text.red('Не нашёл безопасного места в мире шахтёра, попробуй ещё раз.'))
    return 0
  }
  vcTp(server, name, VC_MINE.dim, target.x, target.y, target.z)
  server.runCommandSilent('effect give ' + name + ' minecraft:resistance 5 4 true')
  server.runCommandSilent('effect give ' + name + ' minecraft:slow_falling 5 0 true')
  src.sendSystemMessage(Text.gold('⛏ Мир шахтёра. ').append(Text.yellow('Здесь бесконечные шахты, руды и постройки. /mine — обратно.')))
  console.info('[Vibe Club] ' + name + ' ушёл в мир шахтёра: ' + Math.round(target.x) + ', ' + Math.round(target.y) + ', ' + Math.round(target.z))
  return 1
}

ServerEvents.commandRegistry(function (event) {
  if (!VC_MINE.ok) return
  var Commands = event.commands
  event.register(Commands.literal('mine').executes(function (ctx) { return vcMine(ctx) }))
  event.register(Commands.literal('mines').executes(function (ctx) { return vcMine(ctx) }))
})
