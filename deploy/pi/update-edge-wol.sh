#!/usr/bin/env bash
set -euo pipefail
root=/opt/clubpay-edge-wol
bundle=""
version=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bundle) bundle="$2"; shift 2 ;;
    --version) version="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$bundle" && -n "$version" && -f "$bundle/clubpay-edge-wol" ]] || { echo "invalid relay update bundle" >&2; exit 1; }
backup="$root/updates/versions/previous"
rm -rf "$backup"; mkdir -p "$backup"
cp "$root/clubpay-edge-wol" "$backup/clubpay-edge-wol"
cp "$root/version" "$backup/version" 2>/dev/null || true
rollback() {
  logger -t clubpay-update "update_event component=edge-wol action=rollback version=$version"
  systemctl stop clubpay-edge-wol.service || true
  cp "$backup/clubpay-edge-wol" "$root/clubpay-edge-wol"
  [[ -f "$backup/version" ]] && cp "$backup/version" "$root/version"
  chmod +x "$root/clubpay-edge-wol"
  systemctl start clubpay-edge-wol.service || true
}
trap rollback ERR
systemctl stop clubpay-edge-wol.service
install -m 755 "$bundle/clubpay-edge-wol" "$root/clubpay-edge-wol"
install -m 755 "$bundle/update-edge-wol.sh" "$root/update-edge-wol.sh"
install -m 755 "$bundle/check-edge-wol-update.sh" "$root/check-edge-wol-update.sh"
printf '%s\n' "$version" >"$root/version"
systemctl start clubpay-edge-wol.service
for _ in $(seq 1 20); do
  systemctl is-active --quiet clubpay-edge-wol.service && { trap - ERR; logger -t clubpay-update "update_event component=edge-wol action=healthy version=$version"; exit 0; }
  sleep 3
done
echo "relay did not remain running for 60 seconds" >&2
exit 1
