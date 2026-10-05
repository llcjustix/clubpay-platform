package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type bootLeaseRequest struct {
	ClubID            string `json:"club_id"`
	ExternalPCID      string `json:"external_pc_id"`
	CommandID         string `json:"command_id"`
	Operation         string `json:"operation"`
	LeaseID           string `json:"lease_id,omitempty"`
	Fence             int64  `json:"fence,omitempty"`
	RecoveryCommandID string `json:"recovery_command_id,omitempty"`
	TTLSeconds        int    `json:"ttl_seconds,omitempty"`
}
type bootLease struct {
	ClubID       string    `json:"club_id"`
	ExternalPCID string    `json:"external_pc_id"`
	CommandID    string    `json:"command_id"`
	LeaseID      string    `json:"lease_id"`
	Fence        int64     `json:"fence"`
	ObservedAt   time.Time `json:"observed_at"`
	ValidUntil   time.Time `json:"valid_until"`
}

func (s *Server) bootAuth(w http.ResponseWriter, r *http.Request) bool {
	secret := s.cfg.BootGuardToken
	if secret == "" {
		writeError(w, 503, "boot_guard_disabled")
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		writeError(w, 401, "unauthorized")
		return false
	}
	return true
}

// Enrolment is a separate operator action with a dedicated CPB credential;
// it never uses or exposes Cloud/payment secrets.
func (s *Server) handleBootStation(w http.ResponseWriter, r *http.Request) {
	if !s.bootAuth(w, r) {
		return
	}
	var q struct {
		ClubID        string `json:"club_id"`
		ExternalPCID  string `json:"external_pc_id"`
		ColdProvision bool   `json:"cold_provision"`
	}
	if !mobileDecode(w, r, &q) {
		return
	}
	if q.ClubID != s.cfg.BootGuardClubID || q.ExternalPCID == "" {
		writeError(w, 403, "station_scope")
		return
	}
	_, err := s.db.Exec(r.Context(), `INSERT INTO boot_guard_stations(pc_ref_id,cold_provision) SELECT id,$3 FROM pc_refs WHERE club_id=$1 AND external_pc_id=$2 AND status_cache<>'deleted' ON CONFLICT(pc_ref_id) DO NOTHING`, q.ClubID, q.ExternalPCID, q.ColdProvision)
	if err != nil {
		writeError(w, 409, "station_enrollment_failed")
		return
	}
	var exists bool
	err = s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM boot_guard_stations b JOIN pc_refs p ON p.id=b.pc_ref_id WHERE p.club_id=$1 AND p.external_pc_id=$2)`, q.ClubID, q.ExternalPCID).Scan(&exists)
	if err != nil || !exists {
		writeError(w, 404, "station_not_found")
		return
	}
	writeJSON(w, 200, map[string]bool{"registered": true})
}
func (s *Server) handleBootLease(w http.ResponseWriter, r *http.Request) {
	if !s.bootAuth(w, r) {
		return
	}
	var q bootLeaseRequest
	if !mobileDecode(w, r, &q) {
		return
	}
	if q.ClubID != s.cfg.BootGuardClubID || q.CommandID == "" || q.ExternalPCID == "" {
		writeError(w, 403, "station_scope")
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, 503, "guard_unavailable")
		return
	}
	defer tx.Rollback(ctx)
	var pc string
	var cold bool
	err = tx.QueryRow(ctx, `SELECT p.id,b.cold_provision FROM boot_guard_stations b JOIN pc_refs p ON p.id=b.pc_ref_id WHERE p.club_id=$1 AND p.external_pc_id=$2`, q.ClubID, q.ExternalPCID).Scan(&pc, &cold)
	if err != nil {
		writeError(w, 404, "station_not_registered")
		return
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cpb:' || $1,0))`, pc); err != nil {
		writeError(w, 503, "guard_unavailable")
		return
	}
	var lease bootLease
	var held bool
	lease.ClubID = q.ClubID
	lease.ExternalPCID = q.ExternalPCID
	lease.ObservedAt = time.Now().UTC()
	err = tx.QueryRow(ctx, `SELECT command_id,lease_id,fence,expires_at,held FROM boot_guard_leases WHERE pc_ref_id=$1`, pc).Scan(&lease.CommandID, &lease.LeaseID, &lease.Fence, &lease.ValidUntil, &held)
	action := r.PathValue("action")
	if action == "validate" || action == "release" {
		if err != nil || !held || lease.CommandID != q.CommandID || lease.LeaseID != q.LeaseID || lease.Fence != q.Fence {
			writeError(w, 409, "lease_fenced")
			return
		}
		if action == "validate" {
			if !lease.ObservedAt.Before(lease.ValidUntil) {
				writeError(w, 409, "lease_expired_quarantined")
				return
			}
		} else {
			// Explicit reconciliation can release expired authority. No automatic release.
			_, err = tx.Exec(ctx, `UPDATE boot_guard_leases SET held=false,released_at=now() WHERE pc_ref_id=$1`, pc)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE boot_guard_stations SET cold_provision=false WHERE pc_ref_id=$1`, pc)
			}
		}
	} else if action == "acquire" {
		if q.Operation != "bootstrap" && q.Operation != "reboot_to_image" && q.Operation != "rollback" {
			writeError(w, 400, "unsupported_operation")
			return
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 503, "guard_unavailable")
			return
		}
		if held && lease.CommandID != q.CommandID && q.Operation == "rollback" && q.RecoveryCommandID == lease.CommandID && q.LeaseID == lease.LeaseID && q.Fence == lease.Fence {
			// Explicit recovery transfers the existing quarantine under the same lock.
			// The normal live Agent/session/booking guard below is still mandatory.
			held = false
		}
		if held {
			if lease.CommandID != q.CommandID {
				writeError(w, 409, "station_fenced")
				return
			}
			if !lease.ObservedAt.Before(lease.ValidUntil) {
				writeError(w, 409, "lease_expired_quarantined")
				return
			}
		} else {
			var busy, booking bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM game_access_grants WHERE pc_ref_id=$1 AND status IN ('pending','accepted')),
    EXISTS(SELECT 1 FROM mobile_reservations WHERE pc_ref_id=$1 AND status IN ('confirmed','checked_in','started') AND starts_at+make_interval(mins=>duration_minutes+15)>now() AND starts_at<now()+interval '30 minutes')`, pc).Scan(&busy, &booking)
			if err != nil {
				writeError(w, 503, "guard_unavailable")
				return
			}
			if busy {
				writeError(w, 409, "active_session")
				return
			}
			if booking {
				writeError(w, 409, "upcoming_reservation")
				return
			}
			status, e := s.core.GetPCStatus(ctx, q.ExternalPCID)
			if e != nil {
				writeError(w, 503, "agent_guard_unavailable")
				return
			}
			if !status.AgentOnline && !(cold && q.Operation == "bootstrap") {
				writeError(w, 409, "agent_guard_unavailable")
				return
			}
			if status.CurrentSessionID != "" || status.CurrentGrantID != "" || status.RemainingSeconds > 0 || status.Status == "occupied" {
				writeError(w, 409, "active_session")
				return
			}
			// New Agents explicitly report critical state. Older Agents fail closed;
			// only the one-time, offline cold-provision case may omit it.
			if status.AgentOnline && (status.AgentCritical == nil || *status.AgentCritical || (status.Status != "available" && status.Status != "blocked" && status.Status != "frozen")) {
				writeError(w, 409, "agent_critical_or_unknown")
				return
			}
			if q.TTLSeconds < 10 || q.TTLSeconds > 600 {
				writeError(w, 400, "invalid_ttl")
				return
			}
			lease.CommandID = q.CommandID
			lease.LeaseID = randomHex(32)
			err = tx.QueryRow(ctx, `INSERT INTO boot_guard_leases(pc_ref_id,command_id,lease_id,fence,expires_at,held)
    VALUES($1,$2,$3,nextval('boot_guard_fence'),now()+make_interval(secs=>$4),true)
    ON CONFLICT(pc_ref_id) DO UPDATE SET command_id=EXCLUDED.command_id,lease_id=EXCLUDED.lease_id,fence=EXCLUDED.fence,expires_at=EXCLUDED.expires_at,held=true,released_at=NULL
    RETURNING fence,expires_at`, pc, q.CommandID, lease.LeaseID, q.TTLSeconds).Scan(&lease.Fence, &lease.ValidUntil)
		}
	} else {
		writeError(w, 404, "not_found")
		return
	}
	if err != nil {
		writeError(w, 503, "guard_unavailable")
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(action,entity_type,entity_id,metadata) VALUES($1,'cpb_station',$2,jsonb_build_object('command_id',$3::text,'fence',$4::bigint))`, "cpb_lease_"+action, pc, q.CommandID, lease.Fence); err != nil {
		writeError(w, 503, "guard_audit_unavailable")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		writeError(w, 503, "guard_unavailable")
		return
	}
	writeJSON(w, 200, lease)
}

// Called at the sole WebSocket mutation boundary, holding the same transaction
// lock as lease acquisition and the SQL grant/booking triggers through the ACK.
func (s *Server) bootCommandGate(ctx context.Context, external, name string) (func(), error) {
	if name == "get_status" || s.cfg.BootGuardToken == "" {
		return func() {}, nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	finish := func() { _ = tx.Rollback(context.Background()) }
	var pc string
	err = tx.QueryRow(ctx, `SELECT p.id FROM boot_guard_stations b JOIN pc_refs p ON p.id=b.pc_ref_id WHERE p.club_id=$1 AND p.external_pc_id=$2`, s.cfg.BootGuardClubID, external).Scan(&pc)
	if errors.Is(err, pgx.ErrNoRows) {
		finish()
		return func() {}, nil
	}
	if err != nil {
		finish()
		return nil, err
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cpb:' || $1,0))`, pc)
	if err != nil {
		finish()
		return nil, err
	}
	var held bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM boot_guard_leases WHERE pc_ref_id=$1 AND held)`, pc).Scan(&held); err != nil {
		finish()
		return nil, err
	}
	if held {
		finish()
		return nil, fmt.Errorf("cpb_station_fenced")
	}
	return finish, nil
}

func (s *Server) handleBootGuard(w http.ResponseWriter, r *http.Request) {
	if !s.bootAuth(w, r) {
		return
	}
	var q bootLeaseRequest
	if !mobileDecode(w, r, &q) {
		return
	}
	if q.ClubID != s.cfg.BootGuardClubID {
		writeError(w, 403, "station_scope")
		return
	}
	var cold, busy, booking, held bool
	err := s.db.QueryRow(r.Context(), `SELECT b.cold_provision,
 EXISTS(SELECT 1 FROM game_access_grants WHERE pc_ref_id=p.id AND status IN ('pending','accepted')),
 EXISTS(SELECT 1 FROM mobile_reservations WHERE pc_ref_id=p.id AND status IN ('confirmed','checked_in','started') AND starts_at+make_interval(mins=>duration_minutes+15)>now() AND starts_at<now()+interval '30 minutes'),
 EXISTS(SELECT 1 FROM boot_guard_leases WHERE pc_ref_id=p.id AND held)
 FROM boot_guard_stations b JOIN pc_refs p ON p.id=b.pc_ref_id WHERE p.club_id=$1 AND p.external_pc_id=$2`, q.ClubID, q.ExternalPCID).Scan(&cold, &busy, &booking, &held)
	if err != nil {
		writeError(w, 503, "guard_unavailable")
		return
	}
	// A held fence is expected while the Node is executing and reporting health.
	// The lease endpoint, rather than this read-only snapshot, grants authority.
	status, err := s.core.GetPCStatus(r.Context(), q.ExternalPCID)
	if err != nil || (!status.AgentOnline && !cold && !held) {
		writeError(w, 503, "agent_guard_unavailable")
		return
	}
	critical := status.AgentOnline && (status.AgentCritical == nil || *status.AgentCritical || (status.Status != "available" && status.Status != "blocked" && status.Status != "frozen"))
	busy = busy || status.CurrentSessionID != "" || status.CurrentGrantID != "" || status.RemainingSeconds > 0 || status.Status == "occupied"
	var next *time.Time
	if booking {
		n := time.Now().UTC()
		next = &n
	}
	writeJSON(w, 200, map[string]any{"club_id": q.ClubID, "external_pc_id": q.ExternalPCID, "agent_online": status.AgentOnline, "active_session": busy, "next_reservation_at": next, "agent_critical": critical, "observed_at": time.Now().UTC()})
}
