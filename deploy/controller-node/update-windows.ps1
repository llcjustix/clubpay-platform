$ErrorActionPreference = 'Stop'

$bundle = Split-Path -Parent $MyInvocation.MyCommand.Path
$newExecutable = Join-Path $bundle 'ClubPay.Controller.exe'
if (-not (Test-Path $newExecutable)) {
    throw "ClubPay.Controller.exe was not found next to the updater: $newExecutable"
}

# The active Controller is the installation that owns controller.env. Do not
# trust the folder name: release folders commonly include a version suffix.
$candidate = Get-ChildItem -Path 'C:\ClubPay' -Filter 'controller.env' -File -Recurse -ErrorAction SilentlyContinue |
    ForEach-Object {
        $root = $_.Directory.FullName
        if (Test-Path (Join-Path $root 'ClubPay.Controller.exe')) {
            [pscustomobject]@{ Root = $root; Updated = $_.LastWriteTimeUtc }
        }
    } |
    Sort-Object Updated -Descending |
    Select-Object -First 1

if ($null -eq $candidate) {
    throw 'Configured Controller was not found under C:\ClubPay. Use install-windows.cmd for the first installation.'
}

$target = $candidate.Root
Write-Host "Updating active Controller: $target" -ForegroundColor Cyan

# Stop the scheduled app before replacing its executable. The task remains
# registered and continues to point to this same stable folder after update.
schtasks.exe /End /TN 'ClubPay Controller Node' 2>$null | Out-Null
Get-Process -Name 'ClubPay.Controller' -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

Copy-Item -Path $newExecutable -Destination (Join-Path $target 'ClubPay.Controller.exe') -Force
foreach ($directory in @('web', 'migrations')) {
    $source = Join-Path $bundle $directory
    if (-not (Test-Path $source)) {
        throw "Update package is incomplete: $source"
    }
    $destination = Join-Path $target $directory
    New-Item -ItemType Directory -Path $destination -Force | Out-Null
    Copy-Item -Path (Join-Path $source '*') -Destination $destination -Recurse -Force
}

# controller.env, data and runtime are deliberately never copied or deleted.
schtasks.exe /Run /TN 'ClubPay Controller Node' | Out-Null

# The embedded PostgreSQL instance can take longer than a few seconds to open
# its existing data directory after a Windows reboot or an upgrade. Poll the
# local endpoint rather than treating a normal cold start as a failed update.
$lastError = ''
$healthy = $false
for ($attempt = 1; $attempt -le 18; $attempt++) {
    try {
        $status = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/api/node/status' -TimeoutSec 5
        if ($status.ok) {
            $healthy = $true
            break
        }
        $lastError = 'Controller status is not ok'
    }
    catch {
        $lastError = $_.Exception.Message
    }
    Start-Sleep -Seconds 3
}
if (-not $healthy) {
    throw "Files were updated, but Controller did not become healthy after 54 seconds. $lastError"
}

Write-Host ''
Write-Host 'Controller updated successfully.' -ForegroundColor Green
Write-Host 'Open http://localhost:8080/admin to prepare Agent packages.' -ForegroundColor Green
Read-Host 'Press Enter to close'
