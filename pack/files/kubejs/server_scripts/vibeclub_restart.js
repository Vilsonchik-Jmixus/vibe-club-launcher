// Vibe Club — предупреждения перед перезапуском сервера.
// Сам перезапуск делает расписание в панели хостинга (каждые 3 часа): за 5 минут оно выполняет
// «vc_restart 300», а через 300 секунд перезапускает сервер. Этот скрипт показывает игрокам
// таймер (полоска сверху экрана), предупреждает за 5 минут и за 1 минуту, считает последние
// 10 секунд крупными цифрами и сохраняет мир перед перезапуском.
//
//   /vc_restart <секунды>  — начать отсчёт (только операторы и консоль)
//   /vc_restart cancel     — отменить отсчёт (сам перезапуск в панели этим не отменяется)
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_RS = { end: 0, total: 0, said: {}, bar: 'vibeclub:restart', IntArg: null }

try {
  VC_RS.IntArg = Java.loadClass('com.mojang.brigadier.arguments.IntegerArgumentType')
} catch (e) {
  console.error('[Vibe Club] Отсчёт перезапуска выключен: ' + e)
}

function vcRsFmt(s) {
  var m = Math.floor(s / 60)
  var r = s % 60
  return m + ':' + (r < 10 ? '0' : '') + r
}

function vcRsCmd(server, c) { server.runCommandSilent(c) }

function vcRsAnnounce(server, text, sound) {
  vcRsCmd(server, 'tellraw @a ["",{"text":"⚠ ","color":"gold"},{"text":"' + text + '","color":"yellow","bold":true}]')
  vcRsCmd(server, 'title @a subtitle {"text":"Сохрани вещи и найди безопасное место","color":"yellow"}')
  vcRsCmd(server, 'title @a title {"text":"' + text + '","color":"gold","bold":true}')
  vcRsCmd(server, 'execute as @a at @s run playsound ' + (sound || 'minecraft:block.note_block.bell') + ' master @s ~ ~ ~ 1 1')
}

function vcRsStart(server, secs) {
  VC_RS.total = secs
  VC_RS.end = Date.now() + secs * 1000
  VC_RS.said = {}
  vcRsCmd(server, 'bossbar remove ' + VC_RS.bar)
  vcRsCmd(server, 'bossbar add ' + VC_RS.bar + ' {"text":"Перезапуск сервера"}')
  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' color red')
  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' style notched_10')
  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' max ' + secs)
  console.info('[Vibe Club] Отсчёт до перезапуска: ' + secs + ' с')
}

function vcRsStop(server) {
  VC_RS.end = 0
  vcRsCmd(server, 'bossbar remove ' + VC_RS.bar)
}

ServerEvents.tick(function (event) {
  if (!VC_RS.end) return
  var server = event.server
  var left = Math.max(0, Math.ceil((VC_RS.end - Date.now()) / 1000))
  if (VC_RS.said['bar' + left]) return
  VC_RS.said['bar' + left] = true

  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' players @a')
  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' value ' + left)
  vcRsCmd(server, 'bossbar set ' + VC_RS.bar + ' name {"text":"Перезапуск сервера через ' + vcRsFmt(left) + '","color":"' + (left <= 60 ? 'red' : 'yellow') + '"}')

  if (!VC_RS.said.five && VC_RS.total >= 300 && left <= 300 && left > 60) {
    VC_RS.said.five = true
    vcRsAnnounce(server, 'Перезапуск сервера через 5 минут')
  }
  if (left == 60) vcRsAnnounce(server, 'Перезапуск сервера через 1 минуту', 'minecraft:block.note_block.pling')
  if (left == 30) vcRsCmd(server, 'tellraw @a {"text":"⚠ Перезапуск через 30 секунд","color":"red","bold":true}')
  if (left <= 10 && left >= 1) {
    vcRsCmd(server, 'title @a times 0 25 5')
    vcRsCmd(server, 'title @a subtitle {"text":"перезапуск сервера","color":"yellow"}')
    vcRsCmd(server, 'title @a title {"text":"' + left + '","color":"red","bold":true}')
    vcRsCmd(server, 'execute as @a at @s run playsound minecraft:block.note_block.hat master @s ~ ~ ~ 1 ' + (left <= 3 ? '1.5' : '1'))
  }
  if (left == 5) vcRsCmd(server, 'save-all')
  if (left == 0) {
    vcRsCmd(server, 'title @a times 10 70 20')
    vcRsCmd(server, 'tellraw @a {"text":"Сервер перезапускается. Заходи через пару минут!","color":"gold","bold":true}')
    vcRsStop(server)
  }
})

ServerEvents.commandRegistry(function (event) {
  if (!VC_RS.IntArg) return
  var C = event.commands
  var I = VC_RS.IntArg
  event.register(C.literal('vc_restart').requires(function (src) { return src.hasPermission(4) })
    .then(C.literal('cancel').executes(function (ctx) {
      vcRsStop(ctx.source.server)
      vcRsCmd(ctx.source.server, 'tellraw @a {"text":"Перезапуск сервера отменён.","color":"green"}')
      return 1
    }))
    .then(C.argument('seconds', I.integer(5, 3600)).executes(function (ctx) {
      vcRsStart(ctx.source.server, I.getInteger(ctx, 'seconds'))
      return 1
    })))
})
