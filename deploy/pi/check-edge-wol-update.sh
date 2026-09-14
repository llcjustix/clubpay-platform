#!/usr/bin/env bash
set -euo pipefail
root=/opt/clubpay-edge-wol
repo=llcjustix/clubpay-platform
asset=clubpay-edge-wol-linux-arm64.tar.gz
current="$(cat "$root/version" 2>/dev/null || true)"
stage="$(mktemp -d)"
archive="$(mktemp)"
checksum="$(mktemp)"
cleanup() { rm -rf "$stage" "$archive" "$checksum"; }
trap cleanup EXIT

release_json="$(curl --fail --location --silent --show-error "https://api.github.com/repos/$repo/releases?per_page=30")"
python3 - "$release_json" "$asset" "$current" <<'PY' >"$stage/release"
import json, sys
releases, asset, current = json.loads(sys.argv[1]), sys.argv[2], sys.argv[3].strip()
for release in releases:
    if release.get("draft") or release.get("prerelease"):
        continue
    tag = release.get("tag_name", "")
    if not tag.startswith("edge-") or tag == current:
        continue
    files = {item.get("name"): item.get("browser_download_url") for item in release.get("assets", [])}
    if files.get(asset) and files.get(asset + ".sha256"):
        print(tag); print(files[asset]); print(files[asset + ".sha256"]); break
PY
[[ -s "$stage/release" ]] || exit 0
mapfile -t release <"$stage/release"
tag="${release[0]}"; url="${release[1]}"; checksum_url="${release[2]}"
curl --fail --location --silent --show-error "$url" -o "$archive"
curl --fail --location --silent --show-error "$checksum_url" -o "$checksum"
expected="$(awk '{print $1}' "$checksum" | tr '[:upper:]' '[:lower:]')"
actual="$(sha256sum "$archive" | awk '{print $1}')"
[[ "$expected" =~ ^[a-f0-9]{64}$ && "$actual" == "$expected" ]] || { logger -t clubpay-update "update_event component=edge-wol action=checksum_failed version=$tag"; exit 1; }
mkdir -p "$stage/bundle"
tar -xzf "$archive" -C "$stage/bundle"
"$stage/bundle/update-edge-wol.sh" --bundle "$stage/bundle" --version "$tag"
