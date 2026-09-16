package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const controllerAgentLeaseTTL = 75 * time.Second
const controllerNodeHealthTTL = 15 * time.Second

// controllerNodeForEdgeRequest identifies the enrolled node that made a Cloud
// edge request. The Cloud does not trust a role supplied in JSON: it is bound
// to the node id that was enrolled for this club.
func (s *Server) controllerNodeForEdgeRequest(ctx context.Context, r *http.Request, clubID string) string {
	if !s.cloudNodeMode() || strings.TrimSpace(clubID) == "" {
		return ""
	}
	nodeID := strings.TrimSpace(r.Header.Get("X-Edge-Node-ID"))
	if nodeID == "" {
		// Controller builds released before the failover protocol sent the node
		// id on Cloud GET requests. During their rolling upgrade, identify the
		// only recently healthy enrolled node if there is exactly one. The moment
		// two nodes are healthy we fail closed instead of guessing a command owner.
		rows, err := s.db.Query(ctx, `
			SELECT node_id FROM controller_nodes
			WHERE club_id = $1 AND status = 'active'
			  AND last_seen_at > now() - $2::interval
			ORDER BY last_seen_at DESC
			LIMIT 2
		`, clubID, controllerNodeHealthTTL.String())
		if err != nil {
			return ""
		}
		defer rows.Close()
		if !rows.Next() || rows.Scan(&nodeID) != nil || rows.Next() {
			return ""
		}
		return nodeID
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM controller_nodes
			WHERE club_id = $1 AND node_id = $2 AND status = 'active'
		)
	`, clubID, nodeID).Scan(&exists); err != nil || !exists {
		return ""
	}
	_, _ = s.db.Exec(ctx, `UPDATE controller_nodes SET last_seen_at = now() WHERE node_id = $1`, nodeID)
	return nodeID
}

// acceptControllerAgentEvent fences stale Controller events. agent_online is
// the Agent's takeover proof: one Agent process owns one WebSocket at a time.
// Every later heartbeat/session/state event must carry the same connection id
// and is rejected after another Controller has taken over.
func (s *Server) acceptControllerAgentEvent(ctx context.Context, clubID, nodeID string, event edgeEvent) (bool, error) {
	connectionID := strings.TrimSpace(stringFromPayload(event.Payload, "agent_connection_id"))
	if clubID == "" || nodeID == "" || event.ExternalPCID == "" || connectionID == "" {
		// Older Agents do not carry a connection id. Keep them functional during
		// rollout; command targeting becomes fenced as soon as the Agent updates.
		return true, nil
	}
	now := time.Now().UTC()
	switch normalizeCoreEventType(event.Type) {
	case "agent_online":
		var previousNode string
		_ = s.db.QueryRow(ctx, `
			SELECT node_id FROM controller_agent_leases
			WHERE club_id = $1 AND external_pc_id = $2
		`, clubID, event.ExternalPCID).Scan(&previousNode)
		_, err := s.db.Exec(ctx, `
			INSERT INTO controller_agent_leases
				(club_id, external_pc_id, node_id, agent_connection_id, acquired_at, renewed_at, lease_expires_at)
			VALUES ($1, $2, $3, $4, $5, $5, $6)
			ON CONFLICT (club_id, external_pc_id) DO UPDATE SET
				node_id = EXCLUDED.node_id,
				agent_connection_id = EXCLUDED.agent_connection_id,
				renewed_at = EXCLUDED.renewed_at,
				lease_expires_at = EXCLUDED.lease_expires_at
		`, clubID, event.ExternalPCID, nodeID, connectionID, now, now.Add(controllerAgentLeaseTTL))
		if err == nil && previousNode != "" && previousNode != nodeID {
			metadata, _ := json.Marshal(map[string]string{"from_node": previousNode, "to_node": nodeID, "external_pc_id": event.ExternalPCID})
			_, _ = s.db.Exec(ctx, `INSERT INTO audit_logs (club_id, action, entity_type, entity_id, metadata) VALUES ($1, 'controller_failover', 'pc_ref', NULL, $2)`, clubID, metadata)
		}
		return err == nil, err
	case "heartbeat", "pc_status_changed", "session_started", "session_extended", "session_ended", "command_failed":
		result, err := s.db.Exec(ctx, `
			UPDATE controller_agent_leases
			SET renewed_at = $5, lease_expires_at = $6
			WHERE club_id = $1 AND external_pc_id = $2
			  AND node_id = $3 AND agent_connection_id = $4
			  AND lease_expires_at > $5
		`, clubID, event.ExternalPCID, nodeID, connectionID, now, now.Add(controllerAgentLeaseTTL))
		if err != nil {
			return false, err
		}
		return result.RowsAffected() == 1, nil
	default:
		return true, nil
	}
}

func (s *Server) activeControllerNodeForPC(ctx context.Context, clubID, externalPCID string) (string, error) {
	var nodeID string
	err := s.db.QueryRow(ctx, `
		SELECT l.node_id
		FROM controller_agent_leases l
		JOIN controller_nodes n ON n.node_id = l.node_id AND n.club_id = l.club_id AND n.status = 'active'
		WHERE l.club_id = $1 AND l.external_pc_id = $2 AND l.lease_expires_at > now()
	`, clubID, externalPCID).Scan(&nodeID)
	if err == nil {
		return nodeID, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	// Sleeping Agents have no WebSocket lease. Use a synchronizing primary when
	// available, otherwise the recently healthy Manager can still wake it.
	err = s.db.QueryRow(ctx, `
		SELECT node_id
		FROM controller_nodes
		WHERE club_id = $1 AND status = 'active'
		  AND last_seen_at > now() - $2::interval
		ORDER BY CASE node_mode WHEN 'edge' THEN 0 ELSE 1 END, last_seen_at DESC
		LIMIT 1
	`, clubID, controllerNodeHealthTTL.String()).Scan(&nodeID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return nodeID, err
}

func (s *Server) controllerOwnsActiveLease(ctx context.Context, clubID, nodeID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM controller_agent_leases
			WHERE club_id = $1 AND node_id = $2 AND lease_expires_at > now()
		)
	`, clubID, nodeID).Scan(&exists)
	return exists, err
}
