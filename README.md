# Clubpay MVP

MVP для компьютерных клубов: QR-страница, пакеты, своя сумма, прямые онлайн-оплаты Click/Payme, запуск игровой сессии, админка, отчёт владельца и ваучеры остатка времени.

## Стек

- Go API
- React + TypeScript + Vite
- PostgreSQL через Docker
- Click SHOP API callbacks
- Payme Merchant API callbacks
- Core/iCafe-аналог через mock, HTTP adapter или WebSocket JSON protocol

## Запуск

```bash
docker compose up --build
```

После запуска:

- API health: <http://localhost:8080/api/health>
- QR test page: <http://localhost:5173/qr/pc-001>
- Панель менеджера: <http://localhost:5173/admin>
- Отчёт владельца: <http://localhost:5173/reports>
- Настройки клуба: <http://localhost:5173/settings>

## Доступы demo

- Суперадмин: `superadmin@clubpay.local` / `super123`
- Владелец: `owner@clubpay.local` / `owner123`
- Менеджер клуба: `admin@clubpay.local` / `admin123`

## Переменные оплат

```bash
DEFAULT_PAYMENT_PROVIDER=mock
MOCK_PAYMENTS_ENABLED=true

CLICK_CHECKOUT_URL=https://my.click.uz/services/pay
CLICK_MERCHANT_ID=
CLICK_SERVICE_ID=
CLICK_MERCHANT_USER_ID=
CLICK_SECRET_KEY=

PAYME_CHECKOUT_URL=https://test.paycom.uz
PAYME_MERCHANT_ID=
PAYME_SECRET_KEY=

PLATFORM_FEE_BPS=0
SPLIT_PAYMENTS_ENABLED=false
```

Для нескольких клубов технические ключи можно задавать в настройках клуба под суперадмином. Если поле клуба пустое, API использует env-переменную.

Для dev по умолчанию используется Payme sandbox `https://test.paycom.uz`. В production нужно явно указать `PAYME_CHECKOUT_URL=https://checkout.paycom.uz`.

## Callback URLs

Для реального callback нужен публичный HTTPS URL. Локально можно использовать:

```bash
cloudflared tunnel --url http://localhost:8080
```

Затем передать провайдерам:

- Click Prepare: `https://your-domain/api/payments/click/prepare`
- Click Complete: `https://your-domain/api/payments/click/complete`
- Payme Merchant API: `https://your-domain/api/payments/payme/callback`

И указать:

```bash
PUBLIC_BASE_URL=https://your-domain
FRONTEND_BASE_URL=https://your-frontend-domain
```

## Локальный mock-flow

1. Открыть <http://localhost:5173/qr/pc-001>.
2. Выбрать пакет или ввести свою сумму.
3. Выбрать способ оплаты `Тест`.
4. Нажать оплату.
5. На странице возврата или в панели менеджера нажать `Тестовая оплата`.
6. Проверить, что ПК стал `Занят`, а игровая сессия стала `Сессия запущена`.
7. Завершить сессию в панели менеджера и при необходимости получить ваучер остатка.

Для развернутого тестового стенда `Тест` включается флагом `MOCK_PAYMENTS_ENABLED=true`. Перед реальным production-запуском его нужно выключить, иначе публичный QR сможет создавать сессии без реальной оплаты.

## Payme sandbox-flow

1. В Payme Business создать веб-кассу и взять `Merchant ID` и `TEST_KEY`.
2. В настройках веб-кассы указать Endpoint URL: `https://your-domain/api/payments/payme/callback`.
3. В Clubpay задать `PAYME_MERCHANT_ID` и `PAYME_SECRET_KEY` (`TEST_KEY`) через env или настройки клуба.
4. Открыть QR, выбрать Payme и перейти по checkout-ссылке. Payme открывается через наш промежуточный endpoint `/api/payments/payme/checkout/{invoice_id}`: в production он отправляет POST-форму в Payme, а при `PAYME_CHECKOUT_URL=https://test.paycom.uz` переводит на внутреннюю sandbox-страницу `/api/payments/payme/sandbox/{invoice_id}`. Это нужно потому, что `test.paycom.uz` является кабинетом тестирования Merchant API, а не стабильным клиентским checkout.
5. На sandbox-странице нажать `Оплатить тестово`: backend применит оплату через тот же `payme` provider path и вернет пользователя на `/payment/return`.
6. Для ручной проверки Merchant API в кабинете Payme отдельно прогнать: `CheckPerformTransaction`, `CreateTransaction`, `PerformTransaction`, `CheckTransaction`, `CancelTransaction`, `GetStatement`. В поле суммы Payme ожидает тийины: `8000 сум` нужно вводить как `800000`.

