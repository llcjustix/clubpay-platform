# Raspberry Pi: ClubPay WoL relay

Это маленький сервис для Raspberry Pi. Он нужен **только**, чтобы разбудить спящий игровой ПК
по Wake-on-LAN. Оплата, QR, Telegram Mini App и профиль игрока остаются в облаке.

## Установить

Подключите Pi к сети клуба по Ethernet. На ней должны быть Docker Engine и Docker Compose.

```bash
sudo git clone https://github.com/llcjustix/clubpay-platform.git /opt/clubpay-platform
cd /opt/clubpay-platform
sudo install -d -m 700 /etc/clubpay
sudo cp deploy/pi/edge-wol.env.example /etc/clubpay/edge-wol.env
sudo nano /etc/clubpay/edge-wol.env
```

В файле укажите только выданный отдельно `EDGE_WOL_TOKEN`. Не используйте `CORE_TOKEN` и не
публикуйте этот файл.

```bash
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml logs -f edge-wol
```

Готово, если в логе есть `connected to ClubPay Cloud`.

## Обновить

```bash
cd /opt/clubpay-platform
sudo git pull --ff-only origin main
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build
```

Если Pi уже работает — не копируйте заново `/etc/clubpay/edge-wol.env`: в нём хранится секрет.
