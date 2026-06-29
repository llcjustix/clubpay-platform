package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"clubpay/internal/payments"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg  config.Config
	db   *pgxpool.Pool
	core core.Adapter
}

var paymeSandboxPageTemplate = template.Must(template.New("payme-sandbox").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Payme sandbox</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #08111d; color: #f7fbff; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    main { width: min(420px, calc(100vw - 32px)); padding: 24px; border: 1px solid #243852; border-radius: 16px; background: #101b2a; box-shadow: 0 18px 50px rgba(0,0,0,.25); }
    small { display: block; margin-bottom: 10px; color: #2fd0c4; font-weight: 900; letter-spacing: .08em; text-transform: uppercase; }
    h1 { margin: 0 0 10px; font-size: 24px; line-height: 1.15; }
    p { margin: 0 0 18px; color: #9fb1c8; line-height: 1.45; }
    dl { display: grid; grid-template-columns: 1fr auto; gap: 8px 16px; margin: 0 0 20px; color: #c8d7ea; }
    dt { color: #7f93ab; }
    dd { margin: 0; font-weight: 800; }
    button, a { display: inline-flex; align-items: center; justify-content: center; width: 100%; min-height: 48px; border-radius: 12px; font: 800 16px system-ui, sans-serif; text-decoration: none; box-sizing: border-box; }
    button { border: 0; background: #2fd0c4; color: #031116; cursor: pointer; }
    a { margin-top: 10px; border: 1px solid #31445e; color: #d9e7f8; background: #162336; }
  </style>
</head>
<body>
  <main>
    <small>Payme sandbox</small>
    <h1>{{if .Paid}}Оплата уже проведена{{else}}Тестовая оплата{{end}}</h1>
    <p>{{if .Paid}}Этот заказ уже оплачен. Можно вернуться на страницу проверки.{{else}}Это внутренняя sandbox-страница Clubpay. Она имитирует успешный Payme callback для проверки полного MVP-флоу.{{end}}</p>
    <dl>
      <dt>Заказ</dt><dd>{{.InvoiceID}}</dd>
      <dt>Сумма</dt><dd>{{.AmountUZS}} сум</dd>
    </dl>
    {{if .Paid}}
      <a href="{{.ReturnURL}}">Вернуться в Clubpay</a>
    {{else}}
      <form method="post" action="{{.PayURL}}">
        <button type="submit">Оплатить тестово</button>
      </form>
      <a href="{{.ReturnURL}}">Вернуться без оплаты</a>
    {{end}}
  </main>
</body>
</html>`))

func NewServer(cfg config.Config, db *pgxpool.Pool, coreAdapter core.Adapter) *Server {
	return &Server{cfg: cfg, db: db, core: coreAdapter}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/qr/{token}", s.handleQR)
	mux.HandleFunc("GET /api/orders/{invoice_id}", s.handleOrder)
	mux.HandleFunc("POST /api/checkouts", s.handleCreateCheckout)
	mux.HandleFunc("POST /api/payments/click/prepare", s.handleClickPrepare)
	mux.HandleFunc("POST /api/payments/click/complete", s.handleClickComplete)
	mux.HandleFunc("POST /api/payments/click/callback", s.handleClickCallback)
	mux.HandleFunc("GET /api/payments/payme/sandbox/{invoice_id}", s.handlePaymeSandboxPage)
	mux.HandleFunc("POST /api/payments/payme/sandbox/{invoice_id}/pay", s.handlePaymeSandboxPay)
	mux.HandleFunc("POST /api/payments/payme/callback", s.handlePaymeCallback)
	mux.HandleFunc("POST /api/payments/sync/{invoice_id}", s.handlePaymentSync)
	mux.HandleFunc("POST /api/payments/mock/success/{invoice_id}", s.handleMockPaymentSuccess)
	mux.HandleFunc("POST /api/core/events", s.handleCoreEvent)
	mux.HandleFunc("GET /api/edge/snapshot", s.handleEdgeSnapshot)
	mux.HandleFunc("POST /api/edge/events", s.handleEdgeEvents)
	mux.HandleFunc("GET /api/admin/catalog", s.handleAdminCatalog)
	mux.HandleFunc("GET /api/admin/pcs", s.handleAdminPCs)
	mux.HandleFunc("POST /api/admin/pcs/{pc_id}/status", s.handleAdminPCStatus)
	mux.HandleFunc("GET /api/admin/orders", s.handleAdminOrders)
	mux.HandleFunc("GET /api/admin/grants", s.handleAdminGrants)
	mux.HandleFunc("POST /api/admin/grants/{grant_id}/end", s.handleAdminEndGrant)
	mux.HandleFunc("POST /api/admin/cash-sessions", s.handleCashSession)
	mux.HandleFunc("GET /api/owner/summary", s.handleOwnerSummary)
	mux.HandleFunc("POST /api/vouchers", s.handleCreateVoucher)
	mux.HandleFunc("POST /api/vouchers/check", s.handleCheckVoucher)
	mux.HandleFunc("POST /api/vouchers/redeem", s.handleRedeemVoucher)
	mux.HandleFunc("POST /api/telegram/webhook", s.handleTelegramWebhook)
	mux.HandleFunc("GET /api/backoffice/clubs", s.handleBackofficeClubs)
	mux.HandleFunc("POST /api/backoffice/clubs", s.handleBackofficeCreateClub)
	mux.HandleFunc("GET /api/backoffice/clubs/{club_id}/settings", s.handleBackofficeClubSettings)
	mux.HandleFunc("POST /api/backoffice/clubs/{club_id}", s.handleBackofficeUpdateClub)
	mux.HandleFunc("DELETE /api/backoffice/clubs/{club_id}", s.handleBackofficeDeleteClub)
	mux.HandleFunc("POST /api/backoffice/clubs/{club_id}/zones", s.handleBackofficeCreateZone)
	mux.HandleFunc("POST /api/backoffice/zones/{zone_id}", s.handleBackofficeUpdateZone)
	mux.HandleFunc("DELETE /api/backoffice/zones/{zone_id}", s.handleBackofficeDeleteZone)
	mux.HandleFunc("POST /api/backoffice/clubs/{club_id}/tariffs", s.handleBackofficeCreateTariff)
	mux.HandleFunc("POST /api/backoffice/tariffs/{tariff_id}", s.handleBackofficeUpdateTariff)
	mux.HandleFunc("DELETE /api/backoffice/tariffs/{tariff_id}", s.handleBackofficeDeleteTariff)
	mux.HandleFunc("POST /api/backoffice/clubs/{club_id}/pcs", s.handleBackofficeCreatePC)
	mux.HandleFunc("POST /api/backoffice/pcs/{pc_id}", s.handleBackofficeUpdatePC)
	mux.HandleFunc("DELETE /api/backoffice/pcs/{pc_id}", s.handleBackofficeDeletePC)
	mux.HandleFunc("POST /api/backoffice/clubs/{club_id}/users", s.handleBackofficeCreateUser)
	mux.HandleFunc("POST /api/backoffice/users/{user_id}/clubs/{club_id}", s.handleBackofficeUpdateUserRole)
	mux.HandleFunc("DELETE /api/backoffice/users/{user_id}/clubs/{club_id}", s.handleBackofficeDeleteUserRole)
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "clubpay-api"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Введите логин и пароль")
		return
	}

	var auth authContext
	var storedHash *string
	err := s.db.QueryRow(r.Context(), `
		SELECT id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(global_role, ''), password_hash
		FROM users
		WHERE status = 'active'
		  AND (
			lower(COALESCE(email, '')) = lower($1)
			OR COALESCE(phone, '') = $1
		  )
		LIMIT 1
	`, req.Login).Scan(&auth.UserID, &auth.Name, &auth.Email, &auth.Phone, &auth.GlobalRole, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) || storedHash == nil || *storedHash != hashPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	token := randomHex(32)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = s.db.Exec(r.Context(), `
		INSERT INTO auth_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, auth.UserID, hashToken(token), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	payload, err := s.authPayload(r.Context(), auth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	payload["token"] = token
	payload["expires_at"] = expiresAt
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	payload, err := s.authPayload(r.Context(), auth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token != "" {
		_, _ = s.db.Exec(r.Context(), `DELETE FROM auth_sessions WHERE token_hash = $1`, hashToken(token))
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeClubs(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubs, err := s.clubsForAuth(r.Context(), auth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clubs": clubs})
}

func (s *Server) handleBackofficeCreateClub(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	if !auth.IsSuperAdmin() {
		writeError(w, http.StatusForbidden, "super_admin role required")
		return
	}
	var req clubSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "club name is required")
		return
	}
	slug := slugify(req.Name)
	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO clubs (
			name, slug, legal_name, tin, address, timezone, status,
			click_merchant_id, click_service_id, click_merchant_user_id, click_secret_key,
			payme_merchant_id, payme_secret_key,
			platform_fee_bps, ofd_mxik, ofd_package_code
		)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'active'), $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`, req.Name, slug, req.LegalName, req.TIN, req.Address, defaultString(req.Timezone, "Asia/Tashkent"),
		req.Status, req.ClickMerchantID, req.ClickServiceID, req.ClickMerchantUserID, req.ClickSecretKey, req.PaymeMerchantID, req.PaymeSecretKey,
		req.PlatformFeeBPS, req.OFDMXIK, req.OFDPackageCode).Scan(&id)
	if err != nil {
		if writeConflictIfUnique(w, err, "club name or slug already exists") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleBackofficeClubSettings(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	actorRole, ok := s.requireClubRole(w, r, auth, clubID, "owner")
	if !ok {
		return
	}
	payload, err := s.clubSettings(r.Context(), clubID, actorRole == "super_admin")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleBackofficeUpdateClub(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	actorRole, ok := s.requireClubRole(w, r, auth, clubID, "owner")
	if !ok {
		return
	}
	var req clubSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "club name is required")
		return
	}
	var err error
	if actorRole == "super_admin" {
		_, err = s.db.Exec(r.Context(), `
			UPDATE clubs
			SET name = $2,
			    slug = $3,
			    legal_name = $4,
			    tin = $5,
			    address = $6,
			    timezone = $7,
			    status = $8,
			    click_merchant_id = $9,
			    click_service_id = $10,
			    click_merchant_user_id = $11,
			    click_secret_key = $12,
			    payme_merchant_id = $13,
			    payme_secret_key = $14,
			    platform_fee_bps = $15,
			    ofd_mxik = $16,
			    ofd_package_code = $17
			WHERE id = $1
		`, clubID, strings.TrimSpace(req.Name), slugify(req.Name), req.LegalName, req.TIN,
			req.Address, defaultString(req.Timezone, "Asia/Tashkent"), defaultString(req.Status, "active"),
			req.ClickMerchantID, req.ClickServiceID, req.ClickMerchantUserID, req.ClickSecretKey, req.PaymeMerchantID, req.PaymeSecretKey,
			req.PlatformFeeBPS, req.OFDMXIK, req.OFDPackageCode)
	} else {
		_, err = s.db.Exec(r.Context(), `
			UPDATE clubs
			SET name = $2,
			    slug = $3,
			    legal_name = $4,
			    tin = $5,
			    address = $6,
			    timezone = $7
			WHERE id = $1
		`, clubID, strings.TrimSpace(req.Name), slugify(req.Name), req.LegalName, req.TIN,
			req.Address, defaultString(req.Timezone, "Asia/Tashkent"))
	}
	if err != nil {
		if writeConflictIfUnique(w, err, "club name or slug already exists") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeDeleteClub(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	if !auth.IsSuperAdmin() {
		writeError(w, http.StatusForbidden, "super_admin role required")
		return
	}
	clubID := r.PathValue("club_id")
	if err := s.ensureNoAcceptedGrants(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM game_access_grants WHERE club_id = $1 AND status = 'accepted')
	`, clubID, "club has active sessions"); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `UPDATE clubs SET status = 'deleted' WHERE id = $1 AND status <> 'deleted'`, clubID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "club not found")
		return
	}
	_, _ = tx.Exec(r.Context(), `UPDATE user_club_roles SET status = 'deleted', updated_at = now() WHERE club_id = $1`, clubID)
	_, _ = tx.Exec(r.Context(), `UPDATE zones SET status = 'deleted' WHERE club_id = $1`, clubID)
	_, _ = tx.Exec(r.Context(), `UPDATE tariff_blocks SET status = 'deleted' WHERE club_id = $1`, clubID)
	_, _ = tx.Exec(r.Context(), `UPDATE pc_refs SET status_cache = 'deleted' WHERE club_id = $1`, clubID)
	_, _ = tx.Exec(r.Context(), `UPDATE qr_codes SET status = 'inactive' WHERE club_id = $1`, clubID)
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeCreateZone(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req zoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "zone name is required")
		return
	}
	hourlyPriceTiyin := priceTiyinFromUZS(req.HourlyPriceTiyin, req.HourlyPriceUZS)
	if hourlyPriceTiyin <= 0 {
		writeError(w, http.StatusBadRequest, "hourly price is required")
		return
	}
	sortOrder := req.SortOrder
	if sortOrder <= 0 {
		_ = s.db.QueryRow(r.Context(), `
			SELECT COALESCE(MAX(sort_order), 0) + 10
			FROM zones
			WHERE club_id = $1 AND status <> 'deleted'
		`, clubID).Scan(&sortOrder)
	}
	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order, status)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), 'active'))
		RETURNING id
	`, clubID, strings.TrimSpace(req.Name), hourlyPriceTiyin, sortOrder, req.Status).Scan(&id)
	if err != nil {
		if writeConflictIfUnique(w, err, "zone with this name already exists") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleBackofficeUpdateZone(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	zoneID := r.PathValue("zone_id")
	clubID, err := s.clubIDForEntity(r.Context(), "zones", zoneID)
	if err != nil {
		writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req zoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "zone name is required")
		return
	}
	hourlyPriceTiyin := priceTiyinFromUZS(req.HourlyPriceTiyin, req.HourlyPriceUZS)
	if hourlyPriceTiyin <= 0 {
		writeError(w, http.StatusBadRequest, "hourly price is required")
		return
	}
	_, err = s.db.Exec(r.Context(), `
		UPDATE zones
		SET name = $2,
		    hourly_price_tiyin = $3,
		    sort_order = CASE WHEN $4 > 0 THEN $4 ELSE sort_order END,
		    status = $5
		WHERE id = $1
	`, zoneID, strings.TrimSpace(req.Name), hourlyPriceTiyin, req.SortOrder, defaultString(req.Status, "active"))
	if err != nil {
		if writeConflictIfUnique(w, err, "zone with this name already exists") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeDeleteZone(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	zoneID := r.PathValue("zone_id")
	clubID, err := s.clubIDForEntity(r.Context(), "zones", zoneID)
	if err != nil {
		writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	if err := s.ensureNoAcceptedGrants(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM game_access_grants g
			JOIN pc_refs p ON p.id = g.pc_ref_id
			WHERE p.zone_id = $1 AND g.status = 'accepted'
		)
	`, zoneID, "zone has active sessions"); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `UPDATE zones SET status = 'deleted' WHERE id = $1`, zoneID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	_, _ = tx.Exec(r.Context(), `UPDATE tariff_blocks SET status = 'deleted' WHERE zone_id = $1`, zoneID)
	_, _ = tx.Exec(r.Context(), `UPDATE pc_refs SET status_cache = 'deleted' WHERE zone_id = $1`, zoneID)
	_, _ = tx.Exec(r.Context(), `
		UPDATE qr_codes SET status = 'inactive'
		WHERE pc_ref_id IN (SELECT id FROM pc_refs WHERE zone_id = $1)
	`, zoneID)
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeCreateTariff(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req tariffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	priceTiyin := req.PriceTiyin
	if priceTiyin == 0 && req.PriceUZS > 0 {
		priceTiyin = req.PriceUZS * 100
	}
	if strings.TrimSpace(req.Name) == "" || req.ZoneID == "" || req.DurationMinutes <= 0 || priceTiyin <= 0 {
		writeError(w, http.StatusBadRequest, "name, zone_id, duration_minutes and price are required")
		return
	}
	if !s.zoneBelongsToClub(r.Context(), req.ZoneID, clubID) {
		writeError(w, http.StatusBadRequest, "zone does not belong to club")
		return
	}
	sortOrder := req.SortOrder
	if sortOrder <= 0 {
		_ = s.db.QueryRow(r.Context(), `
			SELECT COALESCE(MAX(sort_order), 0) + 10
			FROM tariff_blocks
			WHERE club_id = $1 AND zone_id = $2 AND status <> 'deleted'
		`, clubID, req.ZoneID).Scan(&sortOrder)
	}
	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order, status)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'active'))
		RETURNING id
	`, clubID, req.ZoneID, strings.TrimSpace(req.Name), req.DurationMinutes, priceTiyin, sortOrder, req.Status).Scan(&id)
	if err != nil {
		if writeConflictIfUnique(w, err, "tariff with this duration already exists in selected zone") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleBackofficeUpdateTariff(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	tariffID := r.PathValue("tariff_id")
	clubID, err := s.clubIDForEntity(r.Context(), "tariff_blocks", tariffID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tariff not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req tariffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	priceTiyin := req.PriceTiyin
	if priceTiyin == 0 && req.PriceUZS > 0 {
		priceTiyin = req.PriceUZS * 100
	}
	if strings.TrimSpace(req.Name) == "" || req.ZoneID == "" || req.DurationMinutes <= 0 || priceTiyin <= 0 {
		writeError(w, http.StatusBadRequest, "name, zone_id, duration_minutes and price are required")
		return
	}
	if !s.zoneBelongsToClub(r.Context(), req.ZoneID, clubID) {
		writeError(w, http.StatusBadRequest, "zone does not belong to club")
		return
	}
	_, err = s.db.Exec(r.Context(), `
		UPDATE tariff_blocks
		SET zone_id = $2,
		    name = $3,
		    duration_minutes = $4,
		    price_tiyin = $5,
		    sort_order = CASE WHEN $6 > 0 THEN $6 ELSE sort_order END,
		    status = $7
		WHERE id = $1
	`, tariffID, req.ZoneID, strings.TrimSpace(req.Name), req.DurationMinutes, priceTiyin, req.SortOrder, defaultString(req.Status, "active"))
	if err != nil {
		if writeConflictIfUnique(w, err, "tariff with this duration already exists in selected zone") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeDeleteTariff(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	tariffID := r.PathValue("tariff_id")
	clubID, err := s.clubIDForEntity(r.Context(), "tariff_blocks", tariffID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tariff not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	result, err := s.db.Exec(r.Context(), `UPDATE tariff_blocks SET status = 'deleted' WHERE id = $1`, tariffID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "tariff not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeCreatePC(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req pcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ZoneID == "" || req.Number <= 0 {
		writeError(w, http.StatusBadRequest, "zone_id and number are required")
		return
	}
	if !s.zoneBelongsToClub(r.Context(), req.ZoneID, clubID) {
		writeError(w, http.StatusBadRequest, "zone does not belong to club")
		return
	}
	label := defaultString(req.Label, fmt.Sprintf("PC #%02d", req.Number))
	externalPCID := strings.TrimSpace(req.ExternalPCID)
	if externalPCID == "" {
		externalPCID = s.uniqueExternalPCID(r.Context(), clubID, label, "")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() {
		_ = tx.Rollback(r.Context())
	}()
	var id string
	err = tx.QueryRow(r.Context(), `
		INSERT INTO pc_refs (club_id, zone_id, external_pc_id, number, label, status_cache)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'available'))
		RETURNING id
	`, clubID, req.ZoneID, externalPCID, req.Number, label, req.Status).Scan(&id)
	if err != nil {
		if writeConflictIfUnique(w, err, "pc number or Core external_pc_id already exists in this club") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token := strings.TrimSpace(req.QRToken)
	if token == "" {
		token = fmt.Sprintf("%s-%s", strings.TrimSuffix(slugify(label), "-"), randomHex(3))
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO qr_codes (club_id, pc_ref_id, public_token, type, status)
		VALUES ($1, $2, $3, 'static_pc', 'active')
	`, clubID, id, token)
	if err != nil {
		if writeConflictIfUnique(w, err, "qr token already exists") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "qr_token": token, "qr_url": s.cfg.FrontendBaseURL + "/qr/" + token})
}

