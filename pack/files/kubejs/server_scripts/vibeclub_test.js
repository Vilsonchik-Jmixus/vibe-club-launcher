// Vibe Club — мир «Тест» (vibeclub:test, генератор «Размышлярий» из Create: Bits 'n' Bobs) для
// проектирования и проверки построек в креативе.
//
//   /test — из любого мира в «Тест» и обратно (туда, откуда пришёл).
//
// При входе в «Тест» обычные вещи игрока убираются на хранение: инвентарь, эндер-сундук, слоты
// Curios, опыт и эффекты; игрок получает креатив. При выходе из «Теста» ЛЮБЫМ способом (команда,
// путевой камень, смерть, выход с сервера и вход в другом мире) креативные вещи удаляются, а
// обычные возвращаются вместе с прежним режимом игры. Хранилище — PlayerPersisted.vc_test в данных
// игрока и в данных сервера (переживает смерть и перезапуск). Проверка идёт каждый тик.
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_TEST = { dim: 'vibeclub:test', ok: false, Tag: null, Stack: null, Effect: null, Curios: null, errAt: {},
  spawn: { x: 0.5, y: -48, z: 0.5 }, cooldown: {} }

try {
  VC_TEST.Tag = Java.loadClass('net.minecraft.nbt.CompoundTag')
  VC_TEST.Stack = Java.loadClass('net.minecraft.world.item.ItemStack')
  VC_TEST.Effect = Java.loadClass('net.minecraft.world.effect.MobEffectInstance')
  VC_TEST.ok = true
} catch (e) {
  console.error('[Vibe Club] Мир «Тест» выключен: ' + e)
}
try {
  VC_TEST.Curios = Java.loadClass('top.theillusivec4.curios.api.CuriosApi')
} catch (e) {
  console.warn('[Vibe Club] Curios не найден, слоты Curios в «Тесте» не переключаются: ' + e)
}

function vcTestDimId(level) {
  var s = String(level.dimension)
  var m = s.match(/([a-z0-9_.-]+:[a-z0-9_.\/-]+)\]?$/)
  return m ? m[1] : s
}

function vcTestLevel(server, id) {
  var it = server.getAllLevels().iterator()
  while (it.hasNext()) {
    var l = it.next()
    if (vcTestDimId(l) == id) return l
  }
  return null
}

// Хранилище вещей — в двух местах сразу:
//  1) основное: данные сервера (server.persistentData.vc_test.<UUID>) — не зависят от смерти игрока;
//  2) запасное: NeoForge-данные игрока, раздел PlayerPersisted (игра переносит его после смерти).
// Читаем из основного, если оно потерялось — из запасного.
function vcTestPlayerTag(pl) {
  var root = pl.forgePersistentData
  if (!root) root = pl.persistentData
  if (!root.contains('PlayerPersisted')) root.put('PlayerPersisted', new VC_TEST.Tag())
  return root.getCompound('PlayerPersisted')
}

function vcTestData(pl) {
  var srv = pl.getServer().persistentData
  if (!srv.contains('vc_test')) srv.put('vc_test', new VC_TEST.Tag())
  var all = srv.getCompound('vc_test')
  // getUUID() в KubeJS скрыт — берём свойство uuid (а если его нет — ник)
  var id = pl.uuid ? String(pl.uuid) : String(pl.username).toLowerCase()
  if (!all.contains(id)) {
    var pp = vcTestPlayerTag(pl)
    all.put(id, pp.contains('vc_test') ? pp.getCompound('vc_test').copy() : new VC_TEST.Tag())
  }
  return all.getCompound(id)
}

// Копия основного хранилища → запасное (вызывать после каждого изменения).
function vcTestSync(pl) {
  vcTestPlayerTag(pl).put('vc_test', vcTestData(pl).copy())
}

function vcTestIsActive(pl) {
  return vcTestData(pl).getBoolean('active')
}

function vcTestMode(pl) {
  if (pl.isSpectator()) return 'spectator'
  if (pl.isCreative()) return 'creative'
  if (!pl.getAbilities().mayBuild) return 'adventure'
  return 'survival'
}

function vcTestCurios(pl) {
  if (!VC_TEST.Curios) return null
  try {
    var opt = VC_TEST.Curios.getCuriosInventory(pl)
    return opt.isPresent() ? opt.get() : null
  } catch (e) {
    return null
  }
}

