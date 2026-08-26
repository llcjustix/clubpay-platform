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

const playerAuthTTL = 10 * time.Minute

type playerIdentity struct {
	ID        string
	Phone     string
	FirstName string
}

func (s *Server) handleStartPlayerAuth(w http.ResponseWriter, r *http.Request) {
	var req startPlayerAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.QRToken) == "" {
		writeError(w, http.StatusBadRequest, "qr_token is required")
		return
	}
	if strings.TrimSpace(s.cfg.TelegramBotToken) == "" {
		writeError(w, http.StatusServiceUnavailable, "Telegram login is not configured")
		return
	}

	var clubID, pcID string
	err := s.db.QueryRow(r.Context(), `
		SELECT q.club_id, q.pc_ref_id
		FROM qr_codes q
		JOIN clubs c ON c.id = q.club_id
		JOIN pc_refs p ON p.id = q.pc_ref_id
		WHERE q.public_token = $1 AND q.status = 'active'
		  AND q.type IN ('static_pc', 'session_extend')
		  AND c.status = 'active' AND p.status_cache <> 'deleted'
	`, req.QRToken).Scan(&clubID, &pcID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "QR token not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	username, err := s.telegramBotUsername(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Telegram login is unavailable")
		return
	}

	token := "auth_" + randomHex(18)
	expiresAt := time.Now().UTC().Add(playerAuthTTL)
	returnURL := fmt.Sprintf("%s/qr/%s?player_auth_token=%s",
		strings.TrimRight(s.cfg.FrontendBaseURL, "/"),
		url.PathEscape(req.QRToken),
		url.QueryEscape(token),
	)
	_, err = s.db.Exec(r.Context(), `
		INSERT INTO player_auth_challenges (token_hash, club_id, pc_ref_id, return_url, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, hashToken(token), clubID, pcID, returnURL, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"auth_token":    token,
		"telegram_link": fmt.Sprintf("https://t.me/%s?start=%s", username, url.QueryEscape(token)),
		"expires_at":    expiresAt,
		"status":        "active",
	})
}

func (s *Server) handlePlayerAuthStatus(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "auth token is required")
		return
	}
	var status string
	var expiresAt time.Time
	var playerID *string
	var clubID string
	err := s.db.QueryRow(r.Context(), `
		SELECT status, expires_at, player_id, club_id
		FROM player_auth_challenges
		WHERE token_hash = $1
	`, hashToken(token)).Scan(&status, &expiresAt, &playerID, &clubID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "authorization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if expiresAt.Before(time.Now()) {
		status = "expired"
		_, _ = s.db.Exec(r.Context(), `UPDATE player_auth_challenges SET status = 'expired', updated_at = now() WHERE token_hash = $1 AND status <> 'expired'`, hashToken(token))
	}
	result := map[string]any{"status": status, "expires_at": expiresAt}
	if status == "verified" && playerID != nil {
		player, err := s.playerForAuthToken(r.Context(), s.db, token)
		if err == nil {
			balance, balanceErr := s.playerBalanceSeconds(r.Context(), s.db, player.ID, clubID)
			if balanceErr == nil {
				result["player"] = map[string]any{"id": player.ID, "phone": player.Phone, "first_name": player.FirstName, "balance_seconds": balance}
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) playerForAuthToken(ctx context.Context, q queryRower, token string) (playerIdentity, error) {
	var player playerIdentity
	err := q.QueryRow(ctx, `
		SELECT p.id, p.phone, COALESCE(p.first_name, '')
		FROM player_auth_challenges c
		JOIN players p ON p.id = c.player_id
		WHERE c.token_hash = $1 AND c.status = 'verified' AND c.expires_at > now() AND p.status = 'active'
	`, hashToken(token)).Scan(&player.ID, &player.Phone, &player.FirstName)
	return player, err
}

func (s *Server) verifiedPlayerForAuthToken(ctx context.Context, q queryRower, token, clubID string) (playerIdentity, error) {
	var player playerIdentity
	err := q.QueryRow(ctx, `
		SELECT p.id, p.phone, COALESCE(p.first_name, '')
		FROM player_auth_challenges c
		JOIN players p ON p.id = c.player_id
		WHERE c.token_hash = $1 AND c.club_id = $2
		  AND c.status = 'verified' AND c.expires_at > now() AND p.status = 'active'
	`, hashToken(token), clubID).Scan(&player.ID, &player.Phone, &player.FirstName)
	return player, err
}

func (s *Server) playerBalanceSeconds(ctx context.Context, q queryRower, playerID, clubID string) (int, error) {
	if clubID == "" {
		var seconds int
		err := q.QueryRow(ctx, `SELECT COALESCE(SUM(seconds_balance), 0) FROM player_club_balances WHERE player_id = $1`, playerID).Scan(&seconds)
		return seconds, err
	}
	var seconds int
	err := q.QueryRow(ctx, `SELECT COALESCE(seconds_balance, 0) FROM player_club_balances WHERE player_id = $1 AND club_id = $2`, playerID, clubID).Scan(&seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return seconds, err
}

// recordPlayerTime writes an immutable ledger record before changing the fast balance projection.
// A duplicate idempotency key is a completed no-op, which makes retrying session-end events safe.
func (s *Server) recordPlayerTime(ctx context.Context, tx pgx.Tx, playerID, clubID string, secondsDelta int, kind, grantID, paymentOrderID, idempotencyKey string) error {
	if secondsDelta == 0 {
		return nil
	}
	var ledgerID string
	err := tx.QueryRow(ctx, `
		INSERT INTO player_time_ledger (player_id, club_id, seconds_delta, kind, game_access_grant_id, payment_order_id, idempotency_key)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`, playerID, clubID, secondsDelta, kind, grantID, paymentOrderID, idempotencyKey).Scan(&ledgerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if secondsDelta > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO player_club_balances (player_id, club_id, seconds_balance)
			VALUES ($1, $2, $3)
			ON CONFLICT (player_id, club_id) DO UPDATE
			SET seconds_balance = player_club_balances.seconds_balance + EXCLUDED.seconds_balance, updated_at = now()
		`, playerID, clubID, secondsDelta)
		return err
	}
	var remaining int
	err = tx.QueryRow(ctx, `
		UPDATE player_club_balances
		SET seconds_balance = seconds_balance + $3, updated_at = now()
		WHERE player_id = $1 AND club_id = $2 AND seconds_balance >= $4
		RETURNING seconds_balance
	`, playerID, clubID, secondsDelta, -secondsDelta).Scan(&remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("insufficient player balance")
	}
	return err
}

