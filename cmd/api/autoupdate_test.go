package main

import (
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
