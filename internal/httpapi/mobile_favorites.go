package httpapi

import (
	"net/http"
	"strings"
)

// The JavaScript Maps key is public once used inside a WebView. Fetching it
// from Cloud at runtime, rather than compiling it into an IPA, means a manual
// development build cannot accidentally ship without map configuration.
func (s *Server) handleMobileMapConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireMobile(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"yandex_maps_api_key": s.cfg.YandexMapsAPIKey,
		"configured":          strings.TrimSpace(s.cfg.YandexMapsAPIKey) != "",
	})
}

func (s *Server) handleMobileFavorites(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	rows, err := s.queryMaps(r.Context(), `SELECT c.id AS club_id,c.name AS club_name,
      COALESCE(c.address,'') AS address,c.latitude,c.longitude,
      CASE WHEN $2::boolean THEN COALESCE(c.controller_synced_at>now()-interval '45 seconds',false)
        ELSE EXISTS(SELECT 1 FROM pc_refs live WHERE live.club_id=c.id AND live.status_cache IN ('available','sleeping','occupied','frozen')) END AS club_online,
      COUNT(*) FILTER (WHERE p.status_cache IN ('available','sleeping') AND held.pc_ref_id IS NULL)::int AS available_pcs,
      COUNT(*)::int AS total_pcs, true AS favorite
    FROM mobile_favorite_clubs f JOIN clubs c ON c.id=f.club_id
    JOIN pc_refs p ON p.club_id=c.id AND p.status_cache<>'deleted'
    JOIN zones z ON z.id=p.zone_id AND z.status='active'
    LEFT JOIN LATERAL (SELECT r.pc_ref_id FROM mobile_reservations r
      WHERE r.pc_ref_id=p.id AND r.status IN ('confirmed','checked_in')
        AND r.starts_at-interval '15 minutes'<=now()
        AND r.starts_at+make_interval(mins=>r.duration_minutes+15)>now() LIMIT 1) held ON true
    WHERE f.player_id=$1 AND c.status='active'
    GROUP BY c.id,c.name,c.address,c.latitude,c.longitude,c.controller_synced_at,f.created_at
    ORDER BY f.created_at DESC`, p.ID, s.localNodeMode())
	if err != nil {
		mobileInternal(w)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"clubs": rows})
}

func (s *Server) handleMobileFavoriteAdd(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	clubID := strings.TrimSpace(r.PathValue("club_id"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "club_id_required")
		return
	}
	tag, err := s.db.Exec(r.Context(), `INSERT INTO mobile_favorite_clubs(player_id,club_id)
      SELECT $1,id FROM clubs WHERE id=$2::uuid AND status='active' ON CONFLICT DO NOTHING`, p.ID, clubID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM clubs WHERE id=$1::uuid AND status='active')`, clubID).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusNotFound, "club_not_found")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"favorite": true, "club_id": clubID})
}

func (s *Server) handleMobileFavoriteRemove(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	clubID := strings.TrimSpace(r.PathValue("club_id"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "club_id_required")
		return
	}
	if _, err := s.db.Exec(r.Context(), `DELETE FROM mobile_favorite_clubs WHERE player_id=$1 AND club_id=$2::uuid`, p.ID, clubID); err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"favorite": false, "club_id": clubID})
}
