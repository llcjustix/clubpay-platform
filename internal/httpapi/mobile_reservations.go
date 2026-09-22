package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	reservationLeadTime      = 15 * time.Minute
	reservationArrivalWindow = 15 * time.Minute
	reservationMaxAhead      = 30 * 24 * time.Hour
)

func reservationJSON(rows []map[string]any) []map[string]any {
	if rows == nil {
		return []map[string]any{}
	}
	return rows
}

func (s *Server) handleMobileReservations(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	// Expiry is also evaluated here, so an unclaimed reservation stops blocking
	// the PC even if a background worker was temporarily unavailable.
	_, _ = s.db.Exec(r.Context(), `UPDATE mobile_reservations
SET status='expired',updated_at=now()
WHERE status IN ('confirmed','checked_in','started') AND starts_at + interval '15 minutes' < now()`)
	_, _ = s.db.Exec(r.Context(), `UPDATE mobile_reservations r
SET status='started',updated_at=now()
WHERE r.status='checked_in' AND EXISTS (
  SELECT 1 FROM game_access_grants g
  WHERE g.pc_ref_id=r.pc_ref_id AND g.player_id=r.player_id AND g.status='accepted'
)`)
	rows, err := s.queryMaps(r.Context(), `SELECT r.id::text,r.pc_ref_id::text,r.status,r.starts_at,
 r.starts_at + make_interval(mins => r.duration_minutes) AS ends_at,
 r.starts_at - interval '15 minutes' AS held_from,
 r.starts_at + interval '15 minutes' AS checkin_deadline,
 r.duration_minutes/60 AS duration_hours,c.name AS club_name,z.name AS zone_name,p.label AS pc_label
FROM mobile_reservations r
JOIN clubs c ON c.id=r.club_id JOIN pc_refs p ON p.id=r.pc_ref_id JOIN zones z ON z.id=p.zone_id
WHERE r.player_id=$1 AND r.status IN ('confirmed','checked_in') AND r.starts_at > now()-interval '24 hours'
ORDER BY r.starts_at ASC`, p.ID)
	if err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservations": reservationJSON(rows)})
}

func (s *Server) handleMobileReservationCreate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	var req struct {
		PCID          string `json:"pc_id"`
		StartsAt      string `json:"starts_at"`
		DurationHours int    `json:"duration_hours"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil || strings.TrimSpace(req.PCID) == "" || req.DurationHours < 1 || req.DurationHours > 24 {
		writeError(w, http.StatusBadRequest, "invalid_reservation")
		return
	}
	now := time.Now().UTC()
	if start.Before(now.Add(reservationLeadTime)) || start.After(now.Add(reservationMaxAhead)) {
		writeError(w, http.StatusBadRequest, "reservation_start_out_of_range")
		return
	}
	start = start.UTC()
	durationMinutes := req.DurationHours * 60
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	var clubID, clubName, zoneName, label string
	err = tx.QueryRow(r.Context(), `SELECT p.club_id::text,c.name,z.name,p.label
FROM pc_refs p JOIN clubs c ON c.id=p.club_id JOIN zones z ON z.id=p.zone_id
WHERE p.id=$1 AND p.status_cache<>'deleted' AND c.status='active' AND z.status='active'
FOR SHARE OF p`, req.PCID).Scan(&clubID, &clubName, &zoneName, &label)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "pc_not_found")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	// One player can hold one future/current reservation at a time. Lock the
	// player first, so two simultaneous requests for different clubs cannot
	// create two bookings.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtext($1::text))`, p.ID); err != nil {
		mobileInternal(w)
		return
	}
	var playerAlreadyReserved bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM mobile_reservations
WHERE player_id=$1 AND status IN ('confirmed','checked_in')
  AND starts_at + interval '15 minutes' >= now())`, p.ID).Scan(&playerAlreadyReserved)
	if err != nil {
		mobileInternal(w)
		return
	}
	if playerAlreadyReserved {
		writeError(w, http.StatusConflict, "player_already_has_reservation")
		return
	}
	// Serialize reservation creation for one PC. This is deliberately held in
	// the transaction rather than delegated to an exclusion index: PostgreSQL
	// rejects the timestamptz interval expression in that index as non-immutable.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtext($1::text))`, req.PCID); err != nil {
		mobileInternal(w)
		return
	}
	windowStart := start.Add(-reservationLeadTime)
	windowEnd := start.Add(time.Duration(durationMinutes)*time.Minute + reservationArrivalWindow)
	var alreadyReserved bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM mobile_reservations
