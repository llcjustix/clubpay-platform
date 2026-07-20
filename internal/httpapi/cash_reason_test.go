package httpapi

import "testing"

func TestIsCashReason(t *testing.T) {
	for _, reason := range []string{"cash_payment", "provider_unavailable", "internet_unavailable", "terminal_fallback"} {
		if !isCashReason(reason) {
			t.Fatalf("expected %q to be accepted", reason)
		}
	}

	for _, reason := range []string{"", "cash", "manual_override"} {
		if isCashReason(reason) {
			t.Fatalf("expected %q to be rejected", reason)
		}
	}
}
