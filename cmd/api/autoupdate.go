package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	// A release check is lightweight and updates are only applied after a
	// checksum verification. Keeping this bounded gives a newly published
	// security or Agent fix a predictable delivery time instead of leaving a
	// club on an older build for an hour because of a stale controller.env.
	maximumAutomaticUpdateCheckInterval = 5 * time.Minute
)

var releaseHTTPClient = &http.Client{Timeout: 20 * time.Second}

// Controllers update themselves only on an installed local node. Agents that
// are connected directly to Cloud are different: Cloud already owns their
// authenticated command socket and can ask a free Agent to fetch a signed
// public release. This keeps direct-cloud clubs on the same safe update path
// as edge-controller clubs without allowing Cloud to replace a Controller.
func runAutomaticUpdateLoop(ctx context.Context, cfg config.Config, server *httpapi.Server, currentVersion string) {
	mode := strings.ToLower(strings.TrimSpace(cfg.NodeMode))
	if !cfg.AutoUpdateEnabled || (mode != "cloud" && mode != "edge" && mode != "manager") {
		return
	}
	interval := time.Duration(cfg.AutoUpdateCheckSeconds) * time.Second
	if interval <= 0 || interval > maximumAutomaticUpdateCheckInterval {
		interval = maximumAutomaticUpdateCheckInterval
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
	if mode == "edge" || mode == "cloud" {
		agentArtifact, err := release.Latest(ctx, releaseHTTPClient, agentReleasesAPI, "v", "ClubPay-Agent-win-x64.zip")
		if err != nil {
			log.Printf("automatic Agent update lookup: %v", err)
		} else {
			server.SetAgentUpdateRelease(core.AgentUpdateCommand{
				Version: agentArtifact.Version, DownloadURL: agentArtifact.DownloadURL, ChecksumURL: agentArtifact.ChecksumURL,
			})
			if mode == "cloud" {
				// Direct-cloud Agents are not part of an edge synchronization loop, so
				// schedule their safe, one-at-a-time roll-out immediately after a
				// release check.
				server.ScheduleAvailableAgentUpdates(ctx)
			}
		}

		// Cloud never replaces its own process from this loop. It publishes Agent
		// releases only; Controller and Manager replacement stays local below.
		if mode == "cloud" {
			return
		}
	}

	var (
		artifact    release.Artifact
		err         error
		archiveName string
		updaterName string
	)
	if mode == "manager" {
		archiveName = "ClubPay-Manager-Desktop-win-x64.zip"
		updaterName = "update-manager.ps1"
		artifact, err = release.Latest(ctx, releaseHTTPClient, agentReleasesAPI, "v", archiveName)
	} else {
		if runtime.GOOS == "linux" {
			archiveName = "ClubPay-Controller-linux-arm64.tar.gz"
			updaterName = "update-linux.sh"
		} else {
			archiveName = "ClubPay-Controller-win-x64.zip"
			updaterName = "update-windows.ps1"
		}
		artifact, err = release.Latest(ctx, releaseHTTPClient, platformReleasesAPI, "controller-v", archiveName)
	}
	if err != nil {
		log.Printf("automatic %s update lookup: %v", mode, err)
		return
	}
	installedVersion := currentVersion
	if mode == "manager" {
		installedVersion = managerInstalledVersion(currentVersion)
	}
	if release.CompareVersions(artifact.Version, installedVersion) <= 0 {
		clearCompletedUpdateMarker()
		return
	}
	if !autoUpdateAllowsLocalNode(cfg) {
		log.Printf("update_event component=%s action=held version=%s ring=%s node_id=%s club_id=%s", mode, artifact.Version, cfg.AutoUpdateRing, localUpdateNodeID(cfg), cfg.EdgeClubID)
		return
	}
	if err := scheduleLocalUpdate(artifact, updaterName); err != nil {
		log.Printf("schedule automatic %s update to %s: %v", mode, artifact.Version, err)
		return
	}
	log.Printf("update_event component=%s action=scheduled version=%s ring=%s node_id=%s club_id=%s", mode, artifact.Version, cfg.AutoUpdateRing, localUpdateNodeID(cfg), cfg.EdgeClubID)
}

func localUpdateNodeID(cfg config.Config) string {
	if strings.TrimSpace(cfg.EdgeNodeID) != "" {
		return strings.TrimSpace(cfg.EdgeNodeID)
	}
	return strings.TrimSpace(cfg.ManagerNodeID)
}

func autoUpdateAllowsLocalNode(cfg config.Config) bool {
	switch strings.ToLower(strings.TrimSpace(cfg.AutoUpdateRing)) {
	case "all":
		return true
	case "selected":
		return containsConfigID(cfg.AutoUpdateSelectedClubIDs, cfg.EdgeClubID)
	case "pilot":
		return containsConfigID(cfg.AutoUpdatePilotClubIDs, cfg.EdgeClubID)
	case "canary":
		return containsConfigID(cfg.AutoUpdateCanaryNodeIDs, localUpdateNodeID(cfg))
	default:
		return false
	}
}

func containsConfigID(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

// A Manager contains a local read-only Controller. Its Controller build tag
// is deliberately independent from the desktop Manager release tag, so use
// the marker packaged with the desktop app when deciding whether it needs a
// new Manager archive. Without it, a newer Controller tag could make the
// Manager schedule the same desktop update forever.
func managerInstalledVersion(fallback string) string {
	controllerRoot := controllerInstallRoot()
	if controllerRoot == "" {
		return fallback
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(controllerRoot), "clubpay-manager-version.txt"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return fallback
	}
	return strings.TrimSpace(string(data))
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

func scheduleLocalUpdate(artifact release.Artifact, updaterName string) error {
	if runtime.GOOS == "linux" {
		return scheduleLinuxUpdate(artifact, updaterName)
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("automatic local updates are supported on Windows and Linux Controllers")
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

func scheduleLinuxUpdate(artifact release.Artifact, updaterName string) error {
	root := controllerInstallRoot()
	if root == "" {
		return fmt.Errorf("locate Controller install directory")
	}
	marker := updateMarkerPath()
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == artifact.Version {
		return nil
	}
	updatesDir := filepath.Join(root, "updates")
	if err := os.MkdirAll(updatesDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(artifact.Version+"\n"), 0o600); err != nil {
		return err
	}
	scriptPath := filepath.Join(updatesDir, "apply-"+safeUpdateFilePart(artifact.Version)+".sh")
	if err := os.WriteFile(scriptPath, []byte(automaticLinuxUpdateScript(artifact, updaterName, marker)), 0o700); err != nil {
		_ = os.Remove(marker)
		return err
	}
	// systemd-run puts the helper outside the Controller service cgroup. The
	// updater can then stop and restart the Controller without being killed
	// together with it.
	result, err := exec.Command("systemd-run", "--unit=clubpay-controller-update", "--collect", "/bin/sh", scriptPath).CombinedOutput()
	if err != nil {
		_ = os.Remove(marker)
		return fmt.Errorf("start Linux update helper: %w: %s", err, strings.TrimSpace(string(result)))
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

func automaticLinuxUpdateScript(artifact release.Artifact, updaterName, markerPath string) string {
	clean := func(value string) string { return strings.ReplaceAll(value, "'", "") }
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
stage="$(mktemp -d)"
archive="$(mktemp)"
checksum="$(mktemp)"
cleanup() { rm -rf "$stage" "$archive" "$checksum"; }
trap cleanup EXIT
curl --fail --location --silent --show-error '%s' -o "$archive"
curl --fail --location --silent --show-error '%s' -o "$checksum"
expected="$(awk '{print $1}' "$checksum" | tr '[:upper:]' '[:lower:]')"
actual="$(sha256sum "$archive" | awk '{print $1}')"
[[ "$expected" =~ ^[a-f0-9]{64}$ && "$actual" == "$expected" ]] || { rm -f '%s'; echo 'ClubPay release checksum verification failed.' >&2; exit 1; }
tar -xzf "$archive" -C "$stage"
updater="$(find "$stage" -type f -name '%s' -print -quit)"
[[ -n "$updater" ]] || { rm -f '%s'; echo 'ClubPay release does not contain its updater.' >&2; exit 1; }
chmod +x "$updater"
exec "$updater" --no-prompt
`, clean(artifact.DownloadURL), clean(artifact.ChecksumURL), clean(markerPath), clean(updaterName), clean(markerPath))
}
