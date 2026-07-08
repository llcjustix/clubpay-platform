package httpapi

import "testing"

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