func (s *Server) handleBackofficeUpdatePC(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	pcID := r.PathValue("pc_id")
	clubID, err := s.clubIDForEntity(r.Context(), "pc_refs", pcID)
	if err != nil {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req pcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ZoneID == "" || req.Number <= 0 {
		writeError(w, http.StatusBadRequest, "zone_id and number are required")
		return
	}
	if !s.zoneBelongsToClub(r.Context(), req.ZoneID, clubID) {
		writeError(w, http.StatusBadRequest, "zone does not belong to club")
		return
	}
	label := defaultString(req.Label, fmt.Sprintf("PC #%02d", req.Number))
	externalPCID := strings.TrimSpace(req.ExternalPCID)
	if externalPCID == "" {
		externalPCID = s.uniqueExternalPCID(r.Context(), clubID, label, pcID)
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() {
		_ = tx.Rollback(r.Context())
	}()
	_, err = tx.Exec(r.Context(), `
		UPDATE pc_refs
		SET zone_id = $2, external_pc_id = $3, number = $4, label = $5, status_cache = $6
		WHERE id = $1
	`, pcID, req.ZoneID, externalPCID, req.Number, label, defaultString(req.Status, "available"))
	if err != nil {
		if writeConflictIfUnique(w, err, "pc number or Core external_pc_id already exists in this club") {
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.QRToken) != "" {
		token := strings.TrimSpace(req.QRToken)
		var existingPCID string
		err = tx.QueryRow(r.Context(), `SELECT pc_ref_id FROM qr_codes WHERE public_token = $1`, token).Scan(&existingPCID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err == nil && existingPCID != pcID {
			writeError(w, http.StatusConflict, "qr token already belongs to another pc")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(r.Context(), `
			INSERT INTO qr_codes (club_id, pc_ref_id, public_token, type, status)
			VALUES ($1, $2, $3, 'static_pc', 'active')
		`, clubID, pcID, token)
		} else {
			_, err = tx.Exec(r.Context(), `UPDATE qr_codes SET club_id = $2, status = 'active' WHERE public_token = $1`, token, clubID)
		}
		if err != nil {
			if writeConflictIfUnique(w, err, "qr token already exists") {
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeDeletePC(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	pcID := r.PathValue("pc_id")
	clubID, err := s.clubIDForEntity(r.Context(), "pc_refs", pcID)
	if err != nil {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	if err := s.ensureNoAcceptedGrants(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM game_access_grants WHERE pc_ref_id = $1 AND status = 'accepted')
	`, pcID, "pc has an active session"); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `UPDATE pc_refs SET status_cache = 'deleted' WHERE id = $1`, pcID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	_, _ = tx.Exec(r.Context(), `UPDATE qr_codes SET status = 'inactive' WHERE pc_ref_id = $1`, pcID)
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeCreateUser(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	actorRole, ok := s.requireClubRole(w, r, auth, clubID, "owner")
	if !ok {
		return
	}
	var req userRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Role = defaultString(req.Role, "admin")
	if req.Name == "" || (req.Email == "" && req.Phone == "") {
		writeError(w, http.StatusBadRequest, "name and email or phone are required")
		return
	}
	if !canAssignClubRole(actorRole, req.Role) {
		writeError(w, http.StatusForbidden, "role is not allowed")
		return
	}
	passwordHash := ""
	if req.Password != "" {
		passwordHash = hashPassword(req.Password)
	}
	var userID string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO users (name, email, phone, role, password_hash, status)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, NULLIF($5, ''), COALESCE(NULLIF($6, ''), 'active'))
		ON CONFLICT DO NOTHING
		RETURNING id
	`, req.Name, req.Email, req.Phone, req.Role, passwordHash, req.Status).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.db.QueryRow(r.Context(), `
			SELECT id FROM users
			WHERE lower(COALESCE(email, '')) = lower($1) OR COALESCE(phone, '') = $2
			LIMIT 1
		`, req.Email, req.Phone).Scan(&userID)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if passwordHash != "" {
		_, _ = s.db.Exec(r.Context(), `UPDATE users SET name = $2, password_hash = $3, updated_at = now() WHERE id = $1`, userID, req.Name, passwordHash)
	}
	_, err = s.db.Exec(r.Context(), `
		INSERT INTO user_club_roles (user_id, club_id, role, status)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'active'))
		ON CONFLICT (user_id, club_id) DO UPDATE SET role = EXCLUDED.role, status = EXCLUDED.status, updated_at = now()
	`, userID, clubID, req.Role, req.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user_id": userID})
}

func (s *Server) handleBackofficeUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("user_id")
	clubID := r.PathValue("club_id")
	actorRole, ok := s.requireClubRole(w, r, auth, clubID, "owner")
	if !ok {
		return
	}
	var req userRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Role = defaultString(req.Role, "admin")
	if !canAssignClubRole(actorRole, req.Role) {
		writeError(w, http.StatusForbidden, "role is not allowed")
		return
	}
	if actorRole != "super_admin" {
		var currentRole string
		err := s.db.QueryRow(r.Context(), `
			SELECT role FROM user_club_roles
			WHERE user_id = $1 AND club_id = $2 AND status <> 'deleted'
		`, userID, clubID).Scan(&currentRole)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user access not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if currentRole != "admin" {
			writeError(w, http.StatusForbidden, "only admin access can be changed")
			return
		}
	}
	if req.Name != "" || req.Email != "" || req.Phone != "" || req.Password != "" {
		passwordSQL := ""
		args := []any{userID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Email), strings.TrimSpace(req.Phone)}
		if req.Password != "" {
			passwordSQL = ", password_hash = $5"
			args = append(args, hashPassword(req.Password))
		}
		_, _ = s.db.Exec(r.Context(), `
			UPDATE users
			SET name = COALESCE(NULLIF($2, ''), name),
			    email = COALESCE(NULLIF($3, ''), email),
			    phone = COALESCE(NULLIF($4, ''), phone),
			    updated_at = now()`+passwordSQL+`
			WHERE id = $1
		`, args...)
	}
	_, err := s.db.Exec(r.Context(), `
		UPDATE user_club_roles SET role = $3, status = $4, updated_at = now()
		WHERE user_id = $1 AND club_id = $2
	`, userID, clubID, req.Role, defaultString(req.Status, "active"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleBackofficeDeleteUserRole(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("user_id")
	clubID := r.PathValue("club_id")
	actorRole, ok := s.requireClubRole(w, r, auth, clubID, "owner")
	if !ok {
		return
	}
	if actorRole != "super_admin" {
		var currentRole string
		err := s.db.QueryRow(r.Context(), `
			SELECT role FROM user_club_roles
			WHERE user_id = $1 AND club_id = $2 AND status <> 'deleted'
		`, userID, clubID).Scan(&currentRole)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user access not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if currentRole != "admin" {
			writeError(w, http.StatusForbidden, "only admin access can be deleted")
			return
		}
	}
	result, err := s.db.Exec(r.Context(), `
		UPDATE user_club_roles SET status = 'deleted', updated_at = now()
		WHERE user_id = $1 AND club_id = $2
	`, userID, clubID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "user access not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing QR token")
		return
	}

	ctx := r.Context()
	if err := s.expireElapsedGrants(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var pc qrPC
	var zoneStatus string
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.name, p.id, p.external_pc_id, p.number, p.label, p.status_cache, z.id, z.name, z.hourly_price_tiyin, z.status,
		       COALESCE(c.click_merchant_id, ''), COALESCE(c.click_service_id, ''), COALESCE(c.click_merchant_user_id, ''), COALESCE(c.click_secret_key, ''),
		       COALESCE(c.payme_merchant_id, ''), COALESCE(c.payme_secret_key, '')
		FROM qr_codes q
		JOIN pc_refs p ON p.id = q.pc_ref_id
		JOIN clubs c ON c.id = q.club_id
		JOIN zones z ON z.id = p.zone_id
		WHERE q.public_token = $1 AND q.status = 'active' AND q.type = 'static_pc'
		  AND c.status = 'active' AND z.status <> 'deleted' AND p.status_cache <> 'deleted'
	`, token).Scan(
		&pc.ClubID, &pc.ClubName, &pc.PCID, &pc.ExternalPCID, &pc.Number, &pc.Label, &pc.Status, &pc.ZoneID, &pc.ZoneName, &pc.HourlyPriceTiyin,
		&zoneStatus, &pc.ClickMerchantID, &pc.ClickServiceID, &pc.ClickMerchantUserID, &pc.ClickSecretKey, &pc.PaymeMerchantID, &pc.PaymeSecretKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "QR token not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if zoneStatus == "maintenance" {
		pc.Status = "maintenance"
	} else {
		pc.Status = s.syncCorePCStatus(ctx, pc.PCID, pc.ExternalPCID, pc.Status)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, name, duration_minutes, price_tiyin
		FROM tariff_blocks
		WHERE club_id = $1 AND zone_id = $2 AND status = 'active'
		ORDER BY sort_order, duration_minutes
	`, pc.ClubID, pc.ZoneID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var tariffs []tariffDTO
	for rows.Next() {
		var t tariffDTO
		if err := rows.Scan(&t.ID, &t.Name, &t.DurationMinutes, &t.PriceTiyin); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		t.PriceUZS = t.PriceTiyin / 100
		tariffs = append(tariffs, t)
	}

	telegramLink, telegramUsername := s.telegramBotPublicLink(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"club": map[string]any{
			"id":   pc.ClubID,
			"name": pc.ClubName,
		},
		"pc": map[string]any{
			"id":             pc.PCID,
			"external_pc_id": pc.ExternalPCID,
			"number":         pc.Number,
			"label":          pc.Label,
			"status":         pc.Status,
		},
		"zone": map[string]any{
			"id":                 pc.ZoneID,
			"name":               pc.ZoneName,
			"hourly_price_tiyin": pc.HourlyPriceTiyin,
			"hourly_price_uzs":   pc.HourlyPriceTiyin / 100,
		},
		"tariffs":           tariffs,
		"payment_providers": s.paymentProviderOptions(pc.ClickMerchantID, pc.ClickServiceID, pc.ClickMerchantUserID, pc.ClickSecretKey, pc.PaymeMerchantID, pc.PaymeSecretKey),
		"telegram": map[string]any{
			"bot_link":     telegramLink,
			"bot_username": telegramUsername,
		},
	})
}

func (s *Server) handleOrder(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("invoice_id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice_id is required")
		return
	}
	order, err := s.orderByInvoiceID(r.Context(), invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (s *Server) handleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	var req createCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.QRToken == "" || (req.TariffBlockID == "" && req.AmountUZS <= 0) {
		writeError(w, http.StatusBadRequest, "qr_token and package or amount_uzs are required")
		return
	}
	if req.AmountUZS < 0 {
		writeError(w, http.StatusBadRequest, "amount_uzs must be positive")
		return
	}

	ctx := r.Context()
	if err := s.expireElapsedGrants(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var orderSeed checkoutSeed
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.name, p.id, p.external_pc_id, p.status_cache, z.id, z.hourly_price_tiyin,
		       COALESCE(c.click_merchant_id, ''), COALESCE(c.click_service_id, ''), COALESCE(c.click_merchant_user_id, ''), COALESCE(c.click_secret_key, ''),
		       COALESCE(c.payme_merchant_id, ''), COALESCE(c.payme_secret_key, ''), COALESCE(c.platform_fee_bps, 0),
		       COALESCE(c.ofd_mxik, ''), COALESCE(c.ofd_package_code, '')
		FROM qr_codes q
		JOIN clubs c ON c.id = q.club_id
		JOIN pc_refs p ON p.id = q.pc_ref_id
		JOIN zones z ON z.id = p.zone_id
		WHERE q.public_token = $1 AND q.status = 'active'
		  AND c.status = 'active' AND z.status = 'active' AND p.status_cache <> 'deleted'
	`, req.QRToken).Scan(
		&orderSeed.ClubID, &orderSeed.ClubName, &orderSeed.PCID, &orderSeed.ExternalPCID, &orderSeed.PCStatus,
		&orderSeed.ZoneID, &orderSeed.HourlyPriceTiyin,
		&orderSeed.ClickMerchantID, &orderSeed.ClickServiceID, &orderSeed.ClickMerchantUserID, &orderSeed.ClickSecretKey,
		&orderSeed.PaymeMerchantID, &orderSeed.PaymeSecretKey, &orderSeed.PlatformFeeBPS,
		&orderSeed.OFDMXIK, &orderSeed.OFDPackageCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "QR token not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.TariffBlockID != "" && req.AmountUZS <= 0 {
		err = s.db.QueryRow(ctx, `
			SELECT id, name, duration_minutes, price_tiyin
			FROM tariff_blocks
			WHERE id = $1 AND club_id = $2 AND zone_id = $3 AND status = 'active'
		`, req.TariffBlockID, orderSeed.ClubID, orderSeed.ZoneID).Scan(
			&orderSeed.TariffID, &orderSeed.TariffName, &orderSeed.DurationMinutes, &orderSeed.AmountTiyin,
		)
		orderSeed.DurationSeconds = orderSeed.DurationMinutes * 60
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "package not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	orderSeed.PCStatus = s.syncCorePCStatus(ctx, orderSeed.PCID, orderSeed.ExternalPCID, orderSeed.PCStatus)
	if !isPayablePCStatus(orderSeed.PCStatus) {
		if orderSeed.PCStatus != "occupied" {
			writeError(w, http.StatusConflict, "PC is not available")
			return
		}
		grant, ok, err := activeGrantForPC(ctx, s.db, orderSeed.PCID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok || grant.CoreSessionID == "" {
			writeError(w, http.StatusConflict, "active session for extension not found")
			return
		}
		orderSeed.ExtensionGrantID = grant.ID
	}
	if req.AmountUZS > 0 {
		amountTiyin := req.AmountUZS * 100
		orderSeed.AmountTiyin = amountTiyin
		orderSeed.DurationSeconds = secondsForAmount(amountTiyin, orderSeed.HourlyPriceTiyin)
		orderSeed.DurationMinutes = secondsToMinutesCeil(orderSeed.DurationSeconds)
		orderSeed.TariffID = ""
		orderSeed.TariffName = "Своя сумма"
	}
	if orderSeed.DurationSeconds <= 0 {
		orderSeed.DurationSeconds = orderSeed.DurationMinutes * 60
	}
	if strings.TrimSpace(req.VoucherCode) != "" {
		voucherID, voucherSeconds, err := s.validVoucherForPC(ctx, req.VoucherCode, orderSeed.ClubID, orderSeed.PCID, orderSeed.PCStatus)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		orderSeed.VoucherID = voucherID
		orderSeed.VoucherSeconds = voucherSeconds
	}

	invoiceID := "cp_" + randomHex(12)
	expiresAt := time.Now().Add(15 * time.Minute)
	provider := payments.NormalizeProvider(req.PaymentProvider, s.cfg.DefaultPaymentProvider)
	if provider == "" {
		writeError(w, http.StatusBadRequest, "payment provider must be click or payme")
		return
	}
	if err := s.ensureCheckoutProviderReady(provider, orderSeed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	platformSplitAmount, clubSplitAmount := s.splitAmountsForClub(orderSeed.AmountTiyin, orderSeed.PlatformFeeBPS)
	providerPrepareID := ""
	if provider == payments.ProviderClick {
		providerPrepareID = randomNumericID()
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	var orderID string
	var tariffIDArg any
	if orderSeed.TariffID != "" {
		tariffIDArg = orderSeed.TariffID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO payment_orders (
			invoice_id, club_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds, voucher_id,
			provider, provider_prepare_id, status, split_platform_amount_tiyin, split_club_amount_tiyin, expires_at,
			extension_grant_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::uuid, $9, NULLIF($10, ''), 'created', $11, $12, $13, NULLIF($14, '')::uuid)
		RETURNING id
	`, invoiceID, orderSeed.ClubID, orderSeed.PCID, tariffIDArg, orderSeed.AmountTiyin, orderSeed.DurationMinutes, orderSeed.DurationSeconds, orderSeed.VoucherID,
		provider, providerPrepareID, platformSplitAmount, clubSplitAmount, expiresAt, orderSeed.ExtensionGrantID).Scan(&orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	returnURL := s.cfg.FrontendBaseURL + "/payment/return?invoice_id=" + invoiceID

	var providerPaymentID, checkoutURL string
	switch provider {
	case payments.ProviderMock:
		providerPaymentID = "mock_" + randomHex(12)
		checkoutURL = s.cfg.FrontendBaseURL + "/payment/mock?invoice_id=" + invoiceID
	case payments.ProviderPayme:
		var err error
		checkoutURL, err = payments.BuildPaymeCheckoutURL(
			s.cfg.PaymeCheckoutURL,
			defaultString(orderSeed.PaymeMerchantID, s.cfg.PaymeMerchantID),
			invoiceID,
			orderSeed.AmountTiyin,
			returnURL,
		)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	case payments.ProviderClick:
		var err error
		checkoutURL, err = payments.BuildClickCheckoutURL(
			s.cfg.ClickCheckoutURL,
			defaultString(orderSeed.ClickMerchantID, s.cfg.ClickMerchantID),
			defaultString(orderSeed.ClickServiceID, s.cfg.ClickServiceID),
			defaultString(orderSeed.ClickMerchantUserID, s.cfg.ClickMerchantUserID),
			invoiceID,
			orderSeed.AmountTiyin,
			returnURL,
		)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE payment_orders
		SET provider_payment_id = NULLIF($1, ''), checkout_url = $2, status = 'payment_pending', updated_at = now()
		WHERE id = $3
	`, providerPaymentID, checkoutURL, orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"order": map[string]any{
			"id":                          orderID,
			"invoice_id":                  invoiceID,
			"provider":                    provider,
			"provider_payment_id":         providerPaymentID,
			"amount_tiyin":                orderSeed.AmountTiyin,
			"amount_uzs":                  orderSeed.AmountTiyin / 100,
			"duration_minutes":            orderSeed.DurationMinutes,
			"duration_seconds":            orderSeed.DurationSeconds,
			"voucher_seconds":             orderSeed.VoucherSeconds,
			"split_platform_amount_tiyin": platformSplitAmount,
			"split_club_amount_tiyin":     clubSplitAmount,
			"expires_at":                  expiresAt,
		},
		"checkout_url": checkoutURL,
	})
}

func (s *Server) handleMockPaymentSuccess(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		writeError(w, http.StatusForbidden, "mock payments are disabled in production")
		return
	}
	invoiceID := r.PathValue("invoice_id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice_id is required")
		return
	}

	var amount int64
	var providerPaymentID *string
	err := s.db.QueryRow(r.Context(), `
		SELECT amount_tiyin, provider_payment_id
		FROM payment_orders
		WHERE invoice_id = $1
	`, invoiceID).Scan(&amount, &providerPaymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	paymentUUID := "mock_" + randomHex(12)
	if providerPaymentID != nil && *providerPaymentID != "" {
		paymentUUID = *providerPaymentID
	}
	rawPayload, _ := json.Marshal(map[string]any{
		"invoice_id": invoiceID,
		"uuid":       paymentUUID,
		"amount":     amount,
		"mock":       true,
	})
	grantID, err := s.applyPaymentSuccess(r.Context(), paymentSuccess{
		Provider:          payments.ProviderMock,
		AmountTiyin:       amount,
		InvoiceID:         invoiceID,
		ProviderPaymentID: paymentUUID,
		ReceiptURL:        s.cfg.FrontendBaseURL + "/payment/mock-receipt?invoice_id=" + invoiceID,
		PaidAt:            time.Now(),
		PS:                "mock",
		CardPAN:           "mock",
	}, rawPayload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "grant_id": grantID})
}

func (s *Server) handleClickPrepare(w http.ResponseWriter, r *http.Request) {
	s.handleClickCallbackAction(w, r, "0")
}

func (s *Server) handleClickComplete(w http.ResponseWriter, r *http.Request) {
	s.handleClickCallbackAction(w, r, "1")
}

func (s *Server) handleClickCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeClickResponse(w, "", "", "", -8, "Invalid form")
		return
	}
	s.handleClickCallbackAction(w, r, r.FormValue("action"))
}

func (s *Server) handleClickCallbackAction(w http.ResponseWriter, r *http.Request, expectedAction string) {
	if err := r.ParseForm(); err != nil {
		writeClickResponse(w, "", "", "", -8, "Invalid form")
		return
	}
	payload := formPayload(r)
	rawPayload, _ := json.Marshal(payload)
	clickTransID := r.FormValue("click_trans_id")
	merchantTransID := r.FormValue("merchant_trans_id")
	merchantPrepareID := r.FormValue("merchant_prepare_id")
	amountText := r.FormValue("amount")
	action := defaultString(r.FormValue("action"), expectedAction)
	eventID, _ := s.insertProviderEvent(r.Context(), payments.ProviderClick, "callback_action_"+action, clickTransID, rawPayload)

	if expectedAction != "" && action != expectedAction {
		s.markProviderEvent(r.Context(), eventID, "invalid_action")
		writeClickResponse(w, clickTransID, merchantTransID, merchantPrepareID, -3, "Action not found")
		return
	}
	order, err := s.clickOrderByInvoice(r.Context(), merchantTransID)
	if errors.Is(err, pgx.ErrNoRows) {
		s.markProviderEvent(r.Context(), eventID, "not_found")
		writeClickResponse(w, clickTransID, merchantTransID, merchantPrepareID, -5, "Order not found")
		return
	}
	if err != nil {
		s.markProviderEvent(r.Context(), eventID, "failed")
		writeClickResponse(w, clickTransID, merchantTransID, merchantPrepareID, -8, err.Error())
		return
	}
	secret := defaultString(order.ClickSecretKey, s.cfg.ClickSecretKey)
	signPrepareID := ""
	if action == "1" {
		signPrepareID = merchantPrepareID
	}
	if !payments.VerifyClickSign(secret, clickTransID, r.FormValue("service_id"), merchantTransID, signPrepareID, amountText, action, r.FormValue("sign_time"), r.FormValue("sign_string")) {
		s.markProviderEvent(r.Context(), eventID, "invalid_sign")
		writeClickResponse(w, clickTransID, merchantTransID, merchantPrepareID, -1, "Sign check failed")
		return
	}
	amountTiyin := clickAmountToTiyin(amountText)
	if amountTiyin != order.AmountTiyin {
		s.markProviderEvent(r.Context(), eventID, "amount_mismatch")
		writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -2, "Incorrect amount")
		return
	}
	if action == "0" {
		if order.Status == "paid" {
			s.markProviderEvent(r.Context(), eventID, "already_paid")
			writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -4, "Already paid")
			return
		}
		_, err = s.db.Exec(r.Context(), `
			UPDATE payment_orders
			SET provider = 'click',
			    provider_payment_id = COALESCE(NULLIF(provider_payment_id, ''), $1),
			    provider_status = 'prepared',
			    provider_payload = $2,
			    updated_at = now()
			WHERE invoice_id = $3
		`, clickTransID, rawPayload, merchantTransID)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -8, err.Error())
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, 0, "Success")
		return
	}
	if r.FormValue("error") != "0" && r.FormValue("error") != "" {
		_, _ = s.db.Exec(r.Context(), `
			UPDATE payment_orders
			SET provider_status = $1, status = 'failed', provider_payload = $2, updated_at = now()
			WHERE invoice_id = $3 AND status <> 'paid'
		`, "click_error_"+r.FormValue("error"), rawPayload, merchantTransID)
		s.markProviderEvent(r.Context(), eventID, "provider_error")
		writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -9, defaultString(r.FormValue("error_note"), "Payment cancelled"))
		return
	}
	if merchantPrepareID != "" && order.ProviderPrepareID != "" && merchantPrepareID != order.ProviderPrepareID {
		s.markProviderEvent(r.Context(), eventID, "prepare_mismatch")
		writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -6, "Transaction not found")
		return
	}
	paidAt := parseOptionalTime(r.FormValue("sign_time"))
	if paidAt == nil {
		now := time.Now()
		paidAt = &now
	}
	grantID, err := s.applyPaymentSuccess(r.Context(), paymentSuccess{
		Provider:          payments.ProviderClick,
		AmountTiyin:       amountTiyin,
		InvoiceID:         merchantTransID,
		ProviderPaymentID: defaultString(clickTransID, r.FormValue("click_paydoc_id")),
		ReceiptURL:        "",
		PaidAt:            *paidAt,
		PS:                "click",
	}, rawPayload)
	if err != nil {
		s.markProviderEvent(r.Context(), eventID, "failed")
		writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, -8, err.Error())
		return
	}
	s.markProviderEvent(r.Context(), eventID, "processed")
	writeClickResponse(w, clickTransID, merchantTransID, order.ProviderPrepareID, 0, "Success", "merchant_confirm_id", order.ProviderPrepareID, "grant_id", grantID)
}

func (s *Server) handlePaymeSandboxPage(w http.ResponseWriter, r *http.Request) {
	if !s.isPaymeSandbox() {
		writeError(w, http.StatusNotFound, "Payme sandbox is disabled")
		return
	}
	invoiceID := strings.TrimSpace(r.PathValue("invoice_id"))
	order, err := s.providerOrderByInvoice(r.Context(), invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order.Provider != payments.ProviderPayme {
		writeError(w, http.StatusBadRequest, "order is not a Payme order")
		return
	}
	returnURL := s.cfg.FrontendBaseURL + "/payment/return?invoice_id=" + url.QueryEscape(invoiceID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = paymeSandboxPageTemplate.Execute(w, map[string]any{
		"InvoiceID": invoiceID,
		"AmountUZS": order.AmountTiyin / 100,
		"Paid":      order.Status == "paid",
		"PayURL":    strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/api/payments/payme/sandbox/" + url.PathEscape(invoiceID) + "/pay",
		"ReturnURL": returnURL,
	})
}

func (s *Server) handlePaymeSandboxPay(w http.ResponseWriter, r *http.Request) {
	if !s.isPaymeSandbox() {
		writeError(w, http.StatusNotFound, "Payme sandbox is disabled")
		return
	}
	invoiceID := strings.TrimSpace(r.PathValue("invoice_id"))
	order, err := s.providerOrderByInvoice(r.Context(), invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order.Provider != payments.ProviderPayme {
		writeError(w, http.StatusBadRequest, "order is not a Payme order")
		return
	}
	returnURL := s.cfg.FrontendBaseURL + "/payment/return?invoice_id=" + url.QueryEscape(invoiceID)
	if order.Status == "paid" {
		http.Redirect(w, r, returnURL, http.StatusSeeOther)
		return
	}
	if order.Status != "payment_pending" && order.Status != "created" {
		writeError(w, http.StatusConflict, "order is not payable")
		return
	}

	now := time.Now()
	providerPaymentID := "payme_sandbox_" + randomHex(12)
	rawPayload, _ := json.Marshal(map[string]any{
		"id":     providerPaymentID,
		"method": "SandboxPerformTransaction",
		"params": map[string]any{
			"amount":  order.AmountTiyin,
			"account": map[string]string{"order_id": invoiceID},
		},
	})
	_, err = s.applyPaymentSuccess(r.Context(), paymentSuccess{
		Provider:          payments.ProviderPayme,
		AmountTiyin:       order.AmountTiyin,
		InvoiceID:         invoiceID,
		ProviderPaymentID: providerPaymentID,
		PaidAt:            now,
		PS:                "payme",
	}, rawPayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

func (s *Server) handlePaymeCallback(w http.ResponseWriter, r *http.Request) {
	var rpc paymeRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
		writePaymeError(w, nil, -32700, "Неверный JSON")
		return
	}
	rawPayload, _ := json.Marshal(rpc)
	orderRef := paymeOrderRef(rpc)
	credentials, _ := s.paymeCredentialsForRequest(r.Context(), rpc, orderRef)
	if !s.verifyPaymeAuth(r, credentials) {
		writePaymeError(w, rpc.ID, -32504, "Недостаточно прав для выполнения метода")
		return
	}
	eventID, _ := s.insertProviderEvent(r.Context(), payments.ProviderPayme, rpc.Method, paymeEventExternalID(rpc, orderRef), rawPayload)

	switch rpc.Method {
	case "CheckPerformTransaction":
		result, err := s.paymeCheckPerform(r.Context(), rpc.Params)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	case "CreateTransaction":
		result, err := s.paymeCreateTransaction(r.Context(), rpc.Params, rawPayload)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	case "PerformTransaction":
		result, err := s.paymePerformTransaction(r.Context(), rpc.Params, rawPayload)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	case "CancelTransaction":
		result, err := s.paymeCancelTransaction(r.Context(), rpc.Params, rawPayload)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	case "CheckTransaction":
		result, err := s.paymeCheckTransaction(r.Context(), rpc.Params)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	case "GetStatement":
		result, err := s.paymeGetStatement(r.Context(), rpc.Params)
		if err != nil {
			s.markProviderEvent(r.Context(), eventID, "failed")
			writePaymeError(w, rpc.ID, paymeErrorCode(err), err.Error(), paymeErrorData(err))
			return
		}
		s.markProviderEvent(r.Context(), eventID, "processed")
		writePaymeResult(w, rpc.ID, result)
	default:
		s.markProviderEvent(r.Context(), eventID, "unknown_method")
		writePaymeError(w, rpc.ID, -32601, "Метод не найден")
	}
}

func (s *Server) handlePaymentSync(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("invoice_id")
	order, err := s.orderByInvoiceID(r.Context(), invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "order": order})
}

func (s *Server) clickOrderByInvoice(ctx context.Context, invoiceID string) (providerOrder, error) {
	var order providerOrder
	err := s.db.QueryRow(ctx, `
		SELECT po.id, po.invoice_id, po.amount_tiyin, po.status, COALESCE(po.provider_prepare_id, ''),
		       COALESCE(c.click_secret_key, ''), COALESCE(c.click_service_id, '')
		FROM payment_orders po
		JOIN clubs c ON c.id = po.club_id
		WHERE po.invoice_id = $1
	`, invoiceID).Scan(&order.ID, &order.InvoiceID, &order.AmountTiyin, &order.Status, &order.ProviderPrepareID, &order.ClickSecretKey, &order.ClickServiceID)
	return order, err
}

func (s *Server) paymeCheckPerform(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var params paymeOrderParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	orderID := params.Account["order_id"]
	if strings.TrimSpace(orderID) == "" {
		return nil, paymeErr(-31050, "Заказ не найден", "order_id")
	}
	order, err := s.providerOrderByInvoice(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, paymeErr(-31050, "Заказ не найден", "order_id")
	}
	if err != nil {
		return nil, err
	}
	if order.AmountTiyin != params.Amount {
		return nil, paymeErr(-31001, "Сумма платежа не совпадает")
	}
	if order.Status == "paid" {
		return nil, paymeErr(-31050, "Заказ уже оплачен", "order_id")
	}
	if order.Status != "payment_pending" && order.Status != "created" {
		return nil, paymeErr(-31050, "Заказ недоступен для оплаты", "order_id")
	}
	return map[string]any{"allow": true}, nil
}

func (s *Server) paymeCreateTransaction(ctx context.Context, raw json.RawMessage, rawPayload []byte) (map[string]any, error) {
	var params paymeOrderParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	orderID := params.Account["order_id"]
	if strings.TrimSpace(orderID) == "" {
		return nil, paymeErr(-31050, "Заказ не найден", "order_id")
	}
	order, err := s.providerOrderByInvoice(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, paymeErr(-31050, "Заказ не найден", "order_id")
	}
	if err != nil {
		return nil, err
	}
	if order.AmountTiyin != params.Amount {
		return nil, paymeErr(-31001, "Сумма платежа не совпадает")
	}
	if order.Status != "payment_pending" && order.Status != "created" {
		return nil, paymeErr(-31008, "Невозможно выполнить операцию")
	}
	if order.ProviderPaymentID != "" && order.ProviderPaymentID != params.ID {
		return nil, paymeErr(-31008, "Невозможно выполнить операцию")
	}
	createTime := params.Time
	if createTime == 0 {
		createTime = unixMilli(time.Now())
	}
	_, err = s.db.Exec(ctx, `
		UPDATE payment_orders
		SET provider = 'payme',
		    provider_payment_id = $1,
		    provider_time_ms = $2,
		    provider_status = 'created',
		    provider_payload = $3,
		    updated_at = now()
		WHERE invoice_id = $4
	`, params.ID, createTime, rawPayload, orderID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"create_time": createTime, "transaction": order.ID, "state": 1}, nil
}

func (s *Server) paymePerformTransaction(ctx context.Context, raw json.RawMessage, rawPayload []byte) (map[string]any, error) {
	var params paymeTransactionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	order, err := s.providerOrderByPaymentID(ctx, payments.ProviderPayme, params.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, paymeErr(-31003, "Транзакция не найдена")
	}
	if err != nil {
		return nil, err
	}
	if order.Status == "paid" {
		return map[string]any{"transaction": order.ID, "perform_time": unixMilli(order.PaidAt), "state": 2}, nil
	}
	if order.Status != "payment_pending" && order.Status != "created" {
		return nil, paymeErr(-31008, "Невозможно выполнить операцию")
	}
	now := time.Now()
	grantID, err := s.applyPaymentSuccess(ctx, paymentSuccess{
		Provider:          payments.ProviderPayme,
		AmountTiyin:       order.AmountTiyin,
		InvoiceID:         order.InvoiceID,
		ProviderPaymentID: params.ID,
		PaidAt:            now,
		PS:                "payme",
	}, rawPayload)
	if err != nil {
		return nil, err
	}
	return map[string]any{"transaction": order.ID, "perform_time": unixMilli(now), "state": 2, "grant_id": grantID}, nil
}

func (s *Server) paymeCancelTransaction(ctx context.Context, raw json.RawMessage, rawPayload []byte) (map[string]any, error) {
	var params paymeCancelParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	order, err := s.providerOrderByPaymentID(ctx, payments.ProviderPayme, params.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, paymeErr(-31003, "Транзакция не найдена")
	}
	if err != nil {
		return nil, err
	}
	if order.Status == "failed" || order.Status == "refunded" {
		return map[string]any{"transaction": order.ID, "cancel_time": unixMilli(order.UpdatedAt), "state": paymeState(order.Status)}, nil
	}
	if order.Status != "payment_pending" && order.Status != "created" && order.Status != "paid" {
		return nil, paymeErr(-31008, "Невозможно выполнить операцию")
	}
	cancelledAt := time.Now()
	cancelTime := unixMilli(cancelledAt)
	state := -1
	nextStatus := "failed"
	if order.Status == "paid" {
		state = -2
		nextStatus = "refunded"
	}
	_, err = s.db.Exec(ctx, `
		UPDATE payment_orders
		SET status = $1, provider_status = 'cancelled', provider_payload = $2, updated_at = $4
		WHERE id = $3 AND status <> 'refunded'
	`, nextStatus, rawPayload, order.ID, cancelledAt)
	if err != nil {
		return nil, err
	}
	return map[string]any{"transaction": order.ID, "cancel_time": cancelTime, "state": state}, nil
}

func (s *Server) paymeCheckTransaction(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var params paymeTransactionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	order, err := s.providerOrderByPaymentID(ctx, payments.ProviderPayme, params.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, paymeErr(-31003, "Транзакция не найдена")
	}
	if err != nil {
		return nil, err
	}
	cancelTime := int64(0)
	if order.Status == "failed" || order.Status == "refunded" {
		cancelTime = unixMilli(order.UpdatedAt)
	}
	return map[string]any{
		"create_time":  order.ProviderTimeMS,
		"perform_time": unixMilli(order.PaidAt),
		"cancel_time":  cancelTime,
		"transaction":  order.ID,
		"state":        paymeState(order.Status),
		"reason":       nil,
	}, nil
}

func (s *Server) paymeGetStatement(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var params paymeStatementParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, paymeErr(-32700, "Неверные параметры")
	}
	if params.From <= 0 || params.To <= 0 || params.From > params.To {
		return nil, paymeErr(-32602, "Неверный период")
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, invoice_id, COALESCE(provider_payment_id, ''), COALESCE(provider_time_ms, 0),
		       amount_tiyin, status, paid_at, updated_at
		FROM payment_orders
		WHERE provider = 'payme'
		  AND provider_payment_id IS NOT NULL
		  AND provider_payment_id <> ''
		  AND provider_time_ms BETWEEN $1 AND $2
		ORDER BY provider_time_ms ASC
	`, params.From, params.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]map[string]any, 0)
	for rows.Next() {
		var id, invoiceID, providerPaymentID, status string
		var createTime, amountTiyin int64
		var paidAt *time.Time
		var updatedAt time.Time
		if err := rows.Scan(&id, &invoiceID, &providerPaymentID, &createTime, &amountTiyin, &status, &paidAt, &updatedAt); err != nil {
			return nil, err
		}
		cancelTime := int64(0)
		if status == "failed" || status == "refunded" {
			cancelTime = unixMilli(&updatedAt)
		}
		transactions = append(transactions, map[string]any{
			"id":           providerPaymentID,
			"time":         createTime,
			"amount":       amountTiyin,
			"account":      map[string]string{"order_id": invoiceID},
			"create_time":  createTime,
			"perform_time": unixMilli(paidAt),
			"cancel_time":  cancelTime,
			"transaction":  id,
			"state":        paymeState(status),
			"reason":       nil,
		})
	}
	return map[string]any{"transactions": transactions}, rows.Err()
}

func (s *Server) providerOrderByInvoice(ctx context.Context, invoiceID string) (providerOrder, error) {
	var order providerOrder
	err := s.db.QueryRow(ctx, `
		SELECT id, invoice_id, provider, COALESCE(provider_payment_id, ''), COALESCE(provider_prepare_id, ''),
		       COALESCE(provider_time_ms, 0), amount_tiyin, status, paid_at, updated_at
		FROM payment_orders
		WHERE invoice_id = $1
	`, invoiceID).Scan(&order.ID, &order.InvoiceID, &order.Provider, &order.ProviderPaymentID, &order.ProviderPrepareID, &order.ProviderTimeMS, &order.AmountTiyin, &order.Status, &order.PaidAt, &order.UpdatedAt)
	return order, err
}

func (s *Server) providerOrderByPaymentID(ctx context.Context, provider, paymentID string) (providerOrder, error) {
	var order providerOrder
	err := s.db.QueryRow(ctx, `
		SELECT id, invoice_id, provider, COALESCE(provider_payment_id, ''), COALESCE(provider_prepare_id, ''),
		       COALESCE(provider_time_ms, 0), amount_tiyin, status, paid_at, updated_at
		FROM payment_orders
		WHERE provider = $1 AND provider_payment_id = $2
	`, provider, paymentID).Scan(&order.ID, &order.InvoiceID, &order.Provider, &order.ProviderPaymentID, &order.ProviderPrepareID, &order.ProviderTimeMS, &order.AmountTiyin, &order.Status, &order.PaidAt, &order.UpdatedAt)
	return order, err
}

func (s *Server) paymeCredentialsForRequest(ctx context.Context, rpc paymeRPCRequest, orderRef string) (paymeAuthCredentials, error) {
	if orderRef != "" {
		var credentials paymeAuthCredentials
		err := s.db.QueryRow(ctx, `
			SELECT COALESCE(c.payme_merchant_id, ''), COALESCE(c.payme_secret_key, '')
			FROM payment_orders po
			JOIN clubs c ON c.id = po.club_id
			WHERE po.invoice_id = $1
		`, orderRef).Scan(&credentials.MerchantID, &credentials.SecretKey)
		if err == nil {
			credentials.MerchantID = defaultString(credentials.MerchantID, s.cfg.PaymeMerchantID)
			credentials.SecretKey = defaultString(credentials.SecretKey, s.cfg.PaymeSecretKey)
			return credentials, nil
		}
	}
	if rpc.Method == "PerformTransaction" || rpc.Method == "CancelTransaction" || rpc.Method == "CheckTransaction" {
		var params paymeTransactionParams
		if json.Unmarshal(rpc.Params, &params) == nil && params.ID != "" {
			var credentials paymeAuthCredentials
			err := s.db.QueryRow(ctx, `
				SELECT COALESCE(c.payme_merchant_id, ''), COALESCE(c.payme_secret_key, '')
				FROM payment_orders po
				JOIN clubs c ON c.id = po.club_id
				WHERE po.provider = 'payme' AND po.provider_payment_id = $1
			`, params.ID).Scan(&credentials.MerchantID, &credentials.SecretKey)
			if err == nil {
				credentials.MerchantID = defaultString(credentials.MerchantID, s.cfg.PaymeMerchantID)
				credentials.SecretKey = defaultString(credentials.SecretKey, s.cfg.PaymeSecretKey)
				return credentials, nil
			}
		}
	}
	return paymeAuthCredentials{MerchantID: s.cfg.PaymeMerchantID, SecretKey: s.cfg.PaymeSecretKey}, nil
}

func (s *Server) verifyPaymeAuth(r *http.Request, credentials paymeAuthCredentials) bool {
	secret := strings.TrimSpace(credentials.SecretKey)
	if secret == "" && !strings.EqualFold(s.cfg.AppEnv, "production") {
		return true
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "basic ") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[6:]))
	if err != nil {
		return false
	}
	login, password, ok := strings.Cut(string(decoded), ":")
	if !ok || strings.TrimSpace(login) == "" || password != secret {
		return false
	}
	merchantID := strings.TrimSpace(credentials.MerchantID)
	return strings.EqualFold(login, "Paycom") || merchantID == "" || login == merchantID
}

func (s *Server) handleCoreEvent(w http.ResponseWriter, r *http.Request) {
	if !s.requireCore(w, r) {
		return
	}
	var req coreEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.EventID == "" || req.EventType == "" {
		writeError(w, http.StatusBadRequest, "event_id and event_type are required")
		return
	}
	occurredAt := parseOptionalTime(req.OccurredAt)
	payload, _ := json.Marshal(req.Payload)
	var eventRowID string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO core_events (event_id, event_type, club_id, external_pc_id, core_session_id, grant_id, payload, occurred_at)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, '')::uuid, $7, $8)
		ON CONFLICT (event_id) DO NOTHING
		RETURNING id
	`, req.EventID, req.EventType, req.ClubID, req.ExternalPCID, req.CoreSessionID, req.GrantID, payload, occurredAt).Scan(&eventRowID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "duplicate": true})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := map[string]any{"success": true, "event_id": req.EventID}
	processStatus := "processed"
	switch req.EventType {
	case "pc_status_changed":
		status := stringFromPayload(req.Payload, "status")
		if req.ExternalPCID != "" && isKnownPCStatus(status) {
			_, err = s.db.Exec(r.Context(), `UPDATE pc_refs SET status_cache = $1 WHERE external_pc_id = $2`, status, req.ExternalPCID)
		}
	case "agent_offline", "controller_offline":
		if req.ExternalPCID != "" {
			_, err = s.db.Exec(r.Context(), `UPDATE pc_refs SET status_cache = 'offline' WHERE external_pc_id = $1`, req.ExternalPCID)
		}
	case "session_started":
		endsAt := parseOptionalTime(stringFromPayload(req.Payload, "ends_at"))
		_, err = s.db.Exec(r.Context(), `
			UPDATE game_access_grants
			SET status = 'accepted',
			    core_session_id = COALESCE(NULLIF($1, ''), core_session_id),
			    accepted_at = COALESCE($2, now()),
			    planned_ends_at = $3,
			    last_error = NULL
			WHERE id = NULLIF($4, '')::uuid
		`, req.CoreSessionID, occurredAt, endsAt, req.GrantID)
		if err == nil && req.ExternalPCID != "" {
			_, err = s.db.Exec(r.Context(), `UPDATE pc_refs SET status_cache = 'occupied' WHERE external_pc_id = $1`, req.ExternalPCID)
		}
	case "session_ended":
		grantID := req.GrantID
		if grantID == "" && req.CoreSessionID != "" {
			_ = s.db.QueryRow(r.Context(), `SELECT id FROM game_access_grants WHERE core_session_id = $1 ORDER BY created_at DESC LIMIT 1`, req.CoreSessionID).Scan(&grantID)
		}
		remainingSeconds := intFromPayload(req.Payload, "remaining_seconds")
		if remainingSeconds <= 0 {
			remainingSeconds = intFromPayload(req.Payload, "remaining_minutes") * 60
		}
		reason := defaultString(stringFromPayload(req.Payload, "reason"), "core_event")
		result, err = s.finishGrant(r.Context(), grantID, reason, remainingSeconds)
	case "session_failed", "command_failed":
		message := defaultString(stringFromPayload(req.Payload, "message"), "core command failed")
		_, err = s.db.Exec(r.Context(), `
			UPDATE game_access_grants
			SET status = 'start_failed', last_error = $1
			WHERE id = NULLIF($2, '')::uuid OR core_session_id = $3
		`, message, req.GrantID, req.CoreSessionID)
	default:
		processStatus = "ignored"
	}
	if err != nil {
		processStatus = "failed"
		_, _ = s.db.Exec(r.Context(), `UPDATE core_events SET status = $1, processed_at = now() WHERE id = $2`, processStatus, eventRowID)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.db.Exec(r.Context(), `UPDATE core_events SET status = $1, processed_at = now() WHERE id = $2`, processStatus, eventRowID)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleEdgeSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	clubID := strings.TrimSpace(r.URL.Query().Get("club_id"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "club_id is required")
		return
	}
	if err := s.expireElapsedGrants(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	snapshot, err := s.edgeSnapshotData(r.Context(), clubID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleEdgeEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req edgeEventBatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ClubID == "" {
		writeError(w, http.StatusBadRequest, "club_id is required")
		return
	}
	processed := 0
	for _, event := range req.Events {
		if event.EventID == "" || event.Type == "" {
			continue
		}
		payload, _ := json.Marshal(event.Payload)
		occurredAt := parseOptionalTime(event.OccurredAt)
		_, err := s.db.Exec(r.Context(), `
			INSERT INTO core_events (event_id, event_type, club_id, external_pc_id, core_session_id, grant_id, payload, occurred_at, status, processed_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7, $8, 'processed', now())
			ON CONFLICT (event_id) DO NOTHING
		`, event.EventID, "edge_"+event.Type, req.ClubID, event.ExternalPCID, event.CoreSessionID, event.GrantID, payload, occurredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		switch event.Type {
		case "edge_snapshot":
			if err := s.applyEdgeSnapshotData(r.Context(), req.ClubID, event.Payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "voucher_redeemed":
			if event.CodeHash != "" {
				_, _ = s.db.Exec(r.Context(), `
					UPDATE vouchers
					SET status = 'redeemed', redeemed_at = COALESCE($2, now())
					WHERE club_id = $1 AND code_hash = $3 AND status = 'active'
				`, req.ClubID, occurredAt, event.CodeHash)
			}
		case "pc_status_changed":
			status := stringFromPayload(event.Payload, "status")
			if event.ExternalPCID != "" && isKnownPCStatus(status) {
				_, _ = s.db.Exec(r.Context(), `UPDATE pc_refs SET status_cache = $1 WHERE club_id = $2 AND external_pc_id = $3`, status, req.ClubID, event.ExternalPCID)
			}
		}
		processed++
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "processed": processed})
}

func (s *Server) RunEdgeSync(ctx context.Context) {
	if !strings.EqualFold(s.cfg.NodeMode, "edge") {
		return
	}
	if strings.TrimSpace(s.cfg.CloudBaseURL) == "" || strings.TrimSpace(s.cfg.EdgeClubID) == "" {
		return
	}
	interval := time.Duration(s.cfg.EdgeSyncIntervalSeconds) * time.Second
	if interval < 5*time.Second {
		interval = 15 * time.Second
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_ = s.syncEdgeOnce(ctx)
			timer.Reset(interval)
		}
	}
}

func (s *Server) syncEdgeOnce(ctx context.Context) error {
	nodeID := s.edgeNodeID()
	clubID := strings.TrimSpace(s.cfg.EdgeClubID)
	runID, _ := s.startEdgeSyncRun(ctx, nodeID, clubID, "push")
	snapshot, err := s.edgeSnapshotData(ctx, clubID, true)
	if err != nil {
		s.finishEdgeSyncRun(ctx, runID, "failed", "", err)
		return err
	}
	eventID := fmt.Sprintf("%s_%s_%d", nodeID, clubID, time.Now().UnixNano())
	event := map[string]any{
		"club_id": clubID,
		"events": []map[string]any{{
			"event_id":    eventID,
			"type":        "edge_snapshot",
			"occurred_at": time.Now().UTC().Format(time.RFC3339),
			"payload":     snapshot,
		}},
	}
	if err := s.postCloudJSON(ctx, "/api/edge/events", event, nil); err != nil {
		s.finishEdgeSyncRun(ctx, runID, "failed", eventID, err)
		return err
	}
	s.finishEdgeSyncRun(ctx, runID, "success", eventID, nil)
	pullID, _ := s.startEdgeSyncRun(ctx, nodeID, clubID, "pull")
	var pulled map[string]any
	if err := s.getCloudJSON(ctx, "/api/edge/snapshot?club_id="+url.QueryEscape(clubID), &pulled); err != nil {
		s.finishEdgeSyncRun(ctx, pullID, "failed", "", err)
		return err
	}
	if err := s.applyEdgeSnapshotData(ctx, clubID, pulled); err != nil {
		s.finishEdgeSyncRun(ctx, pullID, "failed", "", err)
		return err
	}
	s.finishEdgeSyncRun(ctx, pullID, "success", "", nil)
	return nil
}

func (s *Server) postCloudJSON(ctx context.Context, path string, body any, result any) error {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.CloudBaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.EdgeSyncToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.EdgeSyncToken)
		req.Header.Set("X-Edge-Token", s.cfg.EdgeSyncToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloud sync failed: HTTP %d", resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (s *Server) getCloudJSON(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.CloudBaseURL+path, nil)
	if err != nil {
		return err
	}
	if s.cfg.EdgeSyncToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.EdgeSyncToken)
		req.Header.Set("X-Edge-Token", s.cfg.EdgeSyncToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloud pull failed: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

func (s *Server) startEdgeSyncRun(ctx context.Context, nodeID, clubID, direction string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO edge_sync_runs (node_id, club_id, direction)
		VALUES ($1, NULLIF($2, '')::uuid, $3)
		RETURNING id
	`, nodeID, clubID, direction).Scan(&id)
	return id, err
}

func (s *Server) finishEdgeSyncRun(ctx context.Context, runID, status, eventID string, syncErr error) {
	if runID == "" {
		return
	}
	errorText := ""
	if syncErr != nil {
		errorText = syncErr.Error()
	}
	_, _ = s.db.Exec(ctx, `
		UPDATE edge_sync_runs
		SET status = $2,
		    event_id = NULLIF($3, ''),
		    error = NULLIF($4, ''),
		    finished_at = now()
		WHERE id = $1
	`, runID, status, eventID, errorText)
}

func (s *Server) edgeNodeID() string {
	if strings.TrimSpace(s.cfg.EdgeNodeID) != "" {
		return strings.TrimSpace(s.cfg.EdgeNodeID)
	}
	if strings.TrimSpace(s.cfg.EdgeClubID) != "" {
		return "edge-" + strings.TrimSpace(s.cfg.EdgeClubID)
	}
	return "edge-local"
}

func (s *Server) edgeSnapshotData(ctx context.Context, clubID string, includeTechnical bool) (map[string]any, error) {
	clubRows, err := s.queryMaps(ctx, `
		SELECT id, name, COALESCE(slug, '') AS slug, COALESCE(legal_name, '') AS legal_name,
		       COALESCE(tin, '') AS tin, COALESCE(address, '') AS address,
		       timezone, status,
		       COALESCE(click_merchant_id, '') AS click_merchant_id,
		       COALESCE(click_service_id, '') AS click_service_id,
		       COALESCE(click_merchant_user_id, '') AS click_merchant_user_id,
		       COALESCE(click_secret_key, '') AS click_secret_key,
		       COALESCE(payme_merchant_id, '') AS payme_merchant_id,
		       COALESCE(payme_secret_key, '') AS payme_secret_key,
		       platform_fee_bps, COALESCE(ofd_mxik, '') AS ofd_mxik,
		       COALESCE(ofd_package_code, '') AS ofd_package_code,
		       created_at
		FROM clubs
		WHERE id = $1
	`, clubID)
	if err != nil {
		return nil, err
	}
	if len(clubRows) == 0 {
		return nil, pgx.ErrNoRows
	}
	club := clubRows[0]
	if !includeTechnical {
		for _, key := range []string{"click_secret_key", "payme_secret_key"} {
			club[key] = ""
		}
	}
	zones, err := s.queryMaps(ctx, `
		SELECT id, club_id, name, hourly_price_tiyin, sort_order, status, created_at
		FROM zones
		WHERE club_id = $1 AND status <> 'deleted'
		ORDER BY sort_order, name
	`, clubID)
	if err != nil {
		return nil, err
	}
	tariffs, err := s.queryMaps(ctx, `
		SELECT id, club_id, zone_id, name, duration_minutes, price_tiyin, status, sort_order, created_at
		FROM tariff_blocks
		WHERE club_id = $1 AND status <> 'deleted'
		ORDER BY sort_order, duration_minutes
	`, clubID)
	if err != nil {
		return nil, err
	}
	pcs, err := s.queryMaps(ctx, `
		SELECT id, club_id, zone_id, external_pc_id, number, label, status_cache, created_at
		FROM pc_refs
		WHERE club_id = $1 AND status_cache <> 'deleted'
		ORDER BY number
	`, clubID)
	if err != nil {
		return nil, err
	}
	qrs, err := s.queryMaps(ctx, `
		SELECT id, club_id, pc_ref_id, public_token, type, status, created_at
		FROM qr_codes
		WHERE club_id = $1 AND status <> 'deleted'
		ORDER BY created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	users, err := s.queryMaps(ctx, `
		SELECT DISTINCT u.id, u.club_id, u.name, COALESCE(u.email, '') AS email, COALESCE(u.phone, '') AS phone,
		       u.role, COALESCE(u.global_role, '') AS global_role, COALESCE(u.password_hash, '') AS password_hash,
		       u.status, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN user_club_roles ucr ON ucr.user_id = u.id
		WHERE u.club_id = $1 OR ucr.club_id = $1 OR u.global_role = 'super_admin'
		ORDER BY u.name
	`, clubID)
	if err != nil {
		return nil, err
	}
	roles, err := s.queryMaps(ctx, `
		SELECT user_id, club_id, role, status, created_at, updated_at
		FROM user_club_roles
		WHERE club_id = $1
	`, clubID)
	if err != nil {
		return nil, err
	}
	paymentOrders, err := s.queryMaps(ctx, `
		SELECT id, invoice_id, provider, COALESCE(provider_payment_id, '') AS provider_payment_id,
		       COALESCE(provider_prepare_id, '') AS provider_prepare_id,
		       COALESCE(provider_time_ms, 0) AS provider_time_ms,
		       provider_payload, club_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds,
		       status, COALESCE(provider_status, '') AS provider_status, COALESCE(checkout_url, '') AS checkout_url,
		       COALESCE(receipt_url, '') AS receipt_url, receipt_kind, fiscal_status,
		       split_platform_amount_tiyin, split_club_amount_tiyin, expires_at, paid_at, extension_grant_id, voucher_id,
		       created_at, updated_at
		FROM payment_orders
		WHERE club_id = $1
		ORDER BY created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	paymentsRows, err := s.queryMaps(ctx, `
		SELECT p.id, p.payment_order_id, p.provider, p.provider_payment_id, p.amount_tiyin, p.status,
		       COALESCE(p.ps, '') AS ps, COALESCE(p.card_pan, '') AS card_pan,
		       COALESCE(p.receipt_url, '') AS receipt_url, p.paid_at, p.raw_payload, p.created_at
		FROM payments p
		JOIN payment_orders po ON po.id = p.payment_order_id
		WHERE po.club_id = $1
		ORDER BY p.created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	cashPayments, err := s.queryMaps(ctx, `
		SELECT id, club_id, admin_user_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds,
		       reason, COALESCE(fiscal_reference, '') AS fiscal_reference, created_at
		FROM cash_payments
		WHERE club_id = $1
		ORDER BY created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	grants, err := s.queryMaps(ctx, `
		SELECT id, club_id, pc_ref_id, payment_order_id, cash_payment_id, duration_minutes, duration_seconds, status,
		       COALESCE(core_session_id, '') AS core_session_id, voucher_id, returned_voucher_id, source, accepted_at,
		       planned_ends_at, ended_at, COALESCE(end_reason, '') AS end_reason,
		       remaining_minutes, remaining_seconds, COALESCE(last_error, '') AS last_error, created_at
		FROM game_access_grants
		WHERE club_id = $1
		ORDER BY created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	vouchers, err := s.queryMaps(ctx, `
		SELECT id, club_id, original_payment_order_id, minutes_left, seconds_left, code_hash, status, expires_at,
		       redeemed_grant_id, redeemed_at, recipient_phone, delivery_channel, delivery_status,
		       delivered_at, public_code, created_at
		FROM vouchers
		WHERE club_id = $1
		ORDER BY created_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	telegramUsers, err := s.queryMaps(ctx, `
		SELECT id, phone, chat_id, COALESCE(username, '') AS username, COALESCE(first_name, '') AS first_name,
		       status, created_at, updated_at
		FROM telegram_users
		WHERE phone IN (SELECT DISTINCT recipient_phone FROM vouchers WHERE club_id = $1 AND recipient_phone IS NOT NULL)
		ORDER BY updated_at
	`, clubID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"node_id":               s.edgeNodeID(),
		"club_id":               clubID,
		"server_time":           time.Now().UTC(),
		"sync_interval_seconds": s.cfg.EdgeSyncIntervalSeconds,
		"club":                  club,
		"zones":                 zones,
		"tariffs":               tariffs,
		"pcs":                   pcs,
		"qr_codes":              qrs,
		"users":                 users,
		"user_club_roles":       roles,
		"payment_orders":        paymentOrders,
		"payments":              paymentsRows,
		"cash_payments":         cashPayments,
		"game_access_grants":    grants,
		"vouchers":              vouchers,
		"telegram_users":        telegramUsers,
	}, nil
}

func (s *Server) queryMaps(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	result := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(values))
		for index, field := range fields {
			row[string(field.Name)] = snapshotValue(field.DataTypeOID, values[index])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func snapshotValue(typeOID uint32, value any) any {
	if value == nil {
		return nil
	}
	if uuid, ok := value.([16]byte); ok {
		return uuidBytesToString(uuid[:])
	}
	bytes, ok := value.([]byte)
	if !ok {
		return value
	}
	switch typeOID {
	case 2950:
		return uuidBytesToString(bytes)
	case 114, 3802:
		var decoded any
		if json.Unmarshal(bytes, &decoded) == nil {
			return decoded
		}
		return string(bytes)
	default:
		return value
	}
}

func uuidBytesToString(bytes []byte) string {
	if len(bytes) != 16 {
		return string(bytes)
	}
	encoded := hex.EncodeToString(bytes)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func (s *Server) applyEdgeSnapshotData(ctx context.Context, clubID string, payload map[string]any) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if club := payload["club"]; club != nil {
		if err := execJSON(ctx, tx, `
			WITH input AS (
				SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
					id uuid, name text, slug text, legal_name text, tin text, address text,
					timezone text, status text, click_merchant_id text, click_service_id text,
					click_merchant_user_id text, click_secret_key text, payme_merchant_id text, payme_secret_key text,
					platform_fee_bps int, ofd_mxik text, ofd_package_code text, created_at timestamptz
				)
			)
			INSERT INTO clubs (id, name, slug, legal_name, tin, address, timezone, status, click_merchant_id, click_service_id, click_merchant_user_id, click_secret_key, payme_merchant_id, payme_secret_key, platform_fee_bps, ofd_mxik, ofd_package_code, created_at)
			SELECT id, name, slug, legal_name, tin, address, COALESCE(NULLIF(timezone, ''), 'Asia/Tashkent'), COALESCE(NULLIF(status, ''), 'active'), click_merchant_id, click_service_id, click_merchant_user_id, click_secret_key, payme_merchant_id, payme_secret_key, COALESCE(platform_fee_bps, 0), ofd_mxik, ofd_package_code, COALESCE(created_at, now())
			FROM input
			ON CONFLICT (id) DO UPDATE SET
			  name = EXCLUDED.name, slug = EXCLUDED.slug, legal_name = EXCLUDED.legal_name, tin = EXCLUDED.tin,
			  address = EXCLUDED.address, timezone = EXCLUDED.timezone, status = EXCLUDED.status,
			  click_merchant_id = EXCLUDED.click_merchant_id, click_service_id = EXCLUDED.click_service_id,
			  click_merchant_user_id = EXCLUDED.click_merchant_user_id,
			  click_secret_key = EXCLUDED.click_secret_key, payme_merchant_id = EXCLUDED.payme_merchant_id,
			  payme_secret_key = EXCLUDED.payme_secret_key, platform_fee_bps = EXCLUDED.platform_fee_bps,
			  ofd_mxik = EXCLUDED.ofd_mxik, ofd_package_code = EXCLUDED.ofd_package_code
		`, []any{club}); err != nil {
			return err
		}
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
				id uuid, club_id uuid, name text, hourly_price_tiyin bigint, sort_order int, status text, created_at timestamptz
			)
		)
		INSERT INTO zones (id, club_id, name, hourly_price_tiyin, sort_order, status, created_at)
		SELECT id, club_id, name, COALESCE(hourly_price_tiyin, 1500000), COALESCE(sort_order, 0), COALESCE(NULLIF(status, ''), 'active'), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, hourly_price_tiyin = EXCLUDED.hourly_price_tiyin, sort_order = EXCLUDED.sort_order, status = EXCLUDED.status
	`, payload["zones"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
				id uuid, club_id uuid, name text, email text, phone text, role text, global_role text, password_hash text, status text, created_at timestamptz, updated_at timestamptz
			)
		)
		INSERT INTO users (id, club_id, name, email, phone, role, global_role, password_hash, status, created_at, updated_at)
		SELECT id, club_id, name, NULLIF(email, ''), NULLIF(phone, ''), COALESCE(NULLIF(role, ''), 'admin'), COALESCE(global_role, ''), NULLIF(password_hash, ''), COALESCE(NULLIF(status, ''), 'active'), COALESCE(created_at, now()), COALESCE(updated_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET club_id = EXCLUDED.club_id, name = EXCLUDED.name, email = EXCLUDED.email, phone = EXCLUDED.phone, role = EXCLUDED.role, global_role = EXCLUDED.global_role, password_hash = COALESCE(EXCLUDED.password_hash, users.password_hash), status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
	`, payload["users"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(user_id uuid, club_id uuid, role text, status text, created_at timestamptz, updated_at timestamptz)
		)
		INSERT INTO user_club_roles (user_id, club_id, role, status, created_at, updated_at)
		SELECT user_id, club_id, COALESCE(NULLIF(role, ''), 'admin'), COALESCE(NULLIF(status, ''), 'active'), COALESCE(created_at, now()), COALESCE(updated_at, now())
		FROM input
		ON CONFLICT (user_id, club_id) DO UPDATE SET role = EXCLUDED.role, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
	`, payload["user_club_roles"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
				id uuid, club_id uuid, zone_id uuid, external_pc_id text, number int, label text, status_cache text, created_at timestamptz
			)
		)
		INSERT INTO pc_refs (id, club_id, zone_id, external_pc_id, number, label, status_cache, created_at)
		SELECT id, club_id, zone_id, external_pc_id, number, label, COALESCE(NULLIF(status_cache, ''), 'available'), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET zone_id = EXCLUDED.zone_id, external_pc_id = EXCLUDED.external_pc_id, number = EXCLUDED.number, label = EXCLUDED.label, status_cache = EXCLUDED.status_cache
	`, payload["pcs"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
				id uuid, club_id uuid, zone_id uuid, name text, duration_minutes int, price_tiyin bigint, status text, sort_order int, created_at timestamptz
			)
		)
		INSERT INTO tariff_blocks (id, club_id, zone_id, name, duration_minutes, price_tiyin, status, sort_order, created_at)
		SELECT id, club_id, zone_id, name, duration_minutes, price_tiyin, COALESCE(NULLIF(status, ''), 'active'), COALESCE(sort_order, 0), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET zone_id = EXCLUDED.zone_id, name = EXCLUDED.name, duration_minutes = EXCLUDED.duration_minutes, price_tiyin = EXCLUDED.price_tiyin, status = EXCLUDED.status, sort_order = EXCLUDED.sort_order
	`, payload["tariffs"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, club_id uuid, pc_ref_id uuid, public_token text, type text, status text, created_at timestamptz)
		)
		INSERT INTO qr_codes (id, club_id, pc_ref_id, public_token, type, status, created_at)
		SELECT id, club_id, pc_ref_id, public_token, COALESCE(NULLIF(type, ''), 'static_pc'), COALESCE(NULLIF(status, ''), 'active'), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (public_token) DO UPDATE SET club_id = EXCLUDED.club_id, pc_ref_id = EXCLUDED.pc_ref_id, type = EXCLUDED.type, status = EXCLUDED.status
	`, payload["qr_codes"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(
				id uuid, invoice_id text, provider text, provider_payment_id text, provider_prepare_id text, provider_time_ms bigint,
				provider_payload jsonb, club_id uuid, pc_ref_id uuid, tariff_block_id uuid, amount_tiyin bigint,
				duration_minutes int, duration_seconds int, status text, provider_status text, checkout_url text, receipt_url text,
				receipt_kind text, fiscal_status text, split_platform_amount_tiyin bigint, split_club_amount_tiyin bigint,
				expires_at timestamptz, paid_at timestamptz, extension_grant_id uuid, voucher_id uuid, created_at timestamptz, updated_at timestamptz
			)
		)
		INSERT INTO payment_orders (id, invoice_id, provider, provider_payment_id, provider_prepare_id, provider_time_ms, provider_payload, club_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds, status, provider_status, checkout_url, receipt_url, receipt_kind, fiscal_status, split_platform_amount_tiyin, split_club_amount_tiyin, expires_at, paid_at, extension_grant_id, voucher_id, created_at, updated_at)
		SELECT id, invoice_id, COALESCE(NULLIF(provider, ''), 'mock'), NULLIF(provider_payment_id, ''), NULLIF(provider_prepare_id, ''), provider_time_ms, COALESCE(provider_payload, '{}'::jsonb), club_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, COALESCE(NULLIF(duration_seconds, 0), duration_minutes * 60), COALESCE(NULLIF(status, ''), 'created'), NULLIF(provider_status, ''), NULLIF(checkout_url, ''), NULLIF(receipt_url, ''), COALESCE(NULLIF(receipt_kind, ''), 'provider_receipt'), COALESCE(NULLIF(fiscal_status, ''), 'not_requested'), COALESCE(split_platform_amount_tiyin, 0), COALESCE(split_club_amount_tiyin, 0), expires_at, paid_at, extension_grant_id, voucher_id, COALESCE(created_at, now()), COALESCE(updated_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET provider_payment_id = EXCLUDED.provider_payment_id, provider_prepare_id = EXCLUDED.provider_prepare_id, provider_time_ms = EXCLUDED.provider_time_ms, provider_payload = EXCLUDED.provider_payload, amount_tiyin = EXCLUDED.amount_tiyin, duration_minutes = EXCLUDED.duration_minutes, duration_seconds = EXCLUDED.duration_seconds, status = EXCLUDED.status, provider_status = EXCLUDED.provider_status, checkout_url = EXCLUDED.checkout_url, receipt_url = EXCLUDED.receipt_url, receipt_kind = EXCLUDED.receipt_kind, fiscal_status = EXCLUDED.fiscal_status, split_platform_amount_tiyin = EXCLUDED.split_platform_amount_tiyin, split_club_amount_tiyin = EXCLUDED.split_club_amount_tiyin, expires_at = EXCLUDED.expires_at, paid_at = EXCLUDED.paid_at, extension_grant_id = EXCLUDED.extension_grant_id, voucher_id = EXCLUDED.voucher_id, updated_at = EXCLUDED.updated_at
	`, payload["payment_orders"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, club_id uuid, admin_user_id uuid, pc_ref_id uuid, tariff_block_id uuid, amount_tiyin bigint, duration_minutes int, duration_seconds int, reason text, fiscal_reference text, created_at timestamptz)
		)
		INSERT INTO cash_payments (id, club_id, admin_user_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds, reason, fiscal_reference, created_at)
		SELECT id, club_id, admin_user_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, COALESCE(NULLIF(duration_seconds, 0), duration_minutes * 60), COALESCE(NULLIF(reason, ''), 'cash'), NULLIF(fiscal_reference, ''), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET amount_tiyin = EXCLUDED.amount_tiyin, duration_minutes = EXCLUDED.duration_minutes, duration_seconds = EXCLUDED.duration_seconds, reason = EXCLUDED.reason, fiscal_reference = EXCLUDED.fiscal_reference
	`, payload["cash_payments"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, club_id uuid, original_payment_order_id uuid, minutes_left int, seconds_left int, code_hash text, status text, expires_at timestamptz, redeemed_grant_id uuid, redeemed_at timestamptz, recipient_phone text, delivery_channel text, delivery_status text, delivered_at timestamptz, public_code text, created_at timestamptz)
		)
		INSERT INTO vouchers (id, club_id, original_payment_order_id, minutes_left, seconds_left, code_hash, status, expires_at, redeemed_grant_id, redeemed_at, recipient_phone, delivery_channel, delivery_status, delivered_at, public_code, created_at)
		SELECT id, club_id, original_payment_order_id, minutes_left, COALESCE(NULLIF(seconds_left, 0), minutes_left * 60), code_hash, COALESCE(NULLIF(status, ''), 'active'), expires_at, redeemed_grant_id, redeemed_at, NULLIF(recipient_phone, ''), NULLIF(delivery_channel, ''), COALESCE(NULLIF(delivery_status, ''), 'not_requested'), delivered_at, NULLIF(public_code, ''), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (code_hash) DO UPDATE SET minutes_left = EXCLUDED.minutes_left, seconds_left = EXCLUDED.seconds_left, status = EXCLUDED.status, redeemed_grant_id = EXCLUDED.redeemed_grant_id, redeemed_at = EXCLUDED.redeemed_at, recipient_phone = EXCLUDED.recipient_phone, delivery_channel = EXCLUDED.delivery_channel, delivery_status = EXCLUDED.delivery_status, delivered_at = EXCLUDED.delivered_at, public_code = EXCLUDED.public_code
	`, payload["vouchers"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, club_id uuid, pc_ref_id uuid, payment_order_id uuid, cash_payment_id uuid, duration_minutes int, duration_seconds int, status text, core_session_id text, voucher_id uuid, returned_voucher_id uuid, source text, accepted_at timestamptz, planned_ends_at timestamptz, ended_at timestamptz, end_reason text, remaining_minutes int, remaining_seconds int, last_error text, created_at timestamptz)
		)
		INSERT INTO game_access_grants (id, club_id, pc_ref_id, payment_order_id, cash_payment_id, duration_minutes, duration_seconds, status, core_session_id, voucher_id, returned_voucher_id, source, accepted_at, planned_ends_at, ended_at, end_reason, remaining_minutes, remaining_seconds, last_error, created_at)
		SELECT id, club_id, pc_ref_id, payment_order_id, cash_payment_id, duration_minutes, COALESCE(NULLIF(duration_seconds, 0), duration_minutes * 60), COALESCE(NULLIF(status, ''), 'pending'), NULLIF(core_session_id, ''), voucher_id, returned_voucher_id, COALESCE(NULLIF(source, ''), 'online_payment'), accepted_at, planned_ends_at, ended_at, NULLIF(end_reason, ''), COALESCE(remaining_minutes, 0), COALESCE(NULLIF(remaining_seconds, 0), remaining_minutes * 60, 0), NULLIF(last_error, ''), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (id) DO UPDATE SET duration_minutes = EXCLUDED.duration_minutes, duration_seconds = EXCLUDED.duration_seconds, status = EXCLUDED.status, core_session_id = EXCLUDED.core_session_id, voucher_id = EXCLUDED.voucher_id, returned_voucher_id = EXCLUDED.returned_voucher_id, accepted_at = EXCLUDED.accepted_at, planned_ends_at = EXCLUDED.planned_ends_at, ended_at = EXCLUDED.ended_at, end_reason = EXCLUDED.end_reason, remaining_minutes = EXCLUDED.remaining_minutes, remaining_seconds = EXCLUDED.remaining_seconds, last_error = EXCLUDED.last_error
	`, payload["game_access_grants"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, payment_order_id uuid, provider text, provider_payment_id text, amount_tiyin bigint, status text, ps text, card_pan text, receipt_url text, paid_at timestamptz, raw_payload jsonb, created_at timestamptz)
		)
		INSERT INTO payments (id, payment_order_id, provider, provider_payment_id, amount_tiyin, status, ps, card_pan, receipt_url, paid_at, raw_payload, created_at)
		SELECT id, payment_order_id, COALESCE(NULLIF(provider, ''), 'mock'), provider_payment_id, amount_tiyin, status, NULLIF(ps, ''), NULLIF(card_pan, ''), NULLIF(receipt_url, ''), paid_at, COALESCE(raw_payload, '{}'::jsonb), COALESCE(created_at, now())
		FROM input
		ON CONFLICT (provider, provider_payment_id) DO UPDATE SET status = EXCLUDED.status, receipt_url = EXCLUDED.receipt_url, paid_at = EXCLUDED.paid_at, raw_payload = EXCLUDED.raw_payload
	`, payload["payments"]); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
		WITH input AS (
			SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid, phone text, chat_id text, username text, first_name text, status text, created_at timestamptz, updated_at timestamptz)
		)
		INSERT INTO telegram_users (id, phone, chat_id, username, first_name, status, created_at, updated_at)
		SELECT id, phone, chat_id, NULLIF(username, ''), NULLIF(first_name, ''), COALESCE(NULLIF(status, ''), 'active'), COALESCE(created_at, now()), COALESCE(updated_at, now())
		FROM input
		ON CONFLICT (phone) DO UPDATE SET chat_id = EXCLUDED.chat_id, username = EXCLUDED.username, first_name = EXCLUDED.first_name, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
	`, payload["telegram_users"]); err != nil {
		return err
	}
	_, _ = tx.Exec(ctx, `
		UPDATE pc_refs p
		SET status_cache = 'available'
		WHERE p.club_id = $1
		  AND p.status_cache = 'occupied'
		  AND NOT EXISTS (
			SELECT 1 FROM game_access_grants g
			WHERE g.pc_ref_id = p.id AND g.status = 'accepted'
		  )
	`, clubID)
	return tx.Commit(ctx)
}

func execJSON(ctx context.Context, tx pgx.Tx, sql string, value any) error {
	payload, err := json.Marshal(jsonArrayValue(value))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql, payload)
	return err
}

func jsonArrayValue(value any) any {
	if value == nil {
		return []any{}
	}
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		return typed
	case map[string]any:
		return []map[string]any{typed}
	default:
		return value
	}
}

func (s *Server) applyPaymentSuccess(ctx context.Context, success paymentSuccess, rawPayload []byte) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var order paymentOrderForCallback
	err = tx.QueryRow(ctx, `
		SELECT po.id, po.club_id, po.pc_ref_id, po.amount_tiyin, po.duration_minutes, po.duration_seconds, po.status, p.external_pc_id,
		       po.extension_grant_id, po.voucher_id
		FROM payment_orders po
		JOIN pc_refs p ON p.id = po.pc_ref_id
		WHERE po.invoice_id = $1
		FOR UPDATE
	`, success.InvoiceID).Scan(&order.ID, &order.ClubID, &order.PCID, &order.AmountTiyin, &order.DurationMinutes, &order.DurationSeconds, &order.Status, &order.ExternalPCID, &order.ExtensionGrantID, &order.VoucherID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("invoice not found")
	}
	if err != nil {
		return "", err
	}
	if order.AmountTiyin != success.AmountTiyin {
		return "", fmt.Errorf("amount mismatch")
	}
	if order.Status == "paid" && order.ExtensionGrantID != nil && *order.ExtensionGrantID != "" {
		return *order.ExtensionGrantID, tx.Commit(ctx)
	}
	if order.DurationSeconds <= 0 {
		order.DurationSeconds = order.DurationMinutes * 60
	}

	var existingGrantID string
	var existingGrantStatus string
	var existingCoreSessionID *string
	err = tx.QueryRow(ctx, `
		SELECT id, status, core_session_id
		FROM game_access_grants
		WHERE payment_order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, order.ID).Scan(&existingGrantID, &existingGrantStatus, &existingCoreSessionID)
	if err == nil && existingGrantID != "" && existingGrantStatus == "accepted" {
		return existingGrantID, tx.Commit(ctx)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		UPDATE payment_orders
		SET provider = $1,
		    provider_payment_id = COALESCE(NULLIF(provider_payment_id, ''), $2),
		    provider_status = 'success',
		    status = 'paid',
		    receipt_url = NULLIF($3, ''),
		    receipt_kind = 'provider_receipt',
		    fiscal_status = 'pending',
		    paid_at = $4,
		    updated_at = now()
		WHERE id = $5
	`, success.Provider, success.ProviderPaymentID, success.ReceiptURL, success.PaidAt, order.ID)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO payments (payment_order_id, provider, provider_payment_id, amount_tiyin, status, ps, card_pan, receipt_url, paid_at, raw_payload)
		VALUES ($1, $2, $3, $4, 'success', $5, $6, NULLIF($7, ''), $8, $9)
		ON CONFLICT (provider, provider_payment_id) DO NOTHING
	`, order.ID, success.Provider, success.ProviderPaymentID, success.AmountTiyin, success.PS, success.CardPAN, success.ReceiptURL, success.PaidAt, rawPayload)
	if err != nil {
		return "", err
	}

	if order.VoucherID != nil && *order.VoucherID != "" && order.Status != "paid" {
		var voucherMinutes, voucherSecondsValue int
		err = tx.QueryRow(ctx, `
			UPDATE vouchers
			SET status = 'redeemed', redeemed_at = now()
			WHERE id = $1 AND status = 'active' AND expires_at > now()
			RETURNING minutes_left, seconds_left
		`, *order.VoucherID).Scan(&voucherMinutes, &voucherSecondsValue)
		if err == nil {
			order.DurationSeconds += voucherSeconds(voucherSecondsValue, voucherMinutes)
			order.DurationMinutes = secondsToMinutesCeil(order.DurationSeconds)
			_, err = tx.Exec(ctx, `
				UPDATE payment_orders
				SET duration_seconds = $2, duration_minutes = $3, updated_at = now()
				WHERE id = $1
			`, order.ID, order.DurationSeconds, order.DurationMinutes)
			if err != nil {
				return "", err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	if order.ExtensionGrantID != nil && *order.ExtensionGrantID != "" {
		extensionGrant, ok, err := grantByID(ctx, tx, *order.ExtensionGrantID)
		if err != nil {
			return "", err
		}
		if !ok || extensionGrant.CoreSessionID == "" {
			return "", fmt.Errorf("active session for extension not found")
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		if order.VoucherID != nil && *order.VoucherID != "" {
			_, _ = s.db.Exec(ctx, `
				UPDATE vouchers
				SET redeemed_grant_id = $1
				WHERE id = $2 AND redeemed_grant_id IS NULL
			`, extensionGrant.ID, *order.VoucherID)
		}
		err = s.extendGrantSession(ctx, extensionGrant.ID, extensionGrant.CoreSessionID, order.ClubID, order.PCID, order.ExternalPCID, order.DurationSeconds, "online_payment", order.ID, success.InvoiceID)
		if err != nil {
			return "", err
		}
		return extensionGrant.ID, nil
	}

	var grantID string
	var orderVoucherID any
	if order.VoucherID != nil && *order.VoucherID != "" {
		orderVoucherID = *order.VoucherID
	}
	if existingGrantID != "" {
		grantID = existingGrantID
		_, err = tx.Exec(ctx, `
			UPDATE game_access_grants
			SET status = 'pending', duration_minutes = $2, duration_seconds = $3, voucher_id = $4, last_error = NULL
			WHERE id = $1
		`, grantID, order.DurationMinutes, order.DurationSeconds, orderVoucherID)
		if err != nil {
			return "", err
		}
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO game_access_grants (club_id, pc_ref_id, payment_order_id, voucher_id, duration_minutes, duration_seconds, status, source)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', 'online_payment')
			RETURNING id
		`, order.ClubID, order.PCID, order.ID, orderVoucherID, order.DurationMinutes, order.DurationSeconds).Scan(&grantID)
		if err != nil {
			return "", err
		}
	}
	if orderVoucherID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE vouchers
			SET redeemed_grant_id = $1
			WHERE id = $2 AND redeemed_grant_id IS NULL
		`, grantID, orderVoucherID)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	startResult, err := s.core.StartSession(ctx, core.StartSessionCommand{
		RequestID:       "start_" + grantID,
		GrantID:         grantID,
		ClubID:          order.ClubID,
		PCID:            order.PCID,
		PCExternalID:    order.ExternalPCID,
		DurationSeconds: order.DurationSeconds,
		DurationMinutes: order.DurationMinutes,
		Source:          "online_payment",
		PaymentOrderID:  order.ID,
		InvoiceID:       success.InvoiceID,
		ExtendURL:       s.cfg.FrontendBaseURL + "/qr/" + order.ExternalPCID,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		_, _ = s.db.Exec(ctx, `
			UPDATE game_access_grants
			SET status = 'start_failed', last_error = $1
			WHERE id = $2
		`, err.Error(), grantID)
		return "", err
	}
	coreSessionID := startResult.CoreSessionID
	if coreSessionID == "" && existingCoreSessionID != nil {
		coreSessionID = *existingCoreSessionID
	}
	if coreSessionID == "" {
		coreSessionID = "core-session-" + grantID
	}
	plannedEndsAt := startResult.EndsAt
	if plannedEndsAt == nil {
		endsAt := time.Now().UTC().Add(time.Duration(order.DurationSeconds) * time.Second)
		plannedEndsAt = &endsAt
	}

	_, err = s.db.Exec(ctx, `
		UPDATE game_access_grants
		SET status = 'accepted', core_session_id = $1, accepted_at = now(), planned_ends_at = $2, last_error = NULL
		WHERE id = $3
	`, coreSessionID, plannedEndsAt, grantID)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(ctx, `
		UPDATE pc_refs
		SET status_cache = 'occupied'
		WHERE id = $1
	`, order.PCID)
	if err != nil {
		return "", err
	}
	return grantID, nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type activeGrantRow struct {
	ID              string
	CoreSessionID   string
	PlannedEndsAt   *time.Time
	DurationMinutes int
	DurationSeconds int
}

func activeGrantForPC(ctx context.Context, q queryRower, pcID string) (activeGrantRow, bool, error) {
	var grant activeGrantRow
	err := q.QueryRow(ctx, `
		SELECT id, COALESCE(core_session_id, ''), planned_ends_at, duration_minutes, duration_seconds
		FROM game_access_grants
		WHERE pc_ref_id = $1
		  AND status = 'accepted'
		  AND COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes), now() + interval '1 minute') > now()
		ORDER BY accepted_at DESC NULLS LAST, created_at DESC
		LIMIT 1
	`, pcID).Scan(&grant.ID, &grant.CoreSessionID, &grant.PlannedEndsAt, &grant.DurationMinutes, &grant.DurationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return activeGrantRow{}, false, nil
	}
	if err != nil {
		return activeGrantRow{}, false, err
	}
	return grant, true, nil
}

func grantByID(ctx context.Context, q queryRower, grantID string) (activeGrantRow, bool, error) {
	var grant activeGrantRow
	err := q.QueryRow(ctx, `
		SELECT id, COALESCE(core_session_id, ''), planned_ends_at, duration_minutes, duration_seconds
		FROM game_access_grants
		WHERE id = $1 AND status = 'accepted'
		FOR UPDATE
	`, grantID).Scan(&grant.ID, &grant.CoreSessionID, &grant.PlannedEndsAt, &grant.DurationMinutes, &grant.DurationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return activeGrantRow{}, false, nil
	}
	if err != nil {
		return activeGrantRow{}, false, err
	}
	return grant, true, nil
}

func (s *Server) extendGrantSession(ctx context.Context, grantID, coreSessionID, clubID, pcID, externalPCID string, addSeconds int, source, paymentOrderID, invoiceID string) error {
	if addSeconds <= 0 {
		return fmt.Errorf("extension duration is required")
	}
	if coreSessionID == "" {
		return fmt.Errorf("core_session_id is required for extension")
	}
	_, err := s.core.ExtendSession(ctx, coreSessionID, core.ExtendSessionCommand{
		RequestID:      "extend_" + grantID + "_" + randomHex(4),
		GrantID:        grantID,
		ClubID:         clubID,
		ExternalPCID:   externalPCID,
		AddSeconds:     addSeconds,
		Source:         source,
		PaymentOrderID: paymentOrderID,
		InvoiceID:      invoiceID,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		_, _ = s.db.Exec(ctx, `
			UPDATE game_access_grants
			SET last_error = $1
			WHERE id = $2
		`, err.Error(), grantID)
		return err
	}
	_, err = s.db.Exec(ctx, `
		UPDATE game_access_grants
		SET duration_seconds = duration_seconds + $1,
		    duration_minutes = CEIL((duration_seconds + $1) / 60.0)::int,
		    planned_ends_at = GREATEST(
		      COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes), now()),
		      now()
		    ) + make_interval(secs => $1),
		    last_error = NULL
		WHERE id = $2
	`, addSeconds, grantID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE pc_refs SET status_cache = 'occupied' WHERE id = $1`, pcID)
	return err
}

