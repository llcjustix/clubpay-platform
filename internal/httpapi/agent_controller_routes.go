package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

func normalizeAgentControllerRoutes(req agentEnrollmentRequest) (agentEnrollmentRequest, error) {
	primary, err := normalizeAgentControllerURL(req.ControllerURL)
	if err != nil {
		return agentEnrollmentRequest{}, err
	}
	fallback := ""
	if strings.TrimSpace(req.FallbackControllerURL) != "" {
		fallback, err = normalizeAgentControllerURL(req.FallbackControllerURL)
		if err != nil {
			return agentEnrollmentRequest{}, fmt.Errorf("fallback_controller_url: %w", err)
		}
	}
	if fallback == primary {
		fallback = ""
	}
	return agentEnrollmentRequest{ControllerURL: primary, FallbackControllerURL: fallback}, nil
}

func (s *Server) agentControllerRoutes(ctx context.Context, clubID string) (agentEnrollmentRequest, error) {
	var routes agentEnrollmentRequest
	err := s.db.QueryRow(ctx, `SELECT COALESCE(agent_primary_controller_url, ''),
 COALESCE(agent_fallback_controller_url, '') FROM clubs WHERE id=$1 AND status <> 'deleted'`, clubID).
		Scan(&routes.ControllerURL, &routes.FallbackControllerURL)
	return routes, err
}

// This changes future enrollments. Existing Agent routes remain per-PC until
// that PC is explicitly enrolled again, so saving is not a live fleet remap.
func (s *Server) handleBackofficeSaveAgentControllerRoutes(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner"); !ok {
		return
	}
	var req agentEnrollmentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	routes, err := normalizeAgentControllerRoutes(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(r.Context(), `UPDATE clubs SET agent_primary_controller_url=$2,
 agent_fallback_controller_url=NULLIF($3, '') WHERE id=$1 AND status <> 'deleted'`,
		clubID, routes.ControllerURL, routes.FallbackControllerURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "club not found")
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func writeAgentRoutesReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "club not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