// ВАЖНО: KubeJS превращает ListTag в обычный список JS, и обратно в методы игры он не проходит
// («Cannot cast ArrayList to ListTag»). Поэтому вещи сохраняются по одной: каждый предмет — отдельный
// CompoundTag с ключом-номером слота.
function vcTestSaveContainer(c, reg) {
  var t = new VC_TEST.Tag()
  for (var i = 0; i < c.getContainerSize(); i++) {
    var st = c.getItem(i)
    if (!st.isEmpty()) t.put(String(i), st.save(reg))
  }
  return t
}

function vcTestLoadContainer(c, t, reg) {
  c.clearContent()
  vcTestEach(t.getAllKeys(), function (key) {
    var k = String(key)
    var st = VC_TEST.Stack.parseOptional(reg, t.getCompound(k))
    if (!st.isEmpty()) c.setItem(parseInt(k), st)
  })
}

function vcTestCount(t) { return t.getAllKeys().size() }

// Обход Java-коллекции, даже если KubeJS превратил её в массив или объект JS.
function vcTestEach(coll, fn) {
  if (coll == null) return
  if (typeof coll.iterator === 'function') {
    var it = coll.iterator()
    while (it.hasNext()) fn(it.next())
  } else if (typeof coll.length === 'number') {
    for (var i = 0; i < coll.length; i++) fn(coll[i])
  }
}

// Обход Java-словаря (ключ, значение), даже если KubeJS превратил его в объект JS.
function vcTestEachEntry(map, fn) {
  if (map == null) return
  if (typeof map.entrySet === 'function') {
    vcTestEach(map.entrySet(), function (e) { fn(String(e.getKey()), e.getValue()) })
  } else {
    for (var k in map) fn(String(k), map[k])
  }
}

// Curios: каждая группа слотов (кольца, пояс…) — обычные и косметические слоты по номерам.
function vcTestCuriosEach(cur, fn) {
  vcTestEachEntry(cur.getCurios(), function (id, h) {
    fn(id + '|s|', h.getStacks())
    fn(id + '|c|', h.getCosmeticStacks())
  })
}

function vcTestSaveCurios(cur, reg) {
  var t = new VC_TEST.Tag()
  vcTestCuriosEach(cur, function (prefix, stacks) {
    for (var i = 0; i < stacks.getSlots(); i++) {
      var st = stacks.getStackInSlot(i)
      if (!st.isEmpty()) t.put(prefix + i, st.save(reg))
    }
  })
  return t
}

function vcTestClearCurios(cur) {
  vcTestCuriosEach(cur, function (prefix, stacks) {
    for (var i = 0; i < stacks.getSlots(); i++) stacks.setStackInSlot(i, VC_TEST.Stack.EMPTY)
  })
}

function vcTestLoadCurios(cur, t, reg) {
  vcTestClearCurios(cur)
  vcTestCuriosEach(cur, function (prefix, stacks) {
    for (var i = 0; i < stacks.getSlots(); i++) {
      if (t.contains(prefix + i)) stacks.setStackInSlot(i, VC_TEST.Stack.parseOptional(reg, t.getCompound(prefix + i)))
    }
  })
}

// Обычные вещи → хранилище, игрок получает креатив. Сначала всё сохраняется, и только если
// сохранение прошло без ошибок — очищается.
function vcTestEnter(server, pl) {
  var d = vcTestData(pl)
  var name = String(pl.username)
  var reg = pl.registryAccess()
  var inv = pl.getInventory()
  var invTag = vcTestSaveContainer(inv, reg)
  var enderTag = vcTestSaveContainer(pl.getEnderChestInventory(), reg)
  var cur = vcTestCurios(pl)
  var curTag = cur ? vcTestSaveCurios(cur, reg) : null
  var eff = new VC_TEST.Tag()
  var n = 0
  vcTestEach(pl.getActiveEffects(), function (x) { eff.put(String(n++), x.save()) })
  d.put('inv', invTag)
  d.put('ender', enderTag)
  if (curTag) d.put('curios', curTag); else d.remove('curios')
  d.put('effects', eff)
  d.putString('mode', vcTestMode(pl))
  d.putInt('xpl', pl.experienceLevel)
  d.putFloat('xpp', pl.experienceProgress)
  d.putInt('xpt', pl.totalExperience)
  d.putBoolean('active', true)
  vcTestSync(pl)
  // всё сохранено (в двух местах) — теперь очищаем
  inv.clearContent()
  pl.getEnderChestInventory().clearContent()
  if (cur) vcTestClearCurios(cur)
  pl.removeAllEffects()
  pl.inventoryMenu.broadcastChanges()
  server.runCommandSilent('gamemode creative ' + name)
  pl.sendSystemMessage(Text.aqua('🧪 Мир «Тест»: креатив для проектирования. Твои вещи сохранены и вернутся, когда выйдешь. /test или /vibeclub — обратно.'))
  console.info('[Vibe Club] ' + name + ' вошёл в «Тест», вещи сохранены: инвентарь ' + vcTestCount(invTag) + ', эндер-сундук ' + vcTestCount(enderTag) + (curTag ? ', Curios ' + vcTestCount(curTag) : ''))
}

