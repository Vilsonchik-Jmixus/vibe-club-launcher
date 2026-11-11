// Vibe Club — команда /gift <код>: выдаёт подарки обновлений один раз на игрока.
// Сама выдача описана в датапаке «Vibe Club - DLC2 DnD» (function vibeclub:gift/<код>).
// Написано в стиле ES5 (var, function): Rhino в KubeJS плохо переносит const/let внутри блоков.
var VC_GIFTS = {
  dlc2: 'Классы и Расы: Сфера Судьбы, Книга игрока и набор костей D&D'
}

function vcGiveGift(ctx, code) {
  var src = ctx.source
  var pl = src.player
  if (!pl) {
    src.sendFailure(Text.red('Эту команду может использовать только игрок.'))
    return 0
  }
  var nm = String(pl.username)
  if (!/^[A-Za-z0-9_]{1,16}$/.test(nm)) {
    console.error('[Vibe Club] /gift: странный ник ' + nm)
    src.sendFailure(Text.red('Не получилось выдать подарок. Напиши администратору сервера.'))
    return 0
  }
  src.server.runCommandSilent('execute as ' + nm + ' at @s run function vibeclub:gift/' + code)
  return 1
}

function vcGiftList(ctx) {
  ctx.source.sendSystemMessage(Text.gold('Доступные подарки:'))
  Object.keys(VC_GIFTS).forEach(function (code) {
    ctx.source.sendSystemMessage(Text.yellow('/gift ' + code).append(Text.gray(' — ' + VC_GIFTS[code])))
  })
  return 1
}

ServerEvents.commandRegistry(function (event) {
  var Commands = event.commands
  var gift = Commands.literal('gift').executes(function (ctx) { return vcGiftList(ctx) })
  Object.keys(VC_GIFTS).forEach(function (code) {
    gift = gift.then(Commands.literal(code).executes(function (ctx) { return vcGiveGift(ctx, code) }))
  })
  event.register(gift)
})
