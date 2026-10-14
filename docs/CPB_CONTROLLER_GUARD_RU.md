# Controller guard для изолированного CPB-пилота

Интеграция включается явно `CPB_GUARD_TOKEN` и `CPB_GUARD_CLUB_ID`. Используется отдельный длинный токен, не `CORE_TOKEN`, не платёжные или Cloud credentials. В текущем стенде новый Controller разворачивается отдельно на `.159:8090`; существующие `.152` Manager и `.153` edge Controller не обновлялись. Роль `.151` по процессам пока не подтверждена.

`POST /api/cpb/v1/stations` регистрирует существующий PC по `(club_id, external_pc_id)`. `cold_provision=true` разрешает единственный первоначальный bootstrap без подключённого Agent. Это явное enrolment-разрешение для нового холодного клиента; обновление/rollback без доступного Agent запрещены. После release cold-флаг сбрасывается.

`POST /api/cpb/v1/guard` — свежий снимок для допуска задания. `POST /api/cpb/v1/leases/acquire|validate|release` — сохраняемая блокировка и возрастающий fence. TTL — 10–600 секунд. Lease привязан к станции и command ID. Истечение срока снимает право на дисковую операцию, но **не снимает карантин станции**. Освобождение требует точных lease ID и fence. Не вызывать release при неопределённом состоянии диска; сначала сверить журнал Boot Node, фактический LUN и загрузку гостя.

Захват lease, запись новых grants/броней и отправка любых изменяющих состояние WebSocket-команд сериализуются PostgreSQL advisory lock. SQL triggers блокируют все пути создания grant и брони для зарегистрированной CPB-станции. Весь WebSocket mutation ACK находится под той же блокировкой. Снимок отдельно от lease не разрешает операцию. Для нескольких Controller с независимыми локальными БД этот пилотный guard не является общим арбитром: требуется общий scheduling authority или межузловой протокол. Подключать оригинальный Manager failover к пилоту без такого механизма нельзя.

При failed canary явный rollback может перенести существующий карантин на новый command ID с `recovery_command_id`, прежними lease ID и fence. Проверки активной сессии, брони и доступного некритического Agent повторяются. Нет live guard — rollback не выполняется.

Agent возвращает `agent_critical`; старые Agents без поля блокируют обновление. Состояние критической detached-операции сохраняется в текущем процессе до его замены. Зависший/неудачный updater требует сверки; самопроизвольного допуска CPB нет.

Проверки: `go test ./...`, `go vet ./...`; PostgreSQL: `MOBILE_TEST_DATABASE_URL=<disposable database> go test -race ./internal/httpapi -run TestBootGuard -count=1`. Схемы тестов изолированы. Windows Agent CI проверяет сборку и все .NET-тесты. Runtime boot/session-проверки учитываются отдельно от тестов модели транспорта.
