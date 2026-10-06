package bootpower

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObserverRejectsUnboundOrStaleEvidence(t *testing.T) {
	for _, kind := range []string{"valid", "running", "foreign_pc", "foreign_vm", "replay", "stale", "unavailable"} {
		t.Run(kind, func(t *testing.T) {
			token := strings.Repeat("p", 32)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+token {
					t.Error("authentication missing")
				}
				var q Request
				json.NewDecoder(r.Body).Decode(&q)
				out := Observation{Request: q, VMID: 130, Status: "stopped", ObservedAt: time.Now().UTC()}
				switch kind {
				case "running":
					out.Status = "running"
				case "foreign_pc":
					out.ExternalPCID = "other"
				case "foreign_vm":
					out.VMID = 131
				case "replay":
					out.Nonce = strings.Repeat("0", 64)
				case "stale":
					out.ObservedAt = time.Now().Add(-time.Minute)
				case "unavailable":
					w.WriteHeader(503)
					return
				}
				json.NewEncoder(w).Encode(out)
			}))
			defer srv.Close()
			cfg := Config{URL: srv.URL, Token: token, ExternalPCID: "pilot", VMID: 130}
			err := cfg.Stopped(context.Background(), "club", "pilot")
			if (err == nil) != (kind == "valid") {
				t.Fatal(kind, err)
			}
		})
	}
}
