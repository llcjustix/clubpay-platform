# ClubPay Controller Node

This is the local authority for a club. Install it on the primary Windows server or Raspberry Pi.
Install the same package on the manager PC with `NODE_MODE=manager` as the final fallback.

It is **not** a Wake-on-LAN-only relay. It runs a local API, local PWA/admin panel, local
PostgreSQL cache, sessions, cash payments, vouchers, Agent commands, WoL and idempotent cloud sync.
When the cloud is reachable it synchronizes with it; when it is not, the club continues on the local node.

## Windows Server / manager PC

1. Extract the release zip into `C:\ClubPay\Controller` (use a different directory on the manager PC,
   for example `C:\ClubPay\ManagerController`). Install and start Docker Desktop once.
2. Open `controller.env.example`, replace every `CHANGE_ME`, save it as `controller.env`.
   Set `NODE_MODE=edge` for the primary Controller and `NODE_MODE=manager` on the manager PC.
   The `EDGE_SYNC_TOKEN`, club UUID and shared `CORE_TOKEN` must be supplied from the Cloud deployment.
3. Right-click `install-windows.cmd` -> **Run as administrator**.
4. Verify in a browser: `http://localhost:8080/api/node/status`. It must show `local_authority: true`.

## Raspberry Pi (64-bit)

1. Extract the Linux ARM64 release into `/opt/clubpay-controller` and install Docker Engine + Compose.
2. `cp controller.env.example controller.env`, edit it as above, then run `sudo ./install-linux.sh`.
3. Verify: `curl http://127.0.0.1:8080/api/node/status`.

## What works during an outage

- sessions, Agent commands, PC status, Wake-on-LAN, cash and vouchers work against the local node;
- the local `/admin` panel is available from the club LAN;
- all already synchronized data is retained locally and synchronization resumes automatically when Cloud returns.

Card payments during a Cloud outage are enabled only if the club approves and installs the merchant
credentials on its protected primary Controller. Manager fallback deliberately defaults to cash/vouchers:
it must not duplicate card charges while connectivity is ambiguous.

## Telegram limitation during a total Cloud outage

Telegram Mini Apps always load the single HTTPS URL configured in BotFather. They cannot be redirected
to `http://192.168.x.x` inside an isolated club LAN. Therefore the packaged local Controller defaults
to a direct local PWA QR (`TELEGRAM_MINI_APP_ENABLED=false`) and keeps cash/voucher flows available.
Profile login through Telegram continues when Telegram plus the public Mini App are reachable; a fully
offline identity flow needs a separate local credential/card/PIN decision and must not be faked by a
device ID.

## Required network setup

Give each Controller a fixed LAN IP, use Ethernet, and open TCP 8080 inside the club LAN. Agent Core
needs the controller WebSocket URL and bootstrap URL of the primary node plus fallback nodes. The PWA
is served by the Controller itself, so its API base is local and does not depend on the public website.
