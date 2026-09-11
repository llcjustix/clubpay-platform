package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type mobileWebhookKey struct{}

const mobileChallengeTTL = 5 * time.Minute
const mobileAccessTTL = 15 * time.Minute
const mobileSessionTTL = 30 * 24 * time.Hour

var mobilePhonePattern = regexp.MustCompile(`^\+998[0-9]{9}$`)
var mobileDevicePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)

func mobileSecret(prefix string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("secure random unavailable")
	}
	return prefix + hex.EncodeToString(b)
}

// The bot secret is an application-side pepper, combined with the challenge
// hash by callers. A database dump alone cannot brute force six-digit codes.
func mobileOTPHash(challenge, otp string) string {
	m := hmac.New(sha256.New, []byte(challenge))
	_, _ = m.Write([]byte(otp))
	return hex.EncodeToString(m.Sum(nil))
}
func mobileBearer(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer mob_a_") {
		return ""
	}
	return strings.TrimPrefix(value, "Bearer ")
}
func (s *Server) mobilePlayer(ctx context.Context, q queryRower, token string) (playerIdentity, error) {
	var p playerIdentity
	err := q.QueryRow(ctx, `SELECT p.id,p.phone,COALESCE(p.first_name,'') FROM mobile_tokens t
 JOIN mobile_sessions s ON s.id=t.session_id JOIN players p ON p.id=s.player_id
 WHERE t.token_hash=$1 AND t.kind='access' AND t.used_at IS NULL AND t.expires_at>now()
 AND s.revoked_at IS NULL AND s.expires_at>now() AND p.status='active'`, hashToken(token)).Scan(&p.ID, &p.Phone, &p.FirstName)
	return p, err
}
func (s *Server) requireMobile(w http.ResponseWriter, r *http.Request) (playerIdentity, bool) {
	w.Header().Set("Cache-Control", "no-store")
	token := mobileBearer(r)
	if token == "" {
		writeError(w, 401, "mobile_auth_required")
		return playerIdentity{}, false
	}
	p, err := s.mobilePlayer(r.Context(), s.db, token)
	if err != nil {
		writeError(w, 401, "mobile_auth_required")
		return p, false
	}
	return p, true
}
func mobileDecode(w http.ResponseWriter, r *http.Request, value any) bool {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		writeError(w, 400, "invalid_request")
		return false
	}
	return true
}
func mobileInternal(w http.ResponseWriter) { writeError(w, 500, "mobile_service_unavailable") }
func mobileAudit(ctx context.Context, tx pgx.Tx, action, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs(action,entity_type,entity_id,metadata) VALUES ($1,'mobile_session',$2,'{}')`, action, id)
	return err
}
func (s *Server) mobileRate(ctx context.Context, bucket string, limit int, ttl time.Duration) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `INSERT INTO mobile_rate_limits(bucket,hits,expires_at) VALUES($1,1,now()+$2*interval '1 second')
 ON CONFLICT(bucket) DO UPDATE SET hits=CASE WHEN mobile_rate_limits.expires_at<=now() THEN 1 ELSE mobile_rate_limits.hits+1 END,
 expires_at=CASE WHEN mobile_rate_limits.expires_at<=now() THEN EXCLUDED.expires_at ELSE mobile_rate_limits.expires_at END RETURNING hits`, hashToken(bucket), int(ttl.Seconds())).Scan(&count)
	return count <= limit, err
}
func (s *Server) handleMobileChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone  string `json:"phone"`
		Device string `json:"device_id"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	if !mobilePhonePattern.MatchString(req.Phone) || !mobileDevicePattern.MatchString(req.Device) {
		writeError(w, 400, "invalid_phone_or_device")
		return
	}
	if s.cfg.TelegramBotToken == "" {
		writeError(w, 503, "telegram_unavailable")
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	for _, b := range []struct {
		key   string
		limit int
	}{{"phone:" + req.Phone, 5}, {"device:" + req.Device, 8}, {"ip:" + host, 30}} {
		ok, err := s.mobileRate(r.Context(), b.key, b.limit, 15*time.Minute)
		if err != nil {
			mobileInternal(w)
			return
		}
		if !ok {
			w.Header().Set("Retry-After", "900")
			writeError(w, 429, "rate_limited")
			return
		}
	}
	username, err := s.telegramBotUsername(r.Context())
	if err != nil {
		writeError(w, 503, "telegram_unavailable")
		return
	}
	// Telegram start parameters are capped at 64 characters.
	token := mobileSecret("m_")[:62]
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	expires := time.Now().UTC().Add(mobileChallengeTTL)
	err = tx.QueryRow(r.Context(), `INSERT INTO mobile_auth_challenges(token_hash,phone,device_hash,expires_at) VALUES($1,$2,$3,$4) RETURNING id`, hashToken(token), req.Phone, hashToken(req.Device), expires).Scan(&id)
	if err == nil {
		err = mobileAudit(r.Context(), tx, "mobile.challenge_created", id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	// A Telegram deep link only carries its `start` payload while Telegram
	// decides to show its own Start action. Once a player has already opened
	// the bot, opening the same link merely focuses the existing chat and the
	// payload is lost. A verified bot binding is enough to deliver the OTP
	// safely, so do that immediately instead of depending on client-specific
	// deep-link behaviour.
	delivery := "sent"
	if err := s.deliverMobileOTPToBoundTelegram(r.Context(), token, req.Phone); err != nil {
		// Keep the signed Telegram link usable if Telegram temporarily rejects a
		// proactive message (for example, after a player changes their Telegram
		// account). The bot can finish the same pending challenge after /start
		// and a contact confirmation, so a transient send error must not block
		// the player from logging in.
		delivery = "open_link"
	}
	writeJSON(w, 201, map[string]any{"challenge": token, "expires_at": expires, "telegram_link": "https://t.me/" + username + "?start=" + token, "telegram_delivery": delivery})
}

// deliverMobileOTPToBoundTelegram completes a challenge only when this exact
// phone has previously been verified by the same Telegram account. New players
// still use the signed deep link and share-contact flow below.
func (s *Server) deliverMobileOTPToBoundTelegram(ctx context.Context, token, phone string) error {
	var chatIDText, username, firstName string
	err := s.db.QueryRow(ctx, `SELECT chat_id,COALESCE(username,''),COALESCE(first_name,'')
		FROM telegram_users WHERE phone=$1 AND status='active'`, phone).Scan(&chatIDText, &username, &firstName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	chatID, err := strconv.ParseInt(chatIDText, 10, 64)
	if err != nil || chatID <= 0 {
		return nil
	}
	_, err = s.processMobileTelegram(ctx, telegramMessage{
		Chat: telegramChat{ID: chatID},
		From: telegramUser{ID: chatID, Username: username, FirstName: firstName},
	}, token)
	return err
}

// Runs only on updates received from authenticated webhook or Telegram polling.
// Never accepts a forwarded contact, group contact, text phone or mismatched user.
func (s *Server) processMobileTelegram(ctx context.Context, message telegramMessage, payload string) (bool, error) {
	if authenticated, webhook := ctx.Value(mobileWebhookKey{}).(bool); webhook && !authenticated {
		return strings.HasPrefix(payload, "m_"), nil
	}
	chat := fmt.Sprint(message.Chat.ID)
	if strings.HasPrefix(payload, "m_") {
		if message.Chat.ID <= 0 || message.Chat.ID != message.From.ID {
			return true, nil
		}
		tag, err := s.db.Exec(ctx, `UPDATE mobile_auth_challenges SET chat_id=$2,status='contact'
   WHERE token_hash=$1 AND status='pending' AND expires_at>now()`, hashToken(payload), chat)
		if err != nil {
			return true, err
		}
		if tag.RowsAffected() == 0 {
			return true, s.sendTelegramMessage(ctx, chat, "Ссылка устарела. Создайте новый запрос в ClubPay. / Havola eskirgan. ClubPay’da qayta urinib ko‘ring.")
		}
		// A user who already confirmed this exact phone with the same bot does
		// not need to share the contact on every device sign-in. Treat that
		// existing bot binding as the contact confirmation and issue the OTP
		// immediately. This also keeps legacy voucher handling out of the
		// mobile authorization conversation.
		var boundPhone string
		err = s.db.QueryRow(ctx, `SELECT u.phone
			FROM telegram_users u
			JOIN mobile_auth_challenges c ON c.phone = u.phone
			WHERE c.token_hash = $1 AND u.chat_id = $2 AND u.status = 'active'`, hashToken(payload), chat).Scan(&boundPhone)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return true, err
		}
		if err == nil {
			message.Contact = telegramContact{
				PhoneNumber: boundPhone,
				UserID:      message.From.ID,
			}
			return s.processMobileTelegram(ctx, message, "")
		}
		return true, s.sendTelegramMessageWithMarkup(ctx, chat, "Подтвердите номер из приложения своим контактом. / Ilovadagi raqamni o‘z kontaktingiz bilan tasdiqlang.", telegramContactKeyboard())
	}
	if message.Contact.PhoneNumber == "" {
		return s.claimPendingMobileChallengeForBoundChat(ctx, message)
	}
	if message.Chat.ID <= 0 || message.From.ID != message.Chat.ID || message.Contact.UserID != message.From.ID {
		return true, nil
	}
	phone := "+" + strings.TrimPrefix(normalizePhone(message.Contact.PhoneNumber), "+")
	if phone == "+" {
		return true, nil
	}
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mobile_auth_challenges WHERE chat_id=$1 AND status='contact' AND expires_at>now())`, chat).Scan(&exists)
	if err != nil {
		return false, err
	}
	if !exists {
		tag, err := s.db.Exec(ctx, `UPDATE mobile_auth_challenges SET chat_id=$1,status='contact'
			WHERE id=(SELECT id FROM mobile_auth_challenges WHERE phone=$2 AND status='pending' AND expires_at>now() ORDER BY created_at DESC LIMIT 1)
			  AND status='pending'`, chat, phone)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() == 0 {
			return false, nil
		}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return true, err
	}
	defer tx.Rollback(ctx)
	var id, expected, challengeHash string
	err = tx.QueryRow(ctx, `SELECT id,phone,token_hash FROM mobile_auth_challenges WHERE chat_id=$1 AND status='contact' AND expires_at>now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, chat).Scan(&id, &expected, &challengeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	if phone != expected {
		return true, s.sendTelegramMessage(ctx, chat, "Номер не совпадает. Проверьте номер в приложении. / Raqam mos kelmadi. Ilovadagi raqamni tekshiring.")
	}
	codeNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return true, err
	}
	code := fmt.Sprintf("%06d", codeNumber.Int64())
	var playerID string
	// Preserve bans and reject attempts to move an established phone to another Telegram identity.
	err = tx.QueryRow(ctx, `INSERT INTO players(phone,telegram_chat_id,telegram_username,first_name) VALUES($1,$2,$3,$4)
 ON CONFLICT(phone) DO UPDATE SET first_name=EXCLUDED.first_name,telegram_chat_id=EXCLUDED.telegram_chat_id,updated_at=now()
 WHERE players.status='active' AND (players.telegram_chat_id IS NULL OR players.telegram_chat_id=EXCLUDED.telegram_chat_id)
 RETURNING id`, phone, chat, message.From.Username, message.From.FirstName).Scan(&playerID)
	if err != nil {
		return true, fmt.Errorf("mobile contact could not be verified")
	}
	// Bot secret acts as a pepper, separate from the database. Never log it or OTP.
	err = func() error {
		_, e := tx.Exec(ctx, `UPDATE mobile_auth_challenges SET player_id=$2,otp_hash=$3,otp_expires_at=LEAST(expires_at,now()+interval '3 minutes'),status='otp' WHERE id=$1`, id, playerID, mobileOTPHash(s.cfg.TelegramBotToken+challengeHash, code))
		return e
	}()
	if err == nil {
		err = mobileAudit(ctx, tx, "mobile.contact_verified", id)
	}
	if err != nil {
		return true, err
	}
	if err = s.sendTelegramMessage(ctx, chat, "Код входа ClubPay / ClubPay kirish kodi: "+code+"\nНикому не сообщайте код. / Kodni hech kimga bermang."); err != nil {
		return true, fmt.Errorf("mobile code delivery unavailable")
	}
	return true, tx.Commit(ctx)
}

// claimPendingMobileChallengeForBoundChat is the fallback for Telegram
// clients that open an existing bot chat but silently drop the deep-link start
// payload. The stored phone-to-chat binding supplies the same proof a shared
// contact would, so the pending challenge can safely continue.
func (s *Server) claimPendingMobileChallengeForBoundChat(ctx context.Context, message telegramMessage) (bool, error) {
	if message.Chat.ID <= 0 || message.From.ID != message.Chat.ID {
		return false, nil
	}
	chat := fmt.Sprint(message.Chat.ID)
	var phone string
	err := s.db.QueryRow(ctx, `UPDATE mobile_auth_challenges c SET chat_id=$1,status='contact'
		WHERE c.id=(SELECT c2.id FROM mobile_auth_challenges c2
			JOIN telegram_users u ON u.phone=c2.phone
			WHERE u.chat_id=$1 AND u.status='active' AND c2.status='pending' AND c2.expires_at>now()
			ORDER BY c2.created_at DESC LIMIT 1)
		  AND c.status='pending'
		RETURNING c.phone`, chat).Scan(&phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	message.Contact = telegramContact{PhoneNumber: phone, UserID: message.From.ID}
	return s.processMobileTelegram(ctx, message, "")
}
func mobileIssue(ctx context.Context, tx pgx.Tx, session string, sessionExpiry time.Time) (map[string]any, error) {
	access, refresh := mobileSecret("mob_a_"), mobileSecret("mob_r_")
	expiry := time.Now().UTC().Add(mobileAccessTTL)
	if sessionExpiry.Before(expiry) {
		expiry = sessionExpiry
	}
	_, err := tx.Exec(ctx, `INSERT INTO mobile_tokens(token_hash,session_id,kind,expires_at) VALUES($1,$2,'access',$3),($4,$2,'refresh',$5)`, hashToken(access), session, expiry, hashToken(refresh), sessionExpiry)
	return map[string]any{"access_token": access, "refresh_token": refresh, "expires_at": expiry, "refresh_expires_at": sessionExpiry}, err
}
func (s *Server) handleMobileVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Challenge string `json:"challenge"`
		Device    string `json:"device_id"`
		OTP       string `json:"otp"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	if len(req.Challenge) != 62 || !mobileDevicePattern.MatchString(req.Device) || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(req.OTP) {
		writeError(w, 400, "invalid_request")
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	allowed, rateErr := s.mobileRate(r.Context(), "verify-ip:"+host, 120, 15*time.Minute)
	if rateErr != nil {
		mobileInternal(w)
		return
	}
	if !allowed {
		writeError(w, 429, "rate_limited")
		return
	}
	ok, err := s.mobileRate(r.Context(), "verify:"+req.Device, 30, 15*time.Minute)
	if err != nil {
		mobileInternal(w)
		return
	}
	if !ok {
		writeError(w, 429, "rate_limited")
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(ctx)
	var id, player, otp string
	err = tx.QueryRow(ctx, `UPDATE mobile_auth_challenges SET attempts=attempts+1 WHERE token_hash=$1 AND device_hash=$2 AND status='otp' AND expires_at>now() AND otp_expires_at>now() AND attempts<5 RETURNING id,player_id,otp_hash`, hashToken(req.Challenge), hashToken(req.Device)).Scan(&id, &player, &otp)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 401, "otp_invalid_or_expired")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	if !hmac.Equal([]byte(otp), []byte(mobileOTPHash(s.cfg.TelegramBotToken+hashToken(req.Challenge), req.OTP))) {
		if mobileAudit(ctx, tx, "mobile.otp_rejected", id) != nil || tx.Commit(ctx) != nil {
			mobileInternal(w)
			return
		}
		writeError(w, 401, "otp_invalid_or_expired")
		return
	}
	var session string
	expiry := time.Now().UTC().Add(mobileSessionTTL)
	err = tx.QueryRow(ctx, `INSERT INTO mobile_sessions(player_id,device_hash,expires_at) SELECT id,$2,$3 FROM players WHERE id=$1 AND status='active' RETURNING mobile_sessions.id`, player, hashToken(req.Device), expiry).Scan(&session)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE mobile_auth_challenges SET status='used',otp_hash=NULL WHERE id=$1`, id)
	}
	var result map[string]any
	if err == nil {
		result, err = mobileIssue(ctx, tx, session, expiry)
	}
	if err == nil {
		err = mobileAudit(ctx, tx, "mobile.login", session)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) handleMobileRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Refresh string `json:"refresh_token"`
		Device  string `json:"device_id"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	if !strings.HasPrefix(req.Refresh, "mob_r_") || !mobileDevicePattern.MatchString(req.Device) {
		writeError(w, 401, "refresh_invalid")
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(ctx)
	var session string
	var expiry time.Time
	var used *time.Time
	err = tx.QueryRow(ctx, `SELECT s.id,s.expires_at,t.used_at FROM mobile_tokens t JOIN mobile_sessions s ON s.id=t.session_id JOIN players p ON p.id=s.player_id
 WHERE t.token_hash=$1 AND t.kind='refresh' AND s.device_hash=$2 AND t.expires_at>now() AND s.expires_at>now() AND s.revoked_at IS NULL AND p.status='active' FOR UPDATE OF s,t`, hashToken(req.Refresh), hashToken(req.Device)).Scan(&session, &expiry, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 401, "refresh_invalid")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	if used != nil {
		_, err = tx.Exec(ctx, `UPDATE mobile_sessions SET revoked_at=now() WHERE id=$1`, session)
		if err == nil {
			err = mobileAudit(ctx, tx, "mobile.refresh_reuse", session)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			mobileInternal(w)
			return
		}
		writeError(w, 401, "refresh_reused")
		return
	}
	_, err = tx.Exec(ctx, `UPDATE mobile_tokens SET used_at=now() WHERE session_id=$1 AND used_at IS NULL`, session)
	var result map[string]any
	if err == nil {
		result, err = mobileIssue(ctx, tx, session, expiry)
	}
	if err == nil {
		err = mobileAudit(ctx, tx, "mobile.refresh", session)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) handleMobileLogout(w http.ResponseWriter, r *http.Request) {
	// Refresh credential permits logout even after the access token expires.
	var req struct {
		Refresh string `json:"refresh_token"`
		Device  string `json:"device_id"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `UPDATE mobile_sessions s SET revoked_at=now() FROM mobile_tokens t WHERE t.session_id=s.id AND t.token_hash=$1 AND t.kind='refresh' AND s.device_hash=$2 RETURNING s.id`, hashToken(req.Refresh), hashToken(req.Device)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(204)
		return
	}
	if err == nil {
		err = mobileAudit(r.Context(), tx, "mobile.logout", id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) handleMobileMe(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"id": p.ID, "phone": "+" + strings.TrimPrefix(p.Phone, "+"), "first_name": p.FirstName})
}
func (s *Server) handleMobileBalances(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	// Releases before the Agent remainder fallback could end an early profile
	// session with zero seconds when the Agent omitted that field. Repair only
	// those provably early, non-expired sessions once; the ledger key makes a
	// refresh safe and prevents crediting the same grant twice.
	if err := s.reconcileMissingProfileRemainders(r.Context(), p.ID); err != nil {
		mobileInternal(w)
		return
	}
	if err := s.repairProfileBalanceProjection(r.Context(), p.ID); err != nil {
		mobileInternal(w)
		return
	}
	rows, err := s.queryMaps(r.Context(), `
 WITH owned AS (
  SELECT club_id FROM player_club_balances WHERE player_id=$1
  UNION SELECT club_id FROM game_access_grants WHERE player_id=$1
  UNION SELECT club_id FROM payment_orders WHERE player_id=$1
 ), balances AS (
  SELECT c.id AS club_id,c.name AS club_name,COALESCE(b.seconds_balance,0) AS seconds_balance,b.updated_at,
    c.controller_synced_at,
    CASE WHEN $2::boolean THEN COALESCE(c.controller_synced_at>now()-interval '45 seconds',false) ELSE true END AS club_online,
    COALESCE(b.time_value_units,b.seconds_balance::bigint*COALESCE(b.reference_price_tiyin,(SELECT MIN(hourly_price_tiyin) FROM zones WHERE club_id=c.id AND status<>'deleted')),0) AS units
  FROM owned o JOIN clubs c ON c.id=o.club_id LEFT JOIN player_club_balances b ON b.club_id=c.id AND b.player_id=$1
 )
 SELECT b.club_id,b.club_name,b.seconds_balance,b.updated_at,b.controller_synced_at,b.club_online,
  GREATEST(0,ROUND(b.units::numeric/360000))::bigint AS balance_uzs,
  COALESCE((SELECT jsonb_agg(jsonb_build_object('id',z.id,'name',z.name,'hourly_price_tiyin',z.hourly_price_tiyin,'seconds_available',b.units/z.hourly_price_tiyin) ORDER BY z.sort_order,z.name) FROM zones z WHERE z.club_id=b.club_id AND z.status<>'deleted' AND z.hourly_price_tiyin>0),'[]'::jsonb) AS zones
 FROM balances b ORDER BY b.club_name`, p.ID, strings.EqualFold(s.cfg.NodeMode, "cloud"))
	if err != nil {
		mobileInternal(w)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"balances": rows})
}