func (s *Server) handleAdminPCs(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID, ok := s.resolveRequestClub(w, r, auth, "owner", "manager", "admin")
	if !ok {
		return
	}
	if err := s.expireElapsedGrants(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT p.id, p.external_pc_id, p.number, p.label,
		       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END AS display_status,
		       z.id, z.name,
		       CASE WHEN p.status_cache = 'occupied' AND z.status <> 'maintenance' THEN g.id ELSE NULL END,
		       CASE
		         WHEN p.status_cache = 'occupied' AND z.status <> 'maintenance' THEN COALESCE(GREATEST(FLOOR(EXTRACT(EPOCH FROM (g.effective_ends_at - now())))::int, 0), 0)
		         ELSE 0
		       END
		FROM pc_refs p
		JOIN zones z ON z.id = p.zone_id
		LEFT JOIN LATERAL (
			SELECT id, COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes)) AS effective_ends_at
			FROM game_access_grants
			WHERE pc_ref_id = p.id AND status = 'accepted'
			ORDER BY accepted_at DESC NULLS LAST, created_at DESC
			LIMIT 1
		) g ON true
		WHERE p.club_id = $1 AND p.status_cache <> 'deleted' AND z.status <> 'deleted'
		ORDER BY p.number
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	pcs := make([]map[string]any, 0)
	for rows.Next() {
		var id, externalID, label, status, zoneID, zone string
		var activeGrantID *string
		var number int
		var remainingSeconds int
		if err := rows.Scan(&id, &externalID, &number, &label, &status, &zoneID, &zone, &activeGrantID, &remainingSeconds); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pcs = append(pcs, map[string]any{
			"id": id, "external_pc_id": externalID, "number": number, "label": label, "status": status, "zone_id": zoneID, "zone": zone,
			"active_grant_id": activeGrantID, "remaining_seconds": remainingSeconds,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pcs": pcs})
}

func (s *Server) handleAdminCatalog(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID, ok := s.resolveRequestClub(w, r, auth, "owner", "manager", "admin")
	if !ok {
		return
	}
	if err := s.expireElapsedGrants(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pcRows, err := s.db.Query(r.Context(), `
		SELECT p.id, p.external_pc_id, p.number, p.label,
		       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END AS display_status,
		       z.id, z.name,
		       CASE WHEN p.status_cache = 'occupied' AND z.status <> 'maintenance' THEN g.id ELSE NULL END,
		       CASE
		         WHEN p.status_cache = 'occupied' AND z.status <> 'maintenance' THEN COALESCE(GREATEST(FLOOR(EXTRACT(EPOCH FROM (g.effective_ends_at - now())))::int, 0), 0)
		         ELSE 0
		       END
		FROM pc_refs p
		JOIN zones z ON z.id = p.zone_id
		LEFT JOIN LATERAL (
			SELECT id, COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes)) AS effective_ends_at
			FROM game_access_grants
			WHERE pc_ref_id = p.id AND status = 'accepted'
			ORDER BY accepted_at DESC NULLS LAST, created_at DESC
			LIMIT 1
		) g ON true
		WHERE p.club_id = $1 AND p.status_cache <> 'deleted' AND z.status <> 'deleted'
		ORDER BY p.number
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer pcRows.Close()

	pcs := make([]map[string]any, 0)
	for pcRows.Next() {
		var id, externalID, label, status, zoneID, zone string
		var activeGrantID *string
		var number int
		var remainingSeconds int
		if err := pcRows.Scan(&id, &externalID, &number, &label, &status, &zoneID, &zone, &activeGrantID, &remainingSeconds); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pcs = append(pcs, map[string]any{
			"id": id, "external_pc_id": externalID, "number": number, "label": label, "status": status, "zone_id": zoneID, "zone": zone,
			"active_grant_id": activeGrantID, "remaining_seconds": remainingSeconds,
		})
	}

	tariffRows, err := s.db.Query(r.Context(), `
		SELECT t.id, t.zone_id, t.name, t.duration_minutes, t.price_tiyin
		FROM tariff_blocks t
		JOIN zones z ON z.id = t.zone_id
		WHERE t.status = 'active' AND t.club_id = $1 AND z.status = 'active'
		ORDER BY t.sort_order, t.duration_minutes
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tariffRows.Close()

	tariffs := make([]map[string]any, 0)
	for tariffRows.Next() {
		var id, zoneID, name string
		var duration int
		var price int64
		if err := tariffRows.Scan(&id, &zoneID, &name, &duration, &price); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		tariffs = append(tariffs, map[string]any{
			"id": id, "zone_id": zoneID, "name": name, "duration_minutes": duration, "price_tiyin": price, "price_uzs": price / 100,
		})
	}

	zones, err := s.listZones(r.Context(), clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"club_id": clubID, "pcs": pcs, "zones": zones, "tariffs": tariffs})
}

func (s *Server) handleAdminPCStatus(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	pcID := r.PathValue("pc_id")
	var req adminPCStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if pcID == "" || !isKnownPCStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "pc_id and valid status are required")
		return
	}

	clubID, err := s.clubIDForEntity(r.Context(), "pc_refs", pcID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager", "admin"); !ok {
		return
	}
	err = s.db.QueryRow(r.Context(), `
		UPDATE pc_refs
		SET status_cache = $1
		WHERE id = $2
		RETURNING club_id
	`, req.Status, pcID).Scan(&clubID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	metadata, _ := json.Marshal(map[string]any{"status": req.Status, "reason": req.Reason})
	_, _ = s.db.Exec(r.Context(), `
		INSERT INTO audit_logs (club_id, action, entity_type, entity_id, metadata)
		VALUES ($1, 'admin_pc_status', 'pc_ref', $2, $3)
	`, clubID, pcID, metadata)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "pc_id": pcID, "status": req.Status})
}

