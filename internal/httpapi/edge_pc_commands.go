package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"clubpay/internal/core"
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
	GrantID       string `json:"grant_id,omitempty"`
	CoreSessionID string `json:"core_session_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
	TargetNodeID  string `json:"target_node_id,omitempty"`
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

func isEdgePCCommand(status string) bool {
	return isRemotePCStatus(status) || status == "end_session"
}

// enqueuePrimaryPCCommand sends a Manager action to Cloud. The Manager has an
// authenticated local user session, but it never owns Agent connections.
func (s *Server) enqueuePrimaryPCCommand(ctx context.Context, command edgePCCommand) (string, error) {
	if s.cloudNodeMode() {
		var err error
		command.TargetNodeID, err = s.activeControllerNodeForPC(ctx, command.ClubID, command.ExternalPCID)
		if err != nil {
			return "", err
		}
		if command.TargetNodeID == "" {
			return "", errors.New("no healthy Controller is available for this PC")
		}
		// Cloud is the command queue's database owner. Sending this request back
		// through postCloudJSON made the dashboard attempt a fictitious local
		// Manager connection and reject every command before it was queued.
		err = s.db.QueryRow(ctx, `
			SELECT id::text FROM edge_pc_commands
			WHERE club_id = $1 AND pc_ref_id = $2 AND desired_status = $3
			  AND COALESCE(core_session_id, '') = $4 AND status = 'pending'
			ORDER BY created_at DESC LIMIT 1
		`, command.ClubID, command.PCID, command.DesiredStatus, command.CoreSessionID).Scan(&command.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = s.db.QueryRow(ctx, `
				INSERT INTO edge_pc_commands (club_id, pc_ref_id, external_pc_id, desired_status, grant_id, core_session_id, reason, requested_by_node, target_node_id)
				VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)
				RETURNING id::text
			`, command.ClubID, command.PCID, command.ExternalPCID, command.DesiredStatus, command.GrantID, command.CoreSessionID, command.Reason, "cloud", command.TargetNodeID).Scan(&command.ID)
		}
		if err != nil {
			return "", err
		}
		return command.ID, nil
	}
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
	if command.ClubID == "" || command.PCID == "" || !isEdgePCCommand(command.DesiredStatus) || (command.DesiredStatus == "end_session" && strings.TrimSpace(command.CoreSessionID) == "") {
		writeError(w, http.StatusBadRequest, "club_id, pc_id and a supported command are required")
		return
	}
	if authorizedClubID != "" && authorizedClubID != command.ClubID {
		writeError(w, http.StatusForbidden, "controller is not authorized for this club")
		return
	}
	_ = s.controllerNodeForEdgeRequest(r.Context(), r, command.ClubID)
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
	command.TargetNodeID, err = s.activeControllerNodeForPC(r.Context(), command.ClubID, command.ExternalPCID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if command.TargetNodeID == "" {
		writeError(w, http.StatusConflict, "no healthy Controller is available for this PC")
		return
	}

	// Return the existing pending command for a double-click rather than sending
	// two suspend commands to the same PC.
	err = s.db.QueryRow(r.Context(), `
		SELECT id::text FROM edge_pc_commands
		WHERE club_id = $1 AND pc_ref_id = $2 AND desired_status = $3
		  AND COALESCE(core_session_id, '') = $4 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1
	`, command.ClubID, command.PCID, command.DesiredStatus, command.CoreSessionID).Scan(&command.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.db.QueryRow(r.Context(), `
			INSERT INTO edge_pc_commands (club_id, pc_ref_id, external_pc_id, desired_status, grant_id, core_session_id, reason, requested_by_node, target_node_id)
			VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)
			RETURNING id::text
		`, command.ClubID, command.PCID, command.ExternalPCID, command.DesiredStatus, command.GrantID, command.CoreSessionID, command.Reason, r.Header.Get("X-Edge-Node-ID"), command.TargetNodeID).Scan(&command.ID)
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
	nodeID := s.controllerNodeForEdgeRequest(r.Context(), r, clubID)
	if nodeID == "" {
		writeError(w, http.StatusForbidden, "enrolled Controller node id is required")
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT id::text, club_id::text, pc_ref_id::text, external_pc_id, desired_status,
		       COALESCE(grant_id::text, ''), COALESCE(core_session_id, ''), COALESCE(reason, ''), COALESCE(target_node_id, '')
		FROM edge_pc_commands
		WHERE club_id = $1 AND status = 'pending' AND target_node_id = $2
		ORDER BY created_at
		LIMIT 50
	`, clubID, nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	commands := make([]edgePCCommand, 0)
	for rows.Next() {
		var command edgePCCommand
		if err := rows.Scan(&command.ID, &command.ClubID, &command.PCID, &command.ExternalPCID, &command.DesiredStatus, &command.GrantID, &command.CoreSessionID, &command.Reason, &command.TargetNodeID); err != nil {
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
	nodeID := s.controllerNodeForEdgeRequest(r.Context(), r, req.ClubID)
	if nodeID == "" {
		writeError(w, http.StatusForbidden, "enrolled Controller node id is required")
		return
	}
	status := "failed"
	if req.Success {
		status = "succeeded"
	}
	result, err := s.db.Exec(r.Context(), `
		UPDATE edge_pc_commands
		SET status = $1, error = NULLIF($2, ''), completed_at = now()
		WHERE id = $3 AND club_id = $4 AND status = 'pending' AND target_node_id = $5
	`, status, strings.TrimSpace(req.Error), commandID, req.ClubID, nodeID)
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
	// An enrolled Manager is also a primary local Controller: it owns the
	// Agent's LAN WebSocket and therefore must consume Cloud-queued commands.
	// Restricting this loop to edge nodes left Manager deployments able to queue
	// a sleep command forever, without ever delivering it to the PC.
	if (!s.edgeNodeMode() && !s.managerNodeMode()) || strings.TrimSpace(clubID) == "" || strings.TrimSpace(s.cfg.CloudBaseURL) == "" {
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
		if command.ID == "" || command.PCID == "" || command.ExternalPCID == "" || !isEdgePCCommand(command.DesiredStatus) || (command.DesiredStatus == "end_session" && (command.GrantID == "" || command.CoreSessionID == "")) {
			continue
		}
		var err error
		if command.DesiredStatus == "end_session" {
			remaining := s.remainingSecondsForAcceptedGrant(ctx, command.GrantID)
			result, endErr := s.core.EndSession(ctx, command.CoreSessionID, core.EndSessionCommand{
				RequestID: "mobile_end_" + command.GrantID + "_" + randomHex(4), ExternalPCID: command.ExternalPCID,
				Reason: "CLIENT_LEFT", EndedBy: map[string]string{"type": "player"}, CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
			if endErr != nil {
				err = endErr
			} else {
				remaining = boundedSessionRemainder(remaining, result.RemainingSeconds)
				_, err = s.finishGrant(ctx, command.GrantID, "player_left_from_mobile", remaining)
			}
		} else {
			err = s.applyPCStatusCommand(ctx, command.ExternalPCID, command.DesiredStatus, command.Reason)
		}
		// A command ACK only proves that Agent Core received the request. Its
		// pc_status_changed/heartbeat event is authoritative: Windows may refuse
		// sleep (especially in a VM), or wake immediately after accepting it.
		// Updating the cache here made the Manager briefly show "sleeping", then
		// revert to "available" despite the PC never having slept.
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