// handleMobileClubs is the signed-in player catalog. QR codes remain an
// internal routing detail: the client chooses a visible PC and receives its
// opaque static token only for the existing checkout flow.
func (s *Server) handleMobileClubs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireMobile(w, r); !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 80 {
		writeError(w, http.StatusBadRequest, "search query is too long")
		return
	}
	rows, err := s.queryMaps(r.Context(), `
		SELECT c.id AS club_id,c.name AS club_name,COALESCE(c.address,'') AS address,
		  COALESCE(c.controller_synced_at>now()-interval '45 seconds',false) AS club_online,
		  COUNT(*) FILTER (WHERE p.status_cache IN ('available','sleeping'))::int AS available_pcs
		FROM clubs c
		JOIN pc_refs p ON p.club_id=c.id AND p.status_cache<>'deleted'
		JOIN zones z ON z.id=p.zone_id AND z.status='active'
		WHERE c.status='active'
		  AND ($1='' OR c.name ILIKE '%' || $1 || '%' OR COALESCE(c.address,'') ILIKE '%' || $1 || '%')
		GROUP BY c.id,c.name,c.address,c.controller_synced_at
		ORDER BY (COUNT(*) FILTER (WHERE p.status_cache IN ('available','sleeping'))) DESC,c.name
		LIMIT 50
	`, query)
	if err != nil {
		mobileInternal(w)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"clubs": rows})
}

