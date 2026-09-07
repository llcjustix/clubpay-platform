package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"clubpay/internal/httpapi"
	"clubpay/internal/release"
)

const (
	platformReleasesAPI = "https://api.github.com/repos/llcjustix/clubpay-platform/releases?per_page=30"
	agentReleasesAPI    = "https://api.github.com/repos/llcjustix/clubpay-core-agent/releases?per_page=30"
)

// runAutomaticUpdateLoop is intentionally run only by an installed local
// node. Cloud has no right to rewrite a club LAN process; every update is
// pulled by the machine that owns its own configuration and data.
func runAutomaticUpdateLoop(ctx context.Context, cfg config.Config, server *httpapi.Server, currentVersion string) {
	mode := strings.ToLower(strings.TrimSpace(cfg.NodeMode))
	if !cfg.AutoUpdateEnabled || (mode != "edge" && mode != "manager") {
		return
	}
	interval := time.Duration(cfg.AutoUpdateCheckSeconds) * time.Second
	if interval < 5*time.Minute {
		interval = 5 * time.Minute
	}
	check := func() {
		checkAutomaticUpdates(ctx, cfg, server, currentVersion)
	}
	// Let PostgreSQL and the first cloud synchronization settle before an
	// upgrade check. Subsequent checks are deliberately infrequent.
	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			check()
			timer.Reset(interval)
		}
	}
}

func checkAutomaticUpdates(ctx context.Context, cfg config.Config, server *httpapi.Server, currentVersion string) {
	mode := strings.ToLower(strings.TrimSpace(cfg.NodeMode))
	if mode == "edge" {
		agentArtifact, err := release.Latest(ctx, nil, agentReleasesAPI, "v", "ClubPay-Agent-win-x64.zip")
		if err != nil {
			log.Printf("automatic Agent update lookup: %v", err)
		} else {
			server.SetAgentUpdateRelease(core.AgentUpdateCommand{
				Version: agentArtifact.Version, DownloadURL: agentArtifact.DownloadURL, ChecksumURL: agentArtifact.ChecksumURL,
			})
		}
	}

	var (
		artifact     release.Artifact
		err          error
		archiveName string
		updaterName string
	)
	if mode == "manager" {
		archiveName = "ClubPay-Manager-Desktop-win-x64.zip"
		updaterName = "update-manager.ps1"
		artifact, err = release.Latest(ctx, nil, agentReleasesAPI, "v", archiveName)
	} else {
		archiveName = "ClubPay-Controller-win-x64.zip"
		updaterName = "update-windows.ps1"
		artifact, err = release.Latest(ctx, nil, platformReleasesAPI, "controller-v", archiveName)
	}
	if err != nil {
		log.Printf("automatic %s update lookup: %v", mode, err)
		return
	}
	if release.CompareVersions(artifact.Version, currentVersion) <= 0 {
		clearCompletedUpdateMarker()
		return
	}
	if err := scheduleWindowsUpdate(artifact, updaterName); err != nil {
		log.Printf("schedule automatic %s update to %s: %v", mode, artifact.Version, err)
		return
	}
	log.Printf("automatic %s update to %s was scheduled", mode, artifact.Version)
}

func controllerInstallRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(executable)
}

func updateMarkerPath() string {
	root := controllerInstallRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "auto-update.pending")
}

func clearCompletedUpdateMarker() {
	if marker := updateMarkerPath(); marker != "" {
		_ = os.Remove(marker)
	}
}

func scheduleWindowsUpdate(artifact release.Artifact, updaterName string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("automatic local updates are currently supported on Windows Controllers")
	}
	root := controllerInstallRoot()
	if root == "" {
		return fmt.Errorf("locate Controller install directory")
	}
	marker := updateMarkerPath()
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == artifact.Version {
		return nil
	}
	updatesDir := filepath.Join(root, "updates")
	if err := os.MkdirAll(updatesDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(artifact.Version+"\n"), 0o600); err != nil {
		return err
	}
	scriptPath := filepath.Join(updatesDir, "apply-"+safeUpdateFilePart(artifact.Version)+".ps1")
	script := automaticUpdateScript(artifact, updaterName, marker)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		_ = os.Remove(marker)
		return err
	}
	// Run through its own scheduled task, not as a child of this Controller.
	// update-windows.ps1 intentionally ends the Controller task, and Task
	// Scheduler could otherwise terminate the helper with the process tree.
	command := fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%s"`, scriptPath)
	result, err := exec.Command("schtasks.exe", "/Create", "/TN", "ClubPay Automatic Update", "/SC", "ONCE", "/ST", "00:00", "/RU", "SYSTEM", "/RL", "HIGHEST", "/TR", command, "/F").CombinedOutput()
	if err != nil {
		_ = os.Remove(marker)
		return fmt.Errorf("create automatic update task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	result, err = exec.Command("schtasks.exe", "/Run", "/TN", "ClubPay Automatic Update").CombinedOutput()
	if err != nil {
		_ = os.Remove(marker)
		return fmt.Errorf("run automatic update task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func safeUpdateFilePart(value string) string {
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return '-'
	}, value)
	return strings.Trim(value, "-")
}

func automaticUpdateScript(artifact release.Artifact, updaterName, markerPath string) string {
	// URLs are GitHub release asset URLs controlled by the ClubPay repositories.
	// Quote them as single-quoted PowerShell literals after rejecting apostrophes.
	clean := func(value string) string { return strings.ReplaceAll(value, "'", "") }
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
Start-Sleep -Seconds 5
$stage = Join-Path $env:TEMP ('clubpay-auto-update-' + [guid]::NewGuid().ToString())
$zip = Join-Path $env:TEMP ('clubpay-auto-update-' + [guid]::NewGuid().ToString() + '.zip')
$checksum = Join-Path $env:TEMP ('clubpay-auto-update-' + [guid]::NewGuid().ToString() + '.sha256')
try {
  New-Item -ItemType Directory -Path $stage -Force | Out-Null
  Invoke-WebRequest -Uri '%s' -OutFile $zip -UseBasicParsing
  Invoke-WebRequest -Uri '%s' -OutFile $checksum -UseBasicParsing
  $expected = ((Get-Content -LiteralPath $checksum -Raw).Trim() -split '\s+')[0].ToLowerInvariant()
  $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLowerInvariant()
  if ($expected -notmatch '^[a-f0-9]{64}$' -or $actual -ne $expected) { throw 'ClubPay release checksum verification failed.' }
  Expand-Archive -Path $zip -DestinationPath $stage -Force
  $updater = Get-ChildItem -Path $stage -Filter '%s' -File -Recurse | Select-Object -First 1
  if ($null -eq $updater) { throw 'ClubPay release does not contain its updater.' }
  & $updater.FullName -NoPrompt
  if ($LASTEXITCODE -ne 0) { throw 'ClubPay updater returned a failure exit code.' }
}
catch {
  Remove-Item -LiteralPath '%s' -Force -ErrorAction SilentlyContinue
  throw
}
finally {
  Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $checksum -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
}
`, clean(artifact.DownloadURL), clean(artifact.ChecksumURL), clean(updaterName), clean(markerPath))
}
