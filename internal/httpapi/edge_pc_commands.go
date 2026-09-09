package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

// edgePCCommand is deliberately small: the Cloud stores a manager request,
// while only the primary Controller executes it against a LAN Agent.
type edgePCCommand struct {
	ID            string `json:"id"`
	ClubID        string `json:"club_id"`
	PCID          string `json:"pc_id"`
	ExternalPCID  string `json:"external_pc_id"`
	DesiredStatus string `json:"desired_status"`
	Reason        string `json:"reason,omitempty"`
}

type edgePCCommandCompleteRequest struct {
	ClubID  string `json:"club_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func isRemotePCStatus(status string) bool {
	switch status {
	case "available", "sleeping", "maintenance", "blocked":
		return true
	default:
		return false
	}
}

// enqueuePrimaryPCCommand sends a Manager action to Cloud. The Manager has an
// authenticated local user session, but it never owns Agent connections.
func (s *Server) enqueuePrimaryPCCommand(ctx context.Context, command edgePCCommand) (string, error) {
	if !s.managerNodeMode() || strings.TrimSpace(s.cfg.CloudBaseURL) == "" {
		return "", errors.New("primary Controller connection is unavailable")
	}
	var result struct {
		CommandID string `json:"command_id"`
	}
	if err := s.postCloudJSON(ctx, "/api/edge/pc-commands", command, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.CommandID) == "" {
		return "", errors.New("primary Controller did not accept the command")
	}
	return result.CommandID, nil
}

func (s *Server) handleEdgePCCommandEnqueue(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(s.cfg.NodeMode, "cloud") {
		writeError(w, http.StatusNotFound, "edge command queue is available only in Cloud")
		return
	}
	authorizedClubID, ok := s.requireEdge(w, r)
	if !ok {
		return
	}
	var command edgePCCommand
	if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	command.ClubID = strings.TrimSpace(command.ClubID)
	command.PCID = strings.TrimSpace(command.PCID)
	command.ExternalPCID = strings.TrimSpace(command.ExternalPCID)
	command.DesiredStatus = strings.TrimSpace(command.DesiredStatus)
	if command.ClubID == "" || command.PCID == "" || !isRemotePCStatus(command.DesiredStatus) {
		writeError(w, http.StatusBadRequest, "club_id, pc_id and a supported desired_status are required")
		return
	}
	if authorizedClubID != "" && authorizedClubID != command.ClubID {
		writeError(w, http.StatusForbidden, "controller is not authorized for this club")
		return
	}
	var externalPCID string
	err := s.db.QueryRow(r.Context(), `
		SELECT external_pc_id FROM pc_refs
		WHERE id = $1 AND club_id = $2 AND status_cache <> 'deleted'
	`, command.PCID, command.ClubID).Scan(&externalPCID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if command.ExternalPCID != "" && command.ExternalPCID != externalPCID {
		writeError(w, http.StatusBadRequest, "pc does not match external_pc_id")
		return
	}
	command.ExternalPCID = externalPCID
	command.Reason = strings.TrimSpace(command.Reason)

	// Return the existing pending command for a double-click rather than sending
	// two suspend commands to the same PC.
	err = s.db.QueryRow(r.Context(), `
		SELECT id::text FROM edge_pc_commands
		WHERE club_id = $1 AND pc_ref_id = $2 AND desired_status = $3 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1
	`, command.ClubID, command.PCID, command.DesiredStatus).Scan(&command.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.db.QueryRow(r.Context(), `
			INSERT INTO edge_pc_commands (club_id, pc_ref_id, external_pc_id, desired_status, reason, requested_by_node)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
			RETURNING id::text
		`, command.ClubID, command.PCID, command.ExternalPCID, command.DesiredStatus, command.Reason, r.Header.Get("X-Edge-Node-ID")).Scan(&command.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "command_id": command.ID, "status": "queued"})
}

func (s *Server) handleEdgePCCommandList(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(s.cfg.NodeMode, "cloud") {
		writeError(w, http.StatusNotFound, "edge command queue is available only in Cloud")
		return
	}
	authorizedClubID, ok := s.requireEdge(w, r)
	if !ok {
		return
	}
	clubID := strings.TrimSpace(r.URL.Query().Get("club_id"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "club_id is required")
		return
	}
	if authorizedClubID != "" && authorizedClubID != clubID {
		writeError(w, http.StatusForbidden, "controller is not authorized for this club")
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT id::text, club_id::text, pc_ref_id::text, external_pc_id, desired_status, COALESCE(reason, '')
		FROM edge_pc_commands
		WHERE club_id = $1 AND status = 'pending'
		ORDER BY created_at
		LIMIT 50
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	commands := make([]edgePCCommand, 0)
	for rows.Next() {
		var command edgePCCommand
		if err := rows.Scan(&command.ID, &command.ClubID, &command.PCID, &command.ExternalPCID, &command.DesiredStatus, &command.Reason); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": commands})
}

func (s *Server) handleEdgePCCommandComplete(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(s.cfg.NodeMode, "cloud") {
		writeError(w, http.StatusNotFound, "edge command queue is available only in Cloud")
		return
	}
	authorizedClubID, ok := s.requireEdge(w, r)
	if !ok {
		return
	}
	commandID := strings.TrimSpace(r.PathValue("command_id"))
	var req edgePCCommandCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if commandID == "" || strings.TrimSpace(req.ClubID) == "" {
		writeError(w, http.StatusBadRequest, "command_id and club_id are required")
		return
	}
	if authorizedClubID != "" && authorizedClubID != req.ClubID {
		writeError(w, http.StatusForbidden, "controller is not authorized for this club")
		return
	}
	status := "failed"
	if req.Success {
		status = "succeeded"
	}
	result, err := s.db.Exec(r.Context(), `
		UPDATE edge_pc_commands
		SET status = $1, error = NULLIF($2, ''), completed_at = now()
		WHERE id = $3 AND club_id = $4 AND status = 'pending'
	`, status, strings.TrimSpace(req.Error), commandID, req.ClubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "pending command not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) processPendingEdgePCCommands(ctx context.Context, clubID string) bool {
	if !s.edgeNodeMode() || strings.TrimSpace(clubID) == "" || strings.TrimSpace(s.cfg.CloudBaseURL) == "" {
		return false
	}
	var response struct {
		Commands []edgePCCommand `json:"commands"`
	}
	if err := s.getCloudJSON(ctx, "/api/edge/pc-commands?club_id="+url.QueryEscape(clubID), &response); err != nil {
		return false
	}
	changed := false
	for _, command := range response.Commands {
		if command.ID == "" || command.PCID == "" || command.ExternalPCID == "" || !isRemotePCStatus(command.DesiredStatus) {
			continue
		}
		err := s.applyPCStatusCommand(ctx, command.ExternalPCID, command.DesiredStatus, command.Reason)
		if err == nil {
			_, err = s.db.Exec(ctx, `UPDATE pc_refs SET status_cache = $1 WHERE id = $2 AND club_id = $3`, command.DesiredStatus, command.PCID, clubID)
		}
		complete := edgePCCommandCompleteRequest{ClubID: clubID, Success: err == nil}
		if err != nil {
			complete.Error = err.Error()
		} else {
			changed = true
		}
		// A completion failure leaves the command pending. Commands are safe to
		// retry: Agent commands and the desired status are idempotent.
		_ = s.postCloudJSON(ctx, "/api/edge/pc-commands/"+url.PathEscape(command.ID)+"/complete", complete, nil)
	}
	return changed
}

func (s *Server) applyPCStatusCommand(ctx context.Context, externalPCID, status, reason string) error {
	switch status {
	case "available":
		if err := s.core.SetRepair(ctx, externalPCID, false); err != nil {
			return err
		}
		return s.core.Unlock(ctx, externalPCID, defaultString(reason, "admin_available"))
	case "sleeping":
		return s.core.Sleep(ctx, externalPCID)
	case "maintenance":
		return s.core.SetRepair(ctx, externalPCID, true)
	case "blocked":
		return s.core.Lock(ctx, externalPCID, defaultString(reason, "admin_block"))
	default:
		return fmt.Errorf("unsupported PC status %q", status)
	}
}
