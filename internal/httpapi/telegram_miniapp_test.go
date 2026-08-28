package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func signedMiniAppInitData(t *testing.T, botToken string, values url.Values) string {
	t.Helper()
	parts := make([]string, 0, len(values))
	for key, items := range values {
		for _, item := range items {
			parts = append(parts, key+"="+item)
		}
	}
	sort.Strings(parts)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	check := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = check.Write([]byte(strings.Join(parts, "\n")))
	values.Set("hash", hex.EncodeToString(check.Sum(nil)))
	return values.Encode()
}

func TestValidateTelegramMiniAppInitData(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	botToken := "123456:telegram-bot-token"
	raw := signedMiniAppInitData(t, botToken, url.Values{
		"auth_date":   {"1788004800"},
		"query_id":    {"AAH1test"},
		"start_param": {"pc_opaque_qr_token"},
		"user":        {`{"id":123456789,"first_name":"Aleksey","username":"aleksey"}`},
	})

	identity, err := validateTelegramMiniAppInitData(raw, botToken, now)
	if err != nil {
		t.Fatalf("validateTelegramMiniAppInitData returned error: %v", err)
	}
	if identity.User.ID != 123456789 || identity.StartParam != "pc_opaque_qr_token" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestValidateTelegramMiniAppInitDataRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	botToken := "123456:telegram-bot-token"
	values := url.Values{
		"auth_date":   {"1788004800"},
		"start_param": {"pc_opaque_qr_token"},
		"user":        {`{"id":123456789,"first_name":"Aleksey"}`},
	}
	raw := signedMiniAppInitData(t, botToken, values)
	if _, err := validateTelegramMiniAppInitData(strings.Replace(raw, "pc_opaque_qr_token", "pc_other", 1), botToken, now); err == nil {
		t.Fatal("tampered Mini App data was accepted")
	}
	if _, err := validateTelegramMiniAppInitData(raw, botToken, now.Add(telegramInitDataTTL+time.Second)); err == nil {
		t.Fatal("expired Mini App data was accepted")
	}
}

func TestTelegramProfileBalanceText(t *testing.T) {
	if got, want := telegramProfileBalanceText("Test Cyber Club", 4*3600+11*60+6), "Сеанс завершён. На вашем балансе Clubpay в клубе «Test Cyber Club»: 04:11:06."; got != want {
		t.Fatalf("telegramProfileBalanceText() = %q, want %q", got, want)
	}
	if got, want := telegramProfileBalanceText("", 0), "Сеанс завершён. На вашем балансе Clubpay: 0 сек."; got != want {
		t.Fatalf("telegramProfileBalanceText() without club = %q, want %q", got, want)
	}
}
