// Vibe Club — защита мира «Тест» (vibeclub:test): из креативного мира нельзя вынести вещи.
// 1) В «Тесте» не открываются порталы в Незер.
// 2) Из «Теста» в другие миры не перемещаются никакие сущности, кроме игроков (предметы,
//    вагонетки, мобы, лодки с сундуками и т. п.). Игроки — можно: при выходе их креативные вещи
//    удаляет серверный скрипт vibeclub_test.js.
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
function vcGuardDim(level) {
  try {
    var s = String(level.dimension)
    var m = s.match(/([a-z0-9_.-]+:[a-z0-9_.\/-]+)\]?$/)
    return m ? m[1] : s
  } catch (e) {
    return ''
  }
}

try {
  NativeEvents.onEvent('net.neoforged.neoforge.event.level.BlockEvent$PortalSpawnEvent', function (e) {
    if (vcGuardDim(e.getLevel()) == 'vibeclub:test') e.setCanceled(true)
  })
  NativeEvents.onEvent('net.neoforged.neoforge.event.entity.EntityTravelToDimensionEvent', function (e) {
    var ent = e.getEntity()
    if (ent.isPlayer()) return
    var lv = (typeof ent.level === 'function') ? ent.level() : ent.level
    if (vcGuardDim(lv) == 'vibeclub:test' && String(e.getDimension().location()) != 'vibeclub:test') e.setCanceled(true)
  })
} catch (e) {
  console.warn('[Vibe Club] Защита мира «Тест» не включилась: ' + e)
}