func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID, ok := s.resolveRequestClub(w, r, auth, "owner", "manager", "admin")
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT po.id, po.invoice_id, po.provider, po.provider_payment_id, po.amount_tiyin, po.duration_minutes, po.duration_seconds,
		       po.status, po.provider_status, po.checkout_url, po.receipt_url, po.receipt_kind, po.fiscal_status,
		       po.split_platform_amount_tiyin, po.split_club_amount_tiyin, po.created_at,
		       p.label, COALESCE(t.name, 'Своя сумма')
		FROM payment_orders po
		JOIN pc_refs p ON p.id = po.pc_ref_id
		LEFT JOIN tariff_blocks t ON t.id = po.tariff_block_id
		WHERE po.club_id = $1
		ORDER BY po.created_at DESC
		LIMIT 50
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	orders := make([]map[string]any, 0)
	for rows.Next() {
		var id, invoiceID, provider, status, receiptKind, fiscalStatus, pcLabel, tariffName string
		var providerPaymentID, providerStatus, checkoutURL, receiptURL *string
		var amount, splitPlatformAmount, splitClubAmount int64
		var duration, durationSeconds int
		var createdAt time.Time
		if err := rows.Scan(
			&id, &invoiceID, &provider, &providerPaymentID, &amount, &duration, &durationSeconds, &status, &providerStatus,
			&checkoutURL, &receiptURL, &receiptKind, &fiscalStatus, &splitPlatformAmount, &splitClubAmount,
			&createdAt, &pcLabel, &tariffName,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		orders = append(orders, map[string]any{
			"id": id, "invoice_id": invoiceID, "provider": provider, "provider_payment_id": providerPaymentID,
			"amount_tiyin": amount, "amount_uzs": amount / 100,
			"duration_minutes": duration, "duration_seconds": durationSeconds, "status": status, "provider_status": providerStatus, "checkout_url": checkoutURL,
			"receipt_url": receiptURL, "receipt_kind": receiptKind, "fiscal_status": fiscalStatus,
			"split_platform_amount_tiyin": splitPlatformAmount, "split_club_amount_tiyin": splitClubAmount,
			"pc_label": pcLabel, "tariff": tariffName, "created_at": createdAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleAdminGrants(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID, ok := s.resolveRequestClub(w, r, auth, "owner", "manager", "admin")
	if !ok {
		return
	}
	if err := s.expireElapsedGrants(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT g.id, g.duration_minutes, g.duration_seconds, g.status, g.core_session_id, g.source, g.accepted_at,
		       g.planned_ends_at, g.ended_at, g.end_reason, g.remaining_minutes, g.remaining_seconds, g.last_error, g.created_at, p.label
		FROM game_access_grants g
		JOIN pc_refs p ON p.id = g.pc_ref_id
		WHERE g.club_id = $1
		ORDER BY g.created_at DESC
		LIMIT 50
	`, clubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	grants := make([]map[string]any, 0)
	for rows.Next() {
		var id, status, source, pcLabel string
		var coreSessionID, endReason, lastError *string
		var acceptedAt, plannedEndsAt, endedAt *time.Time
		var duration, durationSeconds, remainingMinutes, remainingSeconds int
		var createdAt time.Time
		if err := rows.Scan(
			&id, &duration, &durationSeconds, &status, &coreSessionID, &source, &acceptedAt, &plannedEndsAt,
			&endedAt, &endReason, &remainingMinutes, &remainingSeconds, &lastError, &createdAt, &pcLabel,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		grants = append(grants, map[string]any{
			"id": id, "duration_minutes": duration, "duration_seconds": durationSeconds, "status": status, "core_session_id": coreSessionID, "source": source,
			"accepted_at": acceptedAt, "planned_ends_at": plannedEndsAt, "ended_at": endedAt, "end_reason": endReason,
			"remaining_minutes": remainingMinutes, "remaining_seconds": remainingSeconds, "last_error": lastError, "created_at": createdAt, "pc_label": pcLabel,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"grants": grants})
}

func (s *Server) handleAdminEndGrant(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	grantID := r.PathValue("grant_id")
	var req endGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if grantID == "" {
		writeError(w, http.StatusBadRequest, "grant_id is required")
		return
	}
	reason := defaultString(req.Reason, "admin_request")
	var coreSessionID *string
	var clubID string
	var remainingSeconds int
	err := s.db.QueryRow(r.Context(), `
		SELECT core_session_id,
		       club_id,
		       CASE
		         WHEN status = 'accepted' THEN GREATEST(
		           CEIL(EXTRACT(EPOCH FROM (
		             COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes), now()) - now()
		           )))::int,
		           0
		         )
		         ELSE 0
		       END
		FROM game_access_grants
		WHERE id = $1
	`, grantID).Scan(&coreSessionID, &clubID, &remainingSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "grant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager", "admin"); !ok {
		return
	}
	if coreSessionID != nil && *coreSessionID != "" {
		_, _ = s.core.EndSession(r.Context(), *coreSessionID, core.EndSessionCommand{
			RequestID: "end_" + grantID + "_" + randomHex(4),
			Reason:    reason,
			EndedBy:   map[string]string{"type": "admin", "id": "mvp-admin"},
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	if req.RemainingSeconds > 0 {
		remainingSeconds = req.RemainingSeconds
	} else if req.RemainingMinutes > 0 {
		remainingSeconds = req.RemainingMinutes * 60
	}
	result, err := s.finishGrant(r.Context(), grantID, reason, remainingSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.RecipientPhone) != "" {
		if voucher, ok := result["voucher"].(map[string]any); ok {
			code, _ := voucher["code"].(string)
			seconds, _ := voucher["seconds_left"].(int)
			delivery, _ := s.attachVoucherRecipientAndSend(r.Context(), fmt.Sprint(voucher["id"]), code, seconds, req.RecipientPhone)
			result["voucher_delivery"] = delivery
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCashSession(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	var req cashSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PCID == "" || (req.TariffBlockID == "" && req.AmountUZS <= 0) {
		writeError(w, http.StatusBadRequest, "pc_id and package or amount_uzs are required")
		return
	}
	if req.AmountUZS < 0 {
		writeError(w, http.StatusBadRequest, "amount_uzs must be positive")
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	var clubID, externalPCID, pcStatus string
	var amount, hourlyPriceTiyin int64
	var duration, durationSeconds int
	err = tx.QueryRow(ctx, `
		SELECT p.club_id, p.external_pc_id, p.status_cache, z.hourly_price_tiyin
		FROM pc_refs p
		JOIN zones z ON z.id = p.zone_id
		WHERE p.id = $1 AND p.status_cache <> 'deleted' AND z.status = 'active'
	`, req.PCID).Scan(&clubID, &externalPCID, &pcStatus, &hourlyPriceTiyin)
	if err != nil {
		writeError(w, http.StatusBadRequest, "pc not found")
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager", "admin"); !ok {
		return
	}
	var extensionGrant activeGrantRow
	extendExisting := false
	if !isPayablePCStatus(pcStatus) {
		if pcStatus != "occupied" {
			writeError(w, http.StatusConflict, "PC is not available")
			return
		}
		var ok bool
		extensionGrant, ok, err = activeGrantForPC(ctx, tx, req.PCID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok || extensionGrant.CoreSessionID == "" {
			writeError(w, http.StatusConflict, "active session for extension not found")
			return
		}
		extendExisting = true
	}
	var tariffIDArg any
	if req.AmountUZS > 0 {
		amountTiyin := req.AmountUZS * 100
		amount = amountTiyin
		durationSeconds = secondsForAmount(amountTiyin, hourlyPriceTiyin)
		duration = secondsToMinutesCeil(durationSeconds)
	} else {
		err = tx.QueryRow(ctx, `
			SELECT t.price_tiyin, t.duration_minutes
			FROM tariff_blocks t
			JOIN pc_refs p ON p.zone_id = t.zone_id
			JOIN zones z ON z.id = p.zone_id
			WHERE p.id = $1 AND t.id = $2 AND t.status = 'active' AND z.status = 'active'
		`, req.PCID, req.TariffBlockID).Scan(&amount, &duration)
		durationSeconds = duration * 60
		if err != nil {
			writeError(w, http.StatusBadRequest, "pc/package mismatch")
			return
		}
		tariffIDArg = req.TariffBlockID
	}

	var cashID string
	err = tx.QueryRow(ctx, `
		INSERT INTO cash_payments (club_id, pc_ref_id, tariff_block_id, amount_tiyin, duration_minutes, duration_seconds, reason, fiscal_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, clubID, req.PCID, tariffIDArg, amount, duration, durationSeconds, defaultString(req.Reason, "cash"), req.FiscalReference).Scan(&cashID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if extendExisting {
		if err := tx.Commit(ctx); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.extendGrantSession(ctx, extensionGrant.ID, extensionGrant.CoreSessionID, clubID, req.PCID, externalPCID, durationSeconds, "cash", "", ""); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"cash_payment_id": cashID, "grant_id": extensionGrant.ID, "extended": true})
		return
	}

	var grantID string
	err = tx.QueryRow(ctx, `
		INSERT INTO game_access_grants (club_id, pc_ref_id, cash_payment_id, duration_minutes, duration_seconds, status, source)
		VALUES ($1, $2, $3, $4, $5, 'pending', 'cash')
		RETURNING id
	`, clubID, req.PCID, cashID, duration, durationSeconds).Scan(&grantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	startResult, err := s.core.StartSession(ctx, core.StartSessionCommand{
		RequestID:       "start_" + grantID,
		GrantID:         grantID,
		ClubID:          clubID,
		PCID:            req.PCID,
		PCExternalID:    externalPCID,
		DurationSeconds: durationSeconds,
		DurationMinutes: duration,
		Source:          "cash",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		_, _ = tx.Exec(ctx, `
			UPDATE game_access_grants
			SET status = 'start_failed', last_error = $1
			WHERE id = $2
		`, err.Error(), grantID)
		_ = tx.Commit(ctx)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	coreSessionID := startResult.CoreSessionID
	if coreSessionID == "" {
		coreSessionID = "core-session-" + grantID
	}
	plannedEndsAt := startResult.EndsAt
	if plannedEndsAt == nil {
		endsAt := time.Now().UTC().Add(time.Duration(durationSeconds) * time.Second)
		plannedEndsAt = &endsAt
	}

	_, err = tx.Exec(ctx, `
		UPDATE game_access_grants
		SET status = 'accepted', core_session_id = $1, accepted_at = now(), planned_ends_at = $2
		WHERE id = $3
	`, coreSessionID, plannedEndsAt, grantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = tx.Exec(ctx, `
		UPDATE pc_refs
		SET status_cache = 'occupied'
		WHERE id = $1
	`, req.PCID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"cash_payment_id": cashID, "grant_id": grantID, "core_session_id": coreSessionID, "duration_seconds": durationSeconds})
}

func (s *Server) handleOwnerSummary(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID, ok := s.resolveRequestClub(w, r, auth, "owner", "manager")
	if !ok {
		return
	}
	if err := s.expireElapsedGrants(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var summary struct {
		OnlineRevenueTiyin     int64 `json:"online_revenue_tiyin"`
		CashRevenueTiyin       int64 `json:"cash_revenue_tiyin"`
		ClubOnlineRevenueTiyin int64 `json:"club_online_revenue_tiyin"`
		PlatformFeeTiyin       int64 `json:"platform_fee_tiyin"`
		PaidOrders             int   `json:"paid_orders"`
		CashSessions           int   `json:"cash_sessions"`
		ActiveGrants           int   `json:"active_grants"`
	}
	err := s.db.QueryRow(r.Context(), `
		SELECT
			COALESCE((SELECT SUM(amount_tiyin) FROM payment_orders WHERE status = 'paid' AND club_id = $1), 0),
			COALESCE((SELECT SUM(amount_tiyin) FROM cash_payments WHERE club_id = $1), 0),
			COALESCE((SELECT SUM(
				CASE
					WHEN split_platform_amount_tiyin > 0 OR split_club_amount_tiyin > 0 THEN split_club_amount_tiyin
					ELSE amount_tiyin
				END
			) FROM payment_orders WHERE status = 'paid' AND club_id = $1), 0),
			COALESCE((SELECT SUM(split_platform_amount_tiyin) FROM payment_orders WHERE status = 'paid' AND club_id = $1), 0),
			COALESCE((SELECT COUNT(*)::int FROM payment_orders WHERE status = 'paid' AND club_id = $1), 0),
			COALESCE((SELECT COUNT(*)::int FROM cash_payments WHERE club_id = $1), 0),
			COALESCE((SELECT COUNT(*)::int FROM game_access_grants WHERE status = 'accepted' AND club_id = $1), 0)
	`, clubID).Scan(
		&summary.OnlineRevenueTiyin,
		&summary.CashRevenueTiyin,
		&summary.ClubOnlineRevenueTiyin,
		&summary.PlatformFeeTiyin,
		&summary.PaidOrders,
		&summary.CashSessions,
		&summary.ActiveGrants,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	clubTotalRevenueTiyin := summary.ClubOnlineRevenueTiyin + summary.CashRevenueTiyin
	writeJSON(w, http.StatusOK, map[string]any{
		"club_id":                   clubID,
		"online_revenue_tiyin":      summary.OnlineRevenueTiyin,
		"online_revenue_uzs":        summary.OnlineRevenueTiyin / 100,
		"cash_revenue_tiyin":        summary.CashRevenueTiyin,
		"cash_revenue_uzs":          summary.CashRevenueTiyin / 100,
		"club_online_revenue_tiyin": summary.ClubOnlineRevenueTiyin,
		"club_online_revenue_uzs":   summary.ClubOnlineRevenueTiyin / 100,
		"platform_fee_tiyin":        summary.PlatformFeeTiyin,
		"platform_fee_uzs":          summary.PlatformFeeTiyin / 100,
		"club_total_revenue_tiyin":  clubTotalRevenueTiyin,
		"club_total_revenue_uzs":    clubTotalRevenueTiyin / 100,
		"paid_orders":               summary.PaidOrders,
		"cash_sessions":             summary.CashSessions,
		"active_grants":             summary.ActiveGrants,
	})
}

func (s *Server) handleCreateVoucher(w http.ResponseWriter, r *http.Request) {
	var req createVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	secondsLeft := req.SecondsLeft
	if secondsLeft <= 0 && req.MinutesLeft > 0 {
		secondsLeft = req.MinutesLeft * 60
	}
	minutesLeft := secondsToMinutesCeil(secondsLeft)
	if req.PaymentOrderID == "" || secondsLeft <= 0 {
		writeError(w, http.StatusBadRequest, "payment_order_id and positive seconds_left are required")
		return
	}
	code := strings.ToUpper(randomHex(4) + "-" + randomHex(4))
	hash := hashCode(code)
	expiresAt := time.Now().AddDate(0, 0, s.cfg.VoucherTTLDays)

	var voucherID string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO vouchers (club_id, original_payment_order_id, minutes_left, seconds_left, code_hash, expires_at, public_code)
		SELECT club_id, id, $2, $3, $4, $5, $6
		FROM payment_orders
		WHERE id = $1
		RETURNING id
	`, req.PaymentOrderID, minutesLeft, secondsLeft, hash, expiresAt, code).Scan(&voucherID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"voucher_id": voucherID, "code": code, "minutes_left": minutesLeft, "seconds_left": secondsLeft, "expires_at": expiresAt})
}

func (s *Server) handleRedeemVoucher(w http.ResponseWriter, r *http.Request) {
	var req redeemVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	if req.QRToken != "" {
		result, err := s.redeemVoucherToPC(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	var voucherID string
	var minutes, seconds int
	err := s.db.QueryRow(r.Context(), `
		UPDATE vouchers
		SET status = 'redeemed', redeemed_at = now()
		WHERE code_hash = $1 AND status = 'active' AND expires_at > now()
		RETURNING id, minutes_left, seconds_left
	`, hashCode(strings.ToUpper(req.Code))).Scan(&voucherID, &minutes, &seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "voucher not found or expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"voucher_id": voucherID, "minutes_left": minutes, "seconds_left": voucherSeconds(seconds, minutes), "status": "redeemed"})
}

func (s *Server) handleCheckVoucher(w http.ResponseWriter, r *http.Request) {
	var req redeemVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	var voucherID, voucherClubID string
	var minutes, seconds int
	var expiresAt time.Time
	err := s.db.QueryRow(r.Context(), `
		SELECT id, club_id, minutes_left, seconds_left, expires_at
		FROM vouchers
		WHERE code_hash = $1 AND status = 'active' AND expires_at > now()
	`, hashCode(strings.ToUpper(req.Code))).Scan(&voucherID, &voucherClubID, &minutes, &seconds, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "voucher not found or expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	seconds = voucherSeconds(seconds, minutes)
	result := map[string]any{"voucher_id": voucherID, "minutes_left": secondsToMinutesCeil(seconds), "seconds_left": seconds, "expires_at": expiresAt, "status": "active"}
	if req.QRToken != "" {
		var pcID, pcClubID, pcStatus, zoneID, zoneName string
		err = s.db.QueryRow(r.Context(), `
			SELECT p.id, p.club_id,
			       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END,
			       z.id, z.name
			FROM qr_codes q
			JOIN pc_refs p ON p.id = q.pc_ref_id
			JOIN zones z ON z.id = p.zone_id
			WHERE q.public_token = $1 AND q.status = 'active' AND p.status_cache <> 'deleted' AND z.status <> 'deleted'
		`, req.QRToken).Scan(&pcID, &pcClubID, &pcStatus, &zoneID, &zoneName)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "QR token not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		canRedeem := voucherClubID == pcClubID && (isPayablePCStatus(pcStatus) || pcStatus == "occupied")
		result["can_redeem"] = canRedeem
		result["pc_id"] = pcID
		result["pc_status"] = pcStatus
		result["zone"] = map[string]any{"id": zoneID, "name": zoneName}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) validVoucherForPC(ctx context.Context, code, clubID, pcID, pcStatus string) (string, int, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", 0, fmt.Errorf("code is required")
	}
	var voucherID, voucherClubID string
	var minutes, seconds int
	err := s.db.QueryRow(ctx, `
		SELECT id, club_id, minutes_left, seconds_left
		FROM vouchers
		WHERE code_hash = $1 AND status = 'active' AND expires_at > now()
	`, hashCode(strings.ToUpper(code))).Scan(&voucherID, &voucherClubID, &minutes, &seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, fmt.Errorf("voucher not found or expired")
	}
	if err != nil {
		return "", 0, err
	}
	if voucherClubID != clubID {
		return "", 0, fmt.Errorf("voucher belongs to another club")
	}
	if !isPayablePCStatus(pcStatus) && pcStatus != "occupied" {
		return "", 0, fmt.Errorf("PC is not available")
	}
	if pcID == "" {
		return "", 0, fmt.Errorf("PC is not available")
	}
	return voucherID, voucherSeconds(seconds, minutes), nil
}

func (s *Server) redeemVoucherToPC(ctx context.Context, req redeemVoucherRequest) (map[string]any, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var voucherID, voucherClubID string
	var minutes, seconds int
	err = tx.QueryRow(ctx, `
		SELECT id, club_id, minutes_left, seconds_left
		FROM vouchers
		WHERE code_hash = $1 AND status = 'active' AND expires_at > now()
		FOR UPDATE
	`, hashCode(strings.ToUpper(req.Code))).Scan(&voucherID, &voucherClubID, &minutes, &seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("voucher not found or expired")
	}
	if err != nil {
		return nil, err
	}
	seconds = voucherSeconds(seconds, minutes)
	minutes = secondsToMinutesCeil(seconds)

	var pcID, pcClubID, externalPCID, pcStatus string
	err = tx.QueryRow(ctx, `
		SELECT p.id, p.club_id, p.external_pc_id,
		       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END
		FROM qr_codes q
		JOIN pc_refs p ON p.id = q.pc_ref_id
		JOIN zones z ON z.id = p.zone_id
		WHERE q.public_token = $1 AND q.status = 'active' AND z.status <> 'deleted' AND p.status_cache <> 'deleted'
	`, req.QRToken).Scan(&pcID, &pcClubID, &externalPCID, &pcStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("QR token not found")
	}
	if err != nil {
		return nil, err
	}
	if voucherClubID != pcClubID {
		return nil, fmt.Errorf("voucher belongs to another club")
	}
	var extensionGrant activeGrantRow
	extendExisting := false
	if !isPayablePCStatus(pcStatus) {
		if pcStatus != "occupied" {
			return nil, fmt.Errorf("PC is not available")
		}
		var ok bool
		extensionGrant, ok, err = activeGrantForPC(ctx, tx, pcID)
		if err != nil {
			return nil, err
		}
		if !ok || extensionGrant.CoreSessionID == "" {
			return nil, fmt.Errorf("active session for extension not found")
		}
		extendExisting = true
	}

	if extendExisting {
		_, err = tx.Exec(ctx, `
			UPDATE vouchers
			SET status = 'redeemed', redeemed_at = now(), redeemed_grant_id = $1
			WHERE id = $2
		`, extensionGrant.ID, voucherID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if err := s.extendGrantSession(ctx, extensionGrant.ID, extensionGrant.CoreSessionID, voucherClubID, pcID, externalPCID, seconds, "voucher", "", ""); err != nil {
			return nil, err
		}
		return map[string]any{
			"success": true, "voucher_id": voucherID, "grant_id": extensionGrant.ID,
			"minutes_left": minutes, "seconds_left": seconds, "status": "redeemed", "extended": true,
		}, nil
	}

	var grantID string
	err = tx.QueryRow(ctx, `
		INSERT INTO game_access_grants (club_id, pc_ref_id, voucher_id, duration_minutes, duration_seconds, status, source)
		VALUES ($1, $2, $3, $4, $5, 'pending', 'voucher')
		RETURNING id
	`, voucherClubID, pcID, voucherID, minutes, seconds).Scan(&grantID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE vouchers
		SET status = 'redeemed', redeemed_at = now(), redeemed_grant_id = $1
		WHERE id = $2
	`, grantID, voucherID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	startResult, err := s.core.StartSession(ctx, core.StartSessionCommand{
		RequestID:       "start_" + grantID,
		GrantID:         grantID,
		ClubID:          voucherClubID,
		PCID:            pcID,
		PCExternalID:    externalPCID,
		DurationSeconds: seconds,
		DurationMinutes: minutes,
		Source:          "voucher",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		_, _ = s.db.Exec(ctx, `
			UPDATE game_access_grants
			SET status = 'start_failed', last_error = $1
			WHERE id = $2
		`, err.Error(), grantID)
		return nil, err
	}
	coreSessionID := startResult.CoreSessionID
	if coreSessionID == "" {
		coreSessionID = "core-session-" + grantID
	}
	plannedEndsAt := startResult.EndsAt
	if plannedEndsAt == nil {
		endsAt := time.Now().UTC().Add(time.Duration(seconds) * time.Second)
		plannedEndsAt = &endsAt
	}
	_, err = s.db.Exec(ctx, `
		UPDATE game_access_grants
		SET status = 'accepted', core_session_id = $1, accepted_at = now(), planned_ends_at = $2
		WHERE id = $3
	`, coreSessionID, plannedEndsAt, grantID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(ctx, `UPDATE pc_refs SET status_cache = 'occupied' WHERE id = $1`, pcID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"success": true, "voucher_id": voucherID, "grant_id": grantID, "core_session_id": coreSessionID,
		"minutes_left": minutes, "seconds_left": seconds, "status": "redeemed",
	}, nil
}

func (s *Server) finishGrant(ctx context.Context, grantID, reason string, remainingSeconds int) (map[string]any, error) {
	if grantID == "" {
		return nil, fmt.Errorf("grant_id is required")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var clubID, pcID string
	var paymentOrderID, returnedVoucherID *string
	err = tx.QueryRow(ctx, `
		SELECT club_id, pc_ref_id, payment_order_id, returned_voucher_id
		FROM game_access_grants
		WHERE id = $1
		FOR UPDATE
	`, grantID).Scan(&clubID, &pcID, &paymentOrderID, &returnedVoucherID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("grant not found")
	}
	if err != nil {
		return nil, err
	}
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	remainingMinutes := secondsToMinutesCeil(remainingSeconds)

	_, err = tx.Exec(ctx, `
		UPDATE game_access_grants
		SET status = 'ended', ended_at = now(), end_reason = $1, remaining_minutes = $2, remaining_seconds = $3
		WHERE id = $4
	`, defaultString(reason, "ended"), remainingMinutes, remainingSeconds, grantID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE pc_refs
		SET status_cache = 'available'
		WHERE id = $1
	`, pcID)
	if err != nil {
		return nil, err
	}

	result := map[string]any{"success": true, "grant_id": grantID, "status": "ended", "remaining_minutes": remainingMinutes, "remaining_seconds": remainingSeconds}
	if remainingSeconds > 0 && returnedVoucherID == nil {
		code := strings.ToUpper(randomHex(4) + "-" + randomHex(4))
		hash := hashCode(code)
		expiresAt := time.Now().AddDate(0, 0, s.cfg.VoucherTTLDays)
		var voucherID string
		err = tx.QueryRow(ctx, `
			INSERT INTO vouchers (club_id, original_payment_order_id, minutes_left, seconds_left, code_hash, expires_at, public_code)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, clubID, paymentOrderID, remainingMinutes, remainingSeconds, hash, expiresAt, code).Scan(&voucherID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE game_access_grants SET returned_voucher_id = $1 WHERE id = $2`, voucherID, grantID)
		if err != nil {
			return nil, err
		}
		result["voucher"] = map[string]any{
			"id": voucherID, "code": code, "minutes_left": remainingMinutes, "seconds_left": remainingSeconds, "expires_at": expiresAt,
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Server) attachVoucherRecipientAndSend(ctx context.Context, voucherID, code string, seconds int, phone string) (map[string]any, error) {
	normalizedPhone := normalizePhone(phone)
	if voucherID == "" || normalizedPhone == "" {
		return map[string]any{"status": "not_requested"}, nil
	}
	status := "telegram_waiting_for_user"
	_, err := s.db.Exec(ctx, `
		UPDATE vouchers
		SET recipient_phone = $2,
		    delivery_channel = 'telegram',
		    delivery_status = $3
		WHERE id = $1
	`, voucherID, normalizedPhone, status)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.cfg.TelegramBotToken) == "" {
		_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'telegram_not_configured' WHERE id = $1`, voucherID)
		return map[string]any{"status": "telegram_not_configured", "phone": normalizedPhone}, nil
	}

	var chatID string
	err = s.db.QueryRow(ctx, `
		SELECT chat_id
		FROM telegram_users
		WHERE phone = $1 AND status = 'active'
	`, normalizedPhone).Scan(&chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		delivery := map[string]any{"status": status, "phone": normalizedPhone}
		if link, expiresAt, linkErr := s.createTelegramLink(ctx, normalizedPhone); linkErr == nil {
			delivery["telegram_link"] = link
			delivery["link_expires_at"] = expiresAt
		} else {
			_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'telegram_link_failed' WHERE id = $1`, voucherID)
			delivery["status"] = "telegram_link_failed"
		}
		return delivery, nil
	}
	if err != nil {
		return nil, err
	}
	text := telegramVoucherText(code, seconds)
	if err := s.sendTelegramMessage(ctx, chatID, text); err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'telegram_failed' WHERE id = $1`, voucherID)
		return map[string]any{"status": "telegram_failed", "phone": normalizedPhone}, nil
	}
	_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'sent', delivered_at = now() WHERE id = $1`, voucherID)
	return map[string]any{"status": "sent", "phone": normalizedPhone}, nil
}

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.TelegramWebhookSecret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != s.cfg.TelegramWebhookSecret {
		writeError(w, http.StatusUnauthorized, "telegram webhook secret required")
		return
	}
	var update telegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := s.processTelegramUpdate(r.Context(), update)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) processTelegramUpdate(ctx context.Context, update telegramUpdate) (map[string]any, error) {
	message := update.Message
	if message.Chat.ID == 0 {
		return map[string]any{"success": true, "ignored": true}, nil
	}
	chatID := strconv.FormatInt(message.Chat.ID, 10)
	phone := normalizePhone(message.Contact.PhoneNumber)
	startPayload := ""
	if phone == "" && strings.HasPrefix(strings.TrimSpace(message.Text), "/start") {
		parts := strings.Fields(message.Text)
		if len(parts) > 1 {
			startPayload = strings.TrimSpace(parts[1])
		}
	}
	if phone == "" && startPayload != "" {
		if strings.HasPrefix(startPayload, "cp_") {
			var err error
			phone, err = s.consumeTelegramLinkToken(ctx, startPayload, chatID)
			if err != nil {
				_ = s.sendTelegramMessage(ctx, chatID, "Ссылка для получения ваучера устарела. Попросите менеджера отправить новую ссылку.")
				return map[string]any{"success": true, "status": "link_expired"}, nil
			}
		} else {
			phone = normalizePhone(startPayload)
		}
	}
	if phone == "" {
		_ = s.sendTelegramContactRequest(ctx, chatID)
		return map[string]any{"success": true, "status": "phone_required"}, nil
	}
	_, _ = s.db.Exec(ctx, `DELETE FROM telegram_users WHERE chat_id = $1 AND phone <> $2`, chatID, phone)
	_, err := s.db.Exec(ctx, `
		INSERT INTO telegram_users (phone, chat_id, username, first_name, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (phone) DO UPDATE SET
		  chat_id = EXCLUDED.chat_id,
		  username = EXCLUDED.username,
		  first_name = EXCLUDED.first_name,
		  status = 'active',
		  updated_at = now()
	`, phone, chatID, message.From.Username, message.From.FirstName)
	if err != nil {
		return nil, err
	}
	sentPending, sendErr := s.sendPendingTelegramVouchers(ctx, phone, chatID)
	if sendErr != nil {
		_ = s.sendTelegramMessage(ctx, chatID, "Номер привязан, но ваучер сейчас не удалось отправить. Попросите менеджера повторить отправку или попробуйте позже.")
		return map[string]any{"success": true, "phone": phone, "sent_pending": sentPending, "send_error": sendErr.Error()}, nil
	}
	if sentPending == 0 {
		_ = s.sendTelegramMessage(ctx, chatID, "Готово. Номер привязан. Ожидающих ваучеров сейчас нет.")
	}
	return map[string]any{"success": true, "phone": phone, "sent_pending": sentPending}, nil
}

func (s *Server) RunTelegramPolling(ctx context.Context) {
	if !s.cfg.TelegramPollingEnabled || strings.TrimSpace(s.cfg.TelegramBotToken) == "" {
		return
	}
	webhookConfigured, err := s.telegramWebhookConfigured(ctx)
	if err == nil && webhookConfigured {
		return
	}
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		updates, err := s.fetchTelegramUpdates(ctx, offset)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			_, _ = s.processTelegramUpdate(ctx, update)
		}
	}
}

func (s *Server) fetchTelegramUpdates(ctx context.Context, offset int64) ([]telegramUpdate, error) {
	params := url.Values{}
	params.Set("timeout", "20")
	params.Set("allowed_updates", `["message"]`)
	if offset > 0 {
		params.Set("offset", strconv.FormatInt(offset, 10))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+s.cfg.TelegramBotToken+"/getUpdates?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram getUpdates failed: HTTP %d", resp.StatusCode)
	}
	var result telegramUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram getUpdates failed: %s", result.Description)
	}
	return result.Result, nil
}

func (s *Server) telegramWebhookConfigured(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+s.cfg.TelegramBotToken+"/getWebhookInfo", nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("telegram getWebhookInfo failed: HTTP %d", resp.StatusCode)
	}
	var result telegramWebhookInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	if !result.OK {
		return false, fmt.Errorf("telegram getWebhookInfo failed: %s", result.Description)
	}
	return strings.TrimSpace(result.Result.URL) != "", nil
}

func (s *Server) sendPendingTelegramVouchers(ctx context.Context, phone, chatID string) (int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, COALESCE(public_code, ''), minutes_left, seconds_left
		FROM vouchers
		WHERE recipient_phone = $1
		  AND status = 'active'
		  AND delivery_channel = 'telegram'
		  AND delivery_status IN ('telegram_waiting_for_user', 'telegram_failed', 'telegram_not_configured', 'telegram_link_failed')
		  AND COALESCE(public_code, '') <> ''
		ORDER BY created_at ASC
	`, phone)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	sent := 0
	for rows.Next() {
		var voucherID, code string
		var minutes, seconds int
		if err := rows.Scan(&voucherID, &code, &minutes, &seconds); err != nil {
			return sent, err
		}
		text := telegramVoucherText(code, voucherSeconds(seconds, minutes))
		if err := s.sendTelegramMessage(ctx, chatID, text); err != nil {
			_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'telegram_failed' WHERE id = $1`, voucherID)
			continue
		}
		_, _ = s.db.Exec(ctx, `UPDATE vouchers SET delivery_status = 'sent', delivered_at = now() WHERE id = $1`, voucherID)
		sent++
	}
	return sent, rows.Err()
}

func (s *Server) sendTelegramMessage(ctx context.Context, chatID, text string) error {
	return s.sendTelegramMessageWithMarkup(ctx, chatID, text, nil)
}

func (s *Server) sendTelegramContactRequest(ctx context.Context, chatID string) error {
	markup := map[string]any{
		"keyboard": [][]map[string]any{{
			{"text": "Поделиться номером", "request_contact": true},
		}},
		"resize_keyboard":   true,
		"one_time_keyboard": true,
	}
	return s.sendTelegramMessageWithMarkup(ctx, chatID, "Нажмите кнопку ниже, чтобы привязать номер и получать ваучеры Clubpay.", markup)
}

func (s *Server) sendTelegramMessageWithMarkup(ctx context.Context, chatID, text string, replyMarkup any) error {
	if s.cfg.TelegramBotToken == "" || chatID == "" {
		return fmt.Errorf("telegram bot is not configured")
	}
	payloadMap := map[string]any{"chat_id": chatID, "text": text}
	if replyMarkup != nil {
		payloadMap["reply_markup"] = replyMarkup
	}
	payload, _ := json.Marshal(payloadMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+s.cfg.TelegramBotToken+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) createTelegramLink(ctx context.Context, phone string) (string, time.Time, error) {
	if strings.TrimSpace(s.cfg.TelegramBotToken) == "" {
		return "", time.Time{}, fmt.Errorf("telegram bot is not configured")
	}
	username, err := s.telegramBotUsername(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	token := "cp_" + randomHex(12)
	ttlDays := s.cfg.VoucherTTLDays
	if ttlDays <= 0 {
		ttlDays = 30
	}
	expiresAt := time.Now().UTC().AddDate(0, 0, ttlDays)
	_, err = s.db.Exec(ctx, `
		INSERT INTO telegram_link_tokens (phone, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, phone, hashToken(token), expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return fmt.Sprintf("https://t.me/%s?start=%s", username, url.QueryEscape(token)), expiresAt, nil
}

func (s *Server) telegramBotPublicLink(ctx context.Context) (string, string) {
	if strings.TrimSpace(s.cfg.TelegramBotToken) == "" {
		return "", ""
	}
	username, err := s.telegramBotUsername(ctx)
	if err != nil || username == "" {
		return "", ""
	}
	return fmt.Sprintf("https://t.me/%s", username), username
}

func (s *Server) consumeTelegramLinkToken(ctx context.Context, token, chatID string) (string, error) {
	var phone string
	err := s.db.QueryRow(ctx, `
		UPDATE telegram_link_tokens
		SET status = 'used',
		    used_at = now(),
		    chat_id = $2,
		    updated_at = now()
		WHERE token_hash = $1
		  AND status = 'active'
		  AND expires_at > now()
		RETURNING phone
	`, hashToken(token), chatID).Scan(&phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("telegram link token not found or expired")
	}
	return phone, err
}

func (s *Server) telegramBotUsername(ctx context.Context) (string, error) {
	if s.cfg.TelegramBotUsername != "" {
		return s.cfg.TelegramBotUsername, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+s.cfg.TelegramBotToken+"/getMe", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("telegram getMe failed: HTTP %d", resp.StatusCode)
	}
	var result telegramGetMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK || result.Result.Username == "" {
		return "", fmt.Errorf("telegram bot username is empty")
	}
	return result.Result.Username, nil
}

func telegramVoucherText(code string, seconds int) string {
	return fmt.Sprintf("Ваш ваучер Clubpay: %s\nОстаток: %s.\nВведите код на QR-странице компьютера.", code, formatDurationForMessage(seconds))
}

func (s *Server) orderByInvoiceID(ctx context.Context, invoiceID string) (map[string]any, error) {
	var id, status, receiptKind, fiscalStatus, pcLabel, externalPCID, tariffName string
	var provider string
	var providerPaymentID, providerStatus, checkoutURL, receiptURL *string
	var expiresAt, paidAt *time.Time
	var amount, splitPlatformAmount, splitClubAmount int64
	var duration, durationSeconds int
	var createdAt time.Time
	err := s.db.QueryRow(ctx, `
		SELECT po.id, po.invoice_id, po.provider, po.provider_payment_id, po.amount_tiyin, po.duration_minutes, po.duration_seconds,
		       po.status, po.provider_status, po.checkout_url, po.receipt_url, po.receipt_kind, po.fiscal_status,
		       po.split_platform_amount_tiyin, po.split_club_amount_tiyin, po.expires_at, po.paid_at, po.created_at,
		       p.label, p.external_pc_id, COALESCE(t.name, 'Своя сумма')
		FROM payment_orders po
		JOIN pc_refs p ON p.id = po.pc_ref_id
		LEFT JOIN tariff_blocks t ON t.id = po.tariff_block_id
		WHERE po.invoice_id = $1
	`, invoiceID).Scan(
		&id, &invoiceID, &provider, &providerPaymentID, &amount, &duration, &durationSeconds, &status, &providerStatus, &checkoutURL,
		&receiptURL, &receiptKind, &fiscalStatus, &splitPlatformAmount, &splitClubAmount, &expiresAt,
		&paidAt, &createdAt, &pcLabel, &externalPCID, &tariffName,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "invoice_id": invoiceID, "provider": provider, "provider_payment_id": providerPaymentID,
		"amount_tiyin": amount, "amount_uzs": amount / 100,
		"duration_minutes": duration, "duration_seconds": durationSeconds, "status": status, "provider_status": providerStatus, "checkout_url": checkoutURL,
		"receipt_url": receiptURL, "receipt_kind": receiptKind, "fiscal_status": fiscalStatus,
		"split_platform_amount_tiyin": splitPlatformAmount, "split_club_amount_tiyin": splitClubAmount,
		"expires_at": expiresAt, "paid_at": paidAt, "created_at": createdAt, "pc_label": pcLabel,
		"external_pc_id": externalPCID, "tariff": tariffName,
	}, nil
}

func (s *Server) insertProviderEvent(ctx context.Context, provider, eventType, externalID string, payload []byte) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO provider_events (provider, event_type, external_id, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, provider, eventType, externalID, payload).Scan(&id)
	return id, err
}

func (s *Server) markProviderEvent(ctx context.Context, id, status string) {
	if id == "" {
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE provider_events SET status = $1, processed_at = now() WHERE id = $2`, status, id)
}

type qrPC struct {
	ClubID, ClubName, PCID, ExternalPCID, Label, Status, ZoneID, ZoneName string
	ClickMerchantID, ClickServiceID, ClickMerchantUserID, ClickSecretKey  string
	PaymeMerchantID, PaymeSecretKey                                       string
	Number                                                                int
	HourlyPriceTiyin                                                      int64
}

type tariffDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DurationMinutes int    `json:"duration_minutes"`
	PriceTiyin      int64  `json:"price_tiyin"`
	PriceUZS        int64  `json:"price_uzs"`
}

type authContext struct {
	UserID     string
	Name       string
	Email      string
	Phone      string
	GlobalRole string
}

func (a authContext) IsSuperAdmin() bool {
	return a.GlobalRole == "super_admin"
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type clubSettingsRequest struct {
	Name                string `json:"name"`
	Slug                string `json:"slug"`
	LegalName           string `json:"legal_name"`
	TIN                 string `json:"tin"`
	Address             string `json:"address"`
	Timezone            string `json:"timezone"`
	Status              string `json:"status"`
	ClickMerchantID     string `json:"click_merchant_id"`
	ClickServiceID      string `json:"click_service_id"`
	ClickMerchantUserID string `json:"click_merchant_user_id"`
	ClickSecretKey      string `json:"click_secret_key"`
	PaymeMerchantID     string `json:"payme_merchant_id"`
	PaymeSecretKey      string `json:"payme_secret_key"`
	PlatformFeeBPS      int    `json:"platform_fee_bps"`
	OFDMXIK             string `json:"ofd_mxik"`
	OFDPackageCode      string `json:"ofd_package_code"`
}

type zoneRequest struct {
	Name             string `json:"name"`
	HourlyPriceTiyin int64  `json:"hourly_price_tiyin"`
	HourlyPriceUZS   int64  `json:"hourly_price_uzs"`
	SortOrder        int    `json:"sort_order"`
	Status           string `json:"status"`
}

type tariffRequest struct {
	ZoneID          string `json:"zone_id"`
	Name            string `json:"name"`
	DurationMinutes int    `json:"duration_minutes"`
	PriceTiyin      int64  `json:"price_tiyin"`
	PriceUZS        int64  `json:"price_uzs"`
	SortOrder       int    `json:"sort_order"`
	Status          string `json:"status"`
}

type pcRequest struct {
	ZoneID       string `json:"zone_id"`
	ExternalPCID string `json:"external_pc_id"`
	Number       int    `json:"number"`
	Label        string `json:"label"`
	Status       string `json:"status"`
	QRToken      string `json:"qr_token"`
}

type userRoleRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

type createCheckoutRequest struct {
	QRToken         string `json:"qr_token"`
	TariffBlockID   string `json:"tariff_block_id"`
	AmountUZS       int64  `json:"amount_uzs"`
	PaymentProvider string `json:"payment_provider"`
	VoucherCode     string `json:"voucher_code"`
}

type checkoutSeed struct {
	ClubID, ClubName, PCID, ExternalPCID, PCStatus                       string
	ExtensionGrantID                                                     string
	ZoneID                                                               string
	TariffID, TariffName                                                 string
	DurationMinutes                                                      int
	DurationSeconds                                                      int
	VoucherID                                                            string
	VoucherSeconds                                                       int
	AmountTiyin                                                          int64
	HourlyPriceTiyin                                                     int64
	ClickMerchantID, ClickServiceID, ClickMerchantUserID, ClickSecretKey string
	PaymeMerchantID, PaymeSecretKey                                      string
	PlatformFeeBPS                                                       int
	OFDMXIK, OFDPackageCode                                              string
}

type paymentOrderForCallback struct {
	ID, ClubID, PCID, Status, ExternalPCID string
	ExtensionGrantID                       *string
	VoucherID                              *string
	AmountTiyin                            int64
	DurationMinutes                        int
	DurationSeconds                        int
}

type paymentSuccess struct {
	Provider          string
	AmountTiyin       int64
	InvoiceID         string
	ProviderPaymentID string
	ReceiptURL        string
	PaidAt            time.Time
	PS                string
	CardPAN           string
}

type providerOrder struct {
	ID                string
	InvoiceID         string
	Provider          string
	ProviderPaymentID string
	ProviderPrepareID string
	ProviderTimeMS    int64
	AmountTiyin       int64
	Status            string
	PaidAt            *time.Time
	UpdatedAt         time.Time
	ClickSecretKey    string
	ClickServiceID    string
}

type paymeRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type paymeOrderParams struct {
	ID      string            `json:"id"`
	Time    int64             `json:"time"`
	Amount  int64             `json:"amount"`
	Account map[string]string `json:"account"`
}

type paymeTransactionParams struct {
	ID string `json:"id"`
}

type paymeCancelParams struct {
	ID     string `json:"id"`
	Reason int    `json:"reason"`
}

type paymeStatementParams struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type paymeAuthCredentials struct {
	MerchantID string
	SecretKey  string
}

type paymeAPIError struct {
	Code    int
	Message string
	Data    string
}

func (e paymeAPIError) Error() string {
	return e.Message
}

type cashSessionRequest struct {
	PCID            string `json:"pc_id"`
	TariffBlockID   string `json:"tariff_block_id"`
	AmountUZS       int64  `json:"amount_uzs"`
	Reason          string `json:"reason"`
	FiscalReference string `json:"fiscal_reference"`
}

type adminPCStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type endGrantRequest struct {
	Reason           string `json:"reason"`
	RemainingMinutes int    `json:"remaining_minutes"`
	RemainingSeconds int    `json:"remaining_seconds"`
	RecipientPhone   string `json:"recipient_phone"`
}

type createVoucherRequest struct {
	PaymentOrderID string `json:"payment_order_id"`
	MinutesLeft    int    `json:"minutes_left"`
	SecondsLeft    int    `json:"seconds_left"`
}

type redeemVoucherRequest struct {
	Code    string `json:"code"`
	QRToken string `json:"qr_token"`
}

type coreEventRequest struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"`
	ClubID        string         `json:"club_id"`
	ExternalPCID  string         `json:"external_pc_id"`
	CoreSessionID string         `json:"core_session_id"`
	GrantID       string         `json:"grant_id"`
	OccurredAt    string         `json:"occurred_at"`
	Payload       map[string]any `json:"payload"`
}

type edgeEventBatch struct {
	ClubID string      `json:"club_id"`
	Events []edgeEvent `json:"events"`
}

type edgeEvent struct {
	EventID       string         `json:"event_id"`
	Type          string         `json:"type"`
	ExternalPCID  string         `json:"external_pc_id"`
	CoreSessionID string         `json:"core_session_id"`
	GrantID       string         `json:"grant_id"`
	CodeHash      string         `json:"code_hash"`
	OccurredAt    string         `json:"occurred_at"`
	Payload       map[string]any `json:"payload"`
}

type telegramUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text    string          `json:"text"`
	Chat    telegramChat    `json:"chat"`
	From    telegramUser    `json:"from"`
	Contact telegramContact `json:"contact"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type telegramContact struct {
	PhoneNumber string `json:"phone_number"`
}

type telegramGetMeResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		Username string `json:"username"`
	} `json:"result"`
}

type telegramUpdatesResponse struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description"`
	Result      []telegramUpdate `json:"result"`
}

type telegramWebhookInfoResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		URL string `json:"url"`
	} `json:"result"`
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (authContext, bool) {
	auth, err := s.authFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return authContext{}, false
	}
	return auth, true
}

func (s *Server) authFromRequest(r *http.Request) (authContext, error) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" && s.cfg.AdminAPIToken != "" {
		token = bearerToken(r.Header.Get("X-Admin-Token"))
	}
	if token != "" && s.cfg.AdminAPIToken != "" && token == s.cfg.AdminAPIToken {
		return authContext{Name: "Admin token", GlobalRole: "super_admin"}, nil
	}
	if token != "" {
		var auth authContext
		err := s.db.QueryRow(r.Context(), `
			SELECT u.id, u.name, COALESCE(u.email, ''), COALESCE(u.phone, ''), COALESCE(u.global_role, '')
			FROM auth_sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.expires_at > now() AND u.status = 'active'
		`, hashToken(token)).Scan(&auth.UserID, &auth.Name, &auth.Email, &auth.Phone, &auth.GlobalRole)
		if err == nil {
			return auth, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return authContext{}, err
		}
	}
	if !strings.EqualFold(s.cfg.AppEnv, "production") {
		return authContext{Name: "Development Super Admin", GlobalRole: "super_admin"}, nil
	}
	return authContext{}, fmt.Errorf("auth required")
}

func (s *Server) authPayload(ctx context.Context, auth authContext) (map[string]any, error) {
	clubs, err := s.clubsForAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user": map[string]any{
			"id": auth.UserID, "name": auth.Name, "email": auth.Email, "phone": auth.Phone, "global_role": auth.GlobalRole,
		},
		"clubs": clubs,
	}, nil
}

func (s *Server) clubsForAuth(ctx context.Context, auth authContext) ([]map[string]any, error) {
	var rows pgx.Rows
	var err error
	if auth.IsSuperAdmin() {
		rows, err = s.db.Query(ctx, `
			SELECT id, name, COALESCE(slug, ''), status, 'super_admin' AS role
			FROM clubs
			WHERE status <> 'deleted'
			ORDER BY name
		`)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT c.id, c.name, COALESCE(c.slug, ''), c.status, ucr.role
			FROM user_club_roles ucr
			JOIN clubs c ON c.id = ucr.club_id
			WHERE ucr.user_id = $1 AND ucr.status = 'active' AND c.status <> 'deleted'
			ORDER BY c.name
		`, auth.UserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	clubs := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, slug, status, role string
		if err := rows.Scan(&id, &name, &slug, &status, &role); err != nil {
			return nil, err
		}
		clubs = append(clubs, map[string]any{"id": id, "name": name, "slug": slug, "status": status, "role": role})
	}
	return clubs, nil
}

func (s *Server) resolveRequestClub(w http.ResponseWriter, r *http.Request, auth authContext, roles ...string) (string, bool) {
	clubID := strings.TrimSpace(r.URL.Query().Get("club_id"))
	if clubID == "" {
		var err error
		clubID, err = s.defaultClubID(r.Context(), auth)
		if err != nil {
			writeError(w, http.StatusForbidden, "club access required")
			return "", false
		}
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, roles...); !ok {
		return "", false
	}
	return clubID, true
}

func (s *Server) defaultClubID(ctx context.Context, auth authContext) (string, error) {
	var id string
	if auth.IsSuperAdmin() {
		err := s.db.QueryRow(ctx, `SELECT id FROM clubs WHERE status = 'active' ORDER BY name LIMIT 1`).Scan(&id)
		return id, err
	}
	err := s.db.QueryRow(ctx, `
		SELECT club_id FROM user_club_roles
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at
		LIMIT 1
	`, auth.UserID).Scan(&id)
	return id, err
}

func (s *Server) requireClubRole(w http.ResponseWriter, r *http.Request, auth authContext, clubID string, roles ...string) (string, bool) {
	role, err := s.clubRole(r.Context(), auth, clubID)
	if err != nil {
		writeError(w, http.StatusForbidden, "club access required")
		return "", false
	}
	if auth.IsSuperAdmin() || roleAllowed(role, roles...) {
		return role, true
	}
	writeError(w, http.StatusForbidden, "not enough permissions")
	return "", false
}

func (s *Server) clubRole(ctx context.Context, auth authContext, clubID string) (string, error) {
	if clubID == "" {
		return "", fmt.Errorf("club_id is required")
	}
	if auth.IsSuperAdmin() {
		var exists bool
		err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clubs WHERE id = $1 AND status <> 'deleted')`, clubID).Scan(&exists)
		if err != nil || !exists {
			return "", fmt.Errorf("club access required")
		}
		return "super_admin", nil
	}
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT role FROM user_club_roles
		WHERE user_id = $1 AND club_id = $2 AND status = 'active'
	`, auth.UserID, clubID).Scan(&role)
	return role, err
}

func roleAllowed(role string, roles ...string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, allowed := range roles {
		if role == allowed {
			return true
		}
	}
	return false
}

func minutesForAmount(amountTiyin, hourlyPriceTiyin int64) int {
	return secondsToMinutesCeil(secondsForAmount(amountTiyin, hourlyPriceTiyin))
}

func secondsForAmount(amountTiyin, hourlyPriceTiyin int64) int {
	if amountTiyin <= 0 || hourlyPriceTiyin <= 0 {
		return 0
	}
	seconds := int((amountTiyin*3600 + hourlyPriceTiyin - 1) / hourlyPriceTiyin)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func priceTiyinFromUZS(priceTiyin, priceUZS int64) int64 {
	if priceTiyin > 0 {
		return priceTiyin
	}
	if priceUZS > 0 {
		return priceUZS * 100
	}
	return 0
}

func canAssignClubRole(actorRole string, targetRole string) bool {
	switch actorRole {
	case "super_admin":
		return targetRole == "owner" || targetRole == "admin"
	case "owner":
		return targetRole == "admin"
	default:
		return false
	}
}

func isClubRole(role string) bool {
	switch role {
	case "owner", "admin":
		return true
	default:
		return false
	}
}

func (s *Server) clubSettings(ctx context.Context, clubID string, includeTechnicalFields bool) (map[string]any, error) {
	var club map[string]any
	clubRows, err := s.db.Query(ctx, `
		SELECT id, name, COALESCE(slug, ''), COALESCE(legal_name, ''), COALESCE(tin, ''), COALESCE(address, ''),
		       timezone, status,
		       COALESCE(click_merchant_id, ''), COALESCE(click_service_id, ''), COALESCE(click_merchant_user_id, ''), COALESCE(click_secret_key, ''),
		       COALESCE(payme_merchant_id, ''), COALESCE(payme_secret_key, ''),
		       platform_fee_bps, COALESCE(ofd_mxik, ''), COALESCE(ofd_package_code, ''), created_at
		FROM clubs
		WHERE id = $1
	`, clubID)
	if err != nil {
		return nil, err
	}
	defer clubRows.Close()
	if clubRows.Next() {
		var id, name, slug, legalName, tin, address, timezone, status string
		var clickMerchantID, clickServiceID, clickMerchantUserID, clickSecretKey, paymeMerchantID, paymeSecretKey, mxik, packageCode string
		var feeBPS int
		var createdAt time.Time
		if err := clubRows.Scan(
			&id, &name, &slug, &legalName, &tin, &address, &timezone, &status,
			&clickMerchantID, &clickServiceID, &clickMerchantUserID, &clickSecretKey, &paymeMerchantID, &paymeSecretKey,
			&feeBPS, &mxik, &packageCode, &createdAt,
		); err != nil {
			return nil, err
		}
		clickConnected, _ := s.clickReady(clickMerchantID, clickServiceID, clickMerchantUserID, clickSecretKey)
		paymeConnected, _ := s.paymeReady(paymeMerchantID, paymeSecretKey)
		paymentConnected := clickConnected || paymeConnected
		payoutsConnected := feeBPS > 0
		fiscalConnected := mxik != "" && packageCode != ""
		if !includeTechnicalFields {
			clickMerchantID = ""
			clickServiceID = ""
			clickMerchantUserID = ""
			clickSecretKey = ""
			paymeMerchantID = ""
			paymeSecretKey = ""
			mxik = ""
			packageCode = ""
		}
		club = map[string]any{
			"id": id, "name": name, "slug": slug, "legal_name": legalName, "tin": tin, "address": address,
			"timezone": timezone, "status": status,
			"click_merchant_id": clickMerchantID, "click_service_id": clickServiceID,
			"click_merchant_user_id": clickMerchantUserID, "click_secret_key": clickSecretKey,
			"payme_merchant_id": paymeMerchantID, "payme_secret_key": paymeSecretKey,
			"platform_fee_bps": feeBPS, "ofd_mxik": mxik, "ofd_package_code": packageCode, "created_at": createdAt,
			"payment_connected": paymentConnected, "click_connected": clickConnected, "payme_connected": paymeConnected,
			"payouts_connected": payoutsConnected, "fiscal_connected": fiscalConnected,
		}
	}
	if club == nil {
		return nil, pgx.ErrNoRows
	}

	zones, err := s.listZones(ctx, clubID)
	if err != nil {
		return nil, err
	}
	tariffs, err := s.listTariffs(ctx, clubID)
	if err != nil {
		return nil, err
	}
	pcs, err := s.listPCsWithQR(ctx, clubID)
	if err != nil {
		return nil, err
	}
	users, err := s.listClubUsers(ctx, clubID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"club": club, "zones": zones, "tariffs": tariffs, "pcs": pcs, "users": users}, nil
}

func (s *Server) listZones(ctx context.Context, clubID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, hourly_price_tiyin, sort_order, status FROM zones WHERE club_id = $1 AND status <> 'deleted' ORDER BY sort_order, name`, clubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, status string
		var hourlyPrice int64
		var sortOrder int
		if err := rows.Scan(&id, &name, &hourlyPrice, &sortOrder, &status); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "name": name, "hourly_price_tiyin": hourlyPrice, "hourly_price_uzs": hourlyPrice / 100,
			"sort_order": sortOrder, "status": status,
		})
	}
	return result, nil
}

func (s *Server) listTariffs(ctx context.Context, clubID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.zone_id, z.name, t.name, t.duration_minutes, t.price_tiyin, t.sort_order, t.status
		FROM tariff_blocks t
		JOIN zones z ON z.id = t.zone_id
		WHERE t.club_id = $1 AND t.status <> 'deleted' AND z.status <> 'deleted'
		ORDER BY z.sort_order, t.sort_order, t.duration_minutes
	`, clubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, zoneID, zoneName, name, status string
		var duration, sortOrder int
		var price int64
		if err := rows.Scan(&id, &zoneID, &zoneName, &name, &duration, &price, &sortOrder, &status); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "zone_id": zoneID, "zone": zoneName, "name": name, "duration_minutes": duration,
			"price_tiyin": price, "price_uzs": price / 100, "sort_order": sortOrder, "status": status,
		})
	}
	return result, nil
}

func (s *Server) listPCsWithQR(ctx context.Context, clubID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.id, p.zone_id, z.name, p.external_pc_id, p.number, p.label,
		       CASE WHEN z.status = 'maintenance' THEN 'maintenance' ELSE p.status_cache END,
		       COALESCE(q.public_token, '')
		FROM pc_refs p
		JOIN zones z ON z.id = p.zone_id
		LEFT JOIN LATERAL (
			SELECT public_token FROM qr_codes
			WHERE pc_ref_id = p.id AND type = 'static_pc' AND status = 'active'
			ORDER BY created_at DESC
			LIMIT 1
		) q ON true
		WHERE p.club_id = $1 AND p.status_cache <> 'deleted' AND z.status <> 'deleted'
		ORDER BY p.number
	`, clubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, zoneID, zoneName, externalID, label, status, token string
		var number int
		if err := rows.Scan(&id, &zoneID, &zoneName, &externalID, &number, &label, &status, &token); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "zone_id": zoneID, "zone": zoneName, "external_pc_id": externalID, "number": number,
			"label": label, "status": status, "qr_token": token, "qr_url": qrURL(s.cfg.FrontendBaseURL, token),
		})
	}
	return result, nil
}

