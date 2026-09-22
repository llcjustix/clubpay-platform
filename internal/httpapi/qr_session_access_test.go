package httpapi

import (
	"strings"
	"testing"
	"time"
)

func TestNewStaticQRToken(t *testing.T) {
	first := newStaticQRToken()
	second := newStaticQRToken()
	if !strings.HasPrefix(first, "pc_") {
		t.Fatalf("token %q does not have static QR prefix", first)
	}
	if len(first) != len("pc_")+64 {
		t.Fatalf("token length = %d, want %d", len(first), len("pc_")+64)
	}
	if first == second {
		t.Fatal("two generated static QR tokens are equal")
	}
}

func TestCanUseQRForSession(t *testing.T) {
	tests := []struct {
		name   string
		status string
		qrType string
		want   bool
	}{
		{name: "available static QR starts session", status: "available", qrType: "static_pc", want: true},
		{name: "sleeping static QR starts session", status: "sleeping", qrType: "static_pc", want: true},
		{name: "occupied static QR is blocked", status: "occupied", qrType: "static_pc", want: false},
		{name: "occupied session QR extends session", status: "occupied", qrType: "session_extend", want: true},
		{name: "frozen static QR is blocked", status: "frozen", qrType: "static_pc", want: false},
		{name: "frozen session QR extends during grace", status: "frozen", qrType: "session_extend", want: true},
		{name: "maintenance QR is blocked", status: "maintenance", qrType: "session_extend", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canUseQRForSession(tt.status, tt.qrType); got != tt.want {
				t.Fatalf("canUseQRForSession(%q, %q) = %v, want %v", tt.status, tt.qrType, got, tt.want)
			}
		})
	}
}

func TestSessionExtendQRActive(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(time.Minute)
	expiredAt := now.Add(-time.Second)

	tests := []struct {
		name          string
		boundGrantID  string
		activeGrantID string
		expiresAt     *time.Time
		want          bool
	}{
		{name: "active session QR", boundGrantID: "grant-1", activeGrantID: "grant-1", expiresAt: &validUntil, want: true},
		{name: "QR belongs to another session", boundGrantID: "grant-1", activeGrantID: "grant-2", expiresAt: &validUntil, want: false},
		{name: "QR expired", boundGrantID: "grant-1", activeGrantID: "grant-1", expiresAt: &expiredAt, want: false},
		{name: "session has ended", boundGrantID: "grant-1", expiresAt: &validUntil, want: false},
		{name: "QR has no bound session", activeGrantID: "grant-1", expiresAt: &validUntil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionExtendQRActive(tt.boundGrantID, tt.activeGrantID, tt.expiresAt, now); got != tt.want {
				t.Fatalf("sessionExtendQRActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentEndReason(t *testing.T) {
	tests := map[string]string{
		"time_expired":  "TIME_UP",
		"TIME_UP":       "TIME_UP",
		"refund":        "REFUND",
		"client_left":   "CLIENT_LEFT",
		"error":         "ERROR",
		"admin_request": "MANAGER",
		"":              "MANAGER",
	}

	for input, want := range tests {
		if got := agentEndReason(input); got != want {
			t.Fatalf("agentEndReason(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBoundedSessionRemainder(t *testing.T) {
	tests := []struct {
		name              string
		planned, reported int
		want              int
	}{
		{name: "agent reports tighter current countdown", planned: 3600, reported: 2400, want: 2400},
		{name: "agent omits countdown", planned: 3600, reported: 0, want: 3600},
		{name: "stale pre-extension agent countdown is capped", planned: 3600, reported: 36000, want: 3600},
		{name: "no planned time never returns credit", planned: 0, reported: 3600, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boundedSessionRemainder(tt.planned, tt.reported)
			if got != tt.want {
				t.Fatalf("boundedSessionRemainder(%d, %d) = %d, want %d", tt.planned, tt.reported, got, tt.want)
			}
		})
	}
}
