# Raspberry Pi: ClubPay WoL relay

Это маленький сервис для Raspberry Pi. Он нужен **только**, чтобы разбудить спящий игровой ПК
по Wake-on-LAN. Оплата, QR, Telegram Mini App и профиль игрока остаются в облаке.

## Установить

Подключите Pi к сети клуба по Ethernet. Для автоматических безопасных обновлений
используется обычный systemd-сервис: отдельный updater проверяет SHA-256,
перезапускает relay и за минуту возвращает предыдущую версию, если процесс не
стал стабильно работать.

```bash
Распакуйте релиз `clubpay-edge-wol-linux-arm64.tar.gz`, затем:

```bash
sudo ./install-edge-wol.sh
sudo nano /etc/clubpay/edge-wol.env
```
```

В файле укажите только выданный отдельно `EDGE_WOL_TOKEN`. Не используйте `CORE_TOKEN` и не
публикуйте этот файл.

```bash
sudo systemctl status clubpay-edge-wol
```

Готово, если в логе есть `connected to ClubPay Cloud`.

## Обновление

Таймер проверяет релизы каждые 5 минут. Он не заменяет текущий relay, пока
архив и SHA-256 не совпали. Состояние: `systemctl status clubpay-edge-wol-update.timer`.
При переходе со старого Docker-варианта остановите compose один раз и выполните
установку выше. Не копируйте заново `/etc/clubpay/edge-wol.env`: в нём секрет.
