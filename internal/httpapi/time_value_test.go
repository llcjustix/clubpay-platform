package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"testing"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTimeValueMath(t *testing.T) {
	units, err := timeValue(3600, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if seconds, _ := timeAtRate(units, 3000000); seconds != 1800 {
		t.Fatal(seconds)
	}
	if seconds, _ := timeAtRate(units, 750000); seconds != 7200 {
		t.Fatal(seconds)
	}
	if _, err = timeValue(2, math.MaxInt64); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err = timeAtRate(10, 0); err == nil {
		t.Fatal("zero price accepted")
	}
}

func TestZoneValueIntegration(t *testing.T) {
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "zone_test_" + randomHex(6)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, _ := pgxpool.ParseConfig(raw)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	s := NewServer(config.Config{}, pool, core.NewMockAdapter())
	var club, pc, zone, player string
	if err = pool.QueryRow(ctx, `SELECT p.club_id,p.id,p.zone_id FROM pc_refs p JOIN zones z ON z.id=p.zone_id ORDER BY z.hourly_price_tiyin LIMIT 1`).Scan(&club, &pc, &zone); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE zones SET hourly_price_tiyin=1500000 WHERE club_id=$1`, club); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO players(phone) VALUES('+998900000099') RETURNING id`).Scan(&player); err != nil {
		t.Fatal(err)
	}
	grant := func(label string) string {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO game_access_grants(club_id,pc_ref_id,player_id,duration_minutes,duration_seconds,status,source) VALUES($1,$2,$3,60,3600,'accepted','player_balance') RETURNING id`, club, pc, player).Scan(&id); err != nil {
			t.Fatal(label, err)
		}
		return id
	}
	record := func(seconds int, kind, id, key string) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err = s.recordPlayerTime(ctx, tx, player, club, seconds, kind, id, "", key); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	currentUnits := func() int64 {
		var v int64
		if err := pool.QueryRow(ctx, `SELECT time_value_units FROM player_club_balances WHERE player_id=$1 AND club_id=$2`, player, club).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	original := grant("standard")
	record(3601, "session_remaining", original, "return-original")
	originalUnits := int64(3601) * 1500000
	if currentUnits() != originalUnits {
		t.Fatal("initial credit")
	}
	// Identical controller end events never add the value twice.
	record(3601, "session_remaining", original, "return-original")
	if currentUnits() != originalUnits {
		t.Fatal("duplicate return")
	}
	if _, err = pool.Exec(ctx, `UPDATE zones SET hourly_price_tiyin=3000000 WHERE id=$1`, zone); err != nil {
		t.Fatal(err)
	}
	vip := grant("vip")
	tx, _ := pool.Begin(ctx)
	seconds, err := s.balanceSecondsForPC(ctx, tx, player, club, pc)
	tx.Rollback(ctx)
	if err != nil || seconds != 1800 {
		t.Fatalf("VIP quote %d %v", seconds, err)
	}
	record(-1800, "session_start", vip, "consume-vip")
	if currentUnits() != 1500000 {
		t.Fatal("fractional second discarded")
	}
	record(-1800, "session_start", vip, "consume-vip")
	if currentUnits() != 1500000 {
		t.Fatal("duplicate debit")
	}
	// Price edits must not change the refund or the remaining session value.
	_, _ = pool.Exec(ctx, `UPDATE zones SET hourly_price_tiyin=4500000 WHERE id=$1`, zone)
	s.refundPlayerBalance(ctx, player, club, 1800, vip, "simulated start failure")
	if currentUnits() != originalUnits {
		t.Fatal("refund did not restore exact credit")
	}
	s.refundPlayerBalance(ctx, player, club, 1800, vip, "duplicate failure")
	if currentUnits() != originalUnits {
		t.Fatal("duplicate refund")
	}
	// A session begun at the old Standard price retains that rate on end.
	record(1, "session_remaining", original, "another-second")
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("price edit revalued past time")
	}
	// Snapshot transport preserves value/rate/ledger fields.
	snapshot, err := s.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("snapshot changed credit")
	}
	var rate int64
	if err = pool.QueryRow(ctx, `SELECT time_value_rate FROM game_access_grants WHERE id=$1`, original).Scan(&rate); err != nil || rate != 1500000 {
		t.Fatal(fmt.Sprint(rate, err))
	}

	createEarlyEndGrant := func(coreSessionID string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO game_access_grants (
				club_id, pc_ref_id, player_id, duration_minutes, duration_seconds,
				status, source, core_session_id, accepted_at, planned_ends_at
			)
			VALUES ($1,$2,$3,60,3600,'accepted','online_payment',$4,now(),now()+interval '1 hour')
			RETURNING id
		`, club, pc, player, coreSessionID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	var externalPCID string
	if err := pool.QueryRow(ctx, `SELECT external_pc_id FROM pc_refs WHERE id=$1`, pc).Scan(&externalPCID); err != nil {
		t.Fatal(err)
	}
	// The mock Agent confirms end_session without remaining_seconds. A manual
	// Agent endpoint must still return the unused profile time.
	agentGrant := createEarlyEndGrant("agent-end-without-remaining")
	body, _ := json.Marshal(agentEndSessionRequest{ExternalPCID: externalPCID, CoreSessionID: "agent-end-without-remaining"})
	req := httptest.NewRequest("POST", "/api/core/agent/session/end", bytes.NewReader(body))
	res := httptest.NewRecorder()
	s.handleAgentEndSession(res, req)
	if res.Code != 200 {
		t.Fatalf("agent early end: %d %s", res.Code, res.Body.String())
	}
	var agentResult map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &agentResult); err != nil {
		t.Fatal(err)
	}
	if remaining, _ := agentResult["remaining_seconds"].(float64); remaining < 3500 || remaining > 3600 {
		t.Fatalf("agent end lost remaining time: %#v", agentResult)
	}
	var ended bool
	if err := pool.QueryRow(ctx, `SELECT status='ended' FROM game_access_grants WHERE id=$1`, agentGrant).Scan(&ended); err != nil || !ended {
		t.Fatalf("agent grant not ended: %v %v", ended, err)
	}

	// The asynchronous session_ended event has the same fallback when an older
	// Agent omits the remainder from its payload.
	eventGrant := createEarlyEndGrant("event-end-without-remaining")
	eventResult, status, err := s.processCoreEvent(ctx, coreEventRequest{
		EventID:       "session-ended-without-remaining",
		EventType:     "session_ended",
		CoreSessionID: "event-end-without-remaining",
		Payload:       map[string]any{"reason": "client_left"},
	})
	if err != nil || status != 200 {
		t.Fatalf("session_ended event: status=%d result=%#v err=%v", status, eventResult, err)
	}
	if remaining, _ := eventResult["remaining_seconds"].(int); remaining < 3500 || remaining > 3600 {
		t.Fatalf("event end lost remaining time: %#v", eventResult)
	}
	if err := pool.QueryRow(ctx, `SELECT status='ended' FROM game_access_grants WHERE id=$1`, eventGrant).Scan(&ended); err != nil || !ended {
		t.Fatalf("event grant not ended: %v %v", ended, err)
	}
}
