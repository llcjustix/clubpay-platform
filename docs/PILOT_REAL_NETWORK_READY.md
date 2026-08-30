# ClubPay: быстрый запуск пилота в реальном клубе

Эта инструкция — для **одного игрового ПК** `Pilot PC #01`. Пилот уже создан в production:
не создавайте вручную клуб, зону, тарифы или новый ПК.

## Что ставить и куда

| Устройство | Что сделать |
| --- | --- |
| Игровой ПК с Windows | Установить ClubPay Agent Core и Steam/игру |
| Сервер клуба | На текущем этапе ничего не ставить; позже он сможет заменить Pi для Wake-on-LAN |
| Ноутбук администратора | Ничего не ставить — открыть [админку ClubPay](https://clubpay.justix.uz/admin) в Chrome/Edge |
| Телефон игрока | Telegram для Mini App и оплаты |

> В этом клубе уже есть свой сервер и бездисковые Windows-клиенты. Поэтому **Pi сейчас не нужна**.
> Профиль, QR, оплата, Mini App и Agent работают через облако. Пока не подключён локальный WoL-relay,
> не проверяем только запуск ПК из сна.

## Данные пилота

| Поле | Значение |
| --- | --- |
| Клуб | `Pilot — Real Network` |
| Зона | `Pilot Standard` |
| Игровой ПК | `Pilot PC #01` |
| ID для Agent | `pilot-real-network-pc-001` |
| ID клуба | `564ef225-6cdb-4bf9-a362-960f942b3c4d` |

Сейчас нужен только `CORE_TOKEN` для Agent. `EDGE_WOL_TOKEN` потребуется позднее, если будем
подключать Wake-on-LAN relay на сервере клуба.

Не отправляйте их в общий чат, Git или скриншоты.

## 1. Подготовить игровой ПК

1. Подключите ПК к сети клуба по **Ethernet**.
2. Установите Steam и одну тестовую игру.
3. Установите [Git for Windows](https://git-scm.com/install/windows) и
   [SDK .NET 10 для Windows x64](https://dotnet.microsoft.com/en-us/download/dotnet/10.0).
4. Wake-on-LAN пока не настраивайте: его проверим отдельным этапом на сервере клуба.

## 2. Установить Agent Core в бездисковый образ

Agent ставится **один раз в master-образ Windows на сервере клуба**, а не вручную на каждый
игровой ПК. В консоли этого образа выполните:

```bat
git clone https://github.com/llcjustix/clubpay-core-agent.git C:\ClubPay\source
cd /d C:\ClubPay\source
dotnet publish .\src\ClubPay.Agent.Client\ClubPay.Agent.Client.csproj -c Release -r win-x64 --self-contained true -o C:\ClubPay\Agent
```

### Если пока тестируется один ПК

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

### Когда подключаете несколько бездисковых ПК

Каждому Windows-клиенту сервер должен выдавать **уникальное имя** — например `CP001`, `CP002`.
В админке ClubPay для соответствующих ПК укажите такие же `Core external ID`, но маленькими
буквами: `cp001`, `cp002`.

В общем `appsettings.Local.json` образа замените поля идентификации на:

```json
{
  "Agent": {
    "PcId": "PC {MACHINE_NAME}",
    "DataDirectory": "C:\\ProgramData\\ClubPay\\Agent\\state\\{MACHINE_NAME_LOWER}"
  },
  "Controller": {
    "ExternalPcId": "{MACHINE_NAME_LOWER}",
    "AgentToken": "<CORE_TOKEN_ПИЛОТА>"
  }
}
```

Так один образ не смешивает сессии: каждый Agent использует имя своего Windows-клиента и отдельную
папку состояния. Если сервер не выдаёт клиентам уникальные имена или отдельную writable-папку,
многопользовательский запуск пока не включаем.

## 3. Wake-on-LAN — не делать в текущем выезде

У клуба уже есть свой сервер, поэтому Raspberry Pi не покупаем и не настраиваем. В этом выезде
ПК запускаем включённым через Agent. После базового запуска отдельно согласуем с их инженером
ОС сервера, схему сети и способ запуска relay на этом сервере; только после этого включим тест
пробуждения из сна.

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
| Несколько ПК видны как один | Уникальны ли имена Windows и `ExternalPcId`, используется ли шаблон в конфиге |
| Не открывается Mini App | Напечатан ли текущий QR из админки, установлен ли Telegram |

Полный offline-режим, Captive Portal и работа при падении облака в этот пилот не входят.
