# Пилот ClubPay в реальной компьютерной сети

Инструкция для запуска **одного реального игрового ПК** в уже созданном production-пилоте.

> Статус на 30 августа 2026: рабочий пилот состоит из облачного ClubPay, Telegram Mini App и
> Agent Core на игровом ПК. Для Wake-on-LAN в пилоте используется отдельный Raspberry Pi relay:
> он только будит спящий ПК и не подменяет облако, Mini App или оплату.

## 1. Что именно устанавливается

| Где | Что нужно | Как устанавливается | Статус для пилота |
| --- | --- | --- | --- |
| Игровой ПК с Windows | **ClubPay Agent Core** | Сборка self-contained `.exe` из [репозитория Agent](https://github.com/llcjustix/clubpay-core-agent) | Обязательно |
| Админский ПК | Chrome или Edge и доступ к [веб-админке](https://clubpay.justix.uz/admin) | Ничего скачивать и клонировать не нужно | Обязательно |
| Телефон игрока | Telegram | Обычное приложение Telegram | Обязательно для профильного сценария |
| Raspberry Pi | ClubPay edge WoL relay | Сборка Docker-образа из репозитория платформы | Нужен только для Wake-on-LAN |

У ClubPay сейчас нет отдельного «Agent для ПК администратора». Это не недочёт установки:
администратор управляет клубом через облачную веб-админку. Браузер не заменяет локальный
Controller, но локальный Controller пока не входит в готовый пилот.

Репозитории:

- платформа/API/web: [github.com/llcjustix/clubpay-platform](https://github.com/llcjustix/clubpay-platform);
- Agent для игрового Windows-ПК: [github.com/llcjustix/clubpay-core-agent](https://github.com/llcjustix/clubpay-core-agent).

Готовый скачиваемый релиз Agent пока не публикуется. Это не блокирует установку: команда
ниже собирает self-contained `ClubPay.Agent.Client.exe` из Git, и после публикации .NET Runtime
на игровом ПК уже не требуется.

## 2. Данные готового пилота

Коллеге не нужно создавать сеть, клуб, зону, тарифы или ПК вручную.

| Что | Значение |
| --- | --- |
| Сеть | `Pilot Network` |
| ID сети | `d1f0e651-36de-4dd5-bb79-5b3e1835693b` |
| Клуб | `Pilot — Real Network` |
| ID клуба | `564ef225-6cdb-4bf9-a362-960f942b3c4d` |
| Зона | `Pilot Standard` |
| ID зоны | `5566ddf1-b448-477e-8ce1-4b7c3f2543d6` |
| Игровой ПК | `Pilot PC #01` |
| ID ПК | `96d09011-d692-4d3e-b4f6-beeb180f8d65` |
| ID для Agent | `pilot-real-network-pc-001` |

Тарифы: 1 минута — 1 000 сум, 30 минут — 8 000 сум, 1 час — 15 000 сум, 2 часа — 28 000 сум.

Статический QR не копировать из старых документов. В админке открыть
`Настройки → Компьютеры → Pilot PC #01`, нажать печать QR и разместить **текущую** наклейку у
ПК. При включённом Mini App этот QR открывает профиль ClubPay внутри Telegram. Кнопка
«Перевыпустить QR» немедленно отключает старую наклейку.

Доступ владельца создан для `Aleksey G.`. Пароли, `CORE_TOKEN`, платёжные ключи и Telegram token
передаются только в защищённом канале — их нельзя писать в Git, скриншотах или общем чате.

## 3. До выезда

- Тестовый игровой ПК c Windows 10/11, подключённый к сети по Ethernet.
- Доступ администратора к Windows и BIOS этого ПК.
- Steam и тестовая игра, уже установленные на этом ПК.
- Телефон c Telegram и интернетом.
- Админский ноутбук/ПК с Chrome или Edge и доступом к [админке](https://clubpay.justix.uz/admin).
- `CORE_TOKEN` именно для этого пилота.
- Для реального списания — production-ключи Click/Payme. Без них проверяются наличная сессия и
  тестовый провайдер; публичный production QR не должен оставаться с включённой mock-оплатой.

Для WoL также заранее зафиксируйте MAC **проводной** сетевой карты ПК.

## 4. Установка на игровом Windows-ПК

### 4.1 Подготовить Windows

1. Установить [Git for Windows](https://git-scm.com/install/windows).
2. Установить [SDK .NET 10 для Windows x64](https://dotnet.microsoft.com/en-us/download/dotnet/10.0).
   Нужен именно **SDK**, а не только Runtime: он используется один раз для сборки.
3. Установить Steam и игру. Agent запускает только уже локально установленные приложения.
4. Для WoL в BIOS включить Wake-on-LAN / Power on by PCI-E, а в Windows разрешить сетевой карте
   пробуждение по magic packet. Отключите Windows Fast Startup.

### 4.2 Собрать Agent из Git

Откройте обычный `cmd.exe` и выполните:

```bat
git clone https://github.com/llcjustix/clubpay-core-agent.git C:\ClubPay\source
cd /d C:\ClubPay\source
dotnet publish .\src\ClubPay.Agent.Client\ClubPay.Agent.Client.csproj -c Release -r win-x64 --self-contained true -o C:\ClubPay\Agent
```

После успешной команды файл для запуска находится здесь:

```text
C:\ClubPay\Agent\ClubPay.Agent.Client.exe
```

### 4.3 Привязать Agent к пилоту

Создайте `C:\ClubPay\Agent\appsettings.Local.json`:

```json
{
  "Agent": {
    "PcId": "Pilot PC #01",
    "ClubName": "Pilot — Real Network",
    "Zone": "Pilot Standard",
    "VoiceAnnouncementsEnabled": true,
    "KioskLockdownEnabled": false,
    "HideWindowsTaskbar": false,
    "MaintenanceExitEnabled": true
  },
  "Controller": {
    "WebSocketUrl": "wss://api-clubpay.justix.uz/api/core/ws",
    "BootstrapUrl": "https://api-clubpay.justix.uz/api/core/bootstrap",
    "AgentToken": "<CORE_TOKEN_ПИЛОТА>",
    "ExternalPcId": "pilot-real-network-pc-001"
  },
  "Launcher": {
    "DiscoverSteamGames": true,
    "SteamLibraryRoots": [],
    "Apps": []
  }
}
```

Для первого запуска оставьте режим обслуживания, как в примере выше. Запустите:

```bat
start "" C:\ClubPay\Agent\ClubPay.Agent.Client.exe
```

В админке ПК должен стать online. Только после полного прохождения раздела 6 можно включать
kiosk-режим на игровом ПК. Для него используется отдельный Windows-пользователь `kiosk` и
команда из **Command Prompt от администратора**:

```bat
cd /d C:\ClubPay\Agent
ClubPay.Agent.Client.exe --setup-kiosk=kiosk:ПАРОЛЬ_ПОЛЬЗОВАТЕЛЯ_KIOSK
```

Затем перезагрузите ПК. Не применяйте kiosk-конфигурацию на админском компьютере.

## 5. Raspberry Pi: установка relay для Wake-on-LAN

Pi нужен только для отправки WoL magic packet из сети клуба. Игровой Agent, QR, Telegram Mini App,
оплата и веб-админка **остаются в облаке** — игрок проходит обычный флоу, ничего не меняется.

1. Запишите [Raspberry Pi OS 64-bit](https://www.raspberrypi.com/software/) на Pi 4/5, подключите
   Pi к LAN клуба по Ethernet и выдайте ему постоянный DHCP lease.
2. Установите Docker Engine и Compose по [официальной инструкции Docker](https://docs.docker.com/engine/install/).
3. В GitHub репозитория добавьте secret `EDGE_WOL_TOKEN` (случайная длинная строка) и variable
   `WOL_ENABLED=true`. Этот токен нужен одновременно облаку и Pi; не используйте `CORE_TOKEN`.
4. На Pi выполните:

    git clone https://github.com/llcjustix/clubpay-platform.git /opt/clubpay-platform
    cd /opt/clubpay-platform
    sudo install -d -m 700 /etc/clubpay
    sudo cp deploy/pi/edge-wol.env.example /etc/clubpay/edge-wol.env
    sudo nano /etc/clubpay/edge-wol.env
    sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build

5. В `/etc/clubpay/edge-wol.env` заполните только `EDGE_WOL_TOKEN` тем же значением из GitHub.
   Остальные значения уже содержат endpoint и ID pilot-клуба.
6. Проверьте:

    cd /opt/clubpay-platform
    sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml logs -f edge-wol

В логе должна быть строка о подключении Pi к ClubPay Cloud. Подробности и команда обновления:
[deploy/pi/README.md](../deploy/pi/README.md).

Это **не** полноценный offline edge-узел. Автономная работа без облака, локальная QR-страница,
Captive Portal и failover Click/Payme — следующий отдельный этап; этот relay их не имитирует.

## 6. Проверка cloud-пилота

### Запуск и оплата

1. Убедиться в админке, что `Pilot PC #01` — online и свободен.
2. Отсканировать текущий QR. Должен открыться ClubPay Mini App в Telegram и показать профиль
   игрока, номер ПК и зону.
3. Если профиль уже имеет баланс, выбрать запуск с баланса. Если баланса нет — выбрать пакет или
   сумму, оплатить через доступный способ и запустить сессию.
4. Agent должен показать активную сессию, таймер и **отдельный временный QR продления**.
5. Открыть Steam и тестовую игру из Agent.

### Продление и завершение

1. Во время сессии отсканировать QR продления на экране Agent и добавить время.
2. Проверить, что время добавилось к текущей сессии, а не была создана новая.
3. Завершить сессию из меню Agent или админки.
4. Для авторизованного игрока не должно быть окна ваучера: неиспользованное время сохраняется в
   профиле, а Telegram-бот присылает сообщение с остатком.
5. Для гостевого сценария применяется ваучерный флоу; он не является основным сценарием пилота.

### Восстановление и kiosk

1. Перезапустить Agent во время сессии: он должен восстановить активную сессию и таймер после
   переподключения.
2. Включить kiosk только после успешных предыдущих пунктов.
3. После перезагрузки под пользователем `kiosk` Agent должен открыться автоматически; игрок не
   должен получить Windows Explorer, обычный taskbar или настройки Windows.

## 7. Граница текущего пилота

Готовый результат первого выезда:

- Agent стабильно работает на одном реальном игровом ПК;
- текущий QR открывает Mini App/профиль нужного ПК;
- профильный баланс, оплата, запуск, продление и сохранение остатка работают;
- Steam/игра запускаются из Agent;
- администратор управляет всем через веб-админку.

Не включать в акт первого пилота: автономную работу без облака, Captive Portal и failover реальных
Click/Payme callbacks. Это следующий отдельный этап.
