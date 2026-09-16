package httpapi

import (
	"testing"

	"clubpay/internal/config"
	"clubpay/internal/core"
)

func TestAutoUpdateAllowsAgentByRing(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		club string
		pc   string
		want bool
	}{
		{"canary matches PC", config.Config{AutoUpdateRing: "canary", AutoUpdateCanaryPCIDs: []string{"pc-1"}}, "club-a", "pc-1", true},
		{"canary blocks other PC", config.Config{AutoUpdateRing: "canary", AutoUpdateCanaryPCIDs: []string{"pc-1"}}, "club-a", "pc-2", false},
		{"canary node updates its idle Agent", config.Config{AutoUpdateRing: "canary", EdgeNodeID: "node-1", AutoUpdateCanaryNodeIDs: []string{"node-1"}}, "club-a", "pc-2", true},
		{"canary blocks Agent on other node", config.Config{AutoUpdateRing: "canary", EdgeNodeID: "node-2", AutoUpdateCanaryNodeIDs: []string{"node-1"}}, "club-a", "pc-2", false},
		{"pilot matches club", config.Config{AutoUpdateRing: "pilot", AutoUpdatePilotClubIDs: []string{"club-a"}}, "club-a", "pc-1", true},
		{"selected blocks other club", config.Config{AutoUpdateRing: "selected", AutoUpdateSelectedClubIDs: []string{"club-b"}}, "club-a", "pc-1", false},
		{"all allows", config.Config{AutoUpdateRing: "all"}, "club-a", "pc-1", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(test.cfg, nil, core.NewMockAdapter())
			if got := server.autoUpdateAllowsAgent(test.club, test.pc); got != test.want {
				t.Fatalf("autoUpdateAllowsAgent() = %v, want %v", got, test.want)
			}
		})
	}
}