## Click testing

Платежная ссылка Click строится только когда есть `CLICK_MERCHANT_ID`, `CLICK_SERVICE_ID`, `CLICK_MERCHANT_USER_ID` и `CLICK_SECRET_KEY`. Без этих значений QR-страница не даст открыть Click, чтобы не получать ошибку `Поставщик не найден или заблокирован`.

Платежная ссылка строится как `https://my.click.uz/services/pay/?service_id=...&merchant_id=...&merchant_user_id=...&amount=...&transaction_param=...&order_id=...&return_url=...`; `order_id` и `transaction_param` равны нашему `invoice_id`. После оплаты Click вызывает обязательный SHOP API callback-контур Prepare/Complete. Открытых sandbox credentials в публичной доке не указано, поэтому до выдачи тестовых/боевых доступов можно тестировать только локальную обработку callback-ов и полный mock-flow.

Для MVP split выключен: `SPLIT_PAYMENTS_ENABLED=false`. У каждого клуба своя касса/merchant/service в Click/Payme, клиент платит продавцу услуги напрямую, деньги идут клубу. Clubpay не участвует в расщеплении клиентского платежа и монетизируется отдельно: подписка, договор, счёт или акт за доступ к платформе.

Поля `cntrg_id` и receiver ID оставлены в БД/API как задел на будущую схему split/adaptive payment, но backend не отдаёт `split`/`receivers` провайдерам, пока флаг не включён.

Clubpay поддерживает два callback-формата Click:

- старый form-style `action=0/1`, где заказ приходит как `merchant_trans_id` или `transaction_param`;
- Shop Split JSON из документа Click: `action=1` Prepare и `action=2` Confirm, где заказ приходит в `params.order_id` или `params.transaction_param`.

В MVP даже при JSON callback-формате Clubpay отвечает без `split`. Возврат `split: [{ cntrg_id, amount }]` возможен только в будущей версии после отдельного юридического решения, настройки получателей и включения `SPLIT_PAYMENTS_ENABLED=true`.

## Seller, subscription and fiscalization

Продавец услуги для MVP — клуб. Деньги за компьютерное время идут в Payme/Click кассу клуба. Clubpay не держит деньги клуба и монетизируется подпиской/счётом/договором за доступ к платформе.

Split/adaptive payment пока не часть MVP. Кодовая поддержка оставлена выключенной флагом `SPLIT_PAYMENTS_ENABLED=false`, чтобы мы могли быстро вернуться к этому варианту позже без переписывания платежного ядра.

Если позже включаем Payme adaptive payment, после договора/кассы для клуба в `CreateTransaction` можно вернуть `receivers`. В Clubpay это включается служебными полями:

- `payme_club_receiver_id` — получатель основной суммы клуба;
- `payme_platform_receiver_id` — получатель комиссии Clubpay, если `platform_fee_bps > 0`.

Click Shop Split: в `Prepare` response возвращается `split` со списком `{ cntrg_id, amount }`. Payme adaptive payment: в `CreateTransaction` response возвращается `receivers` со списком `{ id, amount }`. В MVP эти ответы не отправляются, потому что split выключен.

Фискальный чек за компьютерное время должен быть от имени клуба и с ИНН клуба. В одном фискальном чеке нельзя смешивать ИНН клуба и Clubpay. Подписка/комиссия Clubpay фискализируется отдельно по нашей юрсхеме/договору, а чек за компьютерное время формируется по клубу.

