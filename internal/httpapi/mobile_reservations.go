package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	reservationLeadTime      = 30 * time.Minute
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
WHERE status='confirmed' AND starts_at + interval '15 minutes' < now()`)
	rows, err := s.queryMaps(r.Context(), `SELECT r.id::text,r.status,r.starts_at,
 r.starts_at + make_interval(mins => r.duration_minutes) AS ends_at,
 r.starts_at - interval '30 minutes' AS held_from,
 r.starts_at + interval '15 minutes' AS checkin_deadline,
 r.duration_minutes/60 AS duration_hours,c.name AS club_name,z.name AS zone_name,p.label AS pc_label
FROM mobile_reservations r
JOIN clubs c ON c.id=r.club_id JOIN pc_refs p ON p.id=r.pc_ref_id JOIN zones z ON z.id=p.zone_id
WHERE r.player_id=$1 AND r.starts_at > now()-interval '24 hours'
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
  AND starts_at - interval '30 minutes' < $3
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
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO mobile_reservations(player_id,club_id,pc_ref_id,starts_at,duration_minutes)
VALUES($1,$2::uuid,$3::uuid,$4,$5) RETURNING id::text`, p.ID, clubID, req.PCID, start, durationMinutes).Scan(&id)
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
			"id": id, "status": "confirmed", "club_name": clubName, "zone_name": zoneName, "pc_label": label,
			"starts_at": start, "ends_at": start.Add(time.Duration(durationMinutes) * time.Minute),
			"held_from": start.Add(-reservationLeadTime), "checkin_deadline": start.Add(reservationArrivalWindow),
			"duration_hours": req.DurationHours,
		},
	})
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
	// The owner can cancel while the PC is still not in the protected 30 minute
	// window. A manager can handle exceptional late changes.
	tag, err := s.db.Exec(r.Context(), `UPDATE mobile_reservations
SET status='cancelled',cancelled_at=now(),updated_at=now()
WHERE id=$1::uuid AND player_id=$2 AND status='confirmed' AND starts_at>now()+interval '30 minutes'`, id, p.ID)
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
