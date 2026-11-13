// Vibe Club — самопроверка «Теста» на живом игроке, НИЧЕГО НЕ МЕНЯЯ: прогоняет те же функции
// сохранения и восстановления, что и vibeclub_test.js, но восстанавливает в КОПИИ (новый объект
// инвентаря), и сравнивает число предметов. Только для операторов и консоли.
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
      function check(a, b) { if (a != b) ok = false; return a + ' → ' + b + (a == b ? ' ✔' : ' ✘') }
      var Inventory = Java.loadClass('net.minecraft.world.entity.player.Inventory')
      var reg = pl.registryAccess()
      function count(c) { var n = 0; for (var i = 0; i < c.getContainerSize(); i++) if (!c.getItem(i).isEmpty()) n++; return n }
      step('скрипт «Теста» виден', function () { if (typeof vcTestSaveContainer !== 'function') throw 'нет'; return 'да' })
      step('измерение', function () { var lv = (typeof pl.level === 'function') ? pl.level() : pl.level; return vcTestDimId(lv) })
      step('NeoForge PlayerPersisted', function () { return pl.forgePersistentData ? 'есть' : 'нет (KubeJS persistentData)' })
      step('хранилище', function () { var d = vcTestData(pl); return 'ок, active=' + d.getBoolean('active') + ', ключ ' + (pl.uuid ? String(pl.uuid) : '—') })
      step('режим', function () { return vcTestMode(pl) })
      step('инвентарь: сохранить → вернуть в копию', function () {
        var t = vcTestSaveContainer(pl.getInventory(), reg)
        var copy = new Inventory(pl)
        vcTestLoadContainer(copy, t, reg)
        return check(count(pl.getInventory()), count(copy))
      })
      step('эндер-сундук: сохранить → вернуть в копию', function () {
        var t = vcTestSaveContainer(pl.getEnderChestInventory(), reg)
        var copy = new Inventory(pl)
        vcTestLoadContainer(copy, t, reg)
        return check(count(pl.getEnderChestInventory()), count(copy))
      })
      step('Curios: сохранить', function () {
        var cur = vcTestCurios(pl)
        if (!cur) return 'нет инвентаря Curios'
        var n = 0
        vcTestCuriosEach(cur, function (prefix, stacks) { for (var i = 0; i < stacks.getSlots(); i++) if (!stacks.getStackInSlot(i).isEmpty()) n++ })
        return check(n, vcTestCount(vcTestSaveCurios(cur, reg)))
      })
      step('эффекты: сохранить → загрузить', function () {
        var a = 0, b = 0
        vcTestEach(pl.getActiveEffects(), function (x) { a++; if (VC_TEST.Effect.load(x.save())) b++ })
        return check(a, b)
      })
      step('опыт', function () { return pl.experienceLevel + ' ур. + ' + pl.experienceProgress })
      step('/mine данные', function () { return typeof vcMineData === 'function' ? (vcMineData(pl) ? 'ок' : 'нет') : 'скрипт не виден' })
      console.info('[Vibe Club] Самопроверка ' + pl.username + (ok ? ' — ВСЁ ОК' : ' — ЕСТЬ ОШИБКИ') + ':\n  ' + out.join('\n  '))
      ctx.source.sendSystemMessage(Text.of((ok ? 'Самопроверка: всё ок' : 'Самопроверка: есть ошибки') + ' (подробности в логе)'))
      return ok ? 1 : 0
    })))
})
