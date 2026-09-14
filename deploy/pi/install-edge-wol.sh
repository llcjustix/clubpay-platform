#!/usr/bin/env bash
# First install for the lightweight Raspberry Pi LAN relay. The relay runs as
# one systemd service; a separate timer checks signed/checksummed releases and
# never runs a second relay beside the active one.
set -euo pipefail
bundle="$(cd "$(dirname "$0")" && pwd)"
install_root=/opt/clubpay-edge-wol
config_dir=/etc/clubpay

[[ $EUID -eq 0 ]] || { echo "run with sudo" >&2; exit 1; }
[[ -f "$bundle/clubpay-edge-wol" ]] || { echo "relay binary missing" >&2; exit 1; }
install -d -m 755 "$install_root" "$install_root/updates/versions" "$config_dir"
install -m 755 "$bundle/clubpay-edge-wol" "$install_root/clubpay-edge-wol"
install -m 755 "$bundle/update-edge-wol.sh" "$install_root/update-edge-wol.sh"
install -m 755 "$bundle/check-edge-wol-update.sh" "$install_root/check-edge-wol-update.sh"
install -m 644 "$bundle/clubpay-edge-wol-version" "$install_root/version"
if [[ ! -f "$config_dir/edge-wol.env" ]]; then
  install -m 600 "$bundle/edge-wol.env.example" "$config_dir/edge-wol.env"
  echo "Set EDGE_WOL_TOKEN in $config_dir/edge-wol.env before starting." >&2
fi
cat >/etc/systemd/system/clubpay-edge-wol.service <<EOF
[Unit]
Description=ClubPay LAN Wake-on-LAN relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$config_dir/edge-wol.env
ExecStart=$install_root/clubpay-edge-wol
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
cat >/etc/systemd/system/clubpay-edge-wol-update.service <<EOF
[Unit]
Description=Check and safely update ClubPay LAN relay
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=$install_root/check-edge-wol-update.sh
EOF
cat >/etc/systemd/system/clubpay-edge-wol-update.timer <<EOF
[Unit]
Description=ClubPay LAN relay update check

[Timer]
OnBootSec=3min
OnUnitActiveSec=5min
RandomizedDelaySec=30

[Install]
WantedBy=timers.target
EOF
systemctl daemon-reload
systemctl enable --now clubpay-edge-wol.service clubpay-edge-wol-update.timer
echo "ClubPay LAN relay installed. Existing config remains in $config_dir/edge-wol.env"
