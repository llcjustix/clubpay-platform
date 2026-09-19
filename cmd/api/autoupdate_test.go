package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clubpay/internal/config"
)

func TestAutoUpdateAllowsLocalNodeByRing(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want bool
	}{
		{"canary matches node", config.Config{AutoUpdateRing: "canary", EdgeNodeID: "pilot-01", AutoUpdateCanaryNodeIDs: []string{"pilot-01"}}, true},
		{"canary blocks other node", config.Config{AutoUpdateRing: "canary", EdgeNodeID: "pilot-02", AutoUpdateCanaryNodeIDs: []string{"pilot-01"}}, false},
		{"pilot matches club", config.Config{AutoUpdateRing: "pilot", EdgeClubID: "club-a", AutoUpdatePilotClubIDs: []string{"club-a"}}, true},
		{"selected blocks unlisted club", config.Config{AutoUpdateRing: "selected", EdgeClubID: "club-a", AutoUpdateSelectedClubIDs: []string{"club-b"}}, false},
		{"all always matches", config.Config{AutoUpdateRing: "all"}, true},
		{"unknown ring is safe", config.Config{AutoUpdateRing: ""}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := autoUpdateAllowsLocalNode(test.cfg); got != test.want {
				t.Fatalf("autoUpdateAllowsLocalNode() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMigrateLegacyCanaryNodeEnrollment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "controller.env")
	if err := os.WriteFile(path, []byte("NODE_MODE=edge\nEDGE_NODE_ID=pilot-01\nAUTO_UPDATE_ENABLED=true\nAUTO_UPDATE_RING=canary\nAUTO_UPDATE_CANARY_NODE_IDS=\nEDGE_SYNC_TOKEN=keep-private\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{NodeMode: "edge", EdgeNodeID: "pilot-01", AutoUpdateEnabled: true, AutoUpdateRing: "canary"}
	migrated, err := migrateLegacyCanaryNodeEnrollment(path, &cfg)
	if err != nil || !migrated {
		t.Fatalf("migrateLegacyCanaryNodeEnrollment() = %v, %v", migrated, err)
	}
	if got := cfg.AutoUpdateCanaryNodeIDs; len(got) != 1 || got[0] != "pilot-01" {
		t.Fatalf("AutoUpdateCanaryNodeIDs = %v", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "AUTO_UPDATE_CANARY_NODE_IDS=pilot-01"; !strings.Contains(string(contents), want) {
		t.Fatalf("config did not contain %q: %s", want, contents)
	}
	if !strings.Contains(string(contents), "EDGE_SYNC_TOKEN=keep-private") {
		t.Fatal("migration removed unrelated config")
	}
}