// Креативные вещи → удалить, вернуть обычные.
function vcTestLeave(server, pl) {
  var d = vcTestData(pl)
  var name = String(pl.username)
  if (d.getBoolean('broken')) return
  if (!d.contains('inv')) {
    // нечего возвращать (не должно случаться) — просто выключаем креатив
    d.putBoolean('active', false)
    vcTestSync(pl)
    server.runCommandSilent('gamemode survival ' + name)
    console.error('[Vibe Club] ' + name + ' вышел из «Теста», но сохранённых вещей нет')
    return
  }
  var reg = pl.registryAccess()
  try {
    vcTestLoadContainer(pl.getInventory(), d.getCompound('inv'), reg)
  } catch (e) {
    d.putBoolean('broken', true)
    vcTestSync(pl)
    server.runCommandSilent('gamemode survival ' + name)
    console.error('[Vibe Club] НЕ УДАЛОСЬ вернуть вещи ' + name + ' после «Теста» — они сохранены в данных сервера vc_test: ' + e)
    pl.sendSystemMessage(Text.red('Не получилось вернуть твои вещи. Они не пропали — напиши администратору.'))
    return
  }
  var errs = []
  try { vcTestLoadContainer(pl.getEnderChestInventory(), d.getCompound('ender'), reg) } catch (e) { errs.push('эндер-сундук: ' + e) }
  var cur = vcTestCurios(pl)
  if (cur) {
    try {
      if (d.contains('curios')) vcTestLoadCurios(cur, d.getCompound('curios'), reg)
      else vcTestClearCurios(cur)
    } catch (e) { errs.push('Curios: ' + e) }
  }
  try {
    pl.removeAllEffects()
    var eff = d.getCompound('effects')
    vcTestEach(eff.getAllKeys(), function (key) {
      var inst = VC_TEST.Effect.load(eff.getCompound(String(key)))
      if (inst) pl.addEffect(inst)
    })
  } catch (e) { errs.push('эффекты: ' + e) }
  try {
    pl.experienceLevel = d.getInt('xpl')
    pl.experienceProgress = d.getFloat('xpp')
    pl.totalExperience = d.getInt('xpt')
    pl.giveExperienceLevels(0)
  } catch (e) { errs.push('опыт: ' + e) }
  var mode = d.contains('mode') ? String(d.getString('mode')) : 'survival'
  if (mode == 'creative' && !pl.hasPermissions(2)) mode = 'survival'
  server.runCommandSilent('gamemode ' + mode + ' ' + name)
  pl.inventoryMenu.broadcastChanges()
  d.putBoolean('active', false)
  if (errs.length) {
    // что-то вернулось не полностью — сохранённое не удаляем, чтобы администратор мог восстановить
    d.putBoolean('partial', true)
    vcTestSync(pl)
    console.error('[Vibe Club] ' + name + ' вышел из «Теста», ошибки: ' + errs.join('; '))
    pl.sendSystemMessage(Text.yellow('Вещи вернулись, но не всё (см. лог). Сохранённое не удалено — напиши администратору.'))
    return
  }
  d.remove('inv'); d.remove('ender'); d.remove('curios'); d.remove('effects')
  vcTestSync(pl)
  pl.sendSystemMessage(Text.yellow('Твои вещи вернулись. Креатив выключен.'))
  console.info('[Vibe Club] ' + name + ' вышел из «Теста», вещи возвращены')
}

function vcTestTp(server, name, dim, x, y, z) {
  server.runCommandSilent('execute in ' + dim + ' run tp ' + name + ' ' + Number(x).toFixed(2) + ' ' + Number(y).toFixed(2) + ' ' + Number(z).toFixed(2))
}

