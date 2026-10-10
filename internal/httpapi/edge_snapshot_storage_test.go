package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"github.com/jackc/pgx/v5/pgxpool"
)

func snapshotStorageFixture(t *testing.T) (*pgxpool.Pool, *Server, string, string) {
	t.Helper()
	pool := agentRoutesTestDB(t)
	var club, pc string
	if err := pool.QueryRow(context.Background(), "SELECT club_id,id FROM pc_refs ORDER BY id LIMIT 1").Scan(&club, &pc); err != nil {
		t.Fatal(err)
	}
	s := NewServer(config.Config{AppEnv: "production", NodeMode: "cloud", CoreToken: "snapshot-fixture-only"}, pool, core.NewMockAdapter())
	return pool, s, club, pc
}

func sendSnapshotStorageEvent(t *testing.T, s *Server, club, node, token string, events ...edgeEvent) int {
	t.Helper()
	body, err := json.Marshal(edgeEventBatch{ClubID: club, Events: events})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/edge/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if node != "" {
		req.Header.Set("X-Edge-Node-ID", node)
	}
	out := httptest.NewRecorder()
	s.Routes().ServeHTTP(out, req)
	if out.Code != 200 {
		t.Logf("fixture response HTTP %d: %s", out.Code, out.Body.String())
	}
	return out.Code
}

