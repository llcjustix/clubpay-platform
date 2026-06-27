# Clubpay MVP

MVP для компьютерных клубов: QR-страница, пакеты, своя сумма, прямые онлайн-оплаты Click/Payme, запуск игровой сессии, админка, отчёт владельца и ваучеры остатка времени.

## Стек

- Go API
- React + TypeScript + Vite
- PostgreSQL через Docker
- Click SHOP API callbacks
- Payme Merchant API callbacks
- Core/iCafe-аналог через mock или HTTP adapter

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

CLICK_CHECKOUT_URL=https://my.click.uz/services/pay
CLICK_MERCHANT_ID=
CLICK_SERVICE_ID=
CLICK_SECRET_KEY=

PAYME_CHECKOUT_URL=https://test.paycom.uz
PAYME_MERCHANT_ID=
PAYME_SECRET_KEY=

PLATFORM_FEE_BPS=0
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

## Payme sandbox-flow

1. В Payme Business создать веб-кассу и взять `Merchant ID` и `TEST_KEY`.
2. В настройках веб-кассы указать Endpoint URL: `https://your-domain/api/payments/payme/callback`.
3. В Clubpay задать `PAYME_MERCHANT_ID` и `PAYME_SECRET_KEY` (`TEST_KEY`) через env или настройки клуба.
4. Открыть QR, выбрать Payme и оплатить через `https://test.paycom.uz`.
5. Payme sandbox должен дернуть Merchant API: `CheckPerformTransaction`, `CreateTransaction`, `PerformTransaction`, `CheckTransaction`, `CancelTransaction`, `GetStatement`.

## Click testing

Платежная ссылка Click строится только когда есть `CLICK_MERCHANT_ID`, `CLICK_SERVICE_ID` и `CLICK_SECRET_KEY`. Без этих значений QR-страница не даст открыть Click, чтобы не получать ошибку `Поставщик не найден или заблокирован`.

Публичная Click-дока описывает ссылку `https://my.click.uz/services/pay/?service_id=...&merchant_id=...&amount=...&transaction_param=...&return_url=...` и обязательный SHOP API callback-контур Prepare/Complete. Открытых sandbox credentials в публичной доке не указано, поэтому до выдачи тестовых/боевых доступов можно тестировать только локальную обработку callback-ов и полный mock-flow.

## Важные решения MVP

- Суммы хранятся в тийинах.
- `invoice_id` = наш `order_id`.
- `provider` = `click`, `payme` или `mock`.
- `provider_payment_id` = ID транзакции у провайдера.
- Разблокировка ПК происходит только после валидного успешного callback.
- Страница возврата сама опрашивает `/api/orders/{invoice_id}`.
- Фискализация вынесена отдельно: после онлайн-оплаты заказ получает `fiscal_status=pending`.
- Наличные логируются отдельно и запускают сессию без онлайн-провайдера.
- Core/iCafe-аналог пока может работать в `CORE_MODE=mock`.

## Production-блокеры

- Получить production-доступы Click и Payme.
- Подтвердить callback URLs, подписи, retry и IP/HTTPS требования у провайдеров.
- Подключить прямую Soliq/OFD фискализацию: MXIK/ИКПУ, package_code, НДС, продавец по чеку.
- Уточнить split/комиссии/выплаты по Click и Payme.
- Решить фискализацию наличных через онлайн-кассу/Soliq.

## Core/iCafe-аналог

По умолчанию используется mock:

```bash
CORE_MODE=mock
```

Когда контроллер от второго разработчика готов:

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

Минимальные события: `pc_status_changed`, `session_started`, `session_ended`, `session_failed`, `command_failed`.

## Продление сессии

Для MVP продление делаем через тот же физический QR на ПК/столе. Если ПК уже занят и у него есть активная сессия, новая успешная онлайн-оплата, наличная оплата или ваучер не создают вторую сессию, а вызывают `POST /core/v1/sessions/{core_session_id}/extend` и добавляют минуты к текущему окончанию.

QR внутри будущего клиентского приложения можно добавить позже как дополнительный удобный вход, но физический QR остаётся fallback-вариантом.

## Raspberry Pi edge mode

Raspberry Pi запускает тот же API, web и Postgres, но в режиме локального сервера клуба. Если облако недоступно, клиенты и менеджеры работают через Pi: QR-страница, Click/Payme checkout, callbacks провайдеров, наличка, ваучеры, панель менеджера, сессии и команды в Core/Agent выполняются локально.

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
CORE_MODE=http
CORE_BASE_URL=http://controller.local:8081
```

Важно для онлайн-оплат: если падает только наше облако, Pi продолжает принимать Click/Payme при условии, что у него есть интернет до провайдера и публичный HTTPS endpoint для callback. Для этого `PUBLIC_BASE_URL` на Pi должен быть доступен провайдерам через домен/tunnel. Если callback URL у Payme/Click закреплён только за облаком, нужен failover DNS/proxy или отдельная provider-настройка на edge endpoint клуба, иначе провайдер не сможет подтвердить оплату на Pi. Если интернета нет вообще, онлайн-оплата невозможна технически; остаются наличные, локальные ваучеры и ручное управление сессиями.

Для Telegram-ваучеров действует то же правило: бот может доставить `/start` в Pi только если webhook смотрит на Pi/failover-домен или если для этого окружения используется polling без активного webhook. У Telegram у одного бота только один webhook, поэтому это надо решить в инфраструктуре до production fallback-теста.

Синхронизация:

- cloud и edge используют один код и одну схему БД;
- `GET /api/edge/snapshot?club_id=...` отдаёт полный снимок клуба: настройки, зоны, пакеты, ПК/QR, пользователей, платежи, наличку, сессии, ваучеры и Telegram-привязки;
- `POST /api/edge/events` принимает `edge_snapshot` от Pi и применяет его в облаке;
- в `NODE_MODE=edge` API автоматически пушит локальный snapshot в `CLOUD_BASE_URL`, а после успешного push подтягивает snapshot обратно;
- журнал синхронизации хранится в `edge_sync_runs`.

Для production задайте `EDGE_SYNC_TOKEN` и передавайте его как `Authorization: Bearer <token>` или `X-Edge-Token`.

## Telegram ваучеры

После ручного завершения сессии менеджер может указать номер телефона. Если остаток времени больше нуля, номер сохраняется в БД и создаётся ваучер. Если Telegram уже привязан к этому номеру, ваучер отправится автоматически. Если номер еще не привязан, API вернет одноразовую ссылку `https://t.me/<bot>?start=<token>`: клиент открывает ее один раз, бот сохраняет `chat_id` и сразу отправляет ожидающий ваучер.

Webhook бота:

```http
POST /api/telegram/webhook
X-Telegram-Bot-Api-Secret-Token: <TELEGRAM_WEBHOOK_SECRET>
```

`TELEGRAM_BOT_USERNAME` можно указать явно без `@`. Если он пустой, API попробует получить username через Telegram `getMe`.

Для локального тестирования можно не поднимать публичный webhook: в development включён `TELEGRAM_POLLING_ENABLED=true`, и API сам забирает `/start` через Telegram `getUpdates`, если у бота не настроен webhook. В production лучше использовать webhook и выключить polling.
