package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type mobileOperationContext struct{ Player, Phone, Key string }
type mobileOperationKey struct{}

func (s *Server) mobileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mobile/auth/challenge", s.handleMobileChallenge)
	mux.HandleFunc("POST /api/mobile/auth/verify", s.handleMobileVerify)
	mux.HandleFunc("POST /api/mobile/auth/refresh", s.handleMobileRefresh)
	mux.HandleFunc("POST /api/mobile/auth/logout", s.handleMobileLogout)
	mux.HandleFunc("GET /api/mobile/me", s.handleMobileMe)
	mux.HandleFunc("GET /api/mobile/balances", s.handleMobileBalances)
	mux.HandleFunc("GET /api/mobile/clubs", s.handleMobileClubs)
	mux.HandleFunc("GET /api/mobile/clubs/{club_id}", s.handleMobileClub)
	mux.HandleFunc("GET /api/mobile/orders/{invoice_id}", s.handleMobileOrder)
	mux.HandleFunc("GET /api/mobile/sessions/{grant_id}", s.handleMobileSession)
	mux.HandleFunc("GET /api/mobile/operations/{key}", s.handleMobileOperation)
	mux.HandleFunc("POST /api/mobile/payments/test/success/{invoice_id}", s.handleMobileTestPaymentSuccess)
}

// Existing web callers retain their flow. Mobile callers require a durable key.
// A lost response can be recovered through /operations without another charge.
func (s *Server) mobileMutation(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer mob_") {
			next(w, r)
			return
		}
		p, ok := s.requireMobile(w, r)
		if !ok {
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`).MatchString(key) {
			writeError(w, 400, "idempotency_key_required")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			writeError(w, 400, "invalid_request")
			return
		}
		requestHash := hashToken(r.URL.Path + string(body))
		tag, err := s.db.Exec(r.Context(), `INSERT INTO mobile_operations(player_id,request_key,request_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, p.ID, key, requestHash)
		if err != nil {
			mobileInternal(w)
			return
		}
		if tag.RowsAffected() == 0 {
			var stored string
			var status *int
			var response []byte
			err = s.db.QueryRow(r.Context(), `SELECT request_hash,http_status,response FROM mobile_operations WHERE player_id=$1 AND request_key=$2`, p.ID, key).Scan(&stored, &status, &response)
			if err != nil {
				mobileInternal(w)
				return
			}
			if stored != requestHash {
				writeError(w, 409, "idempotency_conflict")
				return
			}
			if status == nil {
				writeError(w, 409, "operation_pending")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(*status)
			_, _ = w.Write(response)
			return
		}
		ctx := context.WithValue(r.Context(), mobileOperationKey{}, mobileOperationContext{p.ID, p.Phone, key})
		// Finish the operation after a browser disconnect, bounded by the API timeout.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = io.NopCloser(bytes.NewReader(body))
		recorder := httptest.NewRecorder()
		next(recorder, r)
		response := recorder.Body.Bytes()
		if recorder.Code >= 400 {
			response = []byte(`{"error":"operation_failed"}`)
		}
		_, err = s.db.Exec(ctx, `UPDATE mobile_operations SET http_status=$3,response=$4 WHERE player_id=$1 AND request_key=$2`, p.ID, key, recorder.Code, response)
		if err != nil {
			mobileInternal(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(response)
	}
}

// Test payment is deliberately a separate mobile-only endpoint.  The public
// mock endpoint remains restricted to development mode and cannot be used to
// turn a real club QR into a free-session link.
func (s *Server) handleMobileTestPaymentSuccess(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	if !s.mobileTestPaymentAllowed(p) {
		writeError(w, http.StatusForbidden, "mobile_test_payment_not_allowed")
		return
	}
	invoiceID := r.PathValue("invoice_id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice_id is required")
		return
	}
	grantID, err := s.completeMockPayment(r.Context(), invoiceID, p.ID)
	if errors.Is(err, errMockOrderNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "grant_id": grantID})
}
func linkMobileOperation(ctx context.Context, tx pgx.Tx, invoice, grant string) error {
	op, ok := ctx.Value(mobileOperationKey{}).(mobileOperationContext)
	if !ok {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE mobile_operations SET invoice_id=NULLIF($3,''),grant_id=NULLIF($4,'')::uuid WHERE player_id=$1 AND request_key=$2`, op.Player, op.Key, invoice, grant)
	return err
}
func (s *Server) handleMobileOperation(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	var status *int
	var response []byte
	var invoice, grant string
	err := s.db.QueryRow(r.Context(), `SELECT http_status,response,COALESCE(invoice_id,''),COALESCE(grant_id::text,'') FROM mobile_operations WHERE player_id=$1 AND request_key=$2`, p.ID, r.PathValue("key")).Scan(&status, &response, &invoice, &grant)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "operation_not_found")
		return
	}
	if err != nil {
		mobileInternal(w)
		return
	}
	var value any
	if len(response) > 0 {
		_ = json.Unmarshal(response, &value)
	}
	writeJSON(w, 200, map[string]any{"http_status": status, "response": value, "invoice_id": invoice, "grant_id": grant})
}
func (s *Server) handleMobileOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	rows, err := s.queryMaps(r.Context(), `SELECT po.invoice_id,po.status,po.duration_seconds,po.checkout_url,pc.label AS pc_label,
 COALESCE(g.id::text,'') AS grant_id,COALESCE(g.status,'pending') AS grant_status,
 COALESCE(g.duration_seconds,po.duration_seconds) AS session_seconds,g.planned_ends_at
 FROM payment_orders po JOIN pc_refs pc ON pc.id=po.pc_ref_id
 LEFT JOIN LATERAL (SELECT * FROM game_access_grants WHERE payment_order_id=po.id ORDER BY created_at DESC LIMIT 1) g ON true
 WHERE po.invoice_id=$1 AND po.player_id=$2`, r.PathValue("invoice_id"), p.ID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if len(rows) == 0 {
		writeError(w, 404, "order_not_found")
		return
	}
	writeJSON(w, 200, rows[0])
}
func (s *Server) handleMobileSession(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireMobile(w, r)
	if !ok {
		return
	}
	rows, err := s.queryMaps(r.Context(), `SELECT g.id AS grant_id,g.status AS grant_status,g.duration_seconds AS session_seconds,p.label AS pc_label,g.planned_ends_at
 FROM game_access_grants g JOIN pc_refs p ON p.id=g.pc_ref_id WHERE g.id::text=$1 AND g.player_id=$2`, r.PathValue("grant_id"), p.ID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if len(rows) == 0 {
		writeError(w, 404, "session_not_found")
		return
	}
	writeJSON(w, 200, rows[0])
}
func (s *Server) paymentReturnURL(ctx context.Context, invoice string) string {
	var mobileURL *string
	err := s.db.QueryRow(ctx, `SELECT client_return_url FROM payment_orders WHERE invoice_id=$1`, invoice).Scan(&mobileURL)
	if err == nil && mobileURL != nil && *mobileURL != "" {
		return *mobileURL
	}
	return s.cfg.FrontendBaseURL + "/payment/return?invoice_id=" + url.QueryEscape(invoice)
}
func (s *Server) mobileReturnURL(invoice string) string {
	return strings.TrimRight(s.cfg.MobileReturnBaseURL, "/") + "/payment/return?invoice_id=" + url.QueryEscape(invoice)
}