Для MVP без split Payme может фискализировать чек на стороне кассы клуба. Clubpay при создании Payme checkout отправляет POST-инициализацию через `/api/payments/payme/checkout/{invoice_id}`. Если у клуба заполнены `ofd_service_name`, `ofd_mxik`, `ofd_package_code`, `ofd_unit_code`, `ofd_vat_percent`, backend добавляет Payme `detail` с `detail.items`; если коды не заполнены, checkout уходит без `detail`, чтобы работала статичная фискализация кассы Payme. В обоих вариантах касса должна быть кассой клуба.

Если позже включаем split/adaptive payment, Payme и Click уже не смогут автоматически сделать один чек от имени двух ИНН. Тогда фискализацию нужно выносить в отдельный контур Soliq/OFD/платежного fiscal API, а split включать только после финальной юридической и технической настройки.

## Важные решения MVP

- Суммы хранятся в тийинах.
- `invoice_id` = наш `order_id`.
- `provider` = `click`, `payme` или `mock`.
- `provider_payment_id` = ID транзакции у провайдера.
- Разблокировка ПК происходит только после валидного успешного callback.
- Страница возврата сама опрашивает `/api/orders/{invoice_id}`.
- Фискализация для Payme без split идёт через кассу клуба: при наличии fiscal-кодов Clubpay отправляет `detail.items`, иначе используется статичная настройка кассы Payme.
- Наличные логируются отдельно и запускают сессию без онлайн-провайдера.
- Core/iCafe-аналог может работать в `CORE_MODE=mock`, `CORE_MODE=http` или `CORE_MODE=ws`.
- Split/adaptive payment по умолчанию выключен; включается только явным `SPLIT_PAYMENTS_ENABLED=true`.

## Production-блокеры

- Получить production-доступы Click и Payme.
- Подтвердить callback URLs, подписи, retry и IP/HTTPS требования у провайдеров.
- Для MVP без split: завести отдельные кассы/merchant/service по клубам или подтвердить другой способ, при котором деньги за услугу идут продавцу-клубу.
- Если позже возвращаем split: получить/занести Click `cntrg_id` и Payme receiver IDs по каждому клубу, подключить отдельную фискализацию и включить `SPLIT_PAYMENTS_ENABLED=true`.
- Финально подтвердить фискальные коды услуги с бухгалтером/налоговым консультантом клуба: MXIK/ИКПУ/SPIC, package_code, unit_code, НДС, название услуги.
- Решить фискализацию наличных через онлайн-кассу/Soliq.

## Core/iCafe-аналог

По умолчанию используется mock:

```bash
CORE_MODE=mock
```

Основной вариант для MVP — WebSocket bidirectional JSON. Agent/Core сам открывает исходящее соединение в Billing:

```bash
CORE_MODE=ws
CORE_TOKEN=shared-secret
```

Agent подключается:

```http
GET /api/core/ws?external_pc_id=pc-001&agent_token=shared-secret
```

Также можно передать токен через `Authorization: Bearer <CORE_TOKEN>` или `X-Agent-Token: <CORE_TOKEN>`.

Перед стартом Agent может забрать конфиг ПК:

```http
GET /api/core/bootstrap?external_pc_id=pc-001
Authorization: Bearer <CORE_TOKEN>
```

Bootstrap возвращает клуб, ПК, зону, пакеты, hourly price и `qr_url`.

Billing отправляет Agent команды:

```json
{
  "type": "command",
  "name": "start_session",
  "command_id": "start_<grant_id>",
  "ts": "2026-06-30T10:00:00Z",
  "payload": {
    "external_pc_id": "pc-001",
    "grant_id": "<grant_uuid>",
    "payment_order_id": "cp_...",
    "granted_seconds": 3600,
    "duration_seconds": 3600,
    "ends_at": "2026-06-30T11:00:00Z",
    "source": "online_payment"
  }
}
```

Agent отвечает:

```json
{
  "type": "command_result",
  "command_id": "start_<grant_id>",
  "status": "ok",
  "payload": {
    "external_pc_id": "pc-001",
    "core_session_id": "core-session-123",
    "grant_id": "<grant_uuid>",
    "started_at": "2026-06-30T10:00:00Z",
    "ends_at": "2026-06-30T11:00:00Z"
  }
}
```

