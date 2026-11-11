// Vibe Club — команда /gift <код>: выдаёт подарки обновлений один раз на игрока.
// Сама выдача описана в датапаке «Vibe Club - DLC2 DnD» (function vibeclub:gift/<код>).
const VC_GIFTS = {
  dlc2: 'Классы и Расы: Сфера Судьбы, Книга игрока и набор костей D&D'
}

ServerEvents.commandRegistry(event => {
  const { commands: Commands } = event

  const giveGift = (ctx, code) => {
    const source = ctx.source
    const player = source.player
    if (!player) {
      source.sendFailure(Text.red('Эту команду может использовать только игрок.'))
      return 0
    }
    source.server.runCommandSilent(`execute as ${player.getStringUUID()} at @s run function vibeclub:gift/${code}`)
    return 1
  }

  let gift = Commands.literal('gift').executes(ctx => {
    ctx.source.sendSystemMessage(Text.gold('Доступные подарки:'))
    Object.keys(VC_GIFTS).forEach(code => {
      ctx.source.sendSystemMessage(Text.yellow(`/gift ${code}`).append(Text.gray(` — ${VC_GIFTS[code]}`)))
    })
    return 1
  })
  Object.keys(VC_GIFTS).forEach(code => {
    gift = gift.then(Commands.literal(code).executes(ctx => giveGift(ctx, code)))
  })
  event.register(gift)
})
