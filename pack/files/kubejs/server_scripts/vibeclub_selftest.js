// Vibe Club — самопроверка скриптов «Теста», /mine и /vibeclub на живом игроке, НИЧЕГО НЕ МЕНЯЯ:
// сохраняет инвентарь, эндер-сундук, Curios, эффекты в NBT и загружает их в КОПИИ (новые объекты),
// сравнивает число предметов. Только для операторов и консоли.
//   /vc_selftest <ник>
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
ServerEvents.commandRegistry(function (event) {
  var C = event.commands
  var EntityArgument = Java.loadClass('net.minecraft.commands.arguments.EntityArgument')
  event.register(C.literal('vc_selftest').requires(function (src) { return src.hasPermission(4) })
    .then(C.argument('player', EntityArgument.player()).executes(function (ctx) {
      var pl = EntityArgument.getPlayer(ctx, 'player')
      var out = []
      var ok = true
      function step(name, fn) {
        try { out.push(name + ': ' + fn()) } catch (e) { ok = false; out.push(name + ': ОШИБКА ' + e) }
      }
      var ListTag = Java.loadClass('net.minecraft.nbt.ListTag')
      var Inventory = Java.loadClass('net.minecraft.world.entity.player.Inventory')
      var Simple = Java.loadClass('net.minecraft.world.SimpleContainer')
      var Effect = Java.loadClass('net.minecraft.world.effect.MobEffectInstance')
      var reg = pl.registryAccess()
      function count(c) { var n = 0; for (var i = 0; i < c.getContainerSize(); i++) if (!c.getItem(i).isEmpty()) n++; return n }
      step('измерение', function () { var lv = (typeof pl.level === 'function') ? pl.level() : pl.level; return String(lv.dimension) })
      step('KubeJS level.dimension', function () { return String(ctx.source.level.dimension) })
      step('NeoForge PlayerPersisted', function () { return pl.forgePersistentData ? 'есть' : 'НЕТ (будет KubeJS persistentData)' })
      step('данные сервера', function () { return pl.getServer().persistentData ? 'есть' : 'НЕТ' })
      step('UUID', function () { return String(pl.uuid) })
      step('режим', function () { return (pl.isCreative() ? 'creative' : pl.isSpectator() ? 'spectator' : pl.getAbilities().mayBuild ? 'survival' : 'adventure') })
      step('инвентарь → NBT → копия', function () {
        var l = new ListTag(); pl.getInventory().save(l)
        var copy = new Inventory(pl); copy.load(l)
        var a = count(pl.getInventory()), b = count(copy)
        if (a != b) ok = false
        return a + ' → ' + b + (a == b ? ' ✔' : ' ✘')
      })
      step('эндер-сундук → NBT → копия', function () {
        var t = pl.getEnderChestInventory().createTag(reg)
        var copy = new Simple(27); copy.fromTag(t, reg)
        var a = count(pl.getEnderChestInventory()), b = count(copy)
        if (a != b) ok = false
        return a + ' → ' + b + (a == b ? ' ✔' : ' ✘')
      })
      step('Curios', function () {
        var opt = Java.loadClass('top.theillusivec4.curios.api.CuriosApi').getCuriosInventory(pl)
        if (!opt.isPresent()) return 'нет инвентаря Curios'
        var h = opt.get()
        return 'saveInventory(false): ' + h.saveInventory(false).size() + ' групп слотов, loadInventory: ' + (typeof h.loadInventory)
      })
      step('эффекты → NBT → обратно', function () {
        var n = 0, m = 0, it = pl.getActiveEffects().iterator()
        while (it.hasNext()) { n++; if (Effect.load(it.next().save())) m++ }
        return n + ' → ' + m
      })
      step('опыт', function () { return pl.experienceLevel + ' ур. + ' + pl.experienceProgress })
      step('/mine данные', function () { return typeof vcMineData === 'function' ? 'ок' : 'скрипт не виден' })
      step('/test данные', function () { return typeof vcTestData === 'function' ? ('ок, active=' + vcTestData(pl).getBoolean('active')) : 'скрипт не виден' })
      console.info('[Vibe Club] Самопроверка ' + pl.username + (ok ? ' — ВСЁ ОК' : ' — ЕСТЬ ОШИБКИ') + ':\n  ' + out.join('\n  '))
      ctx.source.sendSystemMessage(Text.of((ok ? 'Самопроверка: всё ок' : 'Самопроверка: есть ошибки') + ' (подробности в логе)'))
      return ok ? 1 : 0
    })))
})
