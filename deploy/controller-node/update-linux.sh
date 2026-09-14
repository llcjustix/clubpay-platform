#!/usr/bin/env bash
# Invoked from a checksum-verified Controller release bundle. It keeps the
# previous executable, web and migrations on disk until the replacement has
# served a local health check for one minute.
set -euo pipefail

[[ "${1:-}" == "--no-prompt" || -z "${1:-}" ]] || { echo "unknown option: $1" >&2; exit 2; }
bundle="$(cd "$(dirname "$0")" && pwd)"
new_binary="$bundle/clubpay-controller"
[[ -x "$new_binary" || -f "$new_binary" ]] || { echo "clubpay-controller is missing from update bundle" >&2; exit 1; }

find_target() {
  if [[ -n "${CLUBPAY_CONTROLLER_DIR:-}" && -f "$CLUBPAY_CONTROLLER_DIR/controller.env" ]]; then
    printf '%s\n' "$CLUBPAY_CONTROLLER_DIR"; return
  fi
  local candidate
  candidate="$(find /opt /usr/local /home -type f -name controller.env -print 2>/dev/null | while read -r config; do
    root="$(dirname "$config")"
    [[ -x "$root/clubpay-controller" || -f "$root/clubpay-controller" ]] && printf '%s\n' "$root"
  done | head -n1)"
  [[ -n "$candidate" ]] && printf '%s\n' "$candidate"
}

target="$(find_target)"
[[ -n "$target" ]] || { echo "configured Controller was not found" >&2; exit 1; }
[[ -f "$target/controller.env" ]] || { echo "controller.env is missing" >&2; exit 1; }

versions="$target/updates/versions"
backup="$versions/previous"
rm -rf "$backup"
mkdir -p "$backup"
cp "$target/clubpay-controller" "$backup/clubpay-controller"
for directory in web migrations; do
  [[ -d "$target/$directory" ]] && cp -a "$target/$directory" "$backup/$directory"
done

rollback() {
  echo "Controller update failed; restoring previous build" >&2
  systemctl stop clubpay-controller.service || true
  cp "$backup/clubpay-controller" "$target/clubpay-controller"
  chmod +x "$target/clubpay-controller"
  for directory in web migrations; do
    [[ -d "$backup/$directory" ]] && { rm -rf "$target/$directory"; cp -a "$backup/$directory" "$target/$directory"; }
  done
  systemctl start clubpay-controller.service || true
}
trap rollback ERR

systemctl stop clubpay-controller.service
cp "$new_binary" "$target/clubpay-controller"
chmod +x "$target/clubpay-controller"
for directory in web migrations; do
  [[ -d "$bundle/$directory" ]] || { echo "bundle missing $directory" >&2; exit 1; }
  rm -rf "$target/$directory"
  cp -a "$bundle/$directory" "$target/$directory"
done
systemctl start clubpay-controller.service

port="$(awk -F= '$1 == "HTTP_ADDR" {sub(/^:/,"",$2); print $2}' "$target/controller.env" | tail -n1)"
port="${port:-8080}"
healthy=false
for _ in $(seq 1 20); do
  if curl --fail --silent --max-time 5 "http://127.0.0.1:${port}/api/node/status" | grep -q '"ok":true'; then
    healthy=true; break
  fi
  sleep 3
done
[[ "$healthy" == true ]] || { echo "Controller did not become healthy after 60 seconds" >&2; exit 1; }
trap - ERR
echo "Controller updated successfully; previous build retained at $backup"