func (s *Server) listClubUsers(ctx context.Context, clubID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.name, COALESCE(u.email, ''), COALESCE(u.phone, ''), ucr.role, ucr.status, ucr.created_at
		FROM user_club_roles ucr
		JOIN users u ON u.id = ucr.user_id
		WHERE ucr.club_id = $1 AND ucr.status <> 'deleted'
		ORDER BY ucr.role, u.name
	`, clubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, email, phone, role, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &email, &phone, &role, &status, &createdAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "name": name, "email": email, "phone": phone, "role": role, "status": status, "created_at": createdAt,
		})
	}
	return result, nil
}

func (s *Server) clubIDForEntity(ctx context.Context, table, id string) (string, error) {
	allowed := map[string]string{
		"zones":         "SELECT club_id FROM zones WHERE id = $1",
		"tariff_blocks": "SELECT club_id FROM tariff_blocks WHERE id = $1",
		"pc_refs":       "SELECT club_id FROM pc_refs WHERE id = $1",
	}
	query, ok := allowed[table]
	if !ok {
		return "", fmt.Errorf("unsupported entity table")
	}
	var clubID string
	err := s.db.QueryRow(ctx, query, id).Scan(&clubID)
	return clubID, err
}

func (s *Server) zoneBelongsToClub(ctx context.Context, zoneID, clubID string) bool {
	var ok bool
	_ = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1 AND club_id = $2 AND status <> 'deleted')`, zoneID, clubID).Scan(&ok)
	return ok
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeClickResponse(w http.ResponseWriter, clickTransID, merchantTransID, merchantPrepareID string, code int, note string, extra ...any) {
	payload := map[string]any{
		"click_trans_id":    clickTransID,
		"merchant_trans_id": merchantTransID,
		"error":             code,
		"error_note":        note,
	}
	if merchantPrepareID != "" {
		payload["merchant_prepare_id"] = merchantPrepareID
	}
	for i := 0; i+1 < len(extra); i += 2 {
		key, ok := extra[i].(string)
		if ok {
			payload[key] = extra[i+1]
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func writePaymeResult(w http.ResponseWriter, id any, result any) {
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func writePaymeError(w http.ResponseWriter, id any, code int, message string, data ...string) {
	errorPayload := map[string]any{
		"code": code,
		"message": map[string]string{
			"ru": message,
			"uz": message,
			"en": message,
		},
	}
	if len(data) > 0 && strings.TrimSpace(data[0]) != "" {
		errorPayload["data"] = strings.TrimSpace(data[0])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorPayload,
	})
}

func (s *Server) ensureNoAcceptedGrants(ctx context.Context, query string, arg any, message string) error {
	var exists bool
	if err := s.db.QueryRow(ctx, query, arg).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func writeConflictIfUnique(w http.ResponseWriter, err error, message string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeError(w, http.StatusConflict, message)
		return true
	}
	return false
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Edge-Token, X-Telegram-Bot-Api-Secret-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomHex(bytesLen int) string {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToUpper(code))))
	return hex.EncodeToString(sum[:])
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("clubpay-demo-salt:" + password))
	return hex.EncodeToString(sum[:])
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		isAlphaNum := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if isAlphaNum {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "club-" + randomHex(3)
	}
	return result
}

