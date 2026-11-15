// Vibe Club — часы GUI Clock в правом нижнем углу экрана.
// Сам мод умеет ставить часы только слева/по центру/справа и отступ сверху (clockHeightOffset),
// а высота экрана в «GUI-пикселях» зависит от окна и масштаба интерфейса. Поэтому каждый тик
// выставляем отступ сверху так, чтобы часы (время и строка с днём под ним) стояли у нижнего края.
// Стиль ES5 (var, function): Rhino в KubeJS плохо переносит const/let в блоках.
var VC_CLK = { CH: null, last: -1 }

try {
  VC_CLK.CH = Java.loadClass('com.serilum.guiclock_common_neoforge.config.ConfigHandler')
} catch (e) {
  console.warn('[Vibe Club] GUI Clock не найден, часы не двигаю: ' + e)
}

ClientEvents.tick(function (event) {
  if (!VC_CLK.CH) return
  try {
    var h = Client.getWindow().getGuiScaledHeight()
    var lines = VC_CLK.CH.showDaysPlayedWorld ? 2 : 1
    var y = h - 4 - 10 * lines
    if (y != VC_CLK.last) {
      VC_CLK.CH.clockHeightOffset = y
      VC_CLK.last = y
    }
  } catch (e) {
    VC_CLK.CH = null
    console.warn('[Vibe Club] Не получилось сдвинуть часы: ' + e)
  }
})