function vcTestCmd(ctx) {
  var src = ctx.source
  var pl = src.player
  if (!pl) return 0
  var name = String(pl.username)
  if (!/^[A-Za-z0-9_]{1,16}$/.test(name)) return 0
  var server = src.server
  var now = Date.now()
  if (now - (VC_TEST.cooldown[name] || 0) < 5000) {
    src.sendFailure(Text.red('Подожди пару секунд.'))
    return 0
  }
  if (!vcTestLevel(server, VC_TEST.dim)) {
    src.sendFailure(Text.red('Мир «Тест» не загружен на сервере.'))
    return 0
  }
  VC_TEST.cooldown[name] = now
  var d = vcTestData(pl)
  var here = vcTestDimId(src.level)
  if (here == VC_TEST.dim) {
    d.putString('t_dim', here); d.putDouble('t_x', pl.x); d.putDouble('t_y', pl.y); d.putDouble('t_z', pl.z)
    vcTestSync(pl)
    if (d.getBoolean('active')) vcTestLeave(server, pl)
    var dim = d.contains('r_dim') ? String(d.getString('r_dim')) : 'minecraft:overworld'
    if (d.contains('r_dim') && vcTestLevel(server, dim)) {
      vcTestTp(server, name, dim, d.getDouble('r_x'), d.getDouble('r_y'), d.getDouble('r_z'))
    } else {
      var sp = vcTestLevel(server, 'minecraft:overworld').getSharedSpawnPos()
      vcTestTp(server, name, 'minecraft:overworld', sp.getX() + 0.5, sp.getY() + 1, sp.getZ() + 0.5)
    }
    return 1
  }
  d.putString('r_dim', here); d.putDouble('r_x', pl.x); d.putDouble('r_y', pl.y); d.putDouble('r_z', pl.z)
  vcTestSync(pl)
  var t = d.contains('t_dim') ? { x: d.getDouble('t_x'), y: d.getDouble('t_y'), z: d.getDouble('t_z') } : VC_TEST.spawn
  if (!d.getBoolean('active')) {
    try {
      vcTestEnter(server, pl)
    } catch (e) {
      console.error('[Vibe Club] Не удалось сохранить вещи ' + name + ' для «Теста»: ' + e)
      src.sendFailure(Text.red('Не получилось сохранить твои вещи — в «Тест» не переношу. Ничего не пропало.'))
      return 0
    }
  }
  vcTestTp(server, name, VC_TEST.dim, t.x, t.y, t.z)
  return 1
}

// Сторож: каждый тик сверяем, где игрок и в каком он состоянии.
ServerEvents.tick(function (event) {
  if (!VC_TEST.ok) return
  var server = event.server
  var it = server.getPlayerList().getPlayers().iterator()
  while (it.hasNext()) {
    var pl = it.next()
    var lv = (typeof pl.level === 'function') ? pl.level() : pl.level
    var inTest = vcTestDimId(lv) == VC_TEST.dim
    try {
      var active = vcTestIsActive(pl)
      if (inTest && !active) vcTestEnter(server, pl)
      else if (!inTest && active) vcTestLeave(server, pl)
    } catch (e) {
      var nm = String(pl.username)
      var now = Date.now()
      if (now - (VC_TEST.errAt[nm] || 0) > 30000) {
        VC_TEST.errAt[nm] = now
        console.error('[Vibe Club] Ошибка переключения «Теста» для ' + nm + ': ' + e)
        if (inTest && /^[A-Za-z0-9_]{1,16}$/.test(nm)) {
          // в «Тесте», но вещи сохранить не удалось — выводим обратно, креатив не даём
          var sp = vcTestLevel(server, 'minecraft:overworld').getSharedSpawnPos()
          vcTestTp(server, nm, 'minecraft:overworld', sp.getX() + 0.5, sp.getY() + 1, sp.getZ() + 0.5)
          pl.sendSystemMessage(Text.red('«Тест» сейчас не работает, ты перенесён на спавн. Вещи не тронуты.'))
        }
      }
    }
  }
})

ServerEvents.commandRegistry(function (event) {
  if (!VC_TEST.ok) return
  event.register(event.commands.literal('test').executes(function (ctx) { return vcTestCmd(ctx) }))
})
