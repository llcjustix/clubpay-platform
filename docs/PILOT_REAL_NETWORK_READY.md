# ClubPay: пилот в реальном клубе с локальным резервом

Эта инструкция — для **одного игрового ПК** `Pilot PC #01`. Пилот уже создан в production:
не создавайте вручную клуб, зону, тарифы или новый ПК. Перед реальным запуском ставятся две
локальные Controller-ноды: основная на сервере клуба и резервная на ПК менеджера.

## Что ставить и куда

| Устройство | Что сделать |
| --- | --- |
| Игровой ПК с Windows | Установить `ClubPay-Agent-win-x64.zip` и Steam/игру |
| Сервер клуба | `ClubPay-Controller-win-x64.zip`: локальный API, PWA, база, Agent-команды, касса, ваучеры и WoL |
| Компьютер менеджера | `ClubPay-Manager-win-x64.zip` **и** второй `ClubPay-Controller-win-x64.zip` в режиме `manager` |
| Телефон игрока | Telegram для Mini App и оплаты |

> В этом клубе уже есть свой Windows-сервер и бездисковые Windows-клиенты. Поэтому Raspberry Pi
> сейчас не нужна: Windows Server выполняет ту же роль Controller Node. Cloud — основной источник,
> но при его недоступности зал продолжает работать через сервер клуба, затем через ПК менеджера.

## Данные пилота

| Поле | Значение |
| --- | --- |
| Клуб | `Pilot — Real Network` |
| Зона | `Pilot Standard` |
| Игровой ПК | `Pilot PC #01` |
| ID для Agent | `pilot-real-network-pc-001` |
| ID клуба | `564ef225-6cdb-4bf9-a362-960f942b3c4d` |

Коды установки создаются в Web Admin: **Настройки → Клуб → Локальный
Controller**. Каждый действует 30 минут, подходит ровно для одной ноды и не
даёт вручную доступ к токенам клуба.

## 1. Поставить основной Controller на сервер клуба

1. Скачайте `ClubPay-Controller-win-x64.zip` со [страницы релизов Platform](https://github.com/llcjustix/clubpay-platform/releases),
   распакуйте в `C:\ClubPay\Controller`.
2. В Web Admin создайте код **для основного сервера / Raspberry Pi**.
3. Запустите `install-windows.cmd` **от имени администратора** и вставьте код.
   Установщик сам создаст конфиг, встроенную PostgreSQL и Windows-сервис;
   Docker, отдельная база и ручная вставка токенов не нужны.
5. Откройте `http://localhost:8080/api/node/status`. Ответ должен содержать
   `local_authority: true` и `node_mode: edge`.

Это сервис полного локального управления, а не один Wake-on-LAN relay.

## 2. Подготовить игровой ПК

1. Подключите ПК к сети клуба по **Ethernet**.
2. Установите Steam и одну тестовую игру.
3. Скачайте `ClubPay-Agent-win-x64.zip` со страницы [релизов Agent](https://github.com/llcjustix/clubpay-core-agent/releases),
   распакуйте в `C:\ClubPay\Agent`.
4. Wake-on-LAN пока не настраивайте: его проверим отдельным этапом на сервере клуба.

## 3. Установить Agent Core в бездисковый образ

Agent ставится **один раз в master-образ Windows на сервере клуба**, а не вручную на каждый
игровой ПК. Нужны только распакованный релиз и конфиг ниже: Git и .NET SDK на ПК клуба не нужны.

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
    "FallbackWebSocketUrls": ["ws://<IP_СЕРВЕРА>:8080/api/core/ws", "ws://<IP_МЕНЕДЖЕРА>:8080/api/core/ws"],
    "FallbackBootstrapUrls": ["http://<IP_СЕРВЕРА>:8080/api/core/bootstrap", "http://<IP_МЕНЕДЖЕРА>:8080/api/core/bootstrap"],
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

## 4. Manager Client и резервный Controller на компьютере менеджера

Скачайте `ClubPay-Manager-win-x64.zip` с той же [страницы релизов Agent](https://github.com/llcjustix/clubpay-core-agent/releases),
распакуйте, например, в `C:\ClubPay\Manager`, и запустите `ClubPay.Agent.Admin.exe`.
Войдите теми же данными, что и в веб-админке. Внутри EXE доступны те же компьютеры, касса,
пользователи, настройки и отчёты, потому что это одна production-админка, а не урезанная копия.

Поставьте второй `ClubPay-Controller-win-x64.zip` в `C:\ClubPay\ManagerController`.
В Web Admin создайте **код для резервного ПК менеджера**, затем запустите
`install-windows.cmd` от имени администратора и вставьте его. Установщик сам
назначит режим `manager` и отключит рискованный онлайн-эквайринг. Затем в
`C:\ClubPay\Manager\appsettings.Local.json` укажите `Manager:LocalControllerUrls`
с `http://127.0.0.1:8080/admin`.

Manager Client сначала использует Cloud, а при его падении сам открывает локальную админку.

## 5. Wake-on-LAN — часть Controller, не отдельная установка

У клуба уже есть свой сервер, поэтому Raspberry Pi не покупаем и не настраиваем. В `controller.env`
укажите корректный broadcast-адрес сети, заполните MAC-адреса ПК в админке и проверьте кнопку
**Включить** на одном спящем ПК. Всё это выполняет Controller на сервере клуба.

## 6. Пройти короткий тест

1. В админке распечатайте текущий QR для `Pilot PC #01` и разместите его у ПК.
2. Отсканируйте QR телефоном: должен открыться ClubPay Mini App в Telegram с правильными ПК и зоной.
3. Авторизуйтесь, выберите пакет или оплатите сумму и запустите сессию.
4. На игровом ПК Agent должен показать активную сессию и таймер.
5. Откройте Steam и тестовую игру из Agent.
6. Отсканируйте **QR продления на экране Agent**, добавьте время и убедитесь, что оно прибавилось
   к той же сессии.
7. Завершите сессию. Для авторизованного игрока остаток сохраняется в профиль, а бот присылает
   сообщение с оставшимся временем.

## 7. Проверить переключение без Cloud

1. Убедитесь, что основной Controller уже сделал первый sync (в `/api/node/status` указан клуб).
2. На одном Agent временно закройте доступ к `api-clubpay.justix.uz`, не отключая LAN.
3. Agent должен подключиться к URL сервера клуба из `FallbackWebSocketUrls`; QR bootstrap также
   берётся с локальной ноды.
4. Откройте Manager Client: он должен показать «Локальный Controller».
5. Проверьте кассовую сессию или ваучер. Не делайте карточный платёж в этом тесте, пока отдельно
   не согласованы и не проверены merchant secrets на основном Controller.
6. Верните Cloud: синхронизация должна возобновиться автоматически.

## 8. Kiosk включать последним

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
| Не открывается локальная панель | Работает ли `ClubPay Controller Node` и `http://localhost:8080/api/node/status` |
| Не открывается Mini App | Напечатан ли текущий QR из админки, установлен ли Telegram |

При полной потере Cloud Telegram Mini App не может подменить свой BotFather HTTPS-адрес на
локальный IP. Поэтому локальный Controller сохраняет кассу/ваучеры/сессии, а профильный Telegram
вход работает, когда Telegram и public Mini App доступны.
