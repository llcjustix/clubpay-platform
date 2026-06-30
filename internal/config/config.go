package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv          string
	NodeMode        string
	HTTPAddr        string
	DatabaseURL     string
	PublicBaseURL   string
	FrontendBaseURL string
	AdminAPIToken   string
	CloudBaseURL    string

	DefaultPaymentProvider string
	MockPaymentsEnabled    bool
	ClickCheckoutURL       string
	ClickMerchantID        string
	ClickServiceID         string
	ClickMerchantUserID    string
	ClickSecretKey         string
	PaymeCheckoutURL       string
	PaymeMerchantID        string
	PaymeSecretKey         string
	PlatformFeeBPS         int

	CoreMode      string
	CoreBaseURL   string
	CoreToken     string
	CoreTimeoutMS int

	EdgeNodeID              string
	EdgeClubID              string
	EdgeSyncToken           string
	EdgeSyncIntervalSeconds int
	TelegramBotToken        string
	TelegramBotUsername     string
	TelegramWebhookSecret   string
	TelegramPollingEnabled  bool

	VoucherMinMinutes int
	VoucherTTLDays    int
}

func Load() (Config, error) {
	appEnv := env("APP_ENV", "development")
	paymeCheckoutURL := "https://checkout.paycom.uz"
	if !strings.EqualFold(appEnv, "production") {
		paymeCheckoutURL = "https://test.paycom.uz"
	}
	cfg := Config{
		AppEnv:                  appEnv,
		NodeMode:                strings.ToLower(env("NODE_MODE", "cloud")),
		HTTPAddr:                env("HTTP_ADDR", ":8080"),
		DatabaseURL:             env("DATABASE_URL", "postgres://clubpay:clubpay@localhost:5432/clubpay?sslmode=disable"),
		PublicBaseURL:           strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		FrontendBaseURL:         strings.TrimRight(env("FRONTEND_BASE_URL", "http://localhost:5173"), "/"),
		AdminAPIToken:           env("ADMIN_API_TOKEN", ""),
		CloudBaseURL:            strings.TrimRight(env("CLOUD_BASE_URL", ""), "/"),
		DefaultPaymentProvider:  strings.ToLower(env("DEFAULT_PAYMENT_PROVIDER", "mock")),
		MockPaymentsEnabled:     envBool("MOCK_PAYMENTS_ENABLED", !strings.EqualFold(appEnv, "production")),
		ClickCheckoutURL:        strings.TrimRight(env("CLICK_CHECKOUT_URL", "https://my.click.uz/services/pay"), "/"),
		ClickMerchantID:         env("CLICK_MERCHANT_ID", ""),
		ClickServiceID:          env("CLICK_SERVICE_ID", ""),
		ClickMerchantUserID:     env("CLICK_MERCHANT_USER_ID", ""),
		ClickSecretKey:          env("CLICK_SECRET_KEY", ""),
		PaymeCheckoutURL:        strings.TrimRight(env("PAYME_CHECKOUT_URL", paymeCheckoutURL), "/"),
		PaymeMerchantID:         env("PAYME_MERCHANT_ID", ""),
		PaymeSecretKey:          env("PAYME_SECRET_KEY", ""),
		PlatformFeeBPS:          envInt("PLATFORM_FEE_BPS", 0),
		CoreMode:                env("CORE_MODE", "mock"),
		CoreBaseURL:             strings.TrimRight(env("CORE_BASE_URL", "http://controller.local:8081"), "/"),
		CoreToken:               env("CORE_TOKEN", ""),
		CoreTimeoutMS:           envInt("CORE_TIMEOUT_MS", 10000),
		EdgeNodeID:              env("EDGE_NODE_ID", ""),
		EdgeClubID:              env("EDGE_CLUB_ID", ""),
		EdgeSyncToken:           env("EDGE_SYNC_TOKEN", ""),
		EdgeSyncIntervalSeconds: envInt("EDGE_SYNC_INTERVAL_SECONDS", 15),
		TelegramBotToken:        env("TELEGRAM_BOT_TOKEN", ""),
		TelegramBotUsername:     strings.TrimPrefix(strings.TrimSpace(env("TELEGRAM_BOT_USERNAME", "")), "@"),
		TelegramWebhookSecret:   env("TELEGRAM_WEBHOOK_SECRET", ""),
		TelegramPollingEnabled:  envBool("TELEGRAM_POLLING_ENABLED", !strings.EqualFold(appEnv, "production")),
		VoucherMinMinutes:       envInt("VOUCHER_MIN_MINUTES", 5),
		VoucherTTLDays:          envInt("VOUCHER_TTL_DAYS", 30),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
