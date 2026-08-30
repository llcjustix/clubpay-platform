#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [[ -f controller.env ]]; then
  echo "This Controller is already configured. Check: http://localhost:8080/api/node/status"
  exit 0
fi

read -r -p "Paste one-time activation code from ClubPay Web Admin: " ACTIVATION_CODE
if [[ -z "$ACTIVATION_CODE" ]]; then
  echo "Activation code is required."
  exit 1
fi

sudo ./clubpay-controller --setup --activation-code "$ACTIVATION_CODE"
echo "Ready. Check: http://localhost:8080/api/node/status"
