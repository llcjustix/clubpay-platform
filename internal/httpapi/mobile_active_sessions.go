package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"clubpay/internal/core"
)

// handleMobileActiveSession gives the signed-in player their one live session.
// The extension token is never displayed as a QR code in the mobile app: it is
// used only to enter the same checked payment flow from the "Продлить" action.
func (s *Server) handleMobileActiveSession(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}

	rows, err := s.queryMaps(r.Context(), `
		SELECT g.id::text AS grant_id,
		       c.name AS club_name,
		       z.name AS zone_name,
		       pc.label AS pc_label,
		       COALESCE(g.planned_ends_at, g.accepted_at + make_interval(secs => g.duration_seconds), g.accepted_at + make_interval(mins => g.duration_minutes)) AS ends_at,
		       GREATEST(CEIL(EXTRACT(EPOCH FROM (
		         COALESCE(g.planned_ends_at, g.accepted_at + make_interval(secs => g.duration_seconds), g.accepted_at + make_interval(mins => g.duration_minutes)) - now()
		       )))::int, 0) AS remaining_seconds,
		       COALESCE(q.public_token, '') AS extend_token
		FROM game_access_grants g
		JOIN pc_refs pc ON pc.id=g.pc_ref_id
		JOIN clubs c ON c.id=g.club_id
		JOIN zones z ON z.id=pc.zone_id
		LEFT JOIN payment_orders po ON po.id=g.payment_order_id
		LEFT JOIN LATERAL (
		  SELECT public_token
		  FROM qr_codes
		  WHERE session_grant_id=g.id AND type='session_extend' AND status='active' AND expires_at>now()
		  ORDER BY created_at DESC LIMIT 1
		) q ON true
		WHERE g.parent_grant_id IS NULL
		  AND g.status='accepted'
		  AND COALESCE(g.player_id, po.player_id)::text=$1
		  AND COALESCE(g.grace_ends_at, g.planned_ends_at, g.accepted_at + make_interval(secs => g.duration_seconds), g.accepted_at + make_interval(mins => g.duration_minutes)) > now()
		ORDER BY g.accepted_at DESC NULLS LAST, g.created_at DESC
		LIMIT 1
	`, p.ID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if len(rows) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": rows[0]})
}

// handleMobileActiveSessionEnd lets a player leave a live session from their
// phone. The core command is confirmed before ClubPay closes the grant, which
// keeps the Agent screen and the home card in the same state.
func (s *Server) handleMobileActiveSessionEnd(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}

	var grantID, coreSessionID, externalPCID string
	var remainingSeconds int
	err := s.db.QueryRow(r.Context(), `
		SELECT g.id::text, COALESCE(g.core_session_id,''), pc.external_pc_id,
		       GREATEST(CEIL(EXTRACT(EPOCH FROM (
		         COALESCE(g.planned_ends_at, g.accepted_at + make_interval(secs => g.duration_seconds), g.accepted_at + make_interval(mins => g.duration_minutes)) - now()
		       )))::int, 0)
		FROM game_access_grants g
		JOIN pc_refs pc ON pc.id=g.pc_ref_id
		LEFT JOIN payment_orders po ON po.id=g.payment_order_id
		WHERE g.parent_grant_id IS NULL
		  AND g.status='accepted'
		  AND COALESCE(g.player_id, po.player_id)::text=$1
		  AND COALESCE(g.grace_ends_at, g.planned_ends_at, g.accepted_at + make_interval(secs => g.duration_seconds), g.accepted_at + make_interval(mins => g.duration_minutes)) > now()
		ORDER BY g.accepted_at DESC NULLS LAST, g.created_at DESC
		LIMIT 1
		FOR UPDATE OF g
	`, p.ID).Scan(&grantID, &coreSessionID, &externalPCID, &remainingSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "active_session_not_found")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	if strings.TrimSpace(coreSessionID) == "" {
		writeError(w, http.StatusConflict, "active_session_not_ready")
		return
	}

	result, err := s.core.EndSession(r.Context(), coreSessionID, core.EndSessionCommand{
		RequestID:    "mobile_end_" + grantID + "_" + randomHex(4),
		ExternalPCID: externalPCID,
		Reason:       "CLIENT_LEFT",
		EndedBy:      map[string]string{"type": "player", "id": p.ID},
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "agent did not confirm session end: "+err.Error())
		return
	}
	if result.RemainingSeconds > 0 {
		remainingSeconds = result.RemainingSeconds
	}
	finished, err := s.finishGrant(r.Context(), grantID, "player_left_from_mobile", remainingSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "session": finished})
}
