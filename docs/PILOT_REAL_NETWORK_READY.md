# ClubPay: быстрый запуск пилота в реальном клубе

Эта инструкция — для **одного игрового ПК** `Pilot PC #01`. Пилот уже создан в production:
не создавайте вручную клуб, зону, тарифы или новый ПК.

## Что ставить и куда

| Устройство | Что сделать |
| --- | --- |
| Игровой ПК с Windows | Установить ClubPay Agent Core и Steam/игру |
| Raspberry Pi | Установить WoL-relay, только если нужно будить ПК из сна |
| Ноутбук администратора | Ничего не ставить — открыть [админку ClubPay](https://clubpay.justix.uz/admin) в Chrome/Edge |
| Телефон игрока | Telegram для Mini App и оплаты |

> Pi **не** хранит платежи, профили или QR. Он только отправляет в локальную сеть команду
> Wake-on-LAN. Без Pi все остальные сценарии работают; не будет только запуска спящего ПК.

## Данные пилота

| Поле | Значение |
| --- | --- |
| Клуб | `Pilot — Real Network` |
| Зона | `Pilot Standard` |
| Игровой ПК | `Pilot PC #01` |
| ID для Agent | `pilot-real-network-pc-001` |
| ID клуба для Pi | `564ef225-6cdb-4bf9-a362-960f942b3c4d` |

Для Agent и Pi понадобятся два разных секрета, которые передаются отдельно в защищённом канале:

- `CORE_TOKEN` — только для игрового ПК;
- `EDGE_WOL_TOKEN` — только для Raspberry Pi.

Не отправляйте их в общий чат, Git или скриншоты.

## 1. Подготовить игровой ПК

1. Подключите ПК к сети клуба по **Ethernet**.
2. Установите Steam и одну тестовую игру.
3. Установите [Git for Windows](https://git-scm.com/install/windows) и
   [SDK .NET 10 для Windows x64](https://dotnet.microsoft.com/en-us/download/dotnet/10.0).
4. Для проверки Wake-on-LAN включите WoL / `Power on by PCI-E` в BIOS. В Windows разрешите
   сетевой карте пробуждение по magic packet и отключите Fast Startup.

## 2. Установить Agent Core

Откройте обычный `cmd.exe` на игровом ПК и вставьте:

```bat
git clone https://github.com/llcjustix/clubpay-core-agent.git C:\ClubPay\source
cd /d C:\ClubPay\source
dotnet publish .\src\ClubPay.Agent.Client\ClubPay.Agent.Client.csproj -c Release -r win-x64 --self-contained true -o C:\ClubPay\Agent
```

Создайте файл `C:\ClubPay\Agent\appsettings.Local.json` и вставьте в него. Замените только
`<CORE_TOKEN_ПИЛОТА>` на переданный секрет:

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

Запустите Agent:

```bat
start "" C:\ClubPay\Agent\ClubPay.Agent.Client.exe
```

**Проверка:** в админке `Настройки → Компьютеры → Pilot PC #01` статус стал **online**.
Если нет — остановитесь здесь и проверьте `CORE_TOKEN`, интернет и конфиг.

## 3. Подключить Raspberry Pi — только для Wake-on-LAN

Пропустите этот раздел, если не тестируете запуск ПК из сна.

Pi должна быть подключена к **той же сети клуба по Ethernet**, что и игровой ПК. На Pi должны
быть установлены Docker Engine и Docker Compose.

В терминале Pi выполните:

```bash
sudo git clone https://github.com/llcjustix/clubpay-platform.git /opt/clubpay-platform
cd /opt/clubpay-platform
sudo install -d -m 700 /etc/clubpay
sudo cp deploy/pi/edge-wol.env.example /etc/clubpay/edge-wol.env
sudo nano /etc/clubpay/edge-wol.env
```

В открытом файле замените **только** значение `EDGE_WOL_TOKEN` на секрет, переданный для Pi.
Сохранить в `nano`: `Ctrl+O` → Enter → `Ctrl+X`.

Запустите relay:

```bash
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml logs -f edge-wol
```

**Проверка:** в логе есть `connected to ClubPay Cloud`.

В админке откройте `Pilot PC #01 → Изменить` и укажите **проводной MAC-адрес** игрового ПК.
Без него Pi не сможет разбудить компьютер.

## 4. Пройти короткий тест

1. В админке распечатайте текущий QR для `Pilot PC #01` и разместите его у ПК.
2. Отсканируйте QR телефоном: должен открыться ClubPay Mini App в Telegram с правильными ПК и зоной.
3. Авторизуйтесь, выберите пакет или оплатите сумму и запустите сессию.
4. На игровом ПК Agent должен показать активную сессию и таймер.
5. Откройте Steam и тестовую игру из Agent.
6. Отсканируйте **QR продления на экране Agent**, добавьте время и убедитесь, что оно прибавилось
   к той же сессии.
7. Завершите сессию. Для авторизованного игрока остаток сохраняется в профиль, а бот присылает
   сообщение с оставшимся временем.

### Проверка пробуждения из сна

Только если установлен Pi relay:

1. Убедитесь, что Agent был online, затем переведите игровой ПК в сон.
2. В админке ПК должен стать `sleeping`, не `offline`.
3. Запустите сессию обычным Mini App-флоу.
4. Pi будит ПК, Agent переподключается, затем сессия стартует.

## 5. Kiosk включать последним

Включайте kiosk-режим только когда все шаги выше прошли. Создайте отдельного Windows-пользователя
`kiosk`, затем от имени администратора выполните:

```bat
cd /d C:\ClubPay\Agent
ClubPay.Agent.Client.exe --setup-kiosk=kiosk:ПАРОЛЬ_ПОЛЬЗОВАТЕЛЯ_KIOSK
```

Перезагрузите игровой ПК. Не применяйте эту команду на ноутбуке администратора.

## Если что-то не работает

| Симптом | Сначала проверить |
| --- | --- |
| ПК offline | Запущен ли Agent, правильны ли `CORE_TOKEN` и `ExternalPcId` |
| Pi не подключается | Интернет Pi и точность `EDGE_WOL_TOKEN` |
| ПК не просыпается | Pi и ПК в одной LAN, MAC проводной карты, BIOS/Windows WoL |
| Не открывается Mini App | Напечатан ли текущий QR из админки, установлен ли Telegram |

Полный offline-режим, Captive Portal и работа при падении облака в этот пилот не входят.