WHERE pc_ref_id=$1 AND status IN ('confirmed','checked_in')
  AND starts_at - interval '15 minutes' < $3
  AND starts_at + make_interval(mins => duration_minutes + 15) > $2)`, req.PCID, windowStart, windowEnd).Scan(&alreadyReserved)
	if err != nil {
		mobileInternal(w)
		return
	}
	if alreadyReserved {
		writeError(w, http.StatusConflict, "pc_already_reserved")
		return
	}
	// A currently accepted session with no planned end is treated as occupied;
	// otherwise its planned end must clear the reservation hold window.
	var busy bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM game_access_grants
WHERE pc_ref_id=$1 AND status='accepted' AND (planned_ends_at IS NULL OR planned_ends_at > $2))`, req.PCID, start.Add(-reservationLeadTime)).Scan(&busy)
	if err != nil {
		mobileInternal(w)
		return
	}
	if busy {
		writeError(w, http.StatusConflict, "pc_busy_for_reservation")
		return
	}
	entryCode, err := newReservationEntryCode()
	if err != nil {
		mobileInternal(w)
		return
	}
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO mobile_reservations(player_id,club_id,pc_ref_id,starts_at,duration_minutes,entry_code)
VALUES($1,$2::uuid,$3::uuid,$4,$5,$6) RETURNING id::text`, p.ID, clubID, req.PCID, start, durationMinutes, entryCode).Scan(&id)
	if err != nil {
		mobileInternal(w)
		return
	}
	if err = mobileAudit(r.Context(), tx, "mobile_reservation_created", id); err != nil {
		mobileInternal(w)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"reservation": map[string]any{
			"id": id, "pc_id": req.PCID, "status": "confirmed", "club_name": clubName, "zone_name": zoneName, "pc_label": label,
			"starts_at": start, "ends_at": start.Add(time.Duration(durationMinutes) * time.Minute),
			"held_from": start.Add(-reservationLeadTime), "checkin_deadline": start.Add(reservationArrivalWindow),
			"duration_hours": req.DurationHours,
		},
	})
}

func newReservationEntryCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *Server) handleMobileReservationReschedule(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("reservation_id"))
	var req struct {
		StartsAt      string `json:"starts_at"`
		DurationHours int    `json:"duration_hours"`
	}
	if id == "" || !mobileDecode(w, r, &req) {
		if id == "" {
			writeError(w, http.StatusBadRequest, "reservation_id_required")
		}
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil || req.DurationHours < 1 || req.DurationHours > 24 {
		writeError(w, http.StatusBadRequest, "invalid_reservation")
		return
	}
	start = start.UTC()
	if start.Before(time.Now().UTC().Add(reservationLeadTime)) || start.After(time.Now().UTC().Add(reservationMaxAhead)) {
		writeError(w, http.StatusBadRequest, "reservation_start_out_of_range")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	var pcID, clubID, clubName, zoneName, label string
	err = tx.QueryRow(r.Context(), `SELECT r.pc_ref_id::text,r.club_id::text,c.name,z.name,p.label
FROM mobile_reservations r JOIN clubs c ON c.id=r.club_id JOIN pc_refs p ON p.id=r.pc_ref_id JOIN zones z ON z.id=p.zone_id
WHERE r.id=$1::uuid AND r.player_id=$2 AND r.status='confirmed' AND r.starts_at>now()+interval '15 minutes'
FOR UPDATE OF r`, id, p.ID).Scan(&pcID, &clubID, &clubName, &zoneName, &label)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "reservation_cannot_be_cancelled")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtext($1::text))`, pcID); err != nil {
		mobileInternal(w)
		return
	}
	minutes := req.DurationHours * 60
	windowStart := start.Add(-reservationLeadTime)
	windowEnd := start.Add(time.Duration(minutes)*time.Minute + reservationArrivalWindow)
	var conflict bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM mobile_reservations