Минимальные команды: `start_session`, `extend_session`, `end_session`, `lock`, `unlock`, `wake`, `sleep`, `set_repair`, `get_status`.

Agent отправляет события в то же WS-соединение:

```json
{
  "type": "event",
  "name": "pc_status_changed",
  "event_id": "evt_001",
  "ts": "2026-06-30T10:00:00Z",
  "payload": {
    "external_pc_id": "pc-001",
    "status": "occupied"
  }
}
```

Минимальные события: `agent_online`, `agent_offline`, `pc_status_changed`, `session_started`, `session_extended`, `session_ended`, `session_failed`, `command_failed`, `heartbeat`. Alias `pc_state_changed` тоже принимается.

Поддерживаемые статусы ПК: `available`, `occupied`, `sleeping`, `offline`, `maintenance`, `blocked`, `unknown`. Также принимаются Core-синонимы `FREE`, `BUSY`, `SLEEP`, `REPAIR`, `LOCKED` и т.д.

Альтернативный HTTP adapter остаётся для старого/простого контроллера:

```bash
CORE_MODE=http
CORE_BASE_URL=http://controller.local:8081
CORE_TOKEN=shared-secret
```

Billing вызывает:

- `GET /core/v1/pcs/{external_pc_id}/status`
- `POST /core/v1/sessions/start`
- `POST /core/v1/sessions/{core_session_id}/extend`
- `POST /core/v1/sessions/{core_session_id}/end`

Core шлёт события в Billing:

```http
POST /api/core/events
Authorization: Bearer <CORE_TOKEN>
```

Минимальные события для HTTP те же, что и для WebSocket.

## Продление сессии

Статический физический QR на ПК/столе запускает только свободный или спящий ПК. Если ПК уже занят, страница по статическому QR показывает статус и остаток времени, но не даёт оплатить или применить ваучер, чтобы случайный клиент не продлил чужую сессию.

Продление делаем через динамический QR типа `session_extend`, который должен показывать клиентский экран активной сессии в будущем Core/Agent. Для такого QR успешная онлайн-оплата, наличная оплата или ваучер не создают вторую сессию, а вызывают `POST /core/v1/sessions/{core_session_id}/extend` и добавляют время к текущему окончанию. До готовности клиентского приложения продление можно тестировать через менеджерскую панель/ручной mock-flow.

## Raspberry Pi edge mode

Raspberry Pi запускает тот же API, web и Postgres, но в режиме локального сервера клуба. Если облако недоступно, клиенты и менеджеры могут работать через Pi: QR-страница, наличка, локальные ваучеры, панель менеджера, сессии и команды в Core/Agent выполняются локально. Онлайн-оплата через Click/Payme в edge-режиме работает только при отдельной инфраструктурной настройке callback/failover и наличии интернета до провайдера.

Минимальный env для Pi:

```env
NODE_MODE=edge
EDGE_NODE_ID=club-001-pi-a
EDGE_CLUB_ID=<club_uuid>
CLOUD_BASE_URL=https://api.clubpay.uz
EDGE_SYNC_TOKEN=<shared-secret>
PUBLIC_BASE_URL=https://club-001-edge.example.uz
FRONTEND_BASE_URL=https://club-001-edge.example.uz
DATABASE_URL=postgres://clubpay:clubpay@postgres:5432/clubpay?sslmode=disable
CORE_MODE=ws
# или CORE_MODE=http + CORE_BASE_URL=http://controller.local:8081, если Core сделан HTTP-сервисом
```

Важно для онлайн-оплат: если падает только наше облако, Pi продолжает принимать Click/Payme при условии, что у него есть интернет до провайдера и публичный HTTPS endpoint для callback. Для этого `PUBLIC_BASE_URL` на Pi должен быть доступен провайдерам через домен/tunnel. Если callback URL у Payme/Click закреплён только за облаком, нужен failover DNS/proxy или отдельная provider-настройка на edge endpoint клуба, иначе провайдер не сможет подтвердить оплату на Pi. Если интернета нет вообще, онлайн-оплата невозможна технически; остаются наличные, локальные ваучеры и ручное управление сессиями.