func (s *Server) handleMobileClub(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireMobile(w, r); !ok {
		return
	}
	clubID := strings.TrimSpace(r.PathValue("club_id"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "club_id is required")
		return
	}
	var club map[string]any
	rows, err := s.queryMaps(r.Context(), `
		SELECT c.id AS club_id,c.name AS club_name,COALESCE(c.address,'') AS address,
		  COALESCE(c.controller_synced_at>now()-interval '45 seconds',false) AS club_online
		FROM clubs c WHERE c.id=$1 AND c.status='active'
	`, clubID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if len(rows) == 0 {
		writeError(w, http.StatusNotFound, "club not found")
		return
	}
	club = rows[0]
	zones, err := s.queryMaps(r.Context(), `
		SELECT z.id AS zone_id,z.name AS zone_name,z.sort_order,z.hourly_price_tiyin/100 AS hourly_price_uzs,
		  COALESCE(jsonb_agg(jsonb_build_object(
			'id',p.id,'label',p.label,'number',p.number,'status',p.status_cache,
			'qr_token',COALESCE(q.public_token,'')
		  ) ORDER BY p.number,p.label) FILTER (WHERE p.id IS NOT NULL),'[]'::jsonb) AS pcs
		FROM zones z
		LEFT JOIN pc_refs p ON p.zone_id=z.id AND p.status_cache<>'deleted'
		LEFT JOIN LATERAL (
			SELECT public_token FROM qr_codes
			WHERE pc_ref_id=p.id AND type='static_pc' AND status='active'
			ORDER BY created_at DESC LIMIT 1
		) q ON true
		WHERE z.club_id=$1 AND z.status='active'
		GROUP BY z.id,z.name,z.sort_order,z.hourly_price_tiyin
		ORDER BY z.sort_order,z.name
	`, clubID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if zones == nil {
		zones = []map[string]any{}
	}
	club["zones"] = zones
	writeJSON(w, http.StatusOK, map[string]any{"club": club})
}
