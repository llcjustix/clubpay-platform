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
	"time"

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
	// A cash payment can reference an operator whose current club/role fields
	// no longer describe this club. The snapshot must still carry that user:
	// cash_payments.admin_user_id is a foreign key on a fresh Controller.
	var cashAdmin string
	if err = pool.QueryRow(ctx, `
		INSERT INTO users (name, email, role)
		VALUES ('Historical cash operator', 'cash-sync-operator@example.test', 'admin')
		RETURNING id
	`).Scan(&cashAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO cash_payments (club_id, admin_user_id, pc_ref_id, amount_tiyin, duration_minutes, duration_seconds, reason)
		VALUES ($1, $2, $3, 1500000, 60, 3600, 'cash')
	`, club, cashAdmin, pc); err != nil {
		t.Fatal(err)
	}
	// A mobile reservation can be the first interaction a player has with a
	// club. It therefore must bring both the player and reservation into an
	// edge snapshot; otherwise a local Controller cannot show its protected
	// reservation screen to the Agent.
	var reservationPlayer, reservationID string
	if err = pool.QueryRow(ctx, `
		INSERT INTO players (phone) VALUES ('+998900000098') RETURNING id
	`).Scan(&reservationPlayer); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `
		INSERT INTO mobile_reservations (
			player_id, club_id, pc_ref_id, starts_at, duration_minutes, entry_code, status
		) VALUES ($1, $2, $3, now() + interval '5 minutes', 60, '123456', 'confirmed')
		RETURNING id
	`, reservationPlayer, club, pc).Scan(&reservationID); err != nil {
		t.Fatal(err)
	}
	// Snapshot transport preserves value/rate/ledger fields.
	snapshot, err := s.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	users, ok := snapshot["users"].([]map[string]any)
	if !ok {
		t.Fatalf("snapshot users type %T", snapshot["users"])
	}
	foundCashAdmin := false
	for _, user := range users {
		if fmt.Sprint(user["id"]) == cashAdmin {
			foundCashAdmin = true
			break
		}
	}
	if !foundCashAdmin {
		t.Fatal("snapshot omitted cash payment operator")
	}
	reservations, ok := snapshot["mobile_reservations"].([]map[string]any)
	if !ok {
		t.Fatalf("snapshot mobile_reservations type %T", snapshot["mobile_reservations"])
	}
	foundReservation := false
	for _, reservation := range reservations {
		if fmt.Sprint(reservation["id"]) == reservationID {
			foundReservation = true
			break
		}
	}
	if !foundReservation {
		t.Fatal("snapshot omitted mobile reservation")
	}
	// Delete the source rows to prove the import recreates both sides of the
	// reservation foreign keys on a fresh local Controller.
	if _, err = pool.Exec(ctx, `DELETE FROM mobile_reservations WHERE id=$1`, reservationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM players WHERE id=$1`, reservationPlayer); err != nil {
		t.Fatal(err)
	}
	if err = s.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	var importedReservationStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM mobile_reservations WHERE id=$1`, reservationID).Scan(&importedReservationStatus); err != nil || importedReservationStatus != "confirmed" {
		t.Fatalf("reservation snapshot import = %q, %v", importedReservationStatus, err)
	}
	// A lagging edge snapshot must never restore a reservation that Cloud has
	// already completed after the Agent ended the session.
	if _, err = pool.Exec(ctx, `UPDATE mobile_reservations SET status='completed',updated_at=now() WHERE id=$1`, reservationID); err != nil {
		t.Fatal(err)
	}
	if err = s.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM mobile_reservations WHERE id=$1`, reservationID).Scan(&importedReservationStatus); err != nil || importedReservationStatus != "completed" {
		t.Fatalf("stale reservation snapshot overwrote completed status: %q, %v", importedReservationStatus, err)
	}
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("snapshot changed credit")
	}
	// A delayed Controller snapshot can carry a higher, pre-debit balance with
	// a newer wall-clock timestamp. It must never resurrect spent time once the
	// Cloud ledger exists; otherwise an end-session return credits the same time
	// for a second time.
	balances, ok := snapshot["player_club_balances"].([]map[string]any)
	if !ok {
		t.Fatalf("snapshot balance type %T", snapshot["player_club_balances"])
	}
	for _, balance := range balances {
		if fmt.Sprint(balance["player_id"]) == player && fmt.Sprint(balance["club_id"]) == club {
			balance["time_value_units"] = (originalUnits + 1500000) * 2
			balance["seconds_balance"] = 7202
			balance["updated_at"] = time.Now().UTC().Add(time.Hour)
		}
	}
	if err = s.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("stale snapshot resurrected spent balance")
	}
	// The mobile profile read also corrects any inflated cache written by an
	// earlier Controller release, from the immutable ledger total.
	if _, err = pool.Exec(ctx, `UPDATE player_club_balances SET seconds_balance=7202,time_value_units=$3 WHERE player_id=$1 AND club_id=$2`, player, club, (originalUnits+1500000)*2); err != nil {
		t.Fatal(err)
	}
	if err = s.repairProfileBalanceProjection(ctx, player); err != nil {
		t.Fatal(err)
	}
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("ledger did not lower inflated balance projection")
	}
	// A player can redeem immediately, before the mobile app happens to fetch a
	// fresh balance screen. The mutation path itself must therefore use ledger
	// value, not the stale projection it found in player_club_balances.
	if _, err = pool.Exec(ctx, `UPDATE player_club_balances SET seconds_balance=7202,time_value_units=$3 WHERE player_id=$1 AND club_id=$2`, player, club, (originalUnits+1500000)*2); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lockedUnits, _, err := s.lockTimeValue(ctx, tx, player, club)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if lockedUnits != originalUnits+1500000 || currentUnits() != originalUnits+1500000 {
		t.Fatalf("stale balance was accepted for a redemption: locked=%d projected=%d", lockedUnits, currentUnits())
	}
	// The balance row is a projection. If a stale edge snapshot overwrites it,
	// the immutable ledger restores the credited time on the next profile read.
	if _, err = pool.Exec(ctx, `UPDATE player_time_ledger SET time_value_delta=NULL WHERE player_id=$1 AND club_id=$2`, player, club); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE player_club_balances SET seconds_balance=0,time_value_units=0 WHERE player_id=$1 AND club_id=$2`, player, club); err != nil {
		t.Fatal(err)
	}
	if err = s.repairProfileBalanceProjection(ctx, player); err != nil {
		t.Fatal(err)
	}
	if currentUnits() != originalUnits+1500000 {
		t.Fatal("legacy ledger did not restore a stale balance projection")
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

	// The Agent is not the financial authority. Even if a stale/replayed Agent
	// reports a countdown larger than this grant, finishGrant must clamp it to
	// the server-side planned end before it reaches the immutable balance ledger.
	inflatedGrant := createEarlyEndGrant("agent-end-inflated-remaining")
	inflatedResult, err := s.finishGrant(ctx, inflatedGrant, "client_left", 36_000)
	if err != nil {
		t.Fatal(err)
	}
	inflatedRemaining, _ := inflatedResult["remaining_seconds"].(int)
	if inflatedRemaining < 3500 || inflatedRemaining > 3600 {
		t.Fatalf("inflated Agent remainder was credited: %#v", inflatedResult)
	}
	var recordedReturn int
	if err := pool.QueryRow(ctx, `SELECT seconds_delta FROM player_time_ledger WHERE idempotency_key='session-return:' || $1::text`, inflatedGrant).Scan(&recordedReturn); err != nil {
		t.Fatal(err)
	}
	if recordedReturn != inflatedRemaining {
		t.Fatalf("ledger recorded %d seconds, want bounded %d", recordedReturn, inflatedRemaining)
	}

	// A late Agent acknowledgement must not be able to turn a timeout refund
	// into extra player time. Accepted grants are already owned by the session
	// lifecycle, so refundPlayerBalance is a strict pending-only operation.
	lateGrant := createEarlyEndGrant("late-agent-acknowledgement")
	s.refundPlayerBalance(ctx, player, club, 3600, lateGrant, "agent command timeout")
	var refundRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_time_ledger WHERE idempotency_key='session-refund:' || $1::text`, lateGrant).Scan(&refundRows); err != nil {
		t.Fatal(err)
	}
	if refundRows != 0 {
		t.Fatal("accepted grant received a start refund")
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

	// A release made before the fallback can be repaired on the next balance
	// refresh without issuing the same return twice. Older payment grants may
	// miss player_id, so ownership is recovered from the order.
	legacyGrant := createEarlyEndGrant("legacy-end-without-remaining")
	var legacyOrder string
	if err := pool.QueryRow(ctx, `INSERT INTO payment_orders(invoice_id,club_id,pc_ref_id,player_id,amount_tiyin,duration_minutes,duration_seconds) VALUES('legacy-order',$1,$2,$3,100000,60,3600) RETURNING id`, club, pc, player).Scan(&legacyOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE game_access_grants SET player_id=NULL,payment_order_id=$2,status='ended',planned_ends_at=NULL,ended_at=now(),end_reason='client_left',remaining_seconds=0,remaining_minutes=0 WHERE id=$1`, legacyGrant, legacyOrder); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcileMissingProfileRemainders(ctx, player); err != nil {
		t.Fatal(err)
	}
	var repaired int
	if err := pool.QueryRow(ctx, `SELECT remaining_seconds FROM game_access_grants WHERE id=$1`, legacyGrant).Scan(&repaired); err != nil || repaired < 3500 || repaired > 3600 {
		t.Fatalf("legacy remainder was not restored: %d %v", repaired, err)
	}
	if err := s.reconcileMissingProfileRemainders(ctx, player); err != nil {
		t.Fatal(err)
	}
	var returns int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_time_ledger WHERE game_access_grant_id=$1 AND kind='session_remaining'`, legacyGrant).Scan(&returns); err != nil || returns != 1 {
		t.Fatalf("legacy remainder duplicated: %d %v", returns, err)
	}

	// Some controller releases incorrectly label a manually stopped session as
	// time_expired. Its recorded timestamp is still before the planned end, so
	// it must be restored exactly once as well.
	expiredLabelGrant := createEarlyEndGrant("legacy-end-labelled-expired")
	if _, err := pool.Exec(ctx, `UPDATE game_access_grants SET status='ended',ended_at=now(),end_reason='time_expired',remaining_seconds=0,remaining_minutes=0 WHERE id=$1`, expiredLabelGrant); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcileMissingProfileRemainders(ctx, player); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT remaining_seconds FROM game_access_grants WHERE id=$1`, expiredLabelGrant).Scan(&repaired); err != nil || repaired < 3500 || repaired > 3600 {
		t.Fatalf("early session labelled expired was not restored: %d %v", repaired, err)
	}
}

func TestPlayerSessionHandoffIntegration(t *testing.T) {
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
	schema := "handoff_test_" + randomHex(6)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	var club, firstPC, secondPC, firstExternal, secondExternal, player string
	if err = pool.QueryRow(ctx, `
		SELECT p.club_id, p.id, p.external_pc_id
		FROM pc_refs p
		ORDER BY p.number
		LIMIT 1
	`).Scan(&club, &firstPC, &firstExternal); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `
		SELECT p.id, p.external_pc_id
		FROM pc_refs p
		WHERE p.club_id=$1 AND p.id<>$2
		ORDER BY p.number
		LIMIT 1
	`, club, firstPC).Scan(&secondPC, &secondExternal); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO players(phone) VALUES('+998900000077') RETURNING id`).Scan(&player); err != nil {
		t.Fatal(err)
	}
	var previousGrant, nextGrant string
	if err = pool.QueryRow(ctx, `
		INSERT INTO game_access_grants (
			club_id, pc_ref_id, player_id, duration_minutes, duration_seconds,
			status, source, core_session_id, accepted_at, planned_ends_at, grace_ends_at
		) VALUES ($1,$2,$3,60,3600,'accepted','online_payment','old-player-session',now(),now()+interval '1 hour',now()+interval '1 hour')
		RETURNING id
	`, club, firstPC, player).Scan(&previousGrant); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `
		INSERT INTO game_access_grants (
			club_id, pc_ref_id, player_id, duration_minutes, duration_seconds, status, source
		) VALUES ($1,$2,$3,30,1800,'pending','online_payment')
		RETURNING id
	`, club, secondPC, player).Scan(&nextGrant); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE pc_refs SET status_cache='occupied' WHERE id=$1`, firstPC); err != nil {
		t.Fatal(err)
	}

	mock := core.NewMockAdapter()
	controller := NewServer(config.Config{NodeMode: "edge", SessionGraceSeconds: 180}, pool, mock)
	if !controller.startPendingEdgeGrants(ctx, club) {
		t.Fatal("controller did not start the new player session")
	}
	if mock.EndCount("old-player-session") != 1 {
		t.Fatalf("previous Agent session was not closed: %d", mock.EndCount("old-player-session"))
	}
	var oldStatus, oldReason, nextStatus string
	var remainder int
	if err = pool.QueryRow(ctx, `SELECT status, end_reason, remaining_seconds FROM game_access_grants WHERE id=$1`, previousGrant).Scan(&oldStatus, &oldReason, &remainder); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "ended" || oldReason != "player_switched_pc" || remainder <= 0 {
		t.Fatalf("previous grant was not safely returned: %s %s %d", oldStatus, oldReason, remainder)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM game_access_grants WHERE id=$1`, nextGrant).Scan(&nextStatus); err != nil {
		t.Fatal(err)
	}
	if nextStatus != "accepted" {
		t.Fatalf("new player grant did not start: %s", nextStatus)
	}
	var activeSessions int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM game_access_grants
		WHERE player_id=$1 AND status='accepted' AND parent_grant_id IS NULL
		  AND COALESCE(grace_ends_at,planned_ends_at)>now()
	`, player).Scan(&activeSessions); err != nil {
		t.Fatal(err)
	}
	if activeSessions != 1 {
		t.Fatalf("player has %d active sessions, want 1", activeSessions)
	}
	var firstStatus, secondStatus string
	if err = pool.QueryRow(ctx, `SELECT status_cache FROM pc_refs WHERE id=$1`, firstPC).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status_cache FROM pc_refs WHERE id=$1`, secondPC).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "available" || secondStatus != "occupied" {
		t.Fatalf("PC handoff mismatch: %s=%s, %s=%s", firstExternal, firstStatus, secondExternal, secondStatus)
	}
}