func (s *Server) uniqueExternalPCID(ctx context.Context, clubID, label, currentPCID string) string {
	base := slugify(label)
	if strings.HasPrefix(base, "club-") {
		base = "pc-" + randomHex(3)
	}
	for index := 0; index < 200; index++ {
		candidate := base
		if index > 0 {
			candidate = fmt.Sprintf("%s-%d", base, index+1)
		}
		var exists bool
		err := s.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pc_refs
				WHERE club_id = $1 AND external_pc_id = $2 AND id::text <> COALESCE(NULLIF($3, ''), '00000000-0000-0000-0000-000000000000')
			)
		`, clubID, candidate, currentPCID).Scan(&exists)
		if err != nil || !exists {
			return candidate
		}
	}
	return base + "-" + randomHex(3)
}

func qrURL(baseURL, token string) string {
	if token == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/qr/" + token
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (s *Server) paymentProviderOptions(clickMerchantID, clickServiceID, clickMerchantUserID, clickSecretKey, paymeMerchantID, paymeSecretKey string) []map[string]any {
	paymeReady, paymeMessage := s.paymeReady(paymeMerchantID, paymeSecretKey)
	clickReady, clickMessage := s.clickReady(clickMerchantID, clickServiceID, clickMerchantUserID, clickSecretKey)
	return []map[string]any{
		{
			"provider":   payments.ProviderPayme,
			"label":      "Payme",
			"configured": paymeReady,
			"sandbox":    s.isPaymeSandbox(),
			"message":    paymeMessage,
		},
		{
			"provider":   payments.ProviderClick,
			"label":      "Click",
			"configured": clickReady,
			"sandbox":    false,
			"message":    clickMessage,
		},
	}
}

func (s *Server) ensureCheckoutProviderReady(provider string, seed checkoutSeed) error {
	switch provider {
	case payments.ProviderPayme:
		ready, message := s.paymeReady(seed.PaymeMerchantID, seed.PaymeSecretKey)
		if !ready {
			return errors.New(message)
		}
	case payments.ProviderClick:
		ready, message := s.clickReady(seed.ClickMerchantID, seed.ClickServiceID, seed.ClickMerchantUserID, seed.ClickSecretKey)
		if !ready {
			return errors.New(message)
		}
	case payments.ProviderMock:
		if strings.EqualFold(s.cfg.AppEnv, "production") {
			return errors.New("Тестовая оплата отключена в production")
		}
	}
	return nil
}

func (s *Server) isPaymeSandbox() bool {
	return strings.Contains(strings.ToLower(s.cfg.PaymeCheckoutURL), "test.paycom.uz")
}

func (s *Server) paymeReady(clubMerchantID, clubSecretKey string) (bool, string) {
	merchantID := defaultString(clubMerchantID, s.cfg.PaymeMerchantID)
	secretKey := defaultString(clubSecretKey, s.cfg.PaymeSecretKey)
	missing := make([]string, 0, 2)
	if !usableProviderCredential(merchantID) {
		missing = append(missing, "merchant ID")
	}
	if !usableProviderCredential(secretKey) {
		missing = append(missing, "TEST_KEY/secret key")
	}
	if len(missing) > 0 {
		return false, "Payme не настроен: укажите " + strings.Join(missing, " и ")
	}
	if s.isPaymeSandbox() {
		return true, "Payme sandbox готов"
	}
	return true, "Payme готов"
}

func (s *Server) clickReady(clubMerchantID, clubServiceID, clubMerchantUserID, clubSecretKey string) (bool, string) {
	merchantID := defaultString(clubMerchantID, s.cfg.ClickMerchantID)
	serviceID := defaultString(clubServiceID, s.cfg.ClickServiceID)
	merchantUserID := defaultString(clubMerchantUserID, s.cfg.ClickMerchantUserID)
	secretKey := defaultString(clubSecretKey, s.cfg.ClickSecretKey)
	missing := make([]string, 0, 4)
	if !usableProviderCredential(merchantID) {
		missing = append(missing, "merchant ID")
	}
	if !usableProviderCredential(serviceID) {
		missing = append(missing, "service ID")
	}
	if !usableProviderCredential(merchantUserID) {
		missing = append(missing, "merchant user ID")
	}
	if !usableProviderCredential(secretKey) {
		missing = append(missing, "secret key")
	}
	if len(missing) > 0 {
		return false, "Click не настроен: укажите " + strings.Join(missing, ", ")
	}
	return true, "Click готов"
}

func usableProviderCredential(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	placeholders := []string{
		"test_merchant",
		"test_secret",
		"click_test_",
		"payme_test_",
		"placeholder",
		"your_",
		"{",
		"}",
	}
	for _, placeholder := range placeholders {
		if strings.Contains(lower, placeholder) {
			return false
		}
	}
	return true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.AdminAPIToken == "" {
		return true
	}
	if bearerToken(r.Header.Get("Authorization")) == s.cfg.AdminAPIToken {
		return true
	}
	writeError(w, http.StatusUnauthorized, "admin token required")
	return false
}

func (s *Server) requireCore(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.CoreToken == "" {
		return true
	}
	if bearerToken(r.Header.Get("Authorization")) == s.cfg.CoreToken {
		return true
	}
	writeError(w, http.StatusUnauthorized, "core token required")
	return false
}

func (s *Server) requireEdge(w http.ResponseWriter, r *http.Request) bool {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Edge-Token"))
	}
	if s.cfg.EdgeSyncToken == "" {
		return !strings.EqualFold(s.cfg.AppEnv, "production")
	}
	if token == s.cfg.EdgeSyncToken || (s.cfg.CoreToken != "" && token == s.cfg.CoreToken) {
		return true
	}
	writeError(w, http.StatusUnauthorized, "edge token required")
	return false
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return value
}

func normalizePhone(value string) string {
	digits := strings.Builder{}
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	phone := digits.String()
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "998") {
		return "+" + phone
	}
	if len(phone) == 9 {
		return "+998" + phone
	}
	if strings.HasPrefix(strings.TrimSpace(value), "+") {
		return "+" + phone
	}
	return "+" + phone
}

func isPayablePCStatus(status string) bool {
	return status == "available" || status == "sleeping"
}

func isKnownPCStatus(status string) bool {
	switch status {
	case "available", "occupied", "sleeping", "offline", "maintenance", "blocked", "unknown":
		return true
	default:
		return false
	}
}

func parseOptionalTime(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.FixedZone("UZT", 5*60*60)); err == nil {
		return &parsed
	}
	return nil
}

func formPayload(r *http.Request) map[string]any {
	result := make(map[string]any, len(r.Form))
	for key, values := range r.Form {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = values
		}
	}
	return result
}

func clickAmountToTiyin(value string) int64 {
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(parsed * 100))
}

func randomNumericID() string {
	value := strconv.FormatInt(time.Now().UnixNano(), 10)
	if len(value) > 12 {
		value = value[len(value)-12:]
	}
	return value
}

func unixMilli(value any) int64 {
	switch typed := value.(type) {
	case time.Time:
		if typed.IsZero() {
			return 0
		}
		return typed.UnixMilli()
	case *time.Time:
		if typed == nil || typed.IsZero() {
			return 0
		}
		return typed.UnixMilli()
	default:
		return 0
	}
}

func paymeOrderRef(rpc paymeRPCRequest) string {
	var params paymeOrderParams
	if json.Unmarshal(rpc.Params, &params) == nil && params.Account != nil {
		return params.Account["order_id"]
	}
	return ""
}

func paymeEventExternalID(rpc paymeRPCRequest, orderRef string) string {
	if orderRef != "" {
		return orderRef
	}
	var params paymeTransactionParams
	if json.Unmarshal(rpc.Params, &params) == nil {
		return params.ID
	}
	return ""
}

func paymeErr(code int, message string, data ...string) error {
	apiErr := paymeAPIError{Code: code, Message: message}
	if len(data) > 0 {
		apiErr.Data = strings.TrimSpace(data[0])
	}
	return apiErr
}

func paymeErrorCode(err error) int {
	var paymeErr paymeAPIError
	if errors.As(err, &paymeErr) {
		return paymeErr.Code
	}
	return -32400
}

func paymeErrorData(err error) string {
	var paymeErr paymeAPIError
	if errors.As(err, &paymeErr) {
		return paymeErr.Data
	}
	return ""
}

func paymeState(status string) int {
	switch status {
	case "paid":
		return 2
	case "failed":
		return -1
	case "refunded":
		return -2
	default:
		return 1
	}
}

func stringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func intFromPayload(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func secondsToMinutesCeil(seconds int) int {
	if seconds <= 0 {
		return 0
	}
	return (seconds + 59) / 60
}

func voucherSeconds(seconds, minutes int) int {
	if seconds > 0 {
		return seconds
	}
	if minutes > 0 {
		return minutes * 60
	}
	return 0
}

func formatDurationForMessage(seconds int) string {
	if seconds <= 0 {
		return "0 сек"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	rest := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, rest)
	}
	if minutes > 0 {
		return fmt.Sprintf("%02d:%02d", minutes, rest)
	}
	return fmt.Sprintf("%d сек", rest)
}

func (s *Server) expireElapsedGrants(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		WITH expired AS (
			UPDATE game_access_grants
			SET status = 'ended',
			    ended_at = COALESCE(planned_ends_at, accepted_at + make_interval(secs => duration_seconds), accepted_at + make_interval(mins => duration_minutes), now()),
			    end_reason = 'time_expired',
			    remaining_minutes = 0,
			    remaining_seconds = 0
			WHERE status = 'accepted'
			  AND (
				(planned_ends_at IS NOT NULL AND planned_ends_at <= now())
				OR (planned_ends_at IS NULL AND accepted_at IS NOT NULL AND accepted_at + make_interval(secs => duration_seconds) <= now())
				OR (planned_ends_at IS NULL AND accepted_at IS NOT NULL AND duration_seconds <= 0 AND accepted_at + make_interval(mins => duration_minutes) <= now())
			  )
			RETURNING pc_ref_id
		)
		UPDATE pc_refs p
		SET status_cache = 'available'
		WHERE p.status_cache = 'occupied'
		  AND p.id IN (SELECT pc_ref_id FROM expired)
		  AND NOT EXISTS (
			SELECT 1
			FROM game_access_grants g
			WHERE g.pc_ref_id = p.id
			  AND g.status = 'accepted'
			  AND (
				(g.planned_ends_at IS NOT NULL AND g.planned_ends_at > now())
				OR (g.planned_ends_at IS NULL AND g.accepted_at IS NOT NULL AND g.duration_seconds > 0 AND g.accepted_at + make_interval(secs => g.duration_seconds) > now())
				OR (g.planned_ends_at IS NULL AND g.accepted_at IS NOT NULL AND g.duration_seconds <= 0 AND g.accepted_at + make_interval(mins => g.duration_minutes) > now())
				OR (g.planned_ends_at IS NULL AND g.accepted_at IS NULL)
			  )
		  )
	`)
	return err
}