func (s *Server) claimTelegramPlayerAuthChallenge(ctx context.Context, token, chatID string) (knownPlayer bool, returnURL string, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback(ctx)
	var challengeID, storedReturnURL string
	err = tx.QueryRow(ctx, `
		UPDATE player_auth_challenges
		SET chat_id = $2, status = 'awaiting_contact', started_at = now(), updated_at = now()
		WHERE token_hash = $1 AND status = 'active' AND expires_at > now()
		RETURNING id, COALESCE(return_url, '')
	`, hashToken(token), chatID).Scan(&challengeID, &storedReturnURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", fmt.Errorf("authorization link not found or expired")
	}
	if err != nil {
		return false, "", err
	}
	var playerID string
	err = tx.QueryRow(ctx, `SELECT id FROM players WHERE telegram_chat_id = $1 AND status = 'active'`, chatID).Scan(&playerID)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE player_auth_challenges SET player_id = $2, status = 'verified', verified_at = now(), updated_at = now() WHERE id = $1`, challengeID, playerID)
		if err != nil {
			return false, "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, "", err
		}
		return true, storedReturnURL, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, "", err
	}
	return false, storedReturnURL, nil
}

func (s *Server) completeTelegramPlayerAuth(ctx context.Context, chatID, phone, username, firstName string) (completed bool, returnURL string, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback(ctx)
	var challengeID, storedReturnURL string
	err = tx.QueryRow(ctx, `
		SELECT id, COALESCE(return_url, '') FROM player_auth_challenges
		WHERE chat_id = $1 AND status = 'awaiting_contact' AND expires_at > now()
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE
	`, chatID).Scan(&challengeID, &storedReturnURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	var playerID string
	err = tx.QueryRow(ctx, `
		INSERT INTO players (phone, telegram_chat_id, telegram_username, first_name, phone_verified_at, telegram_consent_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), now(), now())
		ON CONFLICT (phone) DO UPDATE SET
		  telegram_chat_id = EXCLUDED.telegram_chat_id,
		  telegram_username = EXCLUDED.telegram_username,
		  first_name = COALESCE(EXCLUDED.first_name, players.first_name),
		  status = 'active', phone_verified_at = now(), telegram_consent_at = now(), updated_at = now()
		RETURNING id
	`, phone, chatID, username, firstName).Scan(&playerID)
	if err != nil {
		return false, "", err
	}
	_, err = tx.Exec(ctx, `
		UPDATE player_auth_challenges
		SET player_id = $2, status = 'verified', verified_at = now(), updated_at = now()
		WHERE id = $1
	`, challengeID, playerID)
	if err != nil {
		return false, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, "", err
	}
	return true, storedReturnURL, nil
}

func (s *Server) handleRedeemPlayerBalance(w http.ResponseWriter, r *http.Request) {
	var req redeemPlayerBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.QRToken) == "" || strings.TrimSpace(req.PlayerAuthToken) == "" {
		writeError(w, http.StatusBadRequest, "qr_token and player_auth_token are required")
		return
	}
	result, err := s.redeemPlayerBalanceToPC(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) redeemPlayerBalanceToPC(ctx context.Context, req redeemPlayerBalanceRequest) (map[string]any, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var pcID, clubID, externalPCID, pcStatus, qrType string
	err = tx.QueryRow(ctx, `
		SELECT p.id, p.club_id, p.external_pc_id, q.type,
		       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END
		FROM qr_codes q JOIN pc_refs p ON p.id = q.pc_ref_id JOIN zones z ON z.id = p.zone_id
		WHERE q.public_token = $1 AND q.status = 'active' AND z.status <> 'deleted' AND p.status_cache <> 'deleted'
	`, req.QRToken).Scan(&pcID, &clubID, &externalPCID, &qrType, &pcStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("QR token not found")
	}
	if err != nil {
		return nil, err
	}
	player, err := s.verifiedPlayerForAuthToken(ctx, tx, req.PlayerAuthToken, clubID)
	if err != nil {
		return nil, fmt.Errorf("Telegram authorization is no longer valid. Sign in again")
	}
	if !canUseQRForSession(pcStatus, qrType) {
		if isSessionExtensionState(pcStatus) && qrType != "session_extend" {
			return nil, fmt.Errorf("PC is occupied. Use session QR to extend")
		}
		return nil, fmt.Errorf("PC is not available")
	}
	var seconds int
	err = tx.QueryRow(ctx, `SELECT seconds_balance FROM player_club_balances WHERE player_id = $1 AND club_id = $2 FOR UPDATE`, player.ID, clubID).Scan(&seconds)
	if errors.Is(err, pgx.ErrNoRows) || seconds <= 0 {
		return nil, fmt.Errorf("there is no available time balance")
	}
	if err != nil {
		return nil, err
	}
	minutes := secondsToMinutesCeil(seconds)
	var parent activeGrantRow
	extending := isSessionExtensionState(pcStatus)
	if extending {
		var ok bool
		parent, ok, err = activeGrantForPC(ctx, tx, pcID)
		if err != nil || !ok || parent.CoreSessionID == "" {
			return nil, fmt.Errorf("active session for extension not found")
		}
		var ownerID *string
		if err := tx.QueryRow(ctx, `SELECT player_id FROM game_access_grants WHERE id = $1 FOR UPDATE`, parent.ID).Scan(&ownerID); err != nil {
			return nil, err
		}
		if ownerID != nil && *ownerID != player.ID {
			return nil, fmt.Errorf("the active session belongs to another player")
		}
		if ownerID == nil {
			_, err = tx.Exec(ctx, `UPDATE game_access_grants SET player_id = $1 WHERE id = $2`, player.ID, parent.ID)
			if err != nil {
				return nil, err
			}
		}
	}
	var grantID string
	if extending {
		err = tx.QueryRow(ctx, `
			INSERT INTO game_access_grants (club_id, pc_ref_id, player_id, parent_grant_id, duration_minutes, duration_seconds, status, core_session_id, source)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, 'player_balance') RETURNING id
		`, clubID, pcID, player.ID, parent.ID, minutes, seconds, parent.CoreSessionID).Scan(&grantID)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO game_access_grants (club_id, pc_ref_id, player_id, duration_minutes, duration_seconds, status, source)
			VALUES ($1, $2, $3, $4, $5, 'pending', 'player_balance') RETURNING id
		`, clubID, pcID, player.ID, minutes, seconds).Scan(&grantID)
	}
	if err != nil {
		return nil, err
	}
	if err := s.recordPlayerTime(ctx, tx, player.ID, clubID, -seconds, "session_start", grantID, "", "session-start:"+grantID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if extending {
		if err := s.extendGrantSession(ctx, grantID, parent.ID, parent.CoreSessionID, clubID, pcID, externalPCID, seconds, "player_balance", "", ""); err != nil {
			s.refundPlayerBalance(ctx, player.ID, clubID, seconds, grantID, err.Error())
			return nil, err
		}
		return map[string]any{"success": true, "grant_id": grantID, "seconds_used": seconds, "minutes_used": minutes, "extended": true}, nil
	}
	extendURL, err := s.createSessionExtendURL(ctx, clubID, pcID, grantID, time.Now().UTC().Add(time.Duration(seconds)*time.Second+s.sessionGraceDuration()))
	if err != nil {
		s.refundPlayerBalance(ctx, player.ID, clubID, seconds, grantID, err.Error())
		return nil, err
	}
	start, err := s.core.StartSession(ctx, core.StartSessionCommand{RequestID: "start_" + grantID, GrantID: grantID, ClubID: clubID, PCID: pcID, PCExternalID: externalPCID, DurationSeconds: seconds, DurationMinutes: minutes, GraceSeconds: s.cfg.SessionGraceSeconds, Source: "player_balance", ExtendURL: extendURL, CreatedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		s.deactivateSessionExtendQR(ctx, grantID)
		s.refundPlayerBalance(ctx, player.ID, clubID, seconds, grantID, err.Error())
		return nil, err
	}
	coreSessionID := start.CoreSessionID
	if coreSessionID == "" {
		coreSessionID = "core-session-" + grantID
	}
	endsAt := time.Now().UTC().Add(time.Duration(seconds) * time.Second)
	if start.EndsAt != nil {
		endsAt = *start.EndsAt
	}
	_, err = s.db.Exec(ctx, `UPDATE game_access_grants SET status = 'accepted', core_session_id = $1, accepted_at = now(), planned_ends_at = $2, grace_ends_at = $3, last_error = NULL WHERE id = $4`, coreSessionID, endsAt, endsAt.Add(s.sessionGraceDuration()), grantID)
	if err != nil {
		return nil, err
	}
	s.updateSessionExtendQRExpiry(ctx, grantID, endsAt.Add(s.sessionGraceDuration()))
	_, _ = s.db.Exec(ctx, `UPDATE pc_refs SET status_cache = 'occupied' WHERE id = $1`, pcID)
	return map[string]any{"success": true, "grant_id": grantID, "core_session_id": coreSessionID, "seconds_used": seconds, "minutes_used": minutes}, nil
}

func (s *Server) refundPlayerBalance(ctx context.Context, playerID, clubID string, seconds int, grantID, cause string) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if err := s.recordPlayerTime(ctx, tx, playerID, clubID, seconds, "session_start_refund", grantID, "", "session-refund:"+grantID); err != nil {
		return
	}
	_, _ = tx.Exec(ctx, `UPDATE game_access_grants SET status = 'start_failed', last_error = $1 WHERE id = $2`, cause, grantID)
	_ = tx.Commit(ctx)
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
