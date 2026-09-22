$ErrorActionPreference = 'Stop'

# This task is deliberately quiet. It only applies a newer immutable Controller
# release when the local node reports no active root session; an update must
# never interrupt a player in the middle of a game.
$installDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path
$currentMarker = Join-Path $installDirectory 'clubpay-version.json'
if (-not (Test-Path $currentMarker)) { exit 0 }

try {
    $node = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/api/node/status' -TimeoutSec 5
    if (-not $node.ok -or [int]$node.active_session_count -gt 0) { exit 0 }

    $current = (Get-Content -LiteralPath $currentMarker -Raw | ConvertFrom-Json).version
    $release = Invoke-RestMethod -Uri 'https://api.github.com/repos/llcjustix/clubpay-platform/releases/latest' -TimeoutSec 15
    $latest = [string]$release.tag_name
    if ([string]::IsNullOrWhiteSpace($latest) -or $latest -notlike 'controller-v*' -or $latest -eq [string]$current) { exit 0 }

    $asset = @($release.assets | Where-Object { $_.name -eq 'ClubPay-Controller-win-x64.zip' })[0]
    $checksumAsset = @($release.assets | Where-Object { $_.name -eq 'ClubPay-Controller-win-x64.zip.sha256' })[0]
    if ($null -eq $asset -or $null -eq $checksumAsset) { throw 'Controller release is missing Windows assets.' }

    $stage = Join-Path $env:TEMP ('clubpay-controller-update-' + [guid]::NewGuid().ToString())
    $zip = Join-Path $env:TEMP ('clubpay-controller-update-' + [guid]::NewGuid().ToString() + '.zip')
    $checksum = Join-Path $env:TEMP ('clubpay-controller-update-' + [guid]::NewGuid().ToString() + '.sha256')
    try {
        New-Item -ItemType Directory -Path $stage -Force | Out-Null
        Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $zip
        Invoke-WebRequest -UseBasicParsing -Uri $checksumAsset.browser_download_url -OutFile $checksum
        $expected = ((Get-Content -LiteralPath $checksum -Raw).Trim() -split '\s+')[0].ToLowerInvariant()
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLowerInvariant()
        if ($expected -notmatch '^[a-f0-9]{64}$' -or $actual -ne $expected) { throw 'Controller release checksum verification failed.' }
        Expand-Archive -Path $zip -DestinationPath $stage -Force
        $updater = Get-ChildItem -Path $stage -Filter 'update-windows.ps1' -File -Recurse | Select-Object -First 1
        if ($null -eq $updater) { throw 'Controller release does not contain its updater.' }
        & $updater.FullName -NoPrompt
    }
    finally {
        Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $checksum -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}
catch {
    # The scheduled task must retry on the next interval. Do not kill or roll
    # back a known-good Controller for a transient GitHub/network failure.
    Write-EventLog -LogName Application -Source 'ClubPay Controller' -EntryType Warning -EventId 1002 -Message "Automatic Controller update skipped: $($_.Exception.Message)" -ErrorAction SilentlyContinue
}