func (s *Server) syncCorePCStatus(ctx context.Context, pcID, externalPCID, fallback string) string {
	if !strings.EqualFold(s.cfg.CoreMode, "http") || externalPCID == "" {
		return fallback
	}
	status, err := s.core.GetPCStatus(ctx, externalPCID)
	if err != nil || !isKnownPCStatus(status.Status) {
		return fallback
	}
	_, _ = s.db.Exec(ctx, `UPDATE pc_refs SET status_cache = $1 WHERE id = $2`, status.Status, pcID)
	return status.Status
}

func (s *Server) splitAmounts(amountTiyin int64) (platformAmount int64, clubAmount int64) {
	if s.cfg.PlatformFeeBPS <= 0 {
		return 0, amountTiyin
	}
	platformAmount = amountTiyin * int64(s.cfg.PlatformFeeBPS) / 10000
	if platformAmount < 0 {
		platformAmount = 0
	}
	if platformAmount > amountTiyin {
		platformAmount = amountTiyin
	}
	return platformAmount, amountTiyin - platformAmount
}

func (s *Server) splitAmountsForClub(amountTiyin int64, feeBPS int) (platformAmount int64, clubAmount int64) {
	if feeBPS <= 0 {
		feeBPS = s.cfg.PlatformFeeBPS
	}
	if feeBPS <= 0 {
		return 0, amountTiyin
	}
	platformAmount = amountTiyin * int64(feeBPS) / 10000
	if platformAmount < 0 {
		platformAmount = 0
	}
	if platformAmount > amountTiyin {
		platformAmount = amountTiyin
	}
	return platformAmount, amountTiyin - platformAmount
}
