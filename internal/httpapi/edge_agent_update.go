package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"clubpay/internal/core"
)

// edgeAgentUpdateRequest is a one-time recovery path for an Agent that is
// still connected to Cloud while its local Controller route is being repaired.
// It carries only a public, checksum-verified release reference; no secrets or
// executable bytes are proxied through Cloud.
type edgeAgentUpdateRequest struct {
	ClubID       string `json:"club_id"`
	ExternalPCID string `json:"external_pc_id"`
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	ChecksumURL  string `json:"checksum_url"`
}

// dispatchAgentUpdate handles the normal signed update command. Older Agents
// treated their idle locked screen as "frozen" and rejected update_agent,
// even though no session was running. For that one legacy state we temporarily
// enter repair mode (the kiosk remains locked), schedule the replacement, then
// restore the normal locked state. Occupied and paused sessions never qualify.
func (s *Server) dispatchAgentUpdate(ctx context.Context, externalPCID string, update core.AgentUpdateCommand) error {
	dispatcher, ok := s.core.(core.AgentUpdateDispatcher)
	if !ok {
		return fmt.Errorf("Agent update dispatcher is unavailable")
	}
	err := dispatcher.UpdateAgent(ctx, externalPCID, update)
	if err == nil || !strings.HasPrefix(strings.ToLower(err.Error()), "pc_busy:") {
		return err
	}

	adapter, ok := s.core.(core.Adapter)
	if !ok {
		return err
	}
	status, statusErr := adapter.GetPCStatus(ctx, externalPCID)
	if statusErr != nil || !strings.EqualFold(status.Status, "frozen") || strings.TrimSpace(status.CurrentSessionID) != "" || status.RemainingSeconds > 0 {
		return err
	}
	if repairErr := adapter.SetRepair(ctx, externalPCID, true); repairErr != nil {
		return fmt.Errorf("legacy idle update preparation: %w", repairErr)
	}
	retryErr := dispatcher.UpdateAgent(ctx, externalPCID, update)
	// UpdateAgent has already handed off its detached updater once it succeeds.
	// Restore the visible locked state either way; the restart will preserve no
	// transient repair flag.
	restoreErr := adapter.SetRepair(ctx, externalPCID, false)
	if retryErr != nil {
		return retryErr
	}
	if restoreErr != nil {
		return fmt.Errorf("Agent update was scheduled but locked state restore failed: %w", restoreErr)
	}
	return nil
}

func (s *Server) requestCloudAgentUpdate(ctx context.Context, clubID, externalPCID string, update core.AgentUpdateCommand) error {
	if !s.edgeNodeMode() || strings.TrimSpace(s.cfg.CloudBaseURL) == "" {
		return fmt.Errorf("cloud recovery is unavailable")
	}
	req := edgeAgentUpdateRequest{
		ClubID:       strings.TrimSpace(clubID),
		ExternalPCID: strings.TrimSpace(externalPCID),
		Version:      strings.TrimSpace(update.Version),
		DownloadURL:  strings.TrimSpace(update.DownloadURL),
		ChecksumURL:  strings.TrimSpace(update.ChecksumURL),
	}
	if req.ClubID == "" || req.ExternalPCID == "" || req.Version == "" || req.DownloadURL == "" || req.ChecksumURL == "" {
		return fmt.Errorf("agent update recovery payload is incomplete")
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := s.postCloudJSON(ctx, "/api/edge/agent-updates", req, &result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("cloud did not accept the Agent update")
	}
	return nil
}

// handleEdgeAgentUpdate asks Cloud's already-authenticated Agent WebSocket to
// perform the normal signed replacement. It is intentionally available only to
// the Controller enrolled for this club; a browser user cannot invoke it.
func (s *Server) handleEdgeAgentUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.cloudNodeMode() {
		writeError(w, http.StatusNotFound, "agent recovery is available only in Cloud")
		return
	}
	authorizedClubID, ok := s.requireEdge(w, r)
	if !ok {
		return
	}
	var req edgeAgentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.ClubID = strings.TrimSpace(req.ClubID)
	req.ExternalPCID = strings.TrimSpace(req.ExternalPCID)
	req.Version = strings.TrimSpace(req.Version)
	req.DownloadURL = strings.TrimSpace(req.DownloadURL)
	req.ChecksumURL = strings.TrimSpace(req.ChecksumURL)
	if req.ClubID == "" || req.ExternalPCID == "" || req.Version == "" || req.DownloadURL == "" || req.ChecksumURL == "" {
		writeError(w, http.StatusBadRequest, "club_id, external_pc_id and complete update artifact are required")
		return
	}
	if authorizedClubID != "" && authorizedClubID != req.ClubID {
		writeError(w, http.StatusForbidden, "controller is not authorized for this club")
		return
	}
	var exists bool
	if err := s.db.QueryRow(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM pc_refs
			WHERE club_id = $1 AND external_pc_id = $2 AND status_cache <> 'deleted'
		)
	`, req.ClubID, req.ExternalPCID).Scan(&exists); err != nil || !exists {
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			writeError(w, http.StatusNotFound, "pc not found")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := s.dispatchAgentUpdate(ctx, req.ExternalPCID, core.AgentUpdateCommand{
		Version: req.Version, DownloadURL: req.DownloadURL, ChecksumURL: req.ChecksumURL,
	}); err != nil {
		writeError(w, http.StatusConflict, "Agent update delivery failed: "+err.Error())
		return
	}
	fmt.Printf("update_event component=agent action=scheduled version=%s club_id=%s external_pc_id=%s channel=cloud_recovery\n", req.Version, req.ClubID, req.ExternalPCID)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true})
}
