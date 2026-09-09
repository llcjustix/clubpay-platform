package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
)

// timeValue is measured in tiyin*seconds/hour, never a cash wallet.
func timeValue(seconds int, rate int64) (int64, error) {
	if seconds < 0 || rate <= 0 || int64(seconds) > math.MaxInt64/rate {
		return 0, fmt.Errorf("invalid time value")
	}
	return int64(seconds) * rate, nil
}
func timeAtRate(units, rate int64) (int, error) {
	if units < 0 || rate <= 0 {
		return 0, fmt.Errorf("invalid time rate")
	}
	seconds := units / rate
	if seconds > math.MaxInt32 {
		return 0, fmt.Errorf("time balance exceeds session limit")
	}
	return int(seconds), nil
}

// Lazy initialization also covers old Controller snapshots and imported rows.
func (s *Server) lockTimeValue(ctx context.Context, tx pgx.Tx, playerID, clubID string) (units, reference int64, err error) {
	_, err = tx.Exec(ctx, `INSERT INTO player_club_balances(player_id,club_id,seconds_balance) VALUES($1,$2,0) ON CONFLICT DO NOTHING`, playerID, clubID)
	if err != nil {
		return
	}
	err = tx.QueryRow(ctx, `SELECT COALESCE(time_value_units, seconds_balance::bigint*COALESCE(reference_price_tiyin,(SELECT MIN(hourly_price_tiyin) FROM zones WHERE club_id=$2 AND status<>'deleted'))), COALESCE(reference_price_tiyin,(SELECT MIN(hourly_price_tiyin) FROM zones WHERE club_id=$2 AND status<>'deleted')) FROM player_club_balances WHERE player_id=$1 AND club_id=$2 FOR UPDATE`, playerID, clubID).Scan(&units, &reference)
	if err != nil {
		return
	}
	if reference <= 0 {
		err = fmt.Errorf("club has no priced zone")
		return
	}
	_, err = tx.Exec(ctx, `UPDATE player_club_balances SET time_value_units=$3,reference_price_tiyin=$4 WHERE player_id=$1 AND club_id=$2`, playerID, clubID, units, reference)
	return
}

func (s *Server) balanceSecondsForPC(ctx context.Context, tx pgx.Tx, playerID, clubID, pcID string) (int, error) {
	units, _, err := s.lockTimeValue(ctx, tx, playerID, clubID)
	if err != nil {
		return 0, err
	}
	var rate int64
	err = tx.QueryRow(ctx, `SELECT CASE WHEN p.status_cache IN ('occupied','frozen') THEN COALESCE((SELECT g.time_value_rate FROM game_access_grants g WHERE g.pc_ref_id=p.id AND g.status='accepted' ORDER BY g.accepted_at DESC NULLS LAST,g.created_at DESC LIMIT 1),z.hourly_price_tiyin) ELSE z.hourly_price_tiyin END FROM pc_refs p JOIN zones z ON z.id=p.zone_id WHERE p.id=$1 AND p.club_id=$2 FOR SHARE OF z`, pcID, clubID).Scan(&rate)
	if err != nil {
		return 0, err
	}
	return timeAtRate(units, rate)
}

func (s *Server) recordTimeValue(ctx context.Context, tx pgx.Tx, playerID, clubID string, secondsDelta int, kind, grantID, paymentOrderID, key string) error {
	units, reference, err := s.lockTimeValue(ctx, tx, playerID, clubID)
	if err != nil {
		return err
	}
	var existing bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_time_ledger WHERE idempotency_key=$1)`, key).Scan(&existing); err != nil {
		return err
	}
	if existing {
		return nil
	}
	rate := reference
	if grantID != "" {
		if err = tx.QueryRow(ctx, `SELECT COALESCE(time_value_rate,$2) FROM game_access_grants WHERE id=$1`, grantID, reference).Scan(&rate); err != nil {
			return err
		}
	}
	absolute := secondsDelta
	if absolute < 0 {
		absolute = -absolute
	}
	delta, err := timeValue(absolute, rate)
	if err != nil {
		return err
	}
	if secondsDelta < 0 {
		delta = -delta
	}
	if kind == "session_start_refund" {
		// Refund the exact consumed value, even if prices changed after submission.
		err = tx.QueryRow(ctx, `SELECT COALESCE(-time_value_delta,$4) FROM player_time_ledger WHERE game_access_grant_id=$1 AND kind='session_start' AND player_id=$2 AND club_id=$3`, grantID, playerID, clubID, delta).Scan(&delta)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	if delta > 0 && units > math.MaxInt64-delta {
		return fmt.Errorf("time balance overflow")
	}
	if units+delta < 0 {
		return fmt.Errorf("insufficient player balance")
	}
	projected, err := timeAtRate(units+delta, reference)
	if err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO player_time_ledger(player_id,club_id,seconds_delta,kind,game_access_grant_id,payment_order_id,idempotency_key,time_value_delta) VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8) ON CONFLICT(idempotency_key) DO NOTHING RETURNING id`, playerID, clubID, secondsDelta, kind, grantID, paymentOrderID, key, delta).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE player_club_balances SET time_value_units=$3,seconds_balance=$4,updated_at=now() WHERE player_id=$1 AND club_id=$2`, playerID, clubID, units+delta, projected)
	return err
}

// Pending checkouts retain their captured rate even if an operator edits prices
// before the provider callback arrives.
func (s *Server) balanceSecondsForGrant(ctx context.Context, tx pgx.Tx, playerID, clubID, pcID, grantID string) (int, error) {
	if grantID == "" {
		return s.balanceSecondsForPC(ctx, tx, playerID, clubID, pcID)
	}
	units, _, err := s.lockTimeValue(ctx, tx, playerID, clubID)
	if err != nil {
		return 0, err
	}
	var rate int64
	err = tx.QueryRow(ctx, `SELECT time_value_rate FROM game_access_grants WHERE id=$1 AND club_id=$2 AND pc_ref_id=$3 FOR UPDATE`, grantID, clubID, pcID).Scan(&rate)
	if err != nil {
		return 0, err
	}
	return timeAtRate(units, rate)
}
