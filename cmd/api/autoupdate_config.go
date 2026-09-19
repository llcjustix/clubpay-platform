package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"clubpay/internal/config"
	"clubpay/internal/envfile"
)

// migrateLegacyCanaryNodeEnrollment repairs Controller installations that
// existed before activation enrolled the local node into its own canary ring.
// New installations already receive this value in setupControllerNode. Keeping
// the migration narrow avoids turning an empty canary ring into a broad rollout.
func migrateLegacyCanaryNodeEnrollment(configPath string, cfg *config.Config) (bool, error) {
	if cfg == nil || !cfg.AutoUpdateEnabled || !strings.EqualFold(strings.TrimSpace(cfg.AutoUpdateRing), "canary") {
		return false, nil
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.NodeMode))
	if mode != "edge" && mode != "manager" {
		return false, nil
	}
	nodeID := localUpdateNodeID(*cfg)
	if nodeID == "" || len(cfg.AutoUpdateCanaryNodeIDs) != 0 {
		return false, nil
	}

	values, err := envfile.Read(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", configPath, err)
	}
	// An explicit non-empty file value always wins, including when the
	// process environment overrode it while loading config.
	if strings.TrimSpace(values["AUTO_UPDATE_CANARY_NODE_IDS"]) != "" {
		return false, nil
	}

	original, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", configPath, err)
	}
	lineEnding := "\n"
	if strings.Contains(string(original), "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(original), "\r\n", "\n"), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "AUTO_UPDATE_CANARY_NODE_IDS=") {
			continue
		}
		lines[i] = "AUTO_UPDATE_CANARY_NODE_IDS=" + nodeID
		replaced = true
	}
	if !replaced {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "AUTO_UPDATE_CANARY_NODE_IDS="+nodeID)
	}
	updated := []byte(strings.Join(lines, lineEnding))
	if !strings.HasSuffix(string(updated), lineEnding) {
		updated = append(updated, []byte(lineEnding)...)
	}

	dir := filepath.Dir(configPath)
	pending, err := os.CreateTemp(dir, ".controller.env-*")
	if err != nil {
		return false, fmt.Errorf("create pending config: %w", err)
	}
	pendingPath := pending.Name()
	defer os.Remove(pendingPath)
	if err := pending.Chmod(0o600); err != nil {
		_ = pending.Close()
		return false, fmt.Errorf("protect pending config: %w", err)
	}
	if _, err := pending.Write(updated); err != nil {
		_ = pending.Close()
		return false, fmt.Errorf("write pending config: %w", err)
	}
	if err := pending.Close(); err != nil {
		return false, fmt.Errorf("close pending config: %w", err)
	}
	if err := os.Rename(pendingPath, configPath); err != nil {
		return false, fmt.Errorf("save %s: %w", configPath, err)
	}
	cfg.AutoUpdateCanaryNodeIDs = []string{nodeID}
	return true, nil
}
