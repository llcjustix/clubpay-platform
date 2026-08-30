# ClubPay LAN relay for a Windows club server

Use this only when the club needs to wake sleeping game PCs. It is optional for
payments, QR, Telegram Mini App and the normal Agent Core flow.

1. Download `clubpay-edge-wol-windows-amd64.zip` from the GitHub Releases page,
   extract it to `C:\ClubPay\EdgeWoL` and copy `edge-wol.env.example` to
   `edge-wol.env`.
2. Paste the separately supplied `EDGE_WOL_TOKEN` and club UUID into the file.
   Do not use `CORE_TOKEN` and do not send the completed file through chat.
3. Open **Command Prompt as administrator**, then run:

   ```bat
   cd /d C:\ClubPay\EdgeWoL
   clubpay-edge-wol.exe --install --config C:\ClubPay\EdgeWoL\edge-wol.env
   ```

4. Restart Windows once, or start the task `ClubPay Edge WoL` from Task Scheduler.
   Its history must show that it connected to ClubPay Cloud.

To remove it later, run `clubpay-edge-wol.exe --uninstall` as administrator.

The account that runs the relay has no access to payments, player profiles or
the admin panel. It accepts only authenticated wake requests from ClubPay Cloud.
