package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mobileTestTransport func(*http.Request) (*http.Response, error)

func (f mobileTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMobileInputAndSecrets(t *testing.T) {
	for _, phone := range []string{"+998901234567", "+998991112233"} {
		if !mobilePhonePattern.MatchString(phone) {
			t.Fatal("valid phone rejected")
		}
	}
	for _, phone := range []string{"+79012345678", "998901234567", "+9989012345678", "+99890abc4567"} {
		if mobilePhonePattern.MatchString(phone) {
			t.Fatal("invalid phone accepted")
		}
	}
	secret := mobileSecret("mob_a_")
	if len(secret) != 70 || secret == hashToken(secret) {
		t.Fatal("bad token")
	}
	if mobileOTPHash("pepper1", "123456") == mobileOTPHash("pepper2", "123456") {
		t.Fatal("OTP not bound to pepper")
	}
	s := NewServer(config.Config{}, nil, core.NewMockAdapter())
	for _, path := range []string{"/api/mobile/me", "/api/mobile/balances", "/api/mobile/orders/test", "/api/mobile/sessions/test"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer auth_qr_token")
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s accepted QR credentials", path)
		}
	}
}

func TestMobileTestPaymentAccess(t *testing.T) {
	profile := playerIdentity{Phone: "+998901234567"}
	if !(&Server{cfg: config.Config{MobileTestPaymentsEnabled: true}}).mobileTestPaymentAllowed(profile) {
		t.Fatal("an empty beta allowlist must enable test payments for every verified mobile profile")
	}
	if (&Server{cfg: config.Config{MobileTestPaymentsEnabled: false}}).mobileTestPaymentAllowed(profile) {
		t.Fatal("disabled beta test payments must remain unavailable")
	}
	if (&Server{cfg: config.Config{MobileTestPaymentsEnabled: true}}).mobileTestPaymentAllowed(playerIdentity{}) {
		t.Fatal("profiles without a verified phone must not access test payments")
	}
}