Для Telegram-ваучеров действует то же правило: бот может доставить `/start` в Pi только если webhook смотрит на Pi/failover-домен или если для этого окружения используется polling без активного webhook. У Telegram у одного бота только один webhook, поэтому это надо решить в инфраструктуре до production fallback-теста.

Синхронизация:

- cloud и edge используют один код и одну схему БД;
- `GET /api/edge/snapshot?club_id=...` отдаёт полный снимок клуба: настройки, зоны, пакеты, ПК/QR, пользователей, платежи, наличку, сессии, ваучеры и Telegram-привязки;
- `POST /api/edge/events` принимает `edge_snapshot` от Pi и применяет его в облаке;
- в `NODE_MODE=edge` или `NODE_MODE=manager` API автоматически пушит локальный snapshot в `CLOUD_BASE_URL`, а после успешного push подтягивает snapshot обратно;
- журнал синхронизации хранится в `edge_sync_runs`.

Для production задайте `EDGE_SYNC_TOKEN` и передавайте его как `Authorization: Bearer <token>` или `X-Edge-Token`.

## Manager PC emergency mode

ПК менеджера может стать аварийным локальным узлом, но для этого на нём должен быть установлен тот же backend Clubpay + локальный Postgres. Одна открытая веб-админка в браузере не заменит контроллер: браузер не сможет хранить общий клубный журнал, принимать callbacks, стабильно синкаться и выполнять команды Core/Agent для всех ПК.

Минимальный env для ПК менеджера:

```env
NODE_MODE=manager
MANAGER_NODE_ID=club-001-manager-pc
MANAGER_CLUB_ID=<club_uuid>
CLOUD_BASE_URL=https://api.clubpay.uz
EDGE_SYNC_TOKEN=<shared-secret>
DATABASE_URL=postgres://clubpay:clubpay@localhost:5432/clubpay?sslmode=disable
CORE_MODE=http
CORE_BASE_URL=http://controller.local:8081
MANAGER_ONLINE_PAYMENTS_ENABLED=false
```

Что доступно в аварийном режиме менеджера:

- панель менеджера, наличные сессии, ручное завершение, ваучеры и локальный журнал;
- команды в Core/Agent, если Core/Agent доступен в локальной сети;
- синхронизация обратно в облако после восстановления связи;
- mock/test-оплата, если `MOCK_PAYMENTS_ENABLED=true`.

Онлайн-оплата Click/Payme на ПК менеджера по умолчанию отключена. Включать `MANAGER_ONLINE_PAYMENTS_ENABLED=true` можно только если у ПК менеджера есть интернет, публичный HTTPS endpoint, корректные callback/failover настройки у провайдеров и понятная операционная процедура. Без интернета до Click/Payme онлайн-оплата невозможна технически; остаются наличные, локальные ваучеры и ручное управление.

Для проверки статуса узла:

```http
GET /api/node/status
```

## Telegram ваучеры

После ручного завершения сессии менеджер может указать номер телефона. Если остаток времени больше нуля, номер сохраняется в БД и создаётся ваучер. Если Telegram уже привязан к этому номеру, ваучер отправится автоматически. Если номер еще не привязан, API вернет одноразовую ссылку `https://t.me/<bot>?start=<token>`: клиент открывает ее один раз, бот сохраняет `chat_id` и сразу отправляет ожидающий ваучер.

Webhook бота:

```http
POST /api/telegram/webhook
X-Telegram-Bot-Api-Secret-Token: <TELEGRAM_WEBHOOK_SECRET>
```

`TELEGRAM_BOT_USERNAME` можно указать явно без `@`. Если он пустой, API попробует получить username через Telegram `getMe`.

Для локального тестирования можно не поднимать публичный webhook: в development включён `TELEGRAM_POLLING_ENABLED=true`, и API сам забирает `/start` через Telegram `getUpdates`, если у бота не настроен webhook. В production лучше использовать webhook и выключить polling.
