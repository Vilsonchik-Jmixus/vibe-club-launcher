# Vibe Club Launcher

Установщик и обновлятор сборки сервера **Vibe Club** (Minecraft 1.21.1, NeoForge).

* Скачать сборку в выбранную папку, обновить одной кнопкой, проверить и починить файлы, переустановить начисто (миры и личные настройки не трогаются).
* Ставит NeoForge и создаёт профиль «Vibe Club» в официальном лаунчере Minecraft.
* Смена скина через мод Quick Skin, который входит в сборку.

## Как устроено

* `main.go`, `manifest.go`, `download.go`, `loader.go`, `skin.go`, `status.go` — программа на Go без сторонних библиотек.
* `web/` — интерфейс (HTML/CSS/JS), встроен в exe; открывается в окне Microsoft Edge в режиме приложения.
* `pack/manifest.json` — список файлов текущей версии сборки: путь, SHA-1, размер, ссылка. Моды качаются напрямую с Modrinth/CurseForge, свои файлы (конфиги, квесты, меню) — из `pack/files/`.
* Политика файлов: обычные файлы всегда приводятся к версии сборки; `"policy": "once"` — личные (options.txt, клавиши, настройки графики) кладутся только если их нет.

## Сборка exe

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o VibeClubLauncher.exe .
```

Тег `vX.Y.Z` запускает GitHub Actions и выкладывает exe в Releases.

## Выпуск обновления сборки

1. Собрать новый `pack/manifest.json` (`go run ./tools/mkmanifest ...`) с новым `version`.
2. Закоммитить `pack/`. Лаунчеры игроков увидят «доступна версия …».

## Linux

Файл `VibeClubLauncher-linux-x64` (x86-64, без зависимостей):

```sh
chmod +x VibeClubLauncher-linux-x64
./VibeClubLauncher-linux-x64
```

- Окно лаунчера открывается в Chrome/Chromium/Edge/Brave/Vivaldi/Яндекс Браузере (режим приложения), иначе — в браузере по умолчанию.
- Выбор папки — через zenity или kdialog; если их нет, путь вводится вручную.
- «Играть» запускает официальный Minecraft Launcher (`minecraft-launcher`, /opt/minecraft-launcher или Flatpak `com.mojang.Minecraft`).
- Java 21 для установщика NeoForge берётся из лаунчера Minecraft/системы или скачивается (Temurin).
- После первого запуска лаунчер появляется в меню приложений.
