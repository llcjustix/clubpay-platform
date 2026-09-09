//go:build mobiledev

package httpapi

// Interactive browser harness, deliberately a tagged _test.go file: no OTP
// shortcut or fake Telegram transport can be included in the production API.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mobileSandboxDatabase(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") &&
		(u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") &&
		u.Path == "/clubpay_mobile_test" &&
		len(u.Query()) == 1 && u.Query().Get("sslmode") == "disable"
}

func TestMobileSandboxGuard(t *testing.T) {
	for _, raw := range []string{
		"postgres://user@remote/clubpay_mobile_test?sslmode=disable",
		"postgres://user@localhost/production?sslmode=disable",
		"postgres://user@localhost/clubpay_mobile_test?sslmode=disable&host=remote",
	} {
		if mobileSandboxDatabase(raw) {
			t.Fatal("unsafe sandbox database accepted")
		}
	}
	if !mobileSandboxDatabase("postgres://user@127.0.0.1:55439/clubpay_mobile_test?sslmode=disable") {
		t.Fatal("local disposable database rejected")
	}
}

func TestMobileBrowserSandbox(t *testing.T) {
	if os.Getenv("CLUBPAY_BROWSER_SANDBOX") != "1" {
		t.Skip("interactive harness: set CLUBPAY_BROWSER_SANDBOX=1 explicitly")
	}
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if !mobileSandboxDatabase(raw) {
		t.Fatal("sandbox requires loopback PostgreSQL database named clubpay_mobile_test with only sslmode=disable")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal("connect sandbox database:", err)
	}
	defer admin.Close()
	const schema = "mobile_browser_sandbox"
	if _, err := admin.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		t.Fatal(err)
	}
	pcfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid sandbox database config")
	}
	pcfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	s := NewServer(config.Config{
		AppEnv: "development", NodeMode: "cloud",
		TelegramBotToken: mobileSecret("sandbox_"), TelegramBotUsername: "local_test_only",
		TelegramWebhookSecret: mobileSecret("unused_"),
		PublicBaseURL:         "http://localhost:8088", MobileReturnBaseURL: "http://localhost:7357",
		FrontendBaseURL: "http://localhost:7357", SessionGraceSeconds: 180,
	}, pool, core.NewMockAdapter())
	// No polling/webhook registration. Reject all real outbound HTTP calls.
	var mu sync.Mutex
	var delivered string
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: mobileTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.telegram.org" || !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			return nil, fmt.Errorf("sandbox outbound request blocked")
		}
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		delivered = body.Text
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	defer func() { http.DefaultClient = previous }()
	routes := s.Routes()
	catalog := mobileSandboxCatalog(routes, &http.Client{
		Transport:     http.DefaultTransport,
		Timeout:       12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Loopback binding plus Host/Origin checks prevent LAN access, DNS rebinding,
		// and an unrelated website triggering development sign-ins.
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		origin := r.Header.Get("Origin")
		if net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() ||
			(r.Host != "localhost:8088" && r.Host != "127.0.0.1:8088") ||
			(origin != "" && origin != "http://localhost:7357") {
			http.Error(w, "local sandbox only", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/mobile/") && !strings.HasPrefix(r.URL.Path, "/api/qr/") && r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/qr/") {
			catalog.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/api/mobile/auth/challenge" || r.Method != http.MethodPost {
			routes.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			writeError(w, 400, "invalid_request")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, r)
		for key, values := range recorder.Header() {
			w.Header()[key] = values
		}
		if recorder.Code != http.StatusCreated {
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		var req struct {
			Phone string `json:"phone"`
		}
		var response map[string]any
		_ = json.Unmarshal(rawBody, &req)
		_ = json.Unmarshal(recorder.Body.Bytes(), &response)
		token, _ := response["challenge"].(string)
		chat, err := strconv.ParseInt(strings.TrimPrefix(req.Phone, "+"), 10, 64)
		if err != nil {
			mobileInternal(w)
			return
		}
		message := telegramMessage{Chat: telegramChat{ID: chat}, From: telegramUser{ID: chat}}
		if _, err := s.processMobileTelegram(r.Context(), message, token); err != nil {
			mobileInternal(w)
			return
		}
		message.Contact = telegramContact{PhoneNumber: req.Phone, UserID: chat}
		delivered = ""
		if _, err := s.processMobileTelegram(r.Context(), message, ""); err != nil {
			mobileInternal(w)
			return
		}
		code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(delivered)
		delivered = ""
		if code == "" {
			mobileInternal(w)
			return
		}
		response["development_otp"] = code
		response["telegram_link"] = ""
		// Contact is simulated immediately, so expose the actual OTP deadline.
		var expires time.Time
		if err := pool.QueryRow(r.Context(), `SELECT otp_expires_at FROM mobile_auth_challenges WHERE token_hash=$1`, hashToken(token)).Scan(&expires); err != nil {
			mobileInternal(w)
			return
		}
		response["expires_at"] = expires
		writeJSON(w, http.StatusCreated, response)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:8088")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	defer server.Close()
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	t.Log("LOCAL OTP SANDBOX ready at http://localhost:8088; Telegram simulated, separate schema, no real payments")
	select {
	case <-ctx.Done():
	case err := <-errs:
		if err != http.ErrServerClosed {
			t.Fatal(err)
		}
	}
}

// Only the development harness uses this fallback. The destination is fixed,
// never taken from the scanned URL; no cookies, mobile tokens or mutations are
// forwarded. Live catalog data is explicitly marked as a read-only preview.
func mobileSandboxCatalog(local http.Handler, client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/api/qr/")
		if r.Method != http.MethodGet || !regexp.MustCompile(`^[A-Za-z0-9_-]{4,256}$`).MatchString(token) {
			writeError(w, 400, "invalid_qr")
			return
		}
		localResponse := httptest.NewRecorder()
		local.ServeHTTP(localResponse, r)
		for key, values := range localResponse.Header() {
			w.Header()[key] = values
		}
		w.Header().Set("Cache-Control", "no-store")
		if localResponse.Code != http.StatusNotFound {
			w.WriteHeader(localResponse.Code)
			_, _ = w.Write(localResponse.Body.Bytes())
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
			"https://api-clubpay.justix.uz/api/qr/"+url.PathEscape(token), nil)
		if err != nil {
			writeError(w, 503, "qr_catalog_unavailable")
			return
		}
		response, err := client.Do(req)
		if err != nil {
			writeError(w, 503, "qr_catalog_unavailable")
			return
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
			writeError(w, 404, "qr_not_found")
			return
		}
		if response.StatusCode != http.StatusOK {
			writeError(w, 503, "qr_catalog_unavailable")
			return
		}
		// Decode only the fields that the read-only mobile PC card needs.
		var result struct {
			Club struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"club"`
			PC struct {
				Label  string `json:"label"`
				Status string `json:"status"`
			} `json:"pc"`
			Zone struct {
				Name        string `json:"name"`
				HourlyPrice int    `json:"hourly_price_uzs"`
			} `json:"zone"`
			QRType  string `json:"qr_type"`
			Tariffs []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Minutes int    `json:"duration_minutes"`
				Price   int    `json:"price_uzs"`
			} `json:"tariffs"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&result); err != nil || result.Club.ID == "" || result.PC.Label == "" || result.Zone.Name == "" || (result.QRType != "static_pc" && result.QRType != "session_extend") {
			writeError(w, 503, "qr_catalog_unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{
			"club": result.Club, "pc": result.PC, "zone": result.Zone, "qr_type": result.QRType,
			"tariffs": result.Tariffs, "payment_providers": []any{}, "development_catalog_preview": true,
		})
	})
}

func TestMobileSandboxCatalog(t *testing.T) {
	for _, status := range []int{200, 404, 502} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			local := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "QR token not found") })
			client := &http.Client{Transport: mobileTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.URL.String() != "https://api-clubpay.justix.uz/api/qr/pc_live" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Fatal("catalog request destination/credentials violated isolation")
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"club":{"id":"live","name":"Club","secret":"hidden"},"pc":{"label":"PC 1","status":"available","credential":"hidden"},"zone":{"name":"Standard","hourly_price_uzs":10000},"qr_type":"static_pc","payment_providers":[{"provider":"click","configured":true}]}`)), Header: make(http.Header)}, nil
			})}
			r := httptest.NewRequest("GET", "/api/qr/pc_live", nil)
			r.Header.Set("Authorization", "Bearer mob_a_local")
			r.Header.Set("Cookie", "session=local")
			w := httptest.NewRecorder()
			mobileSandboxCatalog(local, client).ServeHTTP(w, r)
			want := status
			if status == 502 {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("got %d want %d", w.Code, want)
			}
			if status == 200 && (!strings.Contains(w.Body.String(), `"development_catalog_preview":true`) || strings.Contains(w.Body.String(), "hidden") || strings.Contains(w.Body.String(), "click")) {
				t.Fatal("preview leaked unexpected fields/actions")
			}
			if status == 502 && !strings.Contains(w.Body.String(), "qr_catalog_unavailable") {
				t.Fatal("outage must not be reported as expired QR")
			}
		})
	}
	local := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"local": true}) })
	client := &http.Client{Transport: mobileTestTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("local QR must not reach production")
		return nil, fmt.Errorf("unexpected")
	})}
	w := httptest.NewRecorder()
	mobileSandboxCatalog(local, client).ServeHTTP(w, httptest.NewRequest("GET", "/api/qr/pc_local", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"local":true`) {
		t.Fatal("local catalog changed")
	}
}