WHERE pc_ref_id=$1::uuid AND id<>$2::uuid AND status IN ('confirmed','checked_in')
  AND starts_at - interval '15 minutes' < $4
  AND starts_at + make_interval(mins => duration_minutes + 15) > $3)`, pcID, id, windowStart, windowEnd).Scan(&conflict)
	if err != nil {
		mobileInternal(w)
		return
	}
	if conflict {
		writeError(w, http.StatusConflict, "pc_already_reserved")
		return
	}
	var busy bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM game_access_grants
WHERE pc_ref_id=$1::uuid AND status='accepted' AND (planned_ends_at IS NULL OR planned_ends_at > $2))`, pcID, windowStart).Scan(&busy)
	if err != nil {
		mobileInternal(w)
		return
	}
	if busy {
		writeError(w, http.StatusConflict, "pc_busy_for_reservation")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE mobile_reservations SET starts_at=$3,duration_minutes=$4,updated_at=now() WHERE id=$1::uuid AND player_id=$2`, id, p.ID, start, minutes)
	if err != nil {
		mobileInternal(w)
		return
	}
	if err = mobileAudit(r.Context(), tx, "mobile_reservation_rescheduled", id); err != nil {
		mobileInternal(w)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservation": map[string]any{
		"id": id, "pc_id": pcID, "status": "confirmed",
		"club_name": clubName, "zone_name": zoneName, "pc_label": label,
		"starts_at": start, "ends_at": start.Add(time.Duration(minutes) * time.Minute),
		"held_from": start.Add(-reservationLeadTime), "checkin_deadline": start.Add(reservationArrivalWindow),
		"duration_hours": req.DurationHours,
	}})
}

// handleMobileReservationStart opens the normal mobile payment/start flow for
// the reservation owner. No code is displayed or entered on the PC.
func (s *Server) handleMobileReservationStart(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("reservation_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "reservation_id_required")
		return
	}

	var token string
	err := s.db.QueryRow(r.Context(), `
		SELECT COALESCE(q.public_token, '')
		FROM mobile_reservations r
		JOIN pc_refs pc ON pc.id=r.pc_ref_id
		LEFT JOIN LATERAL (
			SELECT public_token FROM qr_codes
			WHERE pc_ref_id=pc.id AND type='static_pc' AND status='active'
			ORDER BY created_at DESC LIMIT 1
		) q ON true
		WHERE r.id=$1::uuid AND r.player_id=$2 AND r.status IN ('confirmed','checked_in')
		  AND r.starts_at - interval '15 minutes' <= now()
		  AND r.starts_at + interval '15 minutes' >= now()
		FOR UPDATE OF r
	`, id, p.ID).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "reservation_not_active")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	if token == "" {
		writeError(w, http.StatusConflict, "reservation_pc_unavailable")
		return
	}
	if _, err = s.db.Exec(r.Context(), `UPDATE mobile_reservations SET status='started',updated_at=now() WHERE id=$1::uuid AND player_id=$2`, id, p.ID); err != nil {
		mobileInternal(w)
		return
	}
	_, _ = s.db.Exec(r.Context(), `INSERT INTO audit_logs(action,entity_type,entity_id,metadata) VALUES('mobile_reservation_started','mobile_reservation',$1,'{}')`, id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "pc_token": token})
}

// reservationAllowsPlayerSession keeps a held PC private to its reservation
// owner.  Reaching the real checkout or balance-redemption action is itself
// an unambiguous check-in: a Cloud/edge sync can otherwise briefly restore the
// replicated reservation as "confirmed" after the mobile start action has
// completed, trapping its owner behind a stale intermediate state.
func reservationAllowsPlayerSession(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, pcID, playerID string) (bool, error) {
	var reservationPlayerID, status string
	err := q.QueryRow(ctx, `
		SELECT player_id::text,status
		FROM mobile_reservations
		WHERE pc_ref_id=$1::uuid AND status IN ('confirmed','checked_in','started')
		  AND starts_at-interval '15 minutes'<=now()
		  AND starts_at+interval '15 minutes'>=now()
		ORDER BY starts_at ASC LIMIT 1
	`, pcID).Scan(&reservationPlayerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return (status == "confirmed" || status == "checked_in" || status == "started") &&
		playerID != "" && reservationPlayerID == playerID, nil
}

func (s *Server) handleMobileReservationCancel(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("reservation_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "reservation_id_required")
		return
	}
	// The owner can cancel while the PC is still not in the protected 15 minute
	// window. A manager can handle exceptional late changes.
	tag, err := s.db.Exec(r.Context(), `UPDATE mobile_reservations
SET status='cancelled',cancelled_at=now(),updated_at=now()
WHERE id=$1::uuid AND player_id=$2 AND status='confirmed' AND starts_at>now()+interval '15 minutes'`, id, p.ID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "reservation_cannot_be_cancelled")
		return
	}
	_, _ = s.db.Exec(r.Context(), `INSERT INTO audit_logs(action,entity_type,entity_id,metadata) VALUES('mobile_reservation_cancelled','mobile_reservation',$1,'{}')`, id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "reservation_id": id})
}