func TestEdgeSnapshotAppliesWithoutArchivingHistory(t *testing.T) {
	pool, s, club, pc := snapshotStorageFixture(t)
	ctx := context.Background()
	// Existing snapshot history must remain intact until explicitly maintained.
	if _, err := pool.Exec(ctx, `INSERT INTO core_events(event_id,event_type,club_id,payload,status)
		VALUES('historical-snapshot','edge_edge_snapshot',$1,'{"historical_fixture":true}','processed')`, club); err != nil {
		t.Fatal(err)
	}
	var order, grant string
	if err := pool.QueryRow(ctx, `INSERT INTO payment_orders(invoice_id,club_id,pc_ref_id,amount_tiyin,duration_minutes,status,provider_payload)
		VALUES('snapshot-fixture-invoice',$1,$2,1500000,60,'paid','{"receipt_fixture":"preserved"}') RETURNING id`, club, pc).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO payments(payment_order_id,provider_payment_id,amount_tiyin,status,raw_payload)
		VALUES($1,'snapshot-fixture-payment',1500000,'success','{"receipt_fixture":"preserved"}')`, order); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO game_access_grants(club_id,pc_ref_id,payment_order_id,duration_minutes,duration_seconds,status)
		VALUES($1,$2,$3,60,3600,'pending') RETURNING id`, club, pc, order).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	payload, err := s.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the full compatibility payload, including history, plus large
	// harmless input. No durable in-memory deduplication may be required.
	payload["fixture_padding"] = strings.Repeat("snapshot-history", 16000)
	for i := 0; i < 25; i++ {
		pcs := payload["pcs"].([]map[string]any)
		for _, p := range pcs {
			if p["id"] == pc {
				p["label"] = fmt.Sprintf("snapshot-label-%d", i)
			}
		}
		if code := sendSnapshotStorageEvent(t, s, club, "", "snapshot-fixture-only", edgeEvent{
			EventID: fmt.Sprintf("sync-%d", i), Type: "edge_snapshot", Payload: payload,
		}); code != 200 {
			t.Fatalf("snapshot %d: HTTP %d", i, code)
		}
		if i == 12 {
			s = NewServer(s.cfg, pool, core.NewMockAdapter())
		}
	}
	var label string
	var synced bool
	if err := pool.QueryRow(ctx, "SELECT label FROM pc_refs WHERE id=$1", pc).Scan(&label); err != nil || label != "snapshot-label-24" {
		t.Fatalf("snapshot not applied: %q, %v", label, err)
	}
	if err := pool.QueryRow(ctx, "SELECT controller_synced_at IS NOT NULL FROM clubs WHERE id=$1", club).Scan(&synced); err != nil || !synced {
		t.Fatal("sync health not updated", err)
	}
	var archived int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM core_events WHERE event_type='edge_edge_snapshot'").Scan(&archived); err != nil || archived != 1 {
		t.Fatalf("new snapshot copies archived: count=%d err=%v", archived, err)
	}
	var paymentPreserved bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payment_orders po JOIN payments p ON p.payment_order_id=po.id
		WHERE po.id=$1 AND po.status='paid' AND p.status='success' AND po.amount_tiyin=1500000 AND p.amount_tiyin=1500000
		AND po.provider_payload->>'receipt_fixture'='preserved' AND p.raw_payload->>'receipt_fixture'='preserved')`, order).Scan(&paymentPreserved); err != nil || !paymentPreserved {
		t.Fatal("financial history changed during synchronization", err)
	}
	// Actual session events still persist and update the authoritative grant.
	started := edgeEvent{EventID: "real-session-fixture", Type: "session_started", GrantID: grant,
		CoreSessionID: "fixture-session", Payload: map[string]any{"grant_id": grant,
			"core_session_id": "fixture-session", "ends_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}}
	for i := 0; i < 2; i++ {
		if code := sendSnapshotStorageEvent(t, s, club, "", "snapshot-fixture-only", started); code != 200 {
			t.Fatal("session event rejected", code)
		}
	}
	var status, session string
	if err := pool.QueryRow(ctx, "SELECT status,core_session_id FROM game_access_grants WHERE id=$1", grant).Scan(&status, &session); err != nil || status != "accepted" || session != "fixture-session" {
		t.Fatal("session update lost", status, session, err)
	}
	var sessionEvents int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM core_events WHERE event_type IN ('edge_session_started','session_started')").Scan(&sessionEvents); err != nil || sessionEvents != 2 {
		t.Fatalf("session event history/idempotency changed: count=%d err=%v", sessionEvents, err)
	}
}

func TestEdgeSnapshotRejectsUnauthorizedAndRetriesFailedApply(t *testing.T) {
	pool, s, club, pc := snapshotStorageFixture(t)
	bad := edgeEvent{EventID: "retryable-sync", Type: "edge_snapshot", Payload: map[string]any{
		"pcs": []map[string]any{{"id": "not-a-uuid", "club_id": club}},
	}}
	if code := sendSnapshotStorageEvent(t, s, club, "", "wrong-fixture-token", bad); code != 401 {
		t.Fatal("unauthorized snapshot accepted", code)
	}
	if code := sendSnapshotStorageEvent(t, s, club, "", "snapshot-fixture-only", bad); code != 400 {
		t.Fatal("invalid snapshot accepted", code)
	}
	var err error
	bad.Payload, err = s.edgeSnapshotData(context.Background(), club, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range bad.Payload["pcs"].([]map[string]any) {
		if p["id"] == pc {
			p["label"] = "successful-retry"
		}
	}
	if code := sendSnapshotStorageEvent(t, s, club, "", "snapshot-fixture-only", bad); code != 200 {
		t.Fatal("failed apply could not be retried", code)
	}
	var label string
	var archived int
	if err := pool.QueryRow(context.Background(), "SELECT label FROM pc_refs WHERE id=$1", pc).Scan(&label); err != nil || label != "successful-retry" {
		t.Fatal(label, err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM core_events").Scan(&archived); err != nil || archived != 0 {
		t.Fatal("failed/retried snapshot archived", archived, err)
	}
}

func TestEdgeSnapshotRetainsManagerLeaseFence(t *testing.T) {
	pool, s, club, pc := snapshotStorageFixture(t)
	ctx := context.Background()
	for _, node := range []string{"fixture-primary", "fixture-manager"} {
		mode := "edge"
		if node == "fixture-manager" {
			mode = "manager"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO controller_nodes(club_id,node_id,node_mode,sync_token_hash)
			VALUES($1,$2,$3,$4)`, club, node, mode, hashToken(node)); err != nil {
			t.Fatal(err)
		}
	}
	var external, original string
	if err := pool.QueryRow(ctx, "SELECT external_pc_id,label FROM pc_refs WHERE id=$1", pc).Scan(&external, &original); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO controller_agent_leases
		(club_id,external_pc_id,node_id,agent_connection_id,lease_expires_at)
		VALUES($1,$2,'fixture-manager','fixture-connection',now()+interval '75 seconds')`, club, external); err != nil {
		t.Fatal(err)
	}
	event := edgeEvent{EventID: "stale-primary-sync", Type: "edge_snapshot", Payload: map[string]any{
		"pcs": []map[string]any{{"id": pc, "club_id": club, "number": 1, "label": "stale-overwrite", "status_cache": "offline"}},
	}}
	if code := sendSnapshotStorageEvent(t, s, club, "fixture-primary", "fixture-primary", event); code != 200 {
		t.Fatal("stale owner response changed", code)
	}
	var label string
	var archived int
	if err := pool.QueryRow(ctx, "SELECT label FROM pc_refs WHERE id=$1", pc).Scan(&label); err != nil || label != original {
		t.Fatal("stale primary overwrote manager state", label, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM core_events").Scan(&archived); err != nil || archived != 0 {
		t.Fatal("rejected snapshot archived", archived, err)
	}
}