// Uses an isolated schema in an explicitly configured disposable PostgreSQL DB.
// Telegram HTTP calls are intercepted; no real messages or payments are sent.
func TestMobileIntegration(t *testing.T) {
	databaseURL := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MOBILE_TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "mobile_test_" + randomHex(6)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	// Migration must be restart-safe.
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	s := NewServer(config.Config{TelegramBotToken: "test-bot-pepper", TelegramBotUsername: "clubpay_test", TelegramWebhookSecret: "test-hook", MobileReturnBaseURL: "http://localhost:7357", FrontendBaseURL: "http://localhost:5173", PublicBaseURL: "http://localhost:8080", DefaultPaymentProvider: "mock", MobileTestPaymentsEnabled: true, MobileTestPaymentPhones: []string{"+998901234567"}, SessionGraceSeconds: 180}, pool, core.NewMockAdapter())
	var mu sync.Mutex
	var delivered string
	original := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: mobileTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.telegram.org" {
			return nil, fmt.Errorf("unexpected network destination")
		}
		var j map[string]any
		_ = json.NewDecoder(r.Body).Decode(&j)
		mu.Lock()
		delivered, _ = j["text"].(string)
		mu.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	})}
	defer func() { http.DefaultClient = original }()
	call := func(method, path, token, key string, body any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.RemoteAddr = "127.0.0.1:1234"
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, r)
		var j map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &j)
		return w.Code, j
	}
	expect := func(want int, method, path, token, key string, body any) map[string]any {
		t.Helper()
		code, j := call(method, path, token, key, body)
		if code != want {
			t.Fatalf("%s %s: got %d want %d: %v", method, path, code, want, j)
		}
		return j
	}
	device := strings.Repeat("d", 32)
	challenge := func(phone string) string {
		return expect(201, "POST", "/api/mobile/auth/challenge", "", "", map[string]any{"phone": phone, "device_id": device})["challenge"].(string)
	}
	issueOTP := func(ch string, chat int64, phone string) string {
		t.Helper()
		message := telegramMessage{Text: "/start " + ch, Chat: telegramChat{ID: chat}, From: telegramUser{ID: chat, FirstName: "Test Player"}}
		if _, err := s.processTelegramUpdate(ctx, telegramUpdate{Message: message}); err != nil {
			t.Fatal(err)
		}
		message.Text = ""
		message.Contact = telegramContact{PhoneNumber: phone, UserID: chat}
		if _, err := s.processTelegramUpdate(ctx, telegramUpdate{Message: message}); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		text := delivered
		mu.Unlock()
		code := regexp.MustCompile(`[0-9]{6}`).FindString(text)
		if code == "" {
			t.Fatal("OTP not delivered")
		}
		return code
	}
	ch := challenge("+998901234567")
	// Wrong contact and forwarded contact cannot verify the requested phone.
	msg := telegramMessage{Text: "/start " + ch, Chat: telegramChat{ID: 77}, From: telegramUser{ID: 77}}
	if _, err = s.processTelegramUpdate(ctx, telegramUpdate{Message: msg}); err != nil {
		t.Fatal(err)
	}
	for _, contact := range []telegramContact{{PhoneNumber: "+998901111111", UserID: 77}, {PhoneNumber: "+998901234567", UserID: 88}, {PhoneNumber: "+998901234567", UserID: 0}} {
		msg.Text = ""
		msg.Contact = contact
		if _, err = s.processTelegramUpdate(ctx, telegramUpdate{Message: msg}); err != nil {
			t.Fatal(err)
		}
		var status string
		_ = pool.QueryRow(ctx, `SELECT status FROM mobile_auth_challenges WHERE token_hash=$1`, hashToken(ch)).Scan(&status)
		if status != "contact" {
			t.Fatal("foreign contact accepted")
		}
	}
	msg.Contact = telegramContact{PhoneNumber: "+998901234567", UserID: 77}
	if _, err = s.processTelegramUpdate(ctx, telegramUpdate{Message: msg}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	otp := regexp.MustCompile(`[0-9]{6}`).FindString(delivered)
	mu.Unlock()
	if otp == "" {
		t.Fatal("missing OTP")
	}
	verify := map[string]any{"challenge": ch, "device_id": device, "otp": otp}
	expect(401, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": strings.Repeat("x", 32), "otp": otp})
	tokens := expect(200, "POST", "/api/mobile/auth/verify", "", "", verify)
	access := tokens["access_token"].(string)
	refresh := tokens["refresh_token"].(string)
	expect(401, "POST", "/api/mobile/auth/verify", "", "", verify)
	me := expect(200, "GET", "/api/mobile/me", access, "", nil)
	if me["phone"] != "+998901234567" {
		t.Fatal("phone profile mismatch")
	}
	balances := expect(200, "GET", "/api/mobile/balances", access, "", nil)
	if len(balances["balances"].([]any)) != 0 {
		t.Fatal("fake balance")
	}
	// A previously confirmed bot contact receives a new mobile OTP directly
	// from a signed app link; it must not fall through to voucher delivery.
	knownPhone := "+998906666666"
	knownChat := int64(66)
	if _, err = pool.Exec(ctx, `INSERT INTO telegram_users(phone,chat_id,status) VALUES($1,$2,'active')`, knownPhone, fmt.Sprint(knownChat)); err != nil {
		t.Fatal(err)
	}
	ch = challenge(knownPhone)
	mu.Lock()
	delivered = ""
	mu.Unlock()
	msg = telegramMessage{Text: "/start " + ch, Chat: telegramChat{ID: knownChat}, From: telegramUser{ID: knownChat, FirstName: "Known Player"}}
	if _, err = s.processTelegramUpdate(ctx, telegramUpdate{Message: msg}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	knownMessage := delivered
	mu.Unlock()
	if !strings.Contains(knownMessage, "Код входа ClubPay") || strings.Contains(knownMessage, "Ваш ваучер") {
		t.Fatalf("known mobile binding did not receive an OTP: %q", knownMessage)
	}
	knownOTP := regexp.MustCompile(`[0-9]{6}`).FindString(knownMessage)
	if knownOTP == "" {
		t.Fatal("known mobile binding received no OTP digits")
	}
	expect(200, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": device, "otp": knownOTP})
	var rawStored int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM mobile_tokens WHERE token_hash=$1 OR token_hash=$2`, access, refresh).Scan(&rawStored)
	if rawStored != 0 {
		t.Fatal("raw token persisted")
	}
	rotated := expect(200, "POST", "/api/mobile/auth/refresh", "", "", map[string]any{"refresh_token": refresh, "device_id": device})
	expect(401, "GET", "/api/mobile/me", access, "", nil)
	access = rotated["access_token"].(string)
	expect(200, "GET", "/api/mobile/me", access, "", nil)
	expect(401, "POST", "/api/mobile/auth/refresh", "", "", map[string]any{"refresh_token": refresh, "device_id": device})
	expect(401, "GET", "/api/mobile/me", access, "", nil)
	ch = challenge("+998901234567")
	otp = issueOTP(ch, 77, "+998901234567")
	tokens = expect(200, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": device, "otp": otp})
	access = tokens["access_token"].(string)
	// Seed game time directly in the fixture, preserving the existing ledger path.
	var club, pc, qr, tariff string
	err = pool.QueryRow(ctx, `SELECT c.id,p.id,q.public_token,t.id FROM clubs c JOIN pc_refs p ON p.club_id=c.id JOIN qr_codes q ON q.pc_ref_id=p.id JOIN tariff_blocks t ON t.zone_id=p.zone_id WHERE q.type='static_pc' LIMIT 1`).Scan(&club, &pc, &qr, &tariff)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.recordPlayerTime(ctx, tx, me["id"].(string), club, 725, "manual_adjustment", "", "", "fixture-seconds"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	balances = expect(200, "GET", "/api/mobile/balances", access, "", nil)
	balance := balances["balances"].([]any)[0].(map[string]any)
	if balance["seconds_balance"] != float64(725) {
		t.Fatal("seconds changed")
	}
	if amount, ok := balance["balance_uzs"].(float64); !ok || amount <= 0 {
		t.Fatalf("missing monetary club balance: %#v", balance["balance_uzs"])
	}
	body := map[string]any{"qr_token": qr, "tariff_block_id": tariff, "payment_provider": "mock"}
	publicQR := expect(200, "GET", "/api/qr/"+qr, "", "", nil)
	for _, provider := range publicQR["payment_providers"].([]any) {
		if provider.(map[string]any)["provider"] == "mock" {
			t.Fatal("public QR exposed mobile test payment")
		}
	}
	authorizedQR := expect(200, "GET", "/api/qr/"+qr, access, "", nil)
	var mobileTestProvider bool
	for _, provider := range authorizedQR["payment_providers"].([]any) {
		if provider.(map[string]any)["provider"] == "mock" && provider.(map[string]any)["configured"] == true {
			mobileTestProvider = true
		}
	}
	if !mobileTestProvider {
		t.Fatal("allowed mobile profile did not receive test payment")
	}
	expect(400, "POST", "/api/checkouts", access, "", body)
	bodyWithToken := map[string]any{"qr_token": qr, "tariff_block_id": tariff, "payment_provider": "mock", "player_auth_token": access}
	expect(401, "POST", "/api/checkouts", "", "", bodyWithToken)
	checkout := expect(201, "POST", "/api/checkouts", access, "checkout-test-key-001", body)
	duplicate := expect(201, "POST", "/api/checkouts", access, "checkout-test-key-001", body)
	invoice := checkout["order"].(map[string]any)["invoice_id"].(string)
	if duplicate["order"].(map[string]any)["invoice_id"] != invoice {
		t.Fatal("duplicate order")
	}
	expect(409, "POST", "/api/checkouts", access, "checkout-test-key-001", map[string]any{"qr_token": qr, "amount_uzs": 5000})
	status := expect(200, "GET", "/api/mobile/orders/"+invoice, access, "", nil)
	if status["grant_status"] == "accepted" {
		t.Fatal("unpaid session marked started")
	}
	expect(404, "GET", "/api/mobile/orders/cp_other", access, "", nil)
	expect(403, "POST", "/api/payments/mock/success/"+invoice, "", "", nil)
	expect(401, "POST", "/api/mobile/payments/test/success/"+invoice, "", "", nil)
	expect(200, "POST", "/api/mobile/payments/test/success/"+invoice, access, "", nil)
	status = expect(200, "GET", "/api/mobile/orders/"+invoice, access, "", nil)
	if status["grant_status"] != "pending" {
		t.Fatalf("cloud grant should wait for Controller: %v", status)
	}
	controller := NewServer(config.Config{NodeMode: "edge", SessionGraceSeconds: 180}, pool, core.NewMockAdapter())
	if !controller.startPendingEdgeGrants(ctx, club) {
		t.Fatal("controller did not process pending grant")
	}
	status = expect(200, "GET", "/api/mobile/orders/"+invoice, access, "", nil)
	if status["grant_status"] != "accepted" {
		t.Fatalf("grant not accepted: %v", status)
	}
	var ledgerBalance int
	_ = pool.QueryRow(ctx, `SELECT seconds_balance FROM player_club_balances WHERE player_id=$1 AND club_id=$2`, me["id"], club).Scan(&ledgerBalance)
	if ledgerBalance != 0 {
		t.Fatal("existing time was not auto-applied")
	}
	// Foreign mobile player cannot read the order/grant or operation.
	ch2 := challenge("+998902222222")
	otp2 := issueOTP(ch2, 88, "+998902222222")
	other := expect(200, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch2, "device_id": device, "otp": otp2})["access_token"].(string)
	expect(403, "POST", "/api/mobile/payments/test/success/"+invoice, other, "", nil)
	expect(404, "GET", "/api/mobile/orders/"+invoice, other, "", nil)
	expect(404, "GET", "/api/mobile/sessions/"+status["grant_id"].(string), other, "", nil)
	expect(404, "GET", "/api/mobile/operations/checkout-test-key-001", other, "", nil)
	// End the profile session through the existing mechanism and return seconds.
	if _, err = s.finishGrant(ctx, status["grant_id"].(string), "player_end", 321); err != nil {
		t.Fatal(err)
	}
	balances = expect(200, "GET", "/api/mobile/balances", access, "", nil)
	if balances["balances"].([]any)[0].(map[string]any)["seconds_balance"] != float64(321) {
		t.Fatal("remaining seconds not returned")
	}
	// A different zone costs twice as much: 321 Standard seconds become
	// 160 VIP seconds, with the half-second value retained in the club balance.
	var vipZone, vipPC string
	if err = pool.QueryRow(ctx, `INSERT INTO zones(club_id,name,hourly_price_tiyin) SELECT $1,'VIP conversion',reference_price_tiyin*2 FROM player_club_balances WHERE player_id=$2 AND club_id=$1 RETURNING id`, club, me["id"]).Scan(&vipZone); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO pc_refs(club_id,zone_id,external_pc_id,number,label) SELECT $1,$2,'vip-conversion',MAX(number)+1,'VIP conversion PC' FROM pc_refs WHERE club_id=$1 RETURNING id`, club, vipZone).Scan(&vipPC); err != nil {
		t.Fatal(err)
	}
	qr = "pc_vip_conversion"
	if _, err = pool.Exec(ctx, `INSERT INTO qr_codes(club_id,pc_ref_id,type,public_token,status) VALUES($1,$2,'static_pc',$3,'active')`, club, vipPC, qr); err != nil {
		t.Fatal(err)
	}
	balances = expect(200, "GET", "/api/mobile/balances", access, "", nil)
	var foundVIP bool
	for _, value := range balances["balances"].([]any)[0].(map[string]any)["zones"].([]any) {
		z := value.(map[string]any)
		if z["id"] == vipZone {
			foundVIP = true
			if z["seconds_available"] != float64(160) {
				t.Fatal("wrong VIP preview", z)
			}
		}
	}
	if !foundVIP {
		t.Fatal("missing zone equivalents")
	}
	redeem := expect(200, "POST", "/api/player-balance/redeem", access, "redeem-test-key-001", map[string]any{"qr_token": qr})
	again := expect(200, "POST", "/api/player-balance/redeem", access, "redeem-test-key-001", map[string]any{"qr_token": qr})
	if redeem["grant_id"] != again["grant_id"] || redeem["seconds_used"] != float64(160) {
		t.Fatal("duplicate balance redemption")
	}
	balanceStatus := expect(200, "GET", "/api/mobile/sessions/"+redeem["grant_id"].(string), access, "", nil)
	if balanceStatus["grant_status"] != "pending" {
		t.Fatal("mobile cloud balance grant must wait for Controller")
	}
	if !controller.startPendingEdgeGrants(ctx, club) {
		t.Fatal("controller did not start balance grant")
	}
	balanceStatus = expect(200, "GET", "/api/mobile/sessions/"+redeem["grant_id"].(string), access, "", nil)
	if balanceStatus["grant_status"] != "accepted" {
		t.Fatal("balance grant did not start")
	}

	if _, err = s.finishGrant(ctx, redeem["grant_id"].(string), "player_end", 160); err != nil {
		t.Fatal(err)
	}
	balances = expect(200, "GET", "/api/mobile/balances", access, "", nil)
	if balances["balances"].([]any)[0].(map[string]any)["seconds_balance"] != float64(321) {
		t.Fatal("VIP round trip lost value")
	}

	expect(204, "POST", "/api/mobile/auth/logout", "", "", map[string]any{"refresh_token": tokens["refresh_token"], "device_id": device})
	expect(401, "GET", "/api/mobile/me", access, "", nil)
	// OTP lockout and expiration.
	_, _ = pool.Exec(ctx, `DELETE FROM mobile_rate_limits`)
	ch = challenge("+998903333333")
	otp = issueOTP(ch, 99, "+998903333333")
	wrong := "000000"
	if otp == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		expect(401, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": device, "otp": wrong})
	}
	expect(401, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": device, "otp": otp})
	ch = challenge("+998904444444")
	otp = issueOTP(ch, 100, "+998904444444")
	_, _ = pool.Exec(ctx, `UPDATE mobile_auth_challenges SET otp_expires_at=$2 WHERE token_hash=$1`, hashToken(ch), time.Now().Add(-time.Second))
	expect(401, "POST", "/api/mobile/auth/verify", "", "", map[string]any{"challenge": ch, "device_id": device, "otp": otp})
	_, _ = pool.Exec(ctx, `DELETE FROM mobile_rate_limits`)
	for i := 0; i < 3; i++ {
		challenge("+998905555555")
	}
	expect(429, "POST", "/api/mobile/auth/challenge", "", "", map[string]any{"phone": "+998905555555", "device_id": device})
	var count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action LIKE 'mobile.%'`).Scan(&count)
	if count < 10 {
		t.Fatal("missing audit trail")
	}
}
