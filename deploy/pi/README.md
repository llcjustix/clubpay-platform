# ClubPay Raspberry Pi: Wake-on-LAN relay

This is the supported Raspberry Pi component for the current cloud pilot.
It is deliberately small: payment, profile, QR and Telegram Mini App stay in
ClubPay Cloud; the Pi only sends a Wake-on-LAN magic packet inside the club LAN.
This means a real player follows exactly the normal Mini App flow.

## Before installation

1. Use Raspberry Pi OS **64-bit** on Pi 4/5 with Ethernet and a fixed DHCP lease.
2. Install Docker Engine and the Compose plugin following Docker's official
   instructions for your Raspberry Pi OS / Debian ARM64 release.
3. Configure the production GitHub secret `EDGE_WOL_TOKEN` and the production
   variable `WOL_ENABLED=true`. Use a unique random token; do not reuse
   `CORE_TOKEN` and do not put it into Git.
4. In the ClubPay admin panel, add the wired MAC address to every PC that this
   Pi should wake. Enable Wake-on-LAN in the PC BIOS and Windows NIC settings.

## Install from Git

Run on the Pi:

```bash
git clone https://github.com/llcjustix/clubpay-platform.git /opt/clubpay-platform
cd /opt/clubpay-platform
sudo install -d -m 700 /etc/clubpay
sudo cp deploy/pi/edge-wol.env.example /etc/clubpay/edge-wol.env
sudo nano /etc/clubpay/edge-wol.env
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build
```

In `/etc/clubpay/edge-wol.env`, fill only `EDGE_WOL_TOKEN`; the pilot club ID
and endpoint are already prefilled. Never publish this file.

## Verify

```bash
cd /opt/clubpay-platform
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml ps
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml logs -f edge-wol
```

The log must say that the node connected to ClubPay Cloud. Then put the test PC
to sleep, verify that it is marked `sleeping`, and start it through the normal
ClubPay flow. Cloud asks this Pi for WoL, waits for the Agent to reconnect, and
only then starts the session.

## Update

```bash
cd /opt/clubpay-platform
git pull --ff-only origin main
sudo docker compose -f deploy/pi/docker-compose.edge-wol.yml up -d --build
```

This relay is not a cloud-outage fallback and does not host a local QR/payment
site. A full offline edge node remains a separate product phase.
