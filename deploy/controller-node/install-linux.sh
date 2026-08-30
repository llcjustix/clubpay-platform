#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$ROOT"

if [ ! -f controller.env ]; then
  cp controller.env.example controller.env
  echo "Edit $ROOT/controller.env: replace CHANGE_ME and set the node LAN IP, then run this script again."
  exit 1
fi

docker compose --env-file controller.env -f postgres-compose.yml up -d
sudo tee /etc/systemd/system/clubpay-controller.service >/dev/null <<EOF
[Unit]
Description=ClubPay Controller Node
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$ROOT
ExecStart=$ROOT/clubpay-controller --config $ROOT/controller.env
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now clubpay-controller
echo "Controller installed. Verify: curl http://127.0.0.1:8080/api/node/status"
